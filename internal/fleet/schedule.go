package fleet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
	_ "time/tzdata"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const noticeLead = time.Hour
const windowLength = time.Hour
const maxNotices = 20

func validateAnnouncementURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, errURL := url.Parse(raw)
	if errURL != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("announcement_url must be an HTTP(S) URL without user information or a fragment")
	}
	return nil
}

func MaskAnnouncementURL(raw string) string {
	u, errURL := url.Parse(raw)
	if errURL != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func importantNotice(phase string) bool {
	return phase == "done" || phase == "rollback" || phase == "rolled_back" || phase == "rollback_failed"
}

func nextWindow(earliest time.Time) time.Time { return nextWindowAt(earliest, "02:00") }

// On a DST gap, use the first real hour after the gap, never the earlier normalized hour.
func nextWindowAt(earliest time.Time, clock string) time.Time {
	if clock == "" {
		clock = "02:00"
	}
	parsed, _ := time.Parse("15:04", clock)         // Policy validation accepts only HH:MM.
	zone, _ := time.LoadLocation("America/Chicago") // Embedded tzdata.
	day := earliest.In(zone)
	for {
		start := time.Date(day.Year(), day.Month(), day.Day(), parsed.Hour(), parsed.Minute(), 0, 0, zone)
		if start.Hour() < parsed.Hour() {
			start = start.Add(time.Hour)
		}
		if !start.Before(earliest) {
			return start
		}
		day = day.AddDate(0, 0, 1)
	}
}

func windowTime(st *State, w *Window, earliest time.Time) time.Time {
	if w.Optional {
		return nextWindowAt(earliest, st.Policy.OptionalWindow)
	}
	return nextWindow(earliest)
}

func queueNotice(st *State, w *Window, version, phase string, now time.Time) {
	for _, n := range st.Notices {
		if n.WindowID == w.ID && n.Phase == phase {
			return
		}
	}
	st.Notices = append(st.Notices, Notice{ID: uuid.NewString(), WindowID: w.ID, CreatedAt: now, Announcement: Announcement{
		Phase: phase, TargetVersion: version, WindowStart: w.WindowStart, Nodes: append([]string(nil), w.Nodes...),
	}})
	ordinary := 0
	for _, n := range st.Notices {
		if !importantNotice(n.Phase) {
			ordinary++
		}
	}
	if ordinary > maxNotices {
		kept := st.Notices[:0]
		for _, n := range st.Notices {
			if !importantNotice(n.Phase) && ordinary > maxNotices {
				ordinary--
				continue
			}
			kept = append(kept, n)
		}
		st.Notices = kept
		addEvent(st, now, "", "announcement", "queue_capped", version)
	}
}

func scheduleWindow(st *State, now time.Time, version string, nodes []string, optional bool) *Window {
	w := &Window{ID: uuid.NewString(), Nodes: nodes, Optional: optional}
	w.WindowStart = windowTime(st, w, now.Add(noticeLead))
	queueNotice(st, w, version, "scheduled", now)
	addEvent(st, now, "", "schedule", "scheduled", version)
	return w
}

func windows(run *Rollout) []*Window {
	if run == nil {
		return nil
	}
	out := []*Window{}
	if run.Window != nil {
		out = append(out, run.Window)
	}
	for _, w := range run.OptionalWindows {
		if w != nil {
			out = append(out, w)
		}
	}
	return out
}

func findWindow(st *State, id string) *Window {
	for _, w := range windows(st.Rollout) {
		if w.ID == id {
			return w
		}
	}
	return nil
}

func removeWindowNotices(st *State, id string) {
	kept := st.Notices[:0]
	for _, n := range st.Notices {
		if n.WindowID != id || importantNotice(n.Phase) {
			kept = append(kept, n)
		}
	}
	st.Notices = kept
}

func closeWindow(st *State, w *Window, version, phase, reason string, now time.Time) {
	if w == nil || w.Closed {
		return
	}
	removeWindowNotices(st, w.ID)
	w.Closed = true
	if phase == "postponed" && w.AnnouncedAt.IsZero() {
		return
	}
	queueNotice(st, w, version, phase, now)
	st.Notices[len(st.Notices)-1].Reason = reason
}

func reschedule(st *State, w *Window, version string, now, start time.Time) {
	closeWindow(st, w, version, "postponed", "", now)
	w.ID = uuid.NewString()
	w.WindowStart = start
	w.AnnouncedAt = time.Time{}
	w.StartedAt = time.Time{}
	w.Reminder = false
	w.Closed = false
	w.DeliveryFailed = false
	queueNotice(st, w, version, "scheduled", now)
	addEvent(st, now, "", "schedule", "postponed", version)
}

func nodesAwake(st *State, w *Window, now time.Time) bool {
	for _, id := range w.Nodes {
		if st.Rollout != nil && st.Rollout.Window == w {
			if _, ok := st.Rollout.Completed[id]; ok {
				continue
			}
		}
		n, ok := st.Nodes[id]
		if !ok || now.Sub(n.SeenAt) > 5*time.Minute {
			return false
		}
	}
	return true
}

func pruneNotices(st *State, now time.Time) {
	kept := st.Notices[:0]
	for _, n := range st.Notices {
		if n.CreatedAt.IsZero() {
			n.CreatedAt = now
		}
		if importantNotice(n.Phase) || now.Sub(n.CreatedAt) <= 24*time.Hour {
			kept = append(kept, n)
		} else {
			addEvent(st, now, "", "announcement", "expired", n.TargetVersion)
		}
	}
	st.Notices = kept
	if len(st.Notices) == 0 {
		st.NoticeError = ""
		st.NoticeRetryAt = time.Time{}
	}
}

func queueRollback(st *State, now time.Time) {
	run := st.Rollout
	if run.RollbackStartedAt.IsZero() {
		run.RollbackStartedAt = now
	}
	start := now
	if run.Window != nil {
		start = run.Window.WindowStart
	}
	queueNotice(st, &Window{ID: "rollback-" + run.Release.Version, WindowStart: start, Nodes: run.RollbackNodes}, run.Release.Version, "rollback", now)
}

func voidUnstartedJob(st *State, now time.Time) bool {
	job := st.Active
	if job == nil || job.Operation != "upgrade" || job.Running || job.ExpiresAt.After(now) || st.Rollout == nil {
		return false
	}
	w := st.Rollout.Window
	if optional := st.Rollout.OptionalWindows[job.NodeID]; optional != nil {
		w = optional
	}
	if w == nil || (!w.Closed && now.Before(w.WindowStart.Add(windowLength))) {
		return false
	}
	ns := st.Nodes[job.NodeID]
	ns.LastJob = job.ID
	st.Nodes[job.NodeID] = ns
	addEvent(st, now, job.NodeID, "upgrade", "void_unstarted", st.Rollout.Release.Version)
	st.Active = nil
	return true
}

func maintainWindows(st *State, now time.Time) {
	pruneNotices(st, now)
	voidUnstartedJob(st, now)
	if st.Rollout != nil && st.Rollout.Phase == "rollback" && !st.Rollout.RollbackAnnounced && len(st.Rollout.RollbackNodes) > 0 {
		queueRollback(st, now)
	}
	if !st.Policy.Enabled || !st.Policy.PatchUpdates || st.Rollout == nil {
		return
	}
	run := st.Rollout
	if run.Phase != "upgrade" && run.Phase != "complete" {
		return
	}
	for _, w := range windows(run) {
		if w.Closed {
			continue
		}
		if st.Active != nil {
			continue
		} // Reconcile running work before changing its window.
		if w.Optional && st.Policy.OptionalWindow == "" {
			closeWindow(st, w, run.Release.Version, "postponed", "optional_window_unconfigured", now)
			continue
		}
		if w.Optional && len(w.Nodes) > 0 && now.Sub(st.Nodes[w.Nodes[0]].SeenAt) > 24*time.Hour {
			closeWindow(st, w, run.Release.Version, "postponed", "asleep", now)
			continue
		}
		if !now.Before(w.WindowStart.Add(windowLength)) || (!now.Before(w.WindowStart) && !nodesAwake(st, w, now)) {
			reschedule(st, w, run.Release.Version, now, windowTime(st, w, now.Add(noticeLead)))
			continue
		}
		if w.AnnouncedAt.IsZero() {
			queueNotice(st, w, run.Release.Version, "scheduled", now)
			continue
		}
		if !w.Reminder && !now.Before(w.WindowStart.Add(-noticeLead)) {
			queueNotice(st, w, run.Release.Version, "reminder", now)
		}
		if !now.Before(w.WindowStart) && w.StartedAt.IsZero() {
			queueNotice(st, w, run.Release.Version, "started", now)
		}
	}
}

func windowReady(st *State, w *Window, now time.Time) bool {
	return w != nil && !w.Closed && !w.AnnouncedAt.IsZero() && !w.StartedAt.IsZero() &&
		!now.Before(w.WindowStart) && now.Before(w.WindowStart.Add(windowLength)) &&
		st.Policy.AnnouncementURL != "" && st.NoticeError == "" && len(st.Notices) == 0
}

func closeWindows(st *State, reason string, now time.Time) {
	if st.Rollout == nil {
		return
	}
	for _, w := range windows(st.Rollout) {
		closeWindow(st, w, st.Rollout.Release.Version, "postponed", reason, now)
	}
}

func startRollout(st *State, release Release, approved bool, now time.Time) {
	if st.Rollout != nil {
		closeWindows(st, "superseded", now)
		delete(st.PostponedTargets, st.Rollout.Release.Version)
	}
	ids := []string{}
	for _, n := range st.Policy.Nodes {
		if n.Required {
			ids = append(ids, n.ID)
		}
	}
	st.Rollout = &Rollout{Release: release, Approved: approved, Completed: map[string]Inventory{}, Phase: "upgrade", OptionalWindows: map[string]*Window{}}
	st.Rollout.Window = scheduleWindow(st, now, release.Version, ids, false)
}

func maybeSchedule(st *State, now time.Time) {
	if !st.Policy.Enabled || !st.Policy.PatchUpdates || st.Latest == nil || st.Active != nil {
		return
	}
	run := st.Rollout
	if run != nil && (run.Phase == "upgrade" || run.Phase == "rollback" || run.Phase == "held" || run.Release.Version == st.Latest.Version) {
		return
	}
	baseline := ""
	for _, n := range st.Policy.Nodes {
		if !n.Required {
			continue
		}
		ns, ok := st.Nodes[n.ID]
		if !ok || now.Sub(ns.SeenAt) > 5*time.Minute {
			return
		}
		if baseline == "" {
			baseline = ns.Version
		}
		if baseline != ns.Version {
			return
		}
	}
	if PatchAllowed(baseline, st.Latest.Version) {
		startRollout(st, *st.Latest, false, now)
	}
}

func auditEvent(st *State, force bool, caller string) {
	if len(st.History) == 0 {
		return
	}
	last := &st.History[len(st.History)-1]
	last.Force = force
	last.Caller = caller
}

// Postpone keeps the exact target and moves to the next local nightly window, including DST changes.
func (s *Service) Postpone(ctx context.Context, version, nodeID string, force bool, actor ...string) error {
	return s.change(ctx, func(st *State, now time.Time) error {
		if st.Active != nil || st.Rollout == nil || st.Rollout.Release.Version != version {
			return fmt.Errorf("no pending window for target")
		}
		w := st.Rollout.Window
		if nodeID != "" {
			w = st.Rollout.OptionalWindows[nodeID]
		}
		if w == nil || w.Closed || !w.StartedAt.IsZero() {
			return fmt.Errorf("window is not pending")
		}
		if st.NoticeLease.After(now) {
			return fmt.Errorf("announcement is being delivered; retry")
		}
		if st.PostponedTargets[version] && !force {
			return fmt.Errorf("target has already been postponed; operator force required")
		}
		if st.PostponedTargets == nil {
			st.PostponedTargets = map[string]bool{}
		}
		st.PostponedTargets[version] = true
		start := windowTime(st, w, w.WindowStart.Add(time.Second))
		if start.Before(now.Add(noticeLead)) {
			start = windowTime(st, w, now.Add(noticeLead))
		}
		reschedule(st, w, version, now, start)
		caller := ""
		if len(actor) > 0 {
			caller = actor[0]
		}
		auditEvent(st, force, caller)
		return nil
	})
}

type announcementError struct{ Status int }

func (e *announcementError) Error() string { return "announcement_delivery_failed" }

func postAnnouncement(ctx context.Context, endpoint string, notice Notice) error {
	data, errEncode := json.Marshal(notice.Announcement)
	if errEncode != nil {
		return &announcementError{}
	}
	req, errRequest := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if errRequest != nil {
		return &announcementError{}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", notice.ID)
	if token := os.Getenv("FLEET_ANNOUNCE_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, errResponse := client.Do(req)
	if errResponse != nil {
		return &announcementError{}
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			log.Debug("fleet announcement response close failed")
		}
	}()
	_, errRead := io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if errRead != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &announcementError{Status: resp.StatusCode}
	}
	return nil
}

// Delivery is outside CAS retries. A crash can redeliver the same idempotency key.
func (s *Service) deliverNotice(ctx context.Context) error {
	var notice Notice
	endpoint := ""
	errClaim := s.change(ctx, func(st *State, now time.Time) error {
		notice = Notice{}
		maintainWindows(st, now)
		if len(st.Notices) == 0 || st.NoticeLease.After(now) || st.NoticeRetryAt.After(now) {
			return nil
		}
		if st.Policy.AnnouncementURL == "" {
			st.NoticeError = "announcement_hook_not_configured"
			return nil
		}
		notice = st.Notices[0]
		endpoint = st.Policy.AnnouncementURL
		st.NoticeLease = now.Add(time.Minute)
		return nil
	})
	if errClaim != nil || notice.ID == "" {
		return errClaim
	}
	errDeliver := s.Announce(ctx, endpoint, notice)
	status := 0
	var failure *announcementError
	if errors.As(errDeliver, &failure) {
		status = failure.Status
	}
	if errDeliver != nil {
		log.WithFields(log.Fields{"phase": notice.Phase, "http_status": status}).Warn("fleet announcement delivery failed")
	}
	return s.change(ctx, func(st *State, now time.Time) error {
		if len(st.Notices) == 0 || st.Notices[0].ID != notice.ID {
			return nil
		}
		st.NoticeLease = time.Time{}
		w := findWindow(st, notice.WindowID)
		if errDeliver != nil {
			st.NoticeError = "announcement_delivery_failed"
			st.Notices[0].Attempts++
			delay := 5 * time.Minute
			if importantNotice(notice.Phase) {
				for attempt := 1; attempt < st.Notices[0].Attempts && delay < time.Hour; attempt++ {
					delay *= 2
				}
				if delay > time.Hour {
					delay = time.Hour
				}
			}
			st.NoticeRetryAt = now.Add(delay)
			permanent := status >= 400 && status < 500 && status != 408 && status != 429
			if permanent {
				st.Notices = st.Notices[1:]
				st.NoticeRetryAt = time.Time{}
				addEvent(st, now, "", "announcement", "rejected_"+notice.Phase, notice.TargetVersion)
			}
			if w != nil && !w.Closed && (notice.Phase == "scheduled" || notice.Phase == "reminder" || notice.Phase == "started") {
				if !w.DeliveryFailed {
					start := windowTime(st, w, now.Add(noticeLead))
					if !start.After(w.WindowStart) {
						start = windowTime(st, w, w.WindowStart.Add(time.Second))
					}
					reschedule(st, w, notice.TargetVersion, now, start)
					w.DeliveryFailed = true
				}
			}
			return nil
		}
		st.Notices = st.Notices[1:]
		st.NoticeError = ""
		st.NoticeRetryAt = time.Time{}
		if notice.Phase == "rollback" && st.Rollout != nil && st.Rollout.Phase == "rollback" && st.Rollout.Release.Version == notice.TargetVersion {
			st.Rollout.RollbackAnnounced = true
		}
		if w != nil && !w.Closed {
			switch notice.Phase {
			case "scheduled":
				if now.Add(noticeLead).After(w.WindowStart) {
					reschedule(st, w, notice.TargetVersion, now, windowTime(st, w, now.Add(noticeLead)))
				} else {
					w.AnnouncedAt = now
					w.DeliveryFailed = false
				}
			case "reminder":
				w.Reminder = true
			case "started":
				w.StartedAt = now
			}
		}
		addEvent(st, now, "", "announcement", notice.Phase, notice.TargetVersion)
		return nil
	})
}
