package fleet

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/router-for-me/CLIProxyAPIHome/internal/registry"
	log "github.com/sirupsen/logrus"
)

var versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
var hashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

// Service owns policy, serialization, leases and rollout decisions in Home's database.
type Service struct {
	store        Store
	FetchRelease func(context.Context) (Release, error)
	Catalog      func() map[string]map[string]any
	Announce     func(context.Context, string, Notice) error
}

func New(store Store) *Service {
	s := &Service{store: store, Catalog: catalogMetadata, Announce: postAnnouncement}
	s.FetchRelease = func(ctx context.Context) (Release, error) {
		st, errState := s.Status(ctx)
		if errState != nil {
			return Release{}, errState
		}
		return fetchStableRelease(ctx, &http.Client{Timeout: 2 * time.Minute}, st.Latest)
	}
	return s
}

func catalogMetadata() map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, model := range registry.GetGlobalRegistry().GetAvailableModelDefinitions() {
		if model == nil || model.ID == "" || model.ID == "agent-os-home-canary" || strings.HasPrefix(model.ID, "copilot/") {
			continue
		}
		name := model.DisplayName
		if name == "" {
			name = model.ID
		}
		item := map[string]any{"name": name}
		if model.ContextLength > 0 {
			item["contextWindow"] = model.ContextLength
		}
		if model.MaxCompletionTokens > 0 {
			item["maxTokens"] = model.MaxCompletionTokens
		}
		if model.Thinking != nil && len(model.Thinking.Levels) > 0 {
			item["reasoning"] = true
		}
		inputs := []string{}
		valid := true
		for _, input := range model.SupportedInputModalities {
			value := strings.ToLower(input)
			if value != "text" && value != "image" {
				valid = false
				break
			}
			inputs = append(inputs, value)
		}
		if valid && len(inputs) > 0 {
			item["input"] = inputs
		}
		out[model.ID] = item
	}
	return out
}

func decode(raw []byte) (State, error) {
	state := State{Schema: 1, Policy: Policy{IntervalSeconds: 3600}, Nodes: map[string]NodeState{}, History: []Event{}}
	if len(raw) > 0 {
		if errDecode := json.Unmarshal(raw, &state); errDecode != nil {
			return state, errDecode
		}
	}
	if state.Schema != 1 {
		return state, fmt.Errorf("unsupported fleet state schema")
	}
	if state.Nodes == nil {
		state.Nodes = map[string]NodeState{}
	}
	return state, nil
}

func (s *Service) Status(ctx context.Context) (State, error) {
	raw, _, errLoad := s.store.FleetLoad(ctx)
	if errLoad != nil {
		return State{}, errLoad
	}
	state, errDecode := decode(raw)
	if state.Active != nil {
		state.Active.Token = ""
		state.Active.Metadata = nil
	}
	return state, errDecode
}

func (s *Service) change(ctx context.Context, fn func(*State, time.Time) error) error {
	for attempt := 0; attempt < 12; attempt++ {
		raw, exists, errLoad := s.store.FleetLoad(ctx)
		if errLoad != nil {
			return errLoad
		}
		state, errDecode := decode(raw)
		if errDecode != nil {
			return errDecode
		}
		now, errNow := s.store.CurrentDatabaseTime(ctx)
		if errNow != nil {
			return errNow
		}
		if errChange := fn(&state, now); errChange != nil {
			return errChange
		}
		next, errEncode := json.Marshal(state)
		if errEncode != nil {
			return errEncode
		}
		if bytes.Equal(raw, next) {
			return nil
		}
		state.Revision++
		next, errEncode = json.Marshal(state)
		if errEncode != nil {
			return errEncode
		}
		swapped, errSwap := s.store.FleetSwap(ctx, raw, exists, next)
		if errSwap != nil {
			return errSwap
		}
		if swapped {
			return nil
		}
	}
	return fmt.Errorf("fleet state changed concurrently; retry")
}

