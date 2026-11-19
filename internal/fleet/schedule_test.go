package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestChicagoWindowDSTAndMinimumNotice(t *testing.T) {
	for _, tc := range []struct{ earliest, want string }{
		{"2026-01-15T07:00:00Z", "2026-01-15T08:00:00Z"},
		{"2026-07-15T06:00:00Z", "2026-07-15T07:00:00Z"},
		{"2026-07-15T07:00:01Z", "2026-07-16T07:00:00Z"},
		{"2026-03-08T06:00:00Z", "2026-03-08T08:00:00Z"},
		{"2026-11-01T05:00:00Z", "2026-11-01T08:00:00Z"},
	} {
		early, _ := time.Parse(time.RFC3339, tc.earliest)
		if got := nextWindow(early).UTC().Format(time.RFC3339); got != tc.want {
			t.Fatalf("%s: got %s want %s", tc.earliest, got, tc.want)
		}
	}
}

func pending(t *testing.T) (*Service, *memoryStore, *Window) {
	t.Helper()
	s, store := fixture(t)
	ready(t, s, "8.0.29")
	publishRelease(t, s, "8.0.30")
	plan, errPlan := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if errPlan != nil || plan.Job != nil || plan.Status != "scheduled" {
		t.Fatalf("not gated: %+v %v", plan, errPlan)
	}
	st, _ := s.Status(context.Background())
	if st.Rollout.Window.WindowStart.Sub(store.now) < noticeLead {
		t.Fatal("insufficient advance notice")
	}
	return s, store, st.Rollout.Window
}

func TestExactAnnouncedTargetStaysPinnedAndNewLatestNeedsOwnNotice(t *testing.T) {
	s, store, w := pending(t)
	phases := []Announcement{}
	s.Announce = func(_ context.Context, _ string, n Notice) error { phases = append(phases, n.Announcement); return nil }
	advanceWindow(t, s, w)
	if errChange := s.change(context.Background(), func(st *State, _ time.Time) error { r := candidate("8.0.31"); st.Latest = &r; return nil }); errChange != nil {
		t.Fatal(errChange)
	}
	a, errPlan := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if errPlan != nil || a.Job == nil || a.Job.Release.Version != "8.0.30" {
		t.Fatalf("announced target drifted: %+v %v", a, errPlan)
	}
	report(t, s, a.Job, "success")
	b, _ := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	report(t, s, b.Job, "success")
	next, errNext := s.Plan(context.Background(), testIDs[0], Inventory{Version: "8.0.30", SHA256: candidate("8.0.30").SHA256, Size: 101, Platform: "darwin/arm64"})
	if errNext != nil || next.Job != nil {
		t.Fatalf("new latest silently auto-started: %+v %v", next, errNext)
	}
	st, _ := s.Status(context.Background())
	if st.Rollout.Release.Version != "8.0.31" || !st.Rollout.Window.WindowStart.After(store.now.Add(noticeLead)) || !st.Rollout.Window.AnnouncedAt.IsZero() {
		t.Fatal("new release did not get its own pending window")
	}
	want := []string{"scheduled", "reminder", "started", "done"}
	if len(phases) != len(want) {
		t.Fatalf("announcements: %+v", phases)
	}
	for i, phase := range want {
		if phases[i].Phase != phase || phases[i].TargetVersion != "8.0.30" || len(phases[i].Nodes) != 2 {
			t.Fatalf("bad announcement: %+v", phases[i])
		}
	}
}

func TestFailedAndMissingHookNeverDispatchAndReannounceNextNight(t *testing.T) {
	for _, phase := range []string{"scheduled", "reminder", "started"} {
		t.Run(phase, func(t *testing.T) {
			s, store, w := pending(t)
			s.Announce = func(_ context.Context, _ string, n Notice) error {
				if n.Phase == phase {
					return fmt.Errorf("fixture hook unavailable")
				}
				return nil
			}
			if phase != "scheduled" {
				deliverAll(t, s)
				store.now = w.WindowStart.Add(-noticeLead)
			}
			if phase == "started" {
				deliverAll(t, s)
				store.now = w.WindowStart
				refreshSeen(t, s, w.Nodes)
			}
			if errDeliver := s.deliverNotice(context.Background()); errDeliver != nil {
				t.Fatal(errDeliver)
			}
			st, _ := s.Status(context.Background())
			if st.NoticeError != "announcement_delivery_failed" || !st.Rollout.Window.WindowStart.After(w.WindowStart) {
				t.Fatalf("failure not postponed: %+v", st)
			}
			plan, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
			if plan.Job != nil {
				t.Fatal("hook failure dispatched a node")
			}
			s.Announce = func(context.Context, string, Notice) error { return nil }
			store.now = st.NoticeRetryAt
			deliverAll(t, s)
			st, _ = s.Status(context.Background())
			if st.Rollout.Window.AnnouncedAt.IsZero() {
				t.Fatal("next window not re-announced")
			}
		})
	}
	s, _, _ := pending(t)
	if errChange := s.change(context.Background(), func(st *State, _ time.Time) error { st.Policy.AnnouncementURL = ""; return nil }); errChange != nil {
		t.Fatal(errChange)
	}
	if errDeliver := s.deliverNotice(context.Background()); errDeliver != nil {
		t.Fatal(errDeliver)
	}
	st, _ := s.Status(context.Background())
	if st.NoticeError != "announcement_hook_not_configured" || !st.Rollout.Window.AnnouncedAt.IsZero() {
		t.Fatal("legacy policy without hook allowed upgrades")
	}
}

