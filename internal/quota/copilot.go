package quota

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPIHome/internal/cluster"
)

// copilotQuotaBuckets lists the GitHub Copilot quota buckets in display order.
// Unknown buckets are ignored rather than guessed at.
var copilotQuotaBuckets = []struct {
	key   string
	label string
}{
	{key: "premium_interactions", label: "Premium Requests"},
	{key: "chat", label: "Chat"},
	{key: "completions", label: "Completions"},
}

type copilotUserPayload struct {
	CopilotPlan       string                           `json:"copilot_plan"`
	QuotaResetDateUTC string                           `json:"quota_reset_date_utc"`
	QuotaResetDate    string                           `json:"quota_reset_date"`
	QuotaSnapshots    map[string]*copilotQuotaSnapshot `json:"quota_snapshots"`
}

type copilotQuotaSnapshot struct {
	Entitlement      flexFloat `json:"entitlement"`
	Remaining        flexFloat `json:"remaining"`
	PercentRemaining flexFloat `json:"percent_remaining"`
	Unlimited        bool      `json:"unlimited"`
}

// probeCopilot reads the Copilot seat quota that GitHub reports for the
// credential's GitHub OAuth token (the same data the Copilot editor plugins show).
func (c *Collector) probeCopilot(ctx context.Context, auth *coreauth.Auth) ([]cluster.QuotaWindow, *cluster.QuotaPlan, *probeError) {
	headers := http.Header{"Accept": []string{"application/json"}}
	payload, _, errRequest := c.probeRequest(ctx, auth, http.MethodGet, c.options.CopilotUserURL, nil, headers)
	if errRequest != nil {
		return nil, nil, errRequest
	}
	windows, plan, errParse := parseCopilotQuotaWindows(payload, c.options.Now().UTC())
	if errParse != nil || len(windows) == 0 {
		return nil, nil, &probeError{code: "UPSTREAM_RESPONSE_INVALID", message: "Copilot quota response did not contain usable quota buckets.", retryable: true}
	}
	return windows, plan, nil
}

func parseCopilotQuotaWindows(body []byte, observedAt time.Time) ([]cluster.QuotaWindow, *cluster.QuotaPlan, error) {
	var payload copilotUserPayload
	if errDecode := json.Unmarshal(body, &payload); errDecode != nil {
		return nil, nil, fmt.Errorf("decode copilot quota response: %w", errDecode)
	}
	resetAt := parseProviderTime(payload.QuotaResetDateUTC)
	if resetAt == nil {
		resetAt = parseCopilotResetDate(payload.QuotaResetDate)
	}
	windows := make([]cluster.QuotaWindow, 0, len(copilotQuotaBuckets))
	for priority, bucket := range copilotQuotaBuckets {
		snapshot := payload.QuotaSnapshots[bucket.key]
		if snapshot == nil {
			continue
		}
		window := cluster.QuotaWindow{
			ID: "copilot-" + quotaIDSlug(bucket.key), Label: quotaStringPtr(bucket.label), Scope: "account", Mode: "fixed",
			Status: "unknown", Unit: "requests", PeriodUnit: "month", PeriodValue: quotaFloatPtr(1), ResetAt: resetAt,
			Source: "active_probe", ObservedAt: observedAt, Priority: priority,
		}
		if snapshot.Unlimited {
			window.IsUnlimited = true
		} else {
			entitlement := flexFloatValue(snapshot.Entitlement)
			if entitlement == nil || *entitlement <= 0 {
				// A metered bucket without an allowance carries no quota to show.
				continue
			}
			window.Limit = entitlement
			if remaining := flexFloatValue(snapshot.Remaining); remaining != nil {
				window.Remaining = nonNegativeFloat(remaining)
			} else if percent := flexFloatValue(snapshot.PercentRemaining); percent != nil {
				ratio := math.Max(0, math.Min(1, *percent/100))
				window.RemainingRatio = &ratio
			}
		}
		normalizeWindowValues(&window)
		windows = append(windows, window)
	}
	return windows, copilotPlan(payload.CopilotPlan), nil
}

func parseCopilotResetDate(value string) *time.Time {
	parsed, errParse := time.Parse("2006-01-02", strings.TrimSpace(value))
	if errParse != nil {
		return nil
	}
	utc := parsed.UTC()
	return &utc
}

func copilotPlan(value string) *cluster.QuotaPlan {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return nil
	case "free":
		return &cluster.QuotaPlan{Name: "Free"}
	case "individual":
		return &cluster.QuotaPlan{Name: "Pro"}
	case "individual_pro":
		return &cluster.QuotaPlan{Name: "Pro+", Premium: true}
	case "business":
		return &cluster.QuotaPlan{Name: "Business"}
	case "enterprise":
		return &cluster.QuotaPlan{Name: "Enterprise"}
	default:
		return &cluster.QuotaPlan{Name: strings.TrimSpace(value)}
	}
}