func ValidatePolicy(policy Policy) error {
	if policy.OptionalWindow != "" {
		if parsed, errParse := time.Parse("15:04", policy.OptionalWindow); errParse != nil || parsed.Format("15:04") != policy.OptionalWindow {
			return fmt.Errorf("optional_window must be HH:MM in America/Chicago")
		}
	}
	if errURL := validateAnnouncementURL(policy.AnnouncementURL); errURL != nil {
		return errURL
	}
	if policy.Enabled && policy.PatchUpdates && policy.AnnouncementURL == "" {
		return fmt.Errorf("patch updates require announcement_url")
	}
	if policy.IntervalSeconds < 300 || policy.IntervalSeconds > 86400 {
		return fmt.Errorf("interval_seconds must be between 300 and 86400")
	}
	if len(policy.Nodes) > 64 || (policy.Enabled && len(policy.Nodes) == 0) {
		return fmt.Errorf("enabled policy requires 1 to 64 nodes")
	}
	seen := map[string]bool{}
	required := false
	for _, n := range policy.Nodes {
		if _, errID := uuid.Parse(n.ID); errID != nil || seen[n.ID] || len(n.Name) > 128 {
			return fmt.Errorf("node IDs must be unique UUIDs and names at most 128 characters")
		}
		seen[n.ID] = true
		required = required || n.Required
	}
	if policy.Enabled && policy.PatchUpdates && !required {
		return fmt.Errorf("patch updates require an always-on required canary")
	}
	return nil
}

func sameMembership(a, b []NodePolicy) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Required != b[i].Required {
			return false
		}
	}
	return true
}

func (s *Service) SetPolicy(ctx context.Context, policy Policy, operation ...PolicyChange) error {
	if errPolicy := ValidatePolicy(policy); errPolicy != nil {
		return errPolicy
	}
	audit := PolicyChange{}
	if len(operation) > 0 {
		audit = operation[0]
	}
	return s.change(ctx, func(st *State, now time.Time) error {
		if st.Active != nil {
			return fmt.Errorf("a fleet operation is active; wait before changing policy")
		}
		if st.NoticeLease.After(now) {
			return fmt.Errorf("announcement is being delivered; retry")
		}
		if audit.Force {
			closeWindows(st, "operator_force", now)
			if st.Rollout != nil && st.Rollout.Phase == "upgrade" {
				st.Rollout.Phase = "held"
			}
		}
		if !sameMembership(st.Policy.Nodes, policy.Nodes) {
			for _, w := range windows(st.Rollout) {
				if !w.Closed {
					return fmt.Errorf("hold the pending release or use operator force before changing node membership or order")
				}
			}
		}
		if (st.Policy.Enabled && !policy.Enabled) || (st.Policy.PatchUpdates && !policy.PatchUpdates) {
			closeWindows(st, "paused", now)
		}
		changedHook := st.Policy.AnnouncementURL != policy.AnnouncementURL || st.Policy.OptionalWindow != policy.OptionalWindow
		st.Policy = policy
		if changedHook && st.Rollout != nil {
			for _, w := range windows(st.Rollout) {
				if !w.Closed {
					reschedule(st, w, st.Rollout.Release.Version, now, windowTime(st, w, now.Add(noticeLead)))
				}
			}
		}
		st.LastCheck = time.Time{}
		addEvent(st, now, "", "policy", "updated", "")
		auditEvent(st, audit.Force, audit.Caller)
		return nil
	})
}

func addEvent(st *State, now time.Time, nodeID, operation, status, version string) {
	st.History = append(st.History, Event{Time: now, NodeID: nodeID, Operation: operation, Status: status, Version: version})
	if len(st.History) > 100 {
		st.History = st.History[len(st.History)-100:]
	}
}

func validInventory(in Inventory) bool {
	return versionPattern.MatchString(in.Version) && hashPattern.MatchString(in.SHA256) && in.Size > 0 && in.Platform == "darwin/arm64"
}

// PatchAllowed never promotes a different release line or a prerelease automatically.
func PatchAllowed(current, candidate string) bool {
	if !versionPattern.MatchString(current) || !versionPattern.MatchString(candidate) {
		return false
	}
	a, b := strings.Split(current, "."), strings.Split(candidate, ".")
	av, _ := strconv.Atoi(a[2])
	bv, _ := strconv.Atoi(b[2])
	return a[0] == b[0] && a[1] == b[1] && bv > av
}

func VersionNewer(current, candidate string) bool {
	if !versionPattern.MatchString(current) || !versionPattern.MatchString(candidate) {
		return false
	}
	a, b := strings.Split(current, "."), strings.Split(candidate, ".")
	for i := range a {
		av, _ := strconv.Atoi(a[i])
		bv, _ := strconv.Atoi(b[i])
		if av != bv {
			return bv > av
		}
	}
	return false
}

