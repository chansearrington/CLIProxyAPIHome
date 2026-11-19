package fleet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRollbackReceiptsCannotProduceFalseDoneAfterHoldAndApprove(t *testing.T) {
	s, _ := fixture(t)
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	a, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	report(t, s, a.Job, "success")
	b, _ := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	report(t, s, b.Job, "failed")
	if errHold := s.Review(context.Background(), "8.0.30", false); errHold != nil {
		t.Fatal(errHold)
	}
	rb, _ := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	report(t, s, rb.Job, "success")
	ra, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.30"))
	report(t, s, ra.Job, "success")
	st, _ := s.Status(context.Background())
	if len(st.Rollout.Completed) != 0 {
		t.Fatal("rolled back nodes still marked completed")
	}
	if errHold := s.Review(context.Background(), "8.0.30", false); errHold == nil {
		t.Fatal("hold accepted in failed phase")
	}
	if errApprove := s.Review(context.Background(), "8.0.30", true); errApprove != nil {
		t.Fatal(errApprove)
	}
	st, _ = s.Status(context.Background())
	if len(st.Rollout.Window.Nodes) != 2 || st.Rollout.Phase != "upgrade" {
		t.Fatal("retry scheduled zero nodes")
	}
	plan, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if plan.Job != nil || plan.Accepted != nil {
		t.Fatal("old binaries falsely accepted during retry")
	}
}

func TestFreshJobLeaseSurvivesWindowBoundaryBeforeFirstHeartbeat(t *testing.T) {
	s, store := fixture(t)
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	store.now = store.now.Add(windowLength - time.Minute)
	refreshSeen(t, s, testIDs[:2])
	job, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	store.now = store.now.Add(90 * time.Second)
	resume, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if resume.Job == nil || resume.Job.ID != job.Job.ID {
		t.Fatal("fresh job swallowed before its first heartbeat")
	}
	if errReport := s.Report(context.Background(), testIDs[0], Result{ID: job.Job.ID, Token: job.Job.Token, Status: "running"}); errReport != nil {
		t.Fatal(errReport)
	}
	store.now = store.now.Add(2 * time.Hour)
	resume, _ = s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if resume.Job == nil {
		t.Fatal("running operation lost reconciliation")
	}
}

func TestOptionalDefaultIsUnconfiguredAndAcceptedReleaseSurvivesNewCandidate(t *testing.T) {
	s, _ := fixture(t)
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	a, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	report(t, s, a.Job, "success")
	b, _ := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	report(t, s, b.Job, "success")
	st, _ := s.Status(context.Background())
	policy := st.Policy
	policy.OptionalWindow = ""
	if errPolicy := s.SetPolicy(context.Background(), policy); errPolicy != nil {
		t.Fatal(errPolicy)
	}
	if errChange := s.change(context.Background(), func(st *State, _ time.Time) error { r := candidate("8.0.31"); st.Latest = &r; return nil }); errChange != nil {
		t.Fatal(errChange)
	}
	verified := inv("8.0.30")
	verified.SHA256 = candidate("8.0.30").SHA256
	verified.Size = 101
	next, _ := s.Plan(context.Background(), testIDs[0], verified)
	if next.Accepted == nil || next.Accepted.Version != "8.0.30" {
		t.Fatal("pending target replaced last completed attestation")
	}
	// Hold the newly pending required target so this test can inspect optional behavior of the accepted target.
	if errChange := s.change(context.Background(), func(st *State, _ time.Time) error { st.Rollout = nil; return nil }); errChange != nil {
		t.Fatal(errChange)
	}
	if errChange := s.change(context.Background(), func(st *State, _ time.Time) error {
		r := candidate("8.0.30")
		st.Latest = &r
		st.Rollout = &Rollout{Release: r, Phase: "complete", Completed: map[string]Inventory{}}
		st.Notices = nil
		return nil
	}); errChange != nil {
		t.Fatal(errChange)
	}
	wake, _ := s.Plan(context.Background(), testIDs[2], inv("8.0.29"))
	st, _ = s.Status(context.Background())
	if wake.Job != nil || len(st.Rollout.OptionalWindows) != 0 || len(st.Notices) != 0 {
		t.Fatal("unconfigured optional clock announced a daytime-only laptop")
	}
}