func refreshSeen(t *testing.T, s *Service, ids []string) {
	t.Helper()
	if errChange := s.change(context.Background(), func(st *State, now time.Time) error {
		for _, id := range ids {
			n := st.Nodes[id]
			n.SeenAt = now
			st.Nodes[id] = n
		}
		return nil
	}); errChange != nil {
		t.Fatal(errChange)
	}
}

func TestLateNoticeOfflineAndExpiredWindowsCannotUpgradeOnWake(t *testing.T) {
	for _, scenario := range []string{"late_notice", "offline_required", "expired"} {
		t.Run(scenario, func(t *testing.T) {
			s, store, w := pending(t)
			if scenario != "late_notice" {
				deliverAll(t, s)
			}
			switch scenario {
			case "late_notice":
				store.now = w.WindowStart.Add(-30 * time.Minute)
			case "offline_required":
				store.now = w.WindowStart
			case "expired":
				store.now = w.WindowStart.Add(8 * time.Hour)
			}
			if errDeliver := s.deliverNotice(context.Background()); errDeliver != nil {
				t.Fatal(errDeliver)
			}
			plan, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
			st, _ := s.Status(context.Background())
			if plan.Job != nil || !st.Rollout.Window.WindowStart.After(w.WindowStart) {
				t.Fatalf("missed window was not rescheduled: %+v", plan)
			}
		})
	}
}

func TestPostponeOncePerTargetSurvivesServiceRestartAndForce(t *testing.T) {
	s, store, w := pending(t)
	ctx := context.Background()
	if errPostpone := s.Postpone(ctx, "8.0.31", "", false); errPostpone == nil {
		t.Fatal("wrong target postponed")
	}
	if errPostpone := s.Postpone(ctx, "8.0.30", "", false); errPostpone != nil {
		t.Fatal(errPostpone)
	}
	st, _ := s.Status(ctx)
	if !st.Rollout.Window.WindowStart.Equal(w.WindowStart.Add(24 * time.Hour)) {
		t.Fatal("not shifted by 24 hours")
	}
	restarted := New(store)
	if errPostpone := restarted.Postpone(ctx, "8.0.30", "", false); errPostpone == nil {
		t.Fatal("second orchestrator postponement accepted")
	}
	if errPostpone := restarted.Postpone(ctx, "8.0.30", "", true); errPostpone != nil {
		t.Fatal(errPostpone)
	}
	st, _ = s.Status(ctx)
	advanceWindow(t, s, st.Rollout.Window)
	if errPostpone := s.Postpone(ctx, "8.0.30", "", true); errPostpone == nil {
		t.Fatal("started window postponed")
	}
}

func TestOptionalSleepingThroughOwnWindowGetsFreshAnnouncement(t *testing.T) {
	s, store := fixture(t)
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	a, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	report(t, s, a.Job, "success")
	b, _ := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	report(t, s, b.Job, "success")
	wake, _ := s.Plan(context.Background(), testIDs[2], inv("8.0.29"))
	if wake.Job != nil {
		t.Fatal("laptop upgraded on wake")
	}
	deliverAll(t, s)
	st, _ := s.Status(context.Background())
	w := st.Rollout.OptionalWindows[testIDs[2]]
	if len(w.Nodes) != 1 || w.Nodes[0] != testIDs[2] {
		t.Fatal("optional announcement not scoped to laptop")
	}
	store.now = w.WindowStart.Add(8 * time.Hour)
	wake, _ = s.Plan(context.Background(), testIDs[2], inv("8.0.29"))
	st, _ = s.Status(context.Background())
	if wake.Job != nil || !st.Rollout.OptionalWindows[testIDs[2]].WindowStart.After(store.now.Add(noticeLead)) {
		t.Fatal("late optional wake bypassed window")
	}
}

func TestRollbackCompletionAnnouncementAndNoUnscheduledLegacyRollout(t *testing.T) {
	s, _ := fixture(t)
	phases := []string{}
	s.Announce = func(_ context.Context, _ string, n Notice) error { phases = append(phases, n.Phase); return nil }
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	a, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	report(t, s, a.Job, "failed")
	rb, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	report(t, s, rb.Job, "success")
	if phases[len(phases)-1] != "rolled_back" {
		t.Fatalf("rollback not announced: %v", phases)
	}
	if errChange := s.change(context.Background(), func(st *State, _ time.Time) error {
		st.Rollout = &Rollout{Release: candidate("8.0.30"), Phase: "upgrade", Completed: map[string]Inventory{}}
		return nil
	}); errChange != nil {
		t.Fatal(errChange)
	}
	plan, errPlan := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if errPlan != nil || plan.Job != nil || plan.Status != "scheduled" {
		t.Fatalf("legacy unannounced rollout started: %+v %v", plan, errPlan)
	}
}