func (s *Service) Review(ctx context.Context, version string, approve bool) error {
	return s.change(ctx, func(st *State, now time.Time) error {
		run := st.Rollout
		if !approve && run != nil && run.Release.Version == version {
			closeWindows(st, "held", now)
			if run.Phase != "rollback" {
				run.Phase = "held"
			}
			run.Error = "held_by_operator"
			addEvent(st, now, "", "review", "held", version)
			return nil
		}
		if st.Active != nil {
			return fmt.Errorf("a fleet operation is active")
		}
		if run != nil && (run.Phase == "rollback" || (run.Phase == "upgrade" && len(run.Completed) > 0)) {
			return fmt.Errorf("hold the partial rollout before reviewing another release")
		}
		if st.Latest == nil || st.Latest.Version != version {
			return fmt.Errorf("release is not the currently verified candidate")
		}
		status := "held"
		if approve {
			if run != nil && run.Phase == "held" && run.Release.Version == version {
				ids := []string{}
				for _, n := range st.Policy.Nodes {
					if n.Required {
						if _, ok := run.Completed[n.ID]; !ok {
							ids = append(ids, n.ID)
						}
					}
				}
				run.Window = scheduleWindow(st, now, version, ids, false)
				run.Phase = "upgrade"
				run.Approved = true
				run.Error = ""
			} else {
				startRollout(st, *st.Latest, true, now)
			}
			st.CheckError = ""
			status = "approved"
		} else {
			closeWindows(st, "held", now)
			st.Rollout = &Rollout{Release: *st.Latest, Completed: map[string]Inventory{}, Phase: "held", Error: "held_by_operator"}
		}
		addEvent(st, now, "", "review", status, version)
		return nil
	})
}

func registered(policy Policy, id string) bool {
	for _, n := range policy.Nodes {
		if n.ID == id {
			return true
		}
	}
	return false
}