func TestHookChangesAndMaskedRoundTripPreservePostponement(t *testing.T) {
	s, _, _ := pending(t)
	deliverAll(t, s)
	if errPostpone := s.Postpone(context.Background(), "8.0.30", "", false); errPostpone != nil {
		t.Fatal(errPostpone)
	}
	st, _ := s.Status(context.Background())
	start, id := st.Rollout.Window.WindowStart, st.Rollout.Window.ID
	policy := st.Policy
	full := policy.AnnouncementURL
	policy.AnnouncementURL = MaskAnnouncementURL(full)
	if errPolicy := s.SetPolicy(context.Background(), policy); errPolicy != nil {
		t.Fatal(errPolicy)
	}
	st, _ = s.Status(context.Background())
	if st.Policy.AnnouncementURL != full || st.Rollout.Window.ID != id {
		t.Fatal("GET-then-PUT erased hook or rescheduled window")
	}
	policy.AnnouncementURL = "https://new.example.invalid/hook"
	if errPolicy := s.SetPolicy(context.Background(), policy); errPolicy != nil {
		t.Fatal(errPolicy)
	}
	st, _ = s.Status(context.Background())
	if st.Rollout.Window.WindowStart.Before(start) {
		t.Fatal("hook edit cancelled operator postponement")
	}
}

func TestImportantNoticesSurvive4xxExpiryAndQueueCap(t *testing.T) {
	s, store := fixture(t)
	phases := []string{"done", "rolled_back", "rollback", "rollback_failed"}
	if errChange := s.change(context.Background(), func(st *State, now time.Time) error {
		for _, phase := range phases {
			queueNotice(st, &Window{ID: phase, WindowStart: now, Nodes: testIDs[:1]}, "8.0.30", phase, now.Add(-48*time.Hour))
		}
		for i := 0; i < 50; i++ {
			queueNotice(st, &Window{ID: fmt.Sprint(i), WindowStart: now, Nodes: testIDs[:1]}, "8.0.30", "scheduled", now)
		}
		return nil
	}); errChange != nil {
		t.Fatal(errChange)
	}
	st, _ := s.Status(context.Background())
	if len(st.Notices) != maxNotices+len(phases) {
		t.Fatal("queue cap discarded important notice")
	}
	s.Announce = func(context.Context, string, Notice) error { return &announcementError{Status: 400} }
	for i := 0; i < 8; i++ {
		if errDeliver := s.deliverNotice(context.Background()); errDeliver != nil {
			t.Fatal(errDeliver)
		}
		st, _ = s.Status(context.Background())
		if st.Notices[0].Phase != "done" || st.NoticeRetryAt.Sub(store.now) > time.Hour {
			t.Fatal("important 4xx notice dropped or backoff unbounded")
		}
		store.now = st.NoticeRetryAt
	}
	if errChange := s.change(context.Background(), func(st *State, now time.Time) error { pruneNotices(st, now.Add(72*time.Hour)); return nil }); errChange != nil {
		t.Fatal(errChange)
	}
	st, _ = s.Status(context.Background())
	if len(st.Notices) != 4 {
		t.Fatal("important notices expired")
	}
}

