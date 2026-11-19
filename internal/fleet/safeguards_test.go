package fleet

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestPartialRolloutCanBeHeldOrForceClosedWithoutLosingReceipts(t *testing.T) {
	for _, mode := range []string{"hold", "force"} {
		t.Run(mode, func(t *testing.T) {
			s, _ := fixture(t)
			ready(t, s, "8.0.29")
			setRelease(t, s, "8.0.30")
			first, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
			report(t, s, first.Job, "success")
			st, _ := s.Status(context.Background())
			policy := st.Policy
			policy.Nodes = policy.Nodes[:1]
			if mode == "hold" {
				if errHold := s.Review(context.Background(), "8.0.30", false); errHold != nil {
					t.Fatal(errHold)
				}
				if errPolicy := s.SetPolicy(context.Background(), policy); errPolicy != nil {
					t.Fatal(errPolicy)
				}
			} else if errPolicy := s.SetPolicy(context.Background(), policy, PolicyChange{Force: true, Caller: "192.0.2.8:1234"}); errPolicy != nil {
				t.Fatal(errPolicy)
			}
			st, _ = s.Status(context.Background())
			if st.Rollout.Phase != "held" || st.Rollout.Completed[testIDs[0]].Version != "8.0.29" || len(st.Policy.Nodes) != 1 {
				t.Fatal("operator locked out or rollback receipt discarded")
			}
			if mode == "force" {
				event := st.History[len(st.History)-1]
				if !event.Force || event.Caller != "192.0.2.8:1234" {
					t.Fatal("force not audited")
				}
			}
		})
	}
}

func TestOptionalFailureOnlyRollsBackOptionalAndAnnouncesFailedRollback(t *testing.T) {
	s, _ := fixture(t)
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	first, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	report(t, s, first.Job, "success")
	second, _ := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	report(t, s, second.Job, "success")
	_, _ = s.Plan(context.Background(), testIDs[2], inv("8.0.29"))
	st, _ := s.Status(context.Background())
	advanceWindow(t, s, st.Rollout.OptionalWindows[testIDs[2]])
	optional, _ := s.Plan(context.Background(), testIDs[2], inv("8.0.29"))
	announcements := []Announcement{}
	s.Announce = func(_ context.Context, _ string, n Notice) error {
		announcements = append(announcements, n.Announcement)
		return nil
	}
	if errReport := s.Report(context.Background(), testIDs[2], Result{ID: optional.Job.ID, Token: optional.Job.Token, Status: "failed", Error: "fake_failure"}); errReport != nil {
		t.Fatal(errReport)
	}
	waiting, _ := s.Plan(context.Background(), testIDs[2], inv("8.0.29"))
	if waiting.Job != nil || waiting.Status != "rollback_announcement" {
		t.Fatal("rollback dispatched before announcement")
	}
	deliverAll(t, s)
	st, _ = s.Status(context.Background())
	if len(st.Rollout.Rollback) != 1 || st.Rollout.Rollback[0] != testIDs[2] {
		t.Fatal("optional failure rolled back healthy required nodes")
	}
	rb, _ := s.Plan(context.Background(), testIDs[2], inv("8.0.29"))
	report(t, s, rb.Job, "failed")
	st, _ = s.Status(context.Background())
	for _, id := range testIDs[:2] {
		if st.Nodes[id].Version != "8.0.30" {
			t.Fatal("healthy required node changed")
		}
	}
	before, failed := false, false
	for _, n := range announcements {
		if n.Phase == "rollback" {
			before = true
			if len(n.Nodes) != 1 || n.Nodes[0] != testIDs[2] {
				t.Fatal("bad rollback announcement scope")
			}
		}
		if n.Phase == "rollback_failed" {
			failed = true
			if len(n.FailedNodes) != 1 || n.FailedNodes[0] != testIDs[2] {
				t.Fatal("failed rollback missing failed nodes")
			}
		}
	}
	if !before || !failed {
		t.Fatalf("rollback notices missing: %+v", announcements)
	}
}