func (s *Service) Plan(ctx context.Context, nodeID string, in Inventory) (Plan, error) {
	plan := Plan{PollSeconds: 60, Status: "disabled"}
	if !validInventory(in) {
		return plan, fmt.Errorf("invalid worker inventory")
	}
	metadata := s.Catalog()
	data, errEncode := json.Marshal(metadata)
	if errEncode != nil {
		return plan, errEncode
	}
	digest := sha256.Sum256(data)
	catalogHash := hex.EncodeToString(digest[:])
	errPlan := s.change(ctx, func(st *State, now time.Time) error {
		plan = Plan{PollSeconds: 60, Enabled: st.Policy.Enabled, Status: "disabled"}
		if !registered(st.Policy, nodeID) {
			return fmt.Errorf("worker is not registered in fleet policy")
		}
		ns := st.Nodes[nodeID]
		ns.Inventory = in
		ns.SeenAt = now
		st.Nodes[nodeID] = ns
		if !st.Policy.Enabled {
			return nil
		}
		voidUnstartedJob(st, now)
		if st.Active != nil {
			plan.Status = "operation_active"
			if st.Active.NodeID == nodeID {
				job := *st.Active
				job.Metadata = metadata
				plan.Job = &job
				plan.Status = "resume"
			}
			if !st.Active.ExpiresAt.After(now) {
				plan.Status = "operation_needs_reconciliation"
			}
			return nil
		}
		if st.Rollout != nil && st.Rollout.Phase == "rollback" {
			if !st.Rollout.RollbackAnnounced {
				if len(st.Rollout.RollbackNodes) == 0 {
					st.Rollout.RollbackNodes = append([]string(nil), st.Rollout.Rollback...)
				}
				queueRollback(st, now)
				plan.Status = "rollback_announcement"
				return nil
			}
			if len(st.Rollout.Rollback) == 0 {
				st.Rollout.Phase = "failed"
			} else {
				next := st.Rollout.Rollback[0]
				if next == nodeID {
					before := st.Rollout.Completed[next]
					job := newJob(nodeID, "rollback", now, in)
					job.Release = &Release{Version: before.Version, SHA256: before.SHA256, Size: before.Size}
					st.Active = job
					plan.Job = job
				}
				plan.Status = "rollback"
				return nil
			}
		}
		maybeSchedule(st, now)
		if st.Policy.PatchUpdates {
			run := st.Rollout
			if run != nil && run.Phase == "upgrade" {
				// Existing unstarted fleet state also receives a window after upgrading Home.
				if run.Window == nil || run.Window.Closed {
					ids := []string{}
					for _, n := range st.Policy.Nodes {
						if n.Required {
							ids = append(ids, n.ID)
						}
					}
					run.Window = scheduleWindow(st, now, run.Release.Version, ids, false)
				}
				maintainWindows(st, now)
				next := ""
				for _, n := range st.Policy.Nodes {
					if n.Required {
						if _, ok := run.Completed[n.ID]; !ok {
							next = n.ID
							break
						}
					}
				}
				if next == "" {
					run.Phase = "complete"
					closeWindow(st, run.Window, run.Release.Version, "done", "", now)
					delete(st.PostponedTargets, run.Release.Version)
					addEvent(st, now, "", "rollout", "complete_required", run.Release.Version)
				} else {
					plan.Status = "scheduled"
					if windowReady(st, run.Window, now) {
						if next == nodeID {
							if !(PatchAllowed(in.Version, run.Release.Version) || (run.Approved && VersionNewer(in.Version, run.Release.Version)) || (in.Version == run.Release.Version && in.SHA256 == run.Release.SHA256)) {
								return fmt.Errorf("worker version changed during rollout")
							}
							job := newJob(nodeID, "upgrade", now, in)
							job.Release = &run.Release
							st.Active = job
							plan.Job = job
						}
						plan.Status = "rollout"
						return nil
					}
				}
			}
			if run != nil && run.Phase == "complete" && in.Version != run.Release.Version && (PatchAllowed(in.Version, run.Release.Version) || (run.Approved && VersionNewer(in.Version, run.Release.Version))) {
				optional := false
				for _, n := range st.Policy.Nodes {
					if n.ID == nodeID {
						optional = !n.Required
					}
				}
				if optional {
					if run.OptionalWindows == nil {
						run.OptionalWindows = map[string]*Window{}
					}
					w := run.OptionalWindows[nodeID]
					if w == nil || w.Closed {
						w = scheduleWindow(st, now, run.Release.Version, []string{nodeID}, true)
						run.OptionalWindows[nodeID] = w
					}
					maintainWindows(st, now)
					plan.Status = "optional_scheduled"
					if windowReady(st, w, now) {
						job := newJob(nodeID, "upgrade", now, in)
						job.Release = &run.Release
						st.Active = job
						plan.Job = job
						plan.Status = "optional_upgrade"
						return nil
					}
				}
			}
		}
		if st.Policy.CatalogSync && in.Catalog && len(metadata) > 0 && (ns.CatalogHash != catalogHash || now.Sub(ns.CatalogAt) >= time.Duration(st.Policy.IntervalSeconds)*time.Second) {
			job := newJob(nodeID, "catalog", now, in)
			job.CatalogHash = catalogHash
			job.Metadata = metadata
			st.Active = job
			plan.Job = job
			plan.Status = "catalog"
			return nil
		}
		if plan.Status == "disabled" {
			plan.Status = "current"
		}
		return nil
	})
	if errPlan == nil {
		if state, errState := s.Status(ctx); errState == nil && state.Latest != nil {
			accepted := state.Latest
			if state.Rollout != nil {
				accepted = &state.Rollout.Release
			}
			verified := []string{}
			ready := true
			for _, node := range state.Policy.Nodes {
				if !node.Required {
					continue
				}
				status, ok := state.Nodes[node.ID]
				if !ok || status.Version != accepted.Version || status.SHA256 != accepted.SHA256 || status.Size != accepted.Size {
					ready = false
					break
				}
				verified = append(verified, node.ID)
			}
			if ready && len(verified) > 0 {
				release := *accepted
				plan.Accepted = &release
				plan.VerifiedNodes = verified
			}
		}
	}
	return plan, errPlan
}

func newJob(nodeID, operation string, now time.Time, before Inventory) *Job {
	return &Job{ID: uuid.NewString(), NodeID: nodeID, Operation: operation, Token: uuid.NewString(), ExpiresAt: now.Add(5 * time.Minute), Before: before}
}