func TestCatalogContinuesWhileWaitingAndPolicyCannotChangeAnnouncedNodes(t *testing.T) {
	s, _, w := pending(t)
	st, _ := s.Status(context.Background())
	st.Policy.Nodes[1].Required = false
	if errPolicy := s.SetPolicy(context.Background(), st.Policy); errPolicy == nil {
		t.Fatal("announced membership silently changed")
	}
	st, _ = s.Status(context.Background())
	st.Policy.CatalogSync = true
	if errPolicy := s.SetPolicy(context.Background(), st.Policy); errPolicy != nil {
		t.Fatal(errPolicy)
	}
	plan, errPlan := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if errPlan != nil || plan.Job == nil || plan.Job.Operation != "catalog" {
		t.Fatalf("pending patch blocked catalog: %+v %v", plan, errPlan)
	}
	report(t, s, plan.Job, "success")
	st, _ = s.Status(context.Background())
	st.Policy.AnnouncementURL = "https://different.example.invalid/hook"
	if errPolicy := s.SetPolicy(context.Background(), st.Policy); errPolicy != nil {
		t.Fatal(errPolicy)
	}
	st, _ = s.Status(context.Background())
	if st.Rollout.Window.ID == w.ID || !st.Rollout.Window.AnnouncedAt.IsZero() {
		t.Fatal("changed hook did not require a fresh announcement")
	}
}

func TestAnnouncementCASRetriesLeaseAndRestartDoNotDuplicateSideEffects(t *testing.T) {
	s, store, _ := pending(t)
	calls := 0
	s.Announce = func(context.Context, string, Notice) error { calls++; return nil }
	if errChange := s.change(context.Background(), func(st *State, now time.Time) error { st.NoticeLease = now.Add(time.Minute); return nil }); errChange != nil {
		t.Fatal(errChange)
	}
	if errDeliver := s.deliverNotice(context.Background()); errDeliver != nil {
		t.Fatal(errDeliver)
	}
	if calls != 0 {
		t.Fatal("announcement lease ignored")
	}
	store.now = store.now.Add(time.Minute)
	store.conflicts = 2
	if errDeliver := s.deliverNotice(context.Background()); errDeliver != nil {
		t.Fatal(errDeliver)
	}
	if calls != 1 {
		t.Fatalf("CAS retry sent %d requests", calls)
	}
	restarted := New(store)
	restarted.Announce = s.Announce
	if errDeliver := restarted.deliverNotice(context.Background()); errDeliver != nil {
		t.Fatal(errDeliver)
	}
	if calls != 1 {
		t.Fatal("acknowledged notice redelivered after restart")
	}
}

func TestFailedRollbackIsNotAnnouncedAsRolledBackAndReviewCannotDiscardPartialRollout(t *testing.T) {
	s, _ := fixture(t)
	phases := []string{}
	s.Announce = func(_ context.Context, _ string, n Notice) error { phases = append(phases, n.Phase); return nil }
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	a, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	report(t, s, a.Job, "success")
	if errReview := s.Review(context.Background(), "8.0.30", true); errReview == nil {
		t.Fatal("review discarded partially completed rollout")
	}
	b, _ := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	report(t, s, b.Job, "failed")
	rb, _ := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	report(t, s, rb.Job, "failed")
	ra, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.30"))
	report(t, s, ra.Job, "success")
	for _, phase := range phases {
		if phase == "rolled_back" {
			t.Fatal("failed rollback announced as successful")
		}
	}
	st, _ := s.Status(context.Background())
	if !st.Rollout.RollbackFailed || st.Rollout.Phase != "failed" {
		t.Fatal("rollback failure not latched")
	}
}

func TestAnnouncementWebhookPayloadAndFailurePrivacy(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" || r.Header.Get("Idempotency-Key") != "fake-notice" {
			t.Error("invalid hook request")
		}
		var payload map[string]any
		if errDecode := json.NewDecoder(r.Body).Decode(&payload); errDecode != nil || len(payload) != 4 || payload["phase"] != "scheduled" || payload["target_version"] != "8.0.30" {
			t.Errorf("bad payload: %+v %v", payload, errDecode)
		}
		w.Header().Set("Location", "/must-not-follow")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	notice := Notice{ID: "fake-notice", Announcement: Announcement{Phase: "scheduled", TargetVersion: "8.0.30", Nodes: testIDs[:2]}}
	if errPost := postAnnouncement(context.Background(), server.URL+"?fake-private-value=do-not-log", notice); errPost == nil || errPost.Error() != "announcement_http_302" {
		t.Fatalf("unsafe/unexpected delivery error: %v", errPost)
	}
	if calls != 1 {
		t.Fatal("webhook followed redirect")
	}
	for _, raw := range []string{"file:///tmp/no", "https://user:fake@example.invalid/hook", "https://example.invalid/hook#fragment", "https:///no-host"} {
		if validateAnnouncementURL(raw) == nil {
			t.Fatalf("invalid hook accepted: %s", raw)
		}
	}
}