func TestRollbackProceedsAfterFifteenMinuteNoticeOutageAndKeepsNotice(t *testing.T) {
	s, store := fixture(t)
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	a, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	s.Announce = func(context.Context, string, Notice) error { return &announcementError{Status: 400} }
	if errReport := s.Report(context.Background(), testIDs[0], Result{ID: a.Job.ID, Token: a.Job.Token, Status: "failed", Error: "fake_failure"}); errReport != nil {
		t.Fatal(errReport)
	}
	if errDeliver := s.deliverNotice(context.Background()); errDeliver != nil {
		t.Fatal(errDeliver)
	}
	store.now = store.now.Add(14 * time.Minute)
	before, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if before.Job != nil {
		t.Fatal("rollback bypassed notice wait early")
	}
	store.now = store.now.Add(time.Minute)
	after, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if after.Job == nil || after.Job.Operation != "rollback" {
		t.Fatal("hook outage stranded broken node")
	}
	st, _ := s.Status(context.Background())
	found, event := false, false
	for _, n := range st.Notices {
		if n.Phase == "rollback" {
			found = true
		}
	}
	for _, e := range st.History {
		if e.Status == "rollback_unannounced" {
			event = true
		}
	}
	if !found || !event {
		t.Fatal("unannounced rollback not retained and audited")
	}
}

func TestHomeOnlyReceiptsAndAssetMetadataNeverReachWorkerPayload(t *testing.T) {
	s, _ := fixture(t)
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	if errChange := s.change(context.Background(), func(st *State, now time.Time) error {
		st.Rollout.Release.ArchiveSize = 1234
		st.Rollout.Release.AssetUpdatedAt = now
		return nil
	}); errChange != nil {
		t.Fatal(errChange)
	}
	assertWire := func(plan Plan) {
		t.Helper()
		raw, errEncode := json.Marshal(plan)
		if errEncode != nil {
			t.Fatal(errEncode)
		}
		for _, field := range []string{`"running":`, `"archive_size":`, `"asset_updated_at":`} {
			if bytes.Contains(raw, []byte(field)) {
				t.Fatalf("Home-only field reached existing worker: %s", field)
			}
		}
	}
	a, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	assertWire(a)
	if errReport := s.Report(context.Background(), testIDs[0], Result{ID: a.Job.ID, Token: a.Job.Token, Status: "running"}); errReport != nil {
		t.Fatal(errReport)
	}
	resumed, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	assertWire(resumed)
	st, _ := s.Status(context.Background())
	if !st.Active.Running || st.Active.Release.ArchiveSize != 1234 {
		t.Fatal("wire filtering mutated Home receipt")
	}
	report(t, s, a.Job, "success")
	b, _ := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	report(t, s, b.Job, "success")
	laptop, _ := s.Plan(context.Background(), testIDs[2], inv("8.0.29"))
	assertWire(laptop)
	if laptop.Accepted == nil {
		t.Fatal("completed worker attestation missing")
	}
}

func TestDigestlessReleaseCacheUsesSizeAndUpdatedAt(t *testing.T) {
	previous := candidate("8.0.30")
	name := "CLIProxyAPI_8.0.30_darwin_aarch64.tar.gz"
	previous.URL = releasePrefix + "v8.0.30/" + name
	previous.ArchiveSize = 1234
	previous.AssetUpdatedAt = time.Date(2026, 11, 18, 1, 0, 0, 0, time.UTC)
	calls := 0
	size := int64(1234)
	client := &http.Client{Transport: releaseTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := "no checksum"
		if r.URL.String() == releaseAPI {
			body = fmt.Sprintf(`{"tag_name":"v8.0.30","assets":[{"name":%q,"browser_download_url":%q,"size":%d,"updated_at":"2026-11-18T01:00:00Z"},{"name":"checksums.txt","browser_download_url":%q}]}`, name, previous.URL, size, releasePrefix+"v8.0.30/checksums.txt")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	if _, errFetch := fetchStableRelease(context.Background(), client, &previous); errFetch != nil || calls != 1 {
		t.Fatal("unchanged digest-less asset redownloaded", errFetch, calls)
	}
	size++
	if _, errFetch := fetchStableRelease(context.Background(), client, &previous); errFetch == nil || calls != 3 {
		t.Fatal("changed metadata was not reverified", errFetch, calls)
	}
}