func (s *Service) Report(ctx context.Context, nodeID string, result Result) error {
	if result.Status != "running" && result.Status != "success" && result.Status != "failed" && result.Status != "blocked" {
		return fmt.Errorf("invalid fleet result status")
	}
	if len(result.Error) > 256 {
		return fmt.Errorf("fleet result error is too long")
	}
	return s.change(ctx, func(st *State, now time.Time) error {
		if voidUnstartedJob(st, now) {
			return nil
		}
		ns := st.Nodes[nodeID]
		if ns.LastJob == result.ID {
			return nil
		}
		job := st.Active
		if job == nil || job.NodeID != nodeID || job.ID != result.ID || job.Token != result.Token {
			return fmt.Errorf("fleet job identity does not match")
		}
		ns.SeenAt = now
		ns.Status = result.Status
		ns.Error = result.Error
		if result.Status == "running" {
			job.Running = true
			job.ExpiresAt = now.Add(5 * time.Minute)
			st.Nodes[nodeID] = ns
			return nil
		}
		if result.Status == "success" {
			if !validInventory(result.Inventory) {
				return fmt.Errorf("invalid successful worker inventory")
			}
			if job.Operation == "upgrade" || job.Operation == "rollback" {
				if job.Release == nil || result.Inventory.Version != job.Release.Version || result.Inventory.SHA256 != job.Release.SHA256 || result.Inventory.Size != job.Release.Size {
					return fmt.Errorf("installed binary does not match the job")
				}
			}
			ns.Inventory = result.Inventory
			if job.Operation == "catalog" {
				ns.CatalogHash = job.CatalogHash
				ns.CatalogAt = now
			}
			if job.Operation == "upgrade" && st.Rollout != nil {
				st.Rollout.Completed[nodeID] = job.Before
				run := st.Rollout
				if run.Phase == "complete" {
					closeWindow(st, run.OptionalWindows[nodeID], run.Release.Version, "done", "", now)
				} else {
					complete := true
					for _, n := range st.Policy.Nodes {
						if n.Required {
							if _, ok := run.Completed[n.ID]; !ok {
								complete = false
							}
						}
					}
					if complete {
						run.Phase = "complete"
						closeWindow(st, run.Window, run.Release.Version, "done", "", now)
						delete(st.PostponedTargets, run.Release.Version)
						addEvent(st, now, "", "rollout", "complete_required", run.Release.Version)
					}
				}
			}
			if job.Operation == "rollback" && st.Rollout != nil && len(st.Rollout.Rollback) > 0 {
				st.Rollout.Rollback = st.Rollout.Rollback[1:]
			}
		} else if result.Status != "blocked" && (job.Operation == "upgrade" || job.Operation == "rollback") && st.Rollout != nil {
			if job.Operation == "upgrade" {
				run := st.Rollout
				run.Completed[nodeID] = job.Before
				optional := false
				for _, n := range st.Policy.Nodes {
					if n.ID == nodeID {
						optional = !n.Required
					}
				}
				run.Rollback = nil
				if optional {
					run.Rollback = []string{nodeID}
				} else {
					for i := len(st.Policy.Nodes) - 1; i >= 0; i-- {
						id := st.Policy.Nodes[i].ID
						if _, ok := run.Completed[id]; ok {
							run.Rollback = append(run.Rollback, id)
						}
					}
				}
				run.RollbackNodes = append([]string(nil), run.Rollback...)
				run.RollbackAnnounced = false
				run.RollbackFailedNodes = nil
				closeWindows(st, "rollback", now)
				run.Phase = "rollback"
				queueRollback(st, now)
			} else {
				st.Rollout.RollbackFailed = true
				st.Rollout.RollbackFailedNodes = append(st.Rollout.RollbackFailedNodes, nodeID)
				if len(st.Rollout.Rollback) > 0 {
					st.Rollout.Rollback = st.Rollout.Rollback[1:]
				}
				if len(st.Rollout.Rollback) == 0 {
					st.Rollout.Phase = "failed"
				}
			}
			st.Rollout.Error = result.Error
		}
		if st.Rollout != nil && job.Operation == "rollback" && len(st.Rollout.Rollback) == 0 {
			st.Rollout.Phase = "failed"
			run := st.Rollout
			for _, w := range windows(run) {
				removeWindowNotices(st, w.ID)
				w.Closed = true
			}
			start := now
			if run.Window != nil {
				start = run.Window.WindowStart
			}
			phase := "rolled_back"
			if run.RollbackFailed {
				phase = "rollback_failed"
			}
			queueNotice(st, &Window{ID: uuid.NewString(), WindowStart: start, Nodes: run.RollbackNodes}, run.Release.Version, phase, now)
			st.Notices[len(st.Notices)-1].FailedNodes = append([]string(nil), run.RollbackFailedNodes...)
		}
		ns.LastJob = result.ID
		st.Nodes[nodeID] = ns
		version := ""
		if job.Release != nil {
			version = job.Release.Version
		}
		addEvent(st, now, nodeID, job.Operation, result.Status, version)
		st.Active = nil
		return nil
	})
}

