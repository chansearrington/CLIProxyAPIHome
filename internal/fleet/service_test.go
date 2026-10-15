package fleet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu  sync.Mutex
	raw []byte
	now time.Time
}

func (m *memoryStore) FleetLoad(context.Context) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]byte(nil), m.raw...), len(m.raw) > 0, nil
}
func (m *memoryStore) FleetSwap(_ context.Context, old []byte, exists bool, next []byte) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if exists != (len(m.raw) > 0) || !bytes.Equal(old, m.raw) {
		return false, nil
	}
	m.raw = append([]byte(nil), next...)
	return true, nil
}
func (m *memoryStore) CurrentDatabaseTime(context.Context) (time.Time, error) { return m.now, nil }

var testIDs = []string{"00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002", "00000000-0000-4000-8000-000000000003"}

func fixture(t *testing.T) (*Service, *memoryStore) {
	t.Helper()
	store := &memoryStore{now: time.Date(2026, 10, 15, 1, 0, 0, 0, time.UTC)}
	s := New(store)
	s.Catalog = func() map[string]map[string]any { return map[string]map[string]any{"shared": {"name": "Shared"}} }
	policy := Policy{Enabled: true, CatalogSync: false, PatchUpdates: true, IntervalSeconds: 3600, Nodes: []NodePolicy{{ID: testIDs[0], Name: "Canary", Required: true}, {ID: testIDs[1], Name: "Second", Required: true}, {ID: testIDs[2], Name: "Laptop", Required: false}}}
	if errPolicy := s.SetPolicy(context.Background(), policy); errPolicy != nil {
		t.Fatal(errPolicy)
	}
	return s, store
}
func inv(version string) Inventory {
	return Inventory{Version: version, SHA256: strings.Repeat("a", 64), Size: 100, Platform: "darwin/arm64", Catalog: true}
}
func candidate(version string) Release {
	return Release{Version: version, SHA256: strings.Repeat("b", 64), ArchiveSHA256: strings.Repeat("c", 64), Size: 101, URL: releasePrefix + "v" + version + "/fixture.tar.gz"}
}
func ready(t *testing.T, s *Service, version string) {
	t.Helper()
	for _, id := range testIDs[:2] {
		if _, errPlan := s.Plan(context.Background(), id, inv(version)); errPlan != nil {
			t.Fatal(errPlan)
		}
	}
}
func setRelease(t *testing.T, s *Service, version string) {
	t.Helper()
	release := candidate(version)
	s.FetchRelease = func(context.Context) (Release, error) { return release, nil }
	if errCheck := s.Check(context.Background()); errCheck != nil {
		t.Fatal(errCheck)
	}
}
func report(t *testing.T, s *Service, job *Job, status string) {
	t.Helper()
	if job == nil {
		t.Fatal("missing job")
	}
	inventory := job.Before
	if status == "success" && job.Release != nil {
		inventory.Version = job.Release.Version
		inventory.SHA256 = job.Release.SHA256
		inventory.Size = job.Release.Size
	}
	if errReport := s.Report(context.Background(), job.NodeID, Result{ID: job.ID, Token: job.Token, Status: status, Inventory: inventory, Error: map[bool]string{true: "fixture_failure"}[status == "failed"]}); errReport != nil {
		t.Fatal(errReport)
	}
}
func TestPatchReleasePolicy(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		ok   bool
	}{{"8.0.29", "8.0.30", true}, {"8.0.30", "8.0.29", false}, {"8.0.30", "8.1.0", false}, {"8.0.30", "9.0.0", false}, {"8.0.30", "8.0.31-beta", false}} {
		if got := PatchAllowed(tc.a, tc.b); got != tc.ok {
			t.Fatalf("%s -> %s: %v", tc.a, tc.b, got)
		}
	}
}
func TestOptionalOfflineLaptopDoesNotBlockCanaryAndSerialRollout(t *testing.T) {
	s, _ := fixture(t)
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	waiting, errWaiting := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	if errWaiting != nil || waiting.Job != nil {
		t.Fatalf("second node started before canary: %+v %v", waiting, errWaiting)
	}
	first, errFirst := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if errFirst != nil {
		t.Fatal(errFirst)
	}
	if first.Job == nil || first.Job.Operation != "upgrade" {
		t.Fatalf("canary not scheduled: %+v", first)
	}
	concurrent, errConcurrent := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	if errConcurrent != nil || concurrent.Job != nil {
		t.Fatal("concurrent live change")
	}
	report(t, s, first.Job, "success")
	second, errSecond := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	if errSecond != nil {
		t.Fatal(errSecond)
	}
	report(t, s, second.Job, "success")
	laptop, errLaptop := s.Plan(context.Background(), testIDs[2], inv("8.0.29"))
	if errLaptop != nil {
		t.Fatal(errLaptop)
	}
	if laptop.Job == nil || laptop.Job.Operation != "upgrade" {
		t.Fatalf("returning optional node not upgraded: %+v", laptop)
	}
}
func TestLeaseAndCertificateScopeCannotBeSpoofed(t *testing.T) {
	s, _ := fixture(t)
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	plan, errPlan := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if errPlan != nil {
		t.Fatal(errPlan)
	}
	for _, result := range []Result{{ID: plan.Job.ID, Token: "wrong", Status: "running"}, {ID: "wrong", Token: plan.Job.Token, Status: "running"}} {
		if errReport := s.Report(context.Background(), testIDs[0], result); errReport == nil {
			t.Fatal("forged report accepted")
		}
	}
	if errReport := s.Report(context.Background(), testIDs[1], Result{ID: plan.Job.ID, Token: plan.Job.Token, Status: "running"}); errReport == nil {
		t.Fatal("cross-node report accepted")
	}
	state, errState := s.Status(context.Background())
	if errState != nil {
		t.Fatal(errState)
	}
	if state.Active.Token != "" {
		t.Fatal("Management API leaked task token")
	}
}
func TestFailureRollsBackInReverseOrderAndLatchesRelease(t *testing.T) {
	s, _ := fixture(t)
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	a, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	report(t, s, a.Job, "success")
	b, _ := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	report(t, s, b.Job, "failed")
	rb, _ := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	if rb.Job == nil || rb.Job.Operation != "rollback" {
		t.Fatalf("failed node rollback missing: %+v", rb)
	}
	report(t, s, rb.Job, "success")
	ra, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.30"))
	if ra.Job == nil || ra.Job.Operation != "rollback" {
		t.Fatalf("canary rollback missing: %+v", ra)
	}
	report(t, s, ra.Job, "success")
	idle, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if idle.Job != nil {
		t.Fatal("failed release retried without review")
	}
}
func TestExpiredJobDoesNotAdvanceAndOwnerCanReconcile(t *testing.T) {
	s, store := fixture(t)
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	a, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	store.now = store.now.Add(10 * time.Minute)
	other, _ := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	if other.Job != nil || other.Status != "operation_needs_reconciliation" {
		t.Fatal("expired job advanced")
	}
	own, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if own.Job == nil || own.Job.ID != a.Job.ID {
		t.Fatal("owner could not reconcile exact task")
	}
	report(t, s, a.Job, "success")
}
func TestMajorRequiresExplicitReview(t *testing.T) {
	s, _ := fixture(t)
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.1.0")
	a, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if a.Job != nil {
		t.Fatal("minor release installed automatically")
	}
	if errReview := s.Review(context.Background(), "8.1.0", true); errReview != nil {
		t.Fatal(errReview)
	}
	approved, errPlan := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if errPlan != nil || approved.Job == nil {
		t.Fatalf("approved release unavailable: %+v %v", approved, errPlan)
	}
}
func TestHeldReleaseIsNotAutomaticallyReopened(t *testing.T) {
	s, _ := fixture(t)
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	if errHold := s.Review(context.Background(), "8.0.30", false); errHold != nil {
		t.Fatal(errHold)
	}
	a, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if a.Job != nil {
		t.Fatal("held release scheduled")
	}
}
func TestCatalogTasksUseExistingRegistryWithoutChangingSelections(t *testing.T) {
	s, store := fixture(t)
	raw, _, _ := store.FleetLoad(context.Background())
	st, _ := decode(raw)
	st.Policy.PatchUpdates = false
	st.Policy.CatalogSync = true
	if errPolicy := s.SetPolicy(context.Background(), st.Policy); errPolicy != nil {
		t.Fatal(errPolicy)
	}
	a, errPlan := s.Plan(context.Background(), testIDs[0], inv("8.0.30"))
	if errPlan != nil {
		t.Fatal(errPlan)
	}
	if a.Job == nil || a.Job.Operation != "catalog" || a.Job.Metadata["shared"]["name"] != "Shared" {
		t.Fatalf("bad catalog plan: %+v", a)
	}
	report(t, s, a.Job, "success")
	b, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.30"))
	if b.Job != nil {
		t.Fatal("unchanged catalog rewritten")
	}
	store.now = store.now.Add(2 * time.Hour)
	c, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.30"))
	if c.Job == nil {
		t.Fatal("periodic catalog check missing")
	}
}
func TestSuccessfulBinaryMustMatchVerifiedArtifact(t *testing.T) {
	s, _ := fixture(t)
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	a, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if errReport := s.Report(context.Background(), a.Job.NodeID, Result{ID: a.Job.ID, Token: a.Job.Token, Status: "success", Inventory: inv("8.0.30")}); errReport == nil {
		t.Fatal("wrong binary acknowledged")
	}
}
func TestUnregisteredWorkerAndPolicyValidation(t *testing.T) {
	s, _ := fixture(t)
	if _, errPlan := s.Plan(context.Background(), "unknown", inv("8.0.30")); errPlan == nil {
		t.Fatal("unregistered worker got plan")
	}
	for _, policy := range []Policy{{Enabled: true, IntervalSeconds: 3600}, {IntervalSeconds: 1}, {Enabled: true, PatchUpdates: true, IntervalSeconds: 3600, Nodes: []NodePolicy{{ID: testIDs[0]}}}} {
		if errPolicy := ValidatePolicy(policy); errPolicy == nil {
			t.Fatal("invalid policy accepted")
		}
	}
}
func TestReleaseCheckFailureIsRecordedWithoutInventingRelease(t *testing.T) {
	s, _ := fixture(t)
	s.FetchRelease = func(context.Context) (Release, error) { return Release{}, fmt.Errorf("fixture") }
	if errCheck := s.Check(context.Background()); errCheck != nil {
		t.Fatal(errCheck)
	}
	st, _ := s.Status(context.Background())
	if st.Latest != nil || st.CheckError != "release_check_failed" {
		t.Fatal("bad release check state")
	}
}
func TestStateJSONIncludesNoSecretInStatus(t *testing.T) {
	s, _ := fixture(t)
	st, _ := s.Status(context.Background())
	raw, _ := json.Marshal(st)
	if bytes.Contains(raw, []byte(`"token":"`)) {
		t.Fatal("unexpected task token")
	}
}

func TestAcceptedReleaseRequiresAllRequiredBinariesAndDoesNotWaitForLaptop(t *testing.T) {
	s, _ := fixture(t)
	ready(t, s, "8.0.29")
	setRelease(t, s, "8.0.30")
	a, _ := s.Plan(context.Background(), testIDs[0], inv("8.0.29"))
	if a.Accepted != nil {
		t.Fatal("unverified release accepted")
	}
	report(t, s, a.Job, "success")
	b, _ := s.Plan(context.Background(), testIDs[1], inv("8.0.29"))
	report(t, s, b.Job, "success")
	laptop, _ := s.Plan(context.Background(), testIDs[2], inv("8.0.29"))
	if laptop.Accepted == nil || laptop.Accepted.Version != "8.0.30" || len(laptop.VerifiedNodes) != 2 {
		t.Fatal("required-node acceptance attestation missing")
	}
}