func TestQueueBoundsExpiryAndPermanentRejectionDoNotStickOrDrift(t *testing.T) {
	s, store, w := pending(t)
	s.Announce = func(context.Context, string, Notice) error { return &announcementError{Status: 400} }
	if errDeliver := s.deliverNotice(context.Background()); errDeliver != nil {
		t.Fatal(errDeliver)
	}
	st, _ := s.Status(context.Background())
	next := st.Rollout.Window.WindowStart
	if !next.After(w.WindowStart) || len(st.Notices) != 1 || st.Notices[0].Phase != "scheduled" {
		t.Fatal("unannounced window generated postponement spam")
	}
	store.now = store.now.Add(5 * time.Minute)
	if errDeliver := s.deliverNotice(context.Background()); errDeliver != nil {
		t.Fatal(errDeliver)
	}
	st, _ = s.Status(context.Background())
	if !st.Rollout.Window.WindowStart.Equal(next) {
		t.Fatal("persistent failure pushed window arbitrarily far into future")
	}
	store.now = store.now.Add(5 * time.Minute)
	s.Announce = func(context.Context, string, Notice) error { return nil }
	deliverAll(t, s)
	if errChange := s.change(context.Background(), func(st *State, now time.Time) error {
		st.Rollout = nil
		for i := 0; i < 50; i++ {
			queueNotice(st, &Window{ID: fmt.Sprint(i), WindowStart: now, Nodes: testIDs[:1]}, "8.0.30", "scheduled", now.Add(-25*time.Hour))
		}
		return nil
	}); errChange != nil {
		t.Fatal(errChange)
	}
	st, _ = s.Status(context.Background())
	if len(st.Notices) != maxNotices {
		t.Fatal("queue not capped")
	}
	if errDeliver := s.deliverNotice(context.Background()); errDeliver != nil {
		t.Fatal(errDeliver)
	}
	st, _ = s.Status(context.Background())
	if len(st.Notices) != 0 || st.NoticeError != "" {
		t.Fatal("expired receipts block fleet forever")
	}
}

func TestRejectedOldPostponementCannotUndoOperatorDelay(t *testing.T) {
	s, store, _ := pending(t)
	deliverAll(t, s)
	if errPostpone := s.Postpone(context.Background(), "8.0.30", "", false); errPostpone != nil {
		t.Fatal(errPostpone)
	}
	st, _ := s.Status(context.Background())
	delayed := st.Rollout.Window.WindowStart
	s.Announce = func(_ context.Context, _ string, n Notice) error {
		if n.Phase == "postponed" {
			return &announcementError{Status: 400}
		}
		return nil
	}
	if errDeliver := s.deliverNotice(context.Background()); errDeliver != nil {
		t.Fatal(errDeliver)
	}
	store.now = store.now.Add(5 * time.Minute)
	deliverAll(t, s)
	st, _ = s.Status(context.Background())
	if st.Rollout.Window.WindowStart.Before(delayed) || st.Rollout.Window.AnnouncedAt.IsZero() || len(st.Notices) != 0 {
		t.Fatal("obsolete 4xx notice undid postponement or blocked its reannouncement")
	}
}

func TestPostponeUsesLocalNightAcrossBothDSTTransitions(t *testing.T) {
	for _, tc := range []struct{ start, want string }{
		{"2026-10-31T07:00:00Z", "2026-11-01T08:00:00Z"},
		{"2026-03-08T08:00:00Z", "2026-03-09T07:00:00Z"},
	} {
		s, store, _ := pending(t)
		start, _ := time.Parse(time.RFC3339, tc.start)
		store.now = start.Add(-2 * time.Hour)
		if errChange := s.change(context.Background(), func(st *State, _ time.Time) error { st.Rollout.Window.WindowStart = start; return nil }); errChange != nil {
			t.Fatal(errChange)
		}
		if errPostpone := s.Postpone(context.Background(), "8.0.30", "", true, "192.0.2.9:1234"); errPostpone != nil {
			t.Fatal(errPostpone)
		}
		st, _ := s.Status(context.Background())
		if st.Rollout.Window.WindowStart.UTC().Format(time.RFC3339) != tc.want {
			t.Fatalf("wrong DST postponement: %s", st.Rollout.Window.WindowStart)
		}
		event := st.History[len(st.History)-1]
		if !event.Force || event.Caller != "192.0.2.9:1234" {
			t.Fatal("forced postponement not audited")
		}
	}
}

func TestUnstartedJobsAreVoidedAfterWindowButRunningJobsReconcile(t *testing.T) {
	for _, running := range []bool{false, true} {
		t.Run(fmt.Sprint(running), func(t *testing.T) {
			s, store := fixture(t)
			ready(t, s, "8.0.29")
			setRelease(t, s, "8.0.30")
			first, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
			if running {
				if errReport := s.Report(context.Background(), testIDs[0], Result{ID: first.Job.ID, Token: first.Job.Token, Status: "running"}); errReport != nil {
					t.Fatal(errReport)
				}
			}
			store.now = store.now.Add(2 * time.Hour)
			resume, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
			if running {
				if resume.Job == nil || resume.Job.ID != first.Job.ID || resume.Job.Running {
					t.Fatal("running job lost reconciliation")
				}
			} else {
				if resume.Job != nil {
					t.Fatal("expired unstarted job resumed outside window")
				}
				st, _ := s.Status(context.Background())
				if st.Active != nil || st.Nodes[testIDs[0]].LastJob != first.Job.ID {
					t.Fatal("void receipt missing")
				}
			}
		})
	}
}