// Check records a shared lease before any release network request, so cluster peers do not duplicate it.
func (s *Service) checkRelease(ctx context.Context) error {
	initial, errInitial := s.Status(ctx)
	if errInitial != nil {
		return errInitial
	}

	nowDB, errNow := s.store.CurrentDatabaseTime(ctx)
	if errNow != nil {
		return errNow
	}
	if initial.CheckRetryAt.After(nowDB) || initial.CheckLease.After(nowDB) || (initial.CheckRetryAt.IsZero() && !initial.LastCheck.IsZero() && nowDB.Sub(initial.LastCheck) < time.Duration(initial.Policy.IntervalSeconds)*time.Second) {
		return nil
	}
	run := false
	errLease := s.change(ctx, func(st *State, now time.Time) error {
		run = false
		if st.CheckRetryAt.After(now) || st.CheckLease.After(now) || (st.CheckRetryAt.IsZero() && !st.LastCheck.IsZero() && now.Sub(st.LastCheck) < time.Duration(st.Policy.IntervalSeconds)*time.Second) {
			return nil
		}
		st.CheckLease = now.Add(3 * time.Minute)
		run = true
		return nil
	})
	if errLease != nil || !run {
		return errLease
	}
	release, errFetch := s.FetchRelease(ctx)
	return s.change(ctx, func(st *State, now time.Time) error {
		st.LastCheck = now
		st.CheckLease = time.Time{}
		if errFetch != nil {
			st.CheckError = "release_check_failed"
			st.RateLimitReset = time.Time{}
			st.CheckRetryAt = now.Add(time.Duration(st.Policy.IntervalSeconds) * time.Second)
			var failure *ReleaseCheckError
			if errors.As(errFetch, &failure) {
				st.CheckError = failure.Code
				st.RateLimitReset = failure.ResetAt
				if failure.RetryAt.After(st.CheckRetryAt) {
					st.CheckRetryAt = failure.RetryAt
				}
				if now.Add(failure.RetryAfter).After(st.CheckRetryAt) {
					st.CheckRetryAt = now.Add(failure.RetryAfter)
				}
			}
			if st.CheckRetryAt.After(now.Add(6 * time.Hour)) {
				st.CheckRetryAt = now.Add(6 * time.Hour)
			}
			return nil
		}
		if !versionPattern.MatchString(release.Version) || !hashPattern.MatchString(release.SHA256) || !hashPattern.MatchString(release.ArchiveSHA256) || release.Size <= 0 {
			st.CheckError = "invalid_stable_release"
			st.CheckRetryAt = now.Add(time.Duration(st.Policy.IntervalSeconds) * time.Second)
			return nil
		}
		st.Latest = &release
		st.CheckError = ""
		st.CheckRetryAt = time.Time{}
		st.RateLimitReset = time.Time{}
		versions := []string{}
		for _, n := range st.Policy.Nodes {
			if node, ok := st.Nodes[n.ID]; ok && n.Required {
				versions = append(versions, node.Version)
			}
		}
		sort.Strings(versions)
		if len(versions) > 0 && versions[0] != release.Version && !PatchAllowed(versions[0], release.Version) {
			st.CheckError = "release_requires_review"
		}
		return nil
	})
}

func (s *Service) Check(ctx context.Context) error {
	if errNotice := s.deliverNotice(ctx); errNotice != nil {
		return errNotice
	}
	return s.checkRelease(ctx)
}

func (s *Service) Run(ctx context.Context) {
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		if errCheck := s.Check(ctx); errCheck != nil && ctx.Err() == nil {
			log.WithError(errCheck).Warn("fleet release check failed")
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
	}
}
