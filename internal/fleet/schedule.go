package fleet

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
	_ "time/tzdata"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

const noticeLead = time.Hour
const windowLength = time.Hour

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

// On the spring DST gap, 02:00 does not exist; use 03:00, not Go's normalized 01:00.
func nextWindow(earliest time.Time) time.Time {
	zone, _ := time.LoadLocation("America/Chicago") // Embedded tzdata keeps this independent of the host.
	day := earliest.In(zone)
	for {
		start := time.Date(day.Year(), day.Month(), day.Day(), 2, 0, 0, 0, zone)
		if start.Hour() < 2 {
			start = start.Add(time.Hour)
		}
		if !start.Before(earliest) {
			return start
		}
		day = day.AddDate(0, 0, 1)
	}
}

func queueNotice(st *State, w *Window, version, phase string) {
	for _, n := range st.Notices {
		if n.WindowID == w.ID && n.Phase == phase {
			return
		}
	}
	st.Notices = append(st.Notices, Notice{ID: uuid.NewString(), WindowID: w.ID, Announcement: Announcement{
		Phase: phase, TargetVersion: version, WindowStart: w.WindowStart, Nodes: append([]string(nil), w.Nodes...),
	}})
}

func scheduleWindow(st *State, now time.Time, version string, nodes []string) *Window {
	w := &Window{ID: uuid.NewString(), WindowStart: nextWindow(now.Add(noticeLead)), Nodes: nodes}
	queueNotice(st, w, version, "scheduled")
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
		if n.WindowID != id {
			kept = append(kept, n)
		}
	}
	st.Notices = kept
}

func reschedule(st *State, w *Window, version string, now, start time.Time) {
	removeWindowNotices(st, w.ID)
	queueNotice(st, w, version, "postponed")
	w.ID = uuid.NewString()
	w.WindowStart = start
	w.AnnouncedAt = time.Time{}
	w.StartedAt = time.Time{}
	w.Reminder = false
	queueNotice(st, w, version, "scheduled")
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

func maintainWindows(st *State, now time.Time) {
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
		// A running task is always reconciled; never replace its announcement mid-operation.
		if st.Active != nil {
			continue
		}
		if !now.Before(w.WindowStart.Add(windowLength)) || (!now.Before(w.WindowStart) && !nodesAwake(st, w, now)) {
			reschedule(st, w, run.Release.Version, now, nextWindow(now.Add(noticeLead)))
			continue
		}
		if w.AnnouncedAt.IsZero() {
			continue
		}
		if !w.Reminder && !now.Before(w.WindowStart.Add(-noticeLead)) {
			queueNotice(st, w, run.Release.Version, "reminder")
		}
		if !now.Before(w.WindowStart) && w.StartedAt.IsZero() {
			queueNotice(st, w, run.Release.Version, "started")
		}
	}
}

func windowReady(st *State, w *Window, now time.Time) bool {
	return w != nil && !w.Closed && !w.AnnouncedAt.IsZero() && !w.StartedAt.IsZero() &&
		!now.Before(w.WindowStart) && now.Before(w.WindowStart.Add(windowLength)) &&
		st.Policy.AnnouncementURL != "" && st.NoticeError == "" && len(st.Notices) == 0
}

func closeWindow(st *State, w *Window, version, phase string) {
	if w == nil || w.Closed {
		return
	}
	w.Closed = true
	queueNotice(st, w, version, phase)
}

func startRollout(st *State, release Release, approved bool, now time.Time) {
	for _, w := range windows(st.Rollout) {
		if !w.Closed {
			removeWindowNotices(st, w.ID)
			closeWindow(st, w, st.Rollout.Release.Version, "postponed")
		}
	}
	nodes := []string{}
	for _, n := range st.Policy.Nodes {
		if n.Required {
			nodes = append(nodes, n.ID)
		}
	}
	st.Rollout = &Rollout{Release: release, Approved: approved, Completed: map[string]Inventory{}, Phase: "upgrade", OptionalWindows: map[string]*Window{}}
	st.Rollout.Window = scheduleWindow(st, now, release.Version, nodes)
}

func maybeSchedule(st *State, now time.Time) {
	if !st.Policy.Enabled || !st.Policy.PatchUpdates || st.Latest == nil || st.Active != nil {
		return
	}
	run := st.Rollout
	if run != nil && (run.Phase == "upgrade" || run.Phase == "rollback" || run.Release.Version == st.Latest.Version) {
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

// Postpone keeps the exact target and shifts a pending window by 24 hours. Force only lifts the once-per-target limit.
func (s *Service) Postpone(ctx context.Context, version, nodeID string, force bool) error {
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
		start := w.WindowStart.Add(24 * time.Hour)
		if start.Before(now.Add(noticeLead)) {
			start = nextWindow(now.Add(noticeLead))
		}
		reschedule(st, w, version, now, start)
		return nil
	})
}

func postAnnouncement(ctx context.Context, endpoint string, notice Notice) error {
	data, errEncode := json.Marshal(notice.Announcement)
	if errEncode != nil {
		return errEncode
	}
	req, errRequest := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if errRequest != nil {
		return fmt.Errorf("announcement_request_failed")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", notice.ID)
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, errResponse := client.Do(req)
	if errResponse != nil {
		return fmt.Errorf("announcement_delivery_failed")
	}
	defer func() {
		if errClose := resp.Body.Close(); errClose != nil {
			log.Debug("fleet announcement response close failed")
		}
	}()
	_, errRead := io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if errRead != nil {
		return fmt.Errorf("announcement_response_failed")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("announcement_http_%d", resp.StatusCode)
	}
	return nil
}

// Delivery is outside CAS retries. A receipt gates dispatch; a crash can redeliver the same idempotency key.
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
	return s.change(ctx, func(st *State, now time.Time) error {
		if len(st.Notices) == 0 || st.Notices[0].ID != notice.ID {
			return nil
		}
		st.NoticeLease = time.Time{}
		w := findWindow(st, notice.WindowID)
		if errDeliver != nil {
			st.NoticeError = "announcement_delivery_failed"
			st.NoticeRetryAt = now.Add(5 * time.Minute)
			// Failed gating notices require a fresh next-night announcement, never an unannounced late start.
			if w != nil && !w.Closed && (notice.Phase == "scheduled" || notice.Phase == "reminder" || notice.Phase == "started") {
				start := nextWindow(now.Add(noticeLead))
				if !start.After(w.WindowStart) {
					start = nextWindow(w.WindowStart.Add(time.Second))
				}
				reschedule(st, w, notice.TargetVersion, now, start)
			}
			return nil
		}
		st.Notices = st.Notices[1:]
		st.NoticeError = ""
		st.NoticeRetryAt = time.Time{}
		if w != nil && !w.Closed {
			switch notice.Phase {
			case "scheduled":
				if now.Add(noticeLead).After(w.WindowStart) {
					reschedule(st, w, notice.TargetVersion, now, nextWindow(now.Add(noticeLead)))
				} else {
					w.AnnouncedAt = now
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
