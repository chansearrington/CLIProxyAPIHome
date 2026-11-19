package fleet

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAnnouncementBearerTokenIsOptionalAndRedirectsAreNotFollowed(t *testing.T) {
	for _, token := range []string{"", "obviously-fake-announcement-test-token"} {
		t.Setenv("FLEET_ANNOUNCE_TOKEN", token)
		reached := false
		destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			expected := ""
			if token != "" {
				expected = "Bearer " + token
			}
			if r.Header.Get("Authorization") != expected {
				t.Error("wrong announcement authentication header")
			}
			if r.Header.Get("Idempotency-Key") != "fake-id" {
				t.Error("idempotency key lost")
			}
			http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
		}))
		errPost := postAnnouncement(context.Background(), server.URL, Notice{ID: "fake-id", Announcement: Announcement{Phase: "scheduled"}})
		server.Close()
		destination.Close()
		if errPost == nil || reached {
			t.Fatal("redirect followed or accepted")
		}
	}
}

func TestPermanentCriticalRejectionIsDiscardedAndAudited(t *testing.T) {
	for _, phase := range []string{"done", "rolled_back", "rollback", "rollback_failed"} {
		t.Run(phase, func(t *testing.T) {
			s, _ := fixture(t)
			if errChange := s.change(context.Background(), func(st *State, now time.Time) error {
				st.Rollout = &Rollout{Release: Release{Version: "8.0.30"}, Phase: "rollback", Rollback: testIDs[:1]}
				queueNotice(st, &Window{ID: "fake-window", WindowStart: now, Nodes: testIDs[:1]}, "8.0.30", phase, now)
				return nil
			}); errChange != nil {
				t.Fatal(errChange)
			}
			s.Announce = func(context.Context, string, Notice) error { return &announcementError{Status: 403} }
			if errDeliver := s.deliverNotice(context.Background()); errDeliver != nil {
				t.Fatal(errDeliver)
			}
			st, errStatus := s.Status(context.Background())
			if errStatus != nil {
				t.Fatal(errStatus)
			}
			if len(st.Notices) != 0 || !st.NoticeRetryAt.IsZero() {
				t.Fatal("permanent rejection still blocks outbox")
			}
			if st.Rollout.Phase != "rollback" || st.Rollout.RollbackFailed || st.Rollout.RollbackAnnounced || len(st.Rollout.Rollback) != 1 {
				t.Fatal("delivery rejection changed rollback outcome")
			}
			found := false
			for _, event := range st.History {
				if event.Operation == "announcement" && event.Status == "rejected_"+phase {
					found = true
				}
				if event.Operation == "rollback" || event.Status == "rollback_failed" {
					t.Fatal("delivery failure misreported as rollback failure")
				}
			}
			if !found {
				t.Fatal("rejection not audited")
			}
		})
	}
}