func TestOptionalWindowClockAndLongSleepDoNotProduceNightlySpam(t *testing.T) {
	s, store := fixture(t)
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	first, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	report(t, s, first.Job, "success")
	second, _ := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	report(t, s, second.Job, "success")
	st, _ := s.Status(context.Background())
	policy := st.Policy
	policy.OptionalWindow = "09:30"
	if errPolicy := s.SetPolicy(context.Background(), policy); errPolicy != nil {
		t.Fatal(errPolicy)
	}
	calls := 0
	s.Announce = func(context.Context, string, Notice) error { calls++; return nil }
	_, _ = s.Plan(context.Background(), testIDs[2], inv("8.0.29"))
	deliverAll(t, s)
	st, _ = s.Status(context.Background())
	w := st.Rollout.OptionalWindows[testIDs[2]]
	zone, _ := time.LoadLocation("America/Chicago")
	if w.WindowStart.In(zone).Format("15:04") != "09:30" {
		t.Fatal("optional clock ignored")
	}
	store.now = store.now.Add(3 * 24 * time.Hour)
	deliverAll(t, s)
	before := calls
	for i := 0; i < 4; i++ {
		store.now = store.now.Add(24 * time.Hour)
		deliverAll(t, s)
	}
	if calls != before {
		t.Fatal("sleeping laptop generated nightly notices")
	}
	wake, _ := s.Plan(context.Background(), testIDs[2], inv("8.0.29"))
	if wake.Job != nil {
		t.Fatal("long sleeping optional upgraded on wake")
	}
	deliverAll(t, s)
	if calls != before+1 {
		t.Fatal("new awake window not independently announced")
	}
}

func TestPauseNoticeRenameAndNoopMaintenance(t *testing.T) {
	s, _, _ := pending(t)
	deliverAll(t, s)
	st, _ := s.Status(context.Background())
	revision := st.Revision
	for i := 0; i < 3; i++ {
		if errDeliver := s.deliverNotice(context.Background()); errDeliver != nil {
			t.Fatal(errDeliver)
		}
	}
	st, _ = s.Status(context.Background())
	if st.Revision != revision {
		t.Fatal("idle maintenance wrote fleet state")
	}
	policy := st.Policy
	policy.Nodes[0].Name = "Renamed fake node"
	if errPolicy := s.SetPolicy(context.Background(), policy); errPolicy != nil {
		t.Fatal("rename treated as membership change", errPolicy)
	}
	notices := []Announcement{}
	s.Announce = func(_ context.Context, _ string, n Notice) error {
		notices = append(notices, n.Announcement)
		return nil
	}
	policy.PatchUpdates = false
	if errPolicy := s.SetPolicy(context.Background(), policy); errPolicy != nil {
		t.Fatal(errPolicy)
	}
	deliverAll(t, s)
	if errPolicy := s.SetPolicy(context.Background(), policy); errPolicy != nil {
		t.Fatal(errPolicy)
	}
	deliverAll(t, s)
	if len(notices) != 1 || notices[0].Phase != "postponed" || notices[0].Reason != "paused" {
		t.Fatalf("pause missing or spammed: %+v", notices)
	}
}

func TestReleaseDigestCacheAndSixHourBackoffCap(t *testing.T) {
	previous := candidate("8.0.30")
	name := "CLIProxyAPI_8.0.30_darwin_aarch64.tar.gz"
	previous.URL = releasePrefix + "v8.0.30/" + name
	calls := 0
	digest := previous.ArchiveSHA256
	client := &http.Client{Transport: releaseTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := "no checksum"
		if r.URL.String() == releaseAPI {
			body = fmt.Sprintf(`{"tag_name":"v8.0.30","assets":[{"name":%q,"browser_download_url":%q,"digest":%q},{"name":"checksums.txt","browser_download_url":%q}]}`, name, previous.URL, "sha256:"+digest, releasePrefix+"v8.0.30/checksums.txt")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	got, errFetch := fetchStableRelease(context.Background(), client, &previous)
	if errFetch != nil || got != previous || calls != 1 {
		t.Fatal("unchanged digest redownloaded archive", calls, errFetch)
	}
	digest = strings.Repeat("d", 64)
	if _, errNew := fetchStableRelease(context.Background(), client, &previous); errNew == nil || calls != 3 {
		t.Fatal("changed digest did not trigger verification", calls, errNew)
	}
	s, store := fixture(t)
	s.FetchRelease = func(context.Context) (Release, error) {
		return Release{}, &ReleaseCheckError{Code: "rate_limited", RetryAt: store.now.Add(7 * 24 * time.Hour), RetryAfter: 999 * time.Hour}
	}
	if errCheck := s.Check(context.Background()); errCheck != nil {
		t.Fatal(errCheck)
	}
	st, _ := s.Status(context.Background())
	if !st.CheckRetryAt.Equal(store.now.Add(6 * time.Hour)) {
		t.Fatal("untrusted backoff exceeded six hours")
	}
}