func TestRejectedRollbackDoesNotRegenerateAndKeepsEmergencyGate(t *testing.T) {
	s, store := fixture(t)
	ctx := context.Background()
	if errChange := s.change(ctx, func(st *State, now time.Time) error {
		st.Rollout = &Rollout{Release: candidate("8.0.30"), Phase: "rollback", Rollback: testIDs[:1], RollbackNodes: testIDs[:1], Completed: map[string]Inventory{testIDs[0]: inv("8.0.29")}}
		queueRollback(st, now)
		return nil
	}); errChange != nil {
		t.Fatal(errChange)
	}
	calls := 0
	s.Announce = func(context.Context, string, Notice) error {
		calls++
		return &announcementError{Status: 400}
	}
	for cycle := 0; cycle < 2; cycle++ {
		if errDeliver := s.deliverNotice(ctx); errDeliver != nil {
			t.Fatal(errDeliver)
		}
		plan, errPlan := s.Plan(ctx, testIDs[0], inv("8.0.30"))
		if errPlan != nil || plan.Job != nil || plan.Status != "rollback_announcement" {
			t.Fatalf("15-minute gate bypassed: %+v, %v", plan, errPlan)
		}
		st, errStatus := s.Status(ctx)
		if errStatus != nil {
			t.Fatal(errStatus)
		}
		if len(st.Notices) != 0 || !st.Rollout.RollbackNoticeRejected || st.Rollout.RollbackAnnounced || st.Rollout.RollbackFailed {
			t.Fatal("rejected rollback notice regenerated or outcome changed")
		}
		rejected := 0
		for _, event := range st.History {
			if event.Operation == "announcement" && event.Status == "rejected_rollback" {
				rejected++
			}
		}
		if rejected != 1 || calls != 1 {
			t.Fatalf("rejection repeated: %d audits, %d sends", rejected, calls)
		}
	}
	store.now = store.now.Add(15 * time.Minute)
	plan, errPlan := s.Plan(ctx, testIDs[0], inv("8.0.30"))
	if errPlan != nil || plan.Job == nil || plan.Job.Operation != "rollback" {
		t.Fatalf("emergency rollback missing: %+v, %v", plan, errPlan)
	}
	st, _ := s.Status(ctx)
	if !st.Rollout.RollbackUnannounced || st.Rollout.RollbackAnnounced || len(st.Notices) != 0 {
		t.Fatal("emergency rollback misreported or rejected notice regenerated")
	}

	// Another failed upgrade on the same rollout must not inherit the rejection.
	var job *Job
	if errChange := s.change(ctx, func(st *State, now time.Time) error {
		st.Rollout.Phase = "upgrade"
		job = newJob(testIDs[1], "upgrade", now, inv("8.0.29"))
		job.Release = &st.Rollout.Release
		st.Active = job
		return nil
	}); errChange != nil {
		t.Fatal(errChange)
	}
	if errReport := s.Report(ctx, testIDs[1], Result{ID: job.ID, Token: job.Token, Status: "failed", Inventory: inv("8.0.29")}); errReport != nil {
		t.Fatal(errReport)
	}
	st, _ = s.Status(ctx)
	if st.Rollout.RollbackNoticeRejected || len(st.Notices) != 1 || st.Notices[0].Phase != "rollback" || len(st.Notices[0].Nodes) != 2 {
		t.Fatal("new rollback inherited old rejection")
	}
}

func TestPermanentOrdinaryRejectionLeavesOtherWindowsUnchanged(t *testing.T) {
	for _, phase := range []string{"scheduled", "reminder", "started", "postponed"} {
		t.Run(phase, func(t *testing.T) {
			s, store := fixture(t)
			start := nextWindow(store.now.Add(48 * time.Hour))
			if errChange := s.change(context.Background(), func(st *State, now time.Time) error {
				st.Rollout = &Rollout{Release: Release{Version: "8.0.30"}, Window: &Window{ID: "current", WindowStart: start, Nodes: testIDs[:1], AnnouncedAt: now}, OptionalWindows: map[string]*Window{"other": {ID: "other", WindowStart: start, Nodes: testIDs[1:2], AnnouncedAt: now, Optional: true}}}
				queueNotice(st, st.Rollout.Window, "8.0.30", phase, now)
				return nil
			}); errChange != nil {
				t.Fatal(errChange)
			}
			s.Announce = func(context.Context, string, Notice) error { return &announcementError{Status: 400} }
			if errDeliver := s.deliverNotice(context.Background()); errDeliver != nil {
				t.Fatal(errDeliver)
			}
			st, _ := s.Status(context.Background())
			other := st.Rollout.OptionalWindows["other"]
			if other.ID != "other" || !other.WindowStart.Equal(start) || other.AnnouncedAt.IsZero() {
				t.Fatal("unrelated window changed")
			}
			if phase == "postponed" && st.Rollout.Window.ID != "current" {
				t.Fatal("receipt rejection changed current window")
			}
			if phase != "postponed" && st.Rollout.Window.ID == "current" {
				t.Fatal("failed gate did not postpone its own window")
			}
		})
	}
}
