package home

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
	"github.com/tidwall/gjson"
)

// usageFeedbackMaxClockSkew bounds how far a CPA completion may lead Home.
const usageFeedbackMaxClockSkew = 5 * time.Minute

// RecordUsagePayload counts a usage event and applies its scheduler feedback.
func (r *Runtime) RecordUsagePayload(ctx context.Context, payload string) error {
	return r.recordUsagePayload(ctx, payload, false)
}

// ApplyUsagePayloadFeedback retries scheduler feedback for persisted usage.
// Execution identity lets MarkResult apply any missing request statistics once;
// legacy replays without an execution ID continue to suppress request statistics.
func (r *Runtime) ApplyUsagePayloadFeedback(ctx context.Context, payload string) error {
	return r.recordUsagePayload(ctx, payload, true)
}

func (r *Runtime) recordUsagePayload(ctx context.Context, payload string, skipRequestStats bool) error {
	// Validate input data before converting it into runtime state.
	if r == nil || r.coreManager == nil {
		return nil
	}
	payload = strings.TrimSpace(payload)
	if payload == "" || !gjson.Valid(payload) {
		return fmt.Errorf("invalid usage json")
	}

	authIndex := strings.TrimSpace(gjson.Get(payload, "auth_index").String())
	if authIndex == "" {
		return nil
	}

	provider := strings.TrimSpace(gjson.Get(payload, "provider").String())
	model := strings.TrimSpace(gjson.Get(payload, "model").String())
	if coreauth.CanonicalModelID(model) == "" {
		return nil
	}

	statusCode := int(gjson.Get(payload, "fail.status_code").Int())
	if statusCode <= 0 {
		if gjson.Get(payload, "failed").Bool() {
			statusCode = 500
		} else {
			statusCode = 200
		}
	}
	body := gjson.Get(payload, "fail.body").String()
	headers := parseResponseHeaders(payload)
	failCode := strings.TrimSpace(gjson.Get(payload, "fail.code").String())

	observedAt, valid := usagePayloadObservedAt(payload, time.Now())
	if !valid {
		// Acknowledge malformed timing without applying scheduler feedback, rather
		// than poisoning the fence or turning timing errors into ingestion retries.
		return nil
	}
	// Charge before this result changes availability. Replayed persistence rows
	// must also reach accounting: an earlier delivery may have failed after the
	// row committed but before feedback ran. The weighted selector deduplicates
	// by execution identity rather than the database's inserted flag.
	if errAccount := r.accountReusedUsage(ctx, payload); errAccount != nil {
		return errAccount
	}
	result := coreauth.NewUsageResultWithCodeAndHeaders(authIndex, provider, model, statusCode, body, failCode, headers)
	result.AccessTokenSHA256 = strings.TrimSpace(gjson.Get(payload, "access_token_sha256").String())
	result.ObservedAt = observedAt
	result.RequestStatsEventID = strings.TrimSpace(gjson.Get(payload, "execution_id").String())
	result.SkipRequestStats = skipRequestStats
	errMark := r.coreManager.MarkResult(ctx, result)
	if errors.Is(errMark, coreauth.ErrAuthNotFound) || errors.Is(errMark, coreauth.ErrFullAuthNotFound) {
		return nil
	}
	if !errors.Is(errMark, coreauth.ErrAuthNotLoaded) {
		if errMark != nil {
			return errMark
		}
		return r.recordAntigravityTOSPause(ctx, result, skipRequestStats)
	}
	// The credential can exist in the database before this Home has applied
	// the cluster event that introduces it. Load the authoritative record once;
	// a credential that no longer exists keeps the delivery acknowledged.
	if r.clusterAdapter == nil || !r.clusterAdapter.Enabled() {
		return nil
	}
	auth, errLoad := r.clusterAdapter.GetFullAuth(ctx, authIndex)
	if errors.Is(errLoad, coreauth.ErrFullAuthNotFound) || (errLoad == nil && auth == nil) {
		return nil
	}
	if errLoad != nil {
		return errLoad
	}
	r.applyCoreAuthAddOrUpdate(coreauth.WithSkipPersist(ctx), auth)
	if errAccount := r.accountReusedUsage(ctx, payload); errAccount != nil {
		return errAccount
	}
	errRetry := r.coreManager.MarkResult(ctx, result)
	if errors.Is(errRetry, coreauth.ErrAuthNotFound) || errors.Is(errRetry, coreauth.ErrFullAuthNotFound) {
		return nil
	}
	if errRetry != nil {
		return errRetry
	}
	return r.recordAntigravityTOSPause(ctx, result, skipRequestStats)
}

// usagePayloadObservedAt validates the execution completion time. Accepted times
// are never clamped to delivery time, so their identity stays stable on redelivery.
// Legacy payloads without a timestamp remain untimed.
func usagePayloadObservedAt(payload string, now time.Time) (time.Time, bool) {
	raw := strings.TrimSpace(gjson.Get(payload, "timestamp").String())
	if raw == "" {
		return time.Time{}, true
	}
	started, errParse := time.Parse(time.RFC3339Nano, raw)
	if errParse != nil {
		return time.Time{}, false
	}
	var latencyMS int64
	if latency := gjson.Get(payload, "latency_ms"); latency.Exists() {
		var errLatency error
		latencyMS, errLatency = strconv.ParseInt(latency.String(), 10, 64)
		if errLatency != nil || latencyMS < 0 || latencyMS > int64(1<<63-1)/int64(time.Millisecond) {
			return time.Time{}, false
		}
	}
	completed := started.Add(time.Duration(latencyMS) * time.Millisecond).UTC()
	if completed.After(now.Add(usageFeedbackMaxClockSkew)) {
		return time.Time{}, false
	}
	return completed, true
}

// parseResponseHeaders reads a usage payload's header fields into a single
// merged http.Header, so callers see Claude/codex rate-limit hints
// regardless of which of the two wire shapes actually survived sanitizing.
//
// "response_headers" is the node's raw http.Header marshal -- keys are
// header names and values are JSON arrays (Go's encoding/json shape for
// map[string][]string), e.g.
// {"Anthropic-Ratelimit-Unified-Status":["rejected"]} -- never a bare
// string, even for a single value. It is correct to read if it is ever
// present, but internal/cluster/quota_ingestion.go's
// sanitizeUsageQuotaHeaders unconditionally deletes this field from every
// payload before RecordUsagePayload ever sees it (it strips credential
// material) -- so in production this branch is normally empty and the real
// data arrives via quota_headers below.
//
// "quota_headers" is what sanitizeUsageQuotaHeaders re-attaches for a
// provider it recognizes (today: codex, claude): a FLAT map[string]string
// of that provider's allowlisted headers (see the `filtered` map in
// sanitizeUsageQuotaHeaders) -- a different shape from response_headers, so
// it is parsed separately below rather than through the array-unwrapping
// path.
//
// The two are merged; response_headers wins on a key collision (it is a
// stronger signal -- an unsanitized full header set -- on the rare path
// where both are present). Returns nil when neither field is present or
// usable, so callers fall back to body-only parsing.
func parseResponseHeaders(payload string) http.Header {
	headers := make(http.Header)
	parseResponseHeadersArrayShape(payload, headers)
	parseResponseHeadersFlatShape(payload, headers)
	if len(headers) == 0 {
		return nil
	}
	return headers
}

// parseResponseHeadersArrayShape reads "response_headers" (JSON-array
// values) into headers, in place.
func parseResponseHeadersArrayShape(payload string, headers http.Header) {
	node := gjson.Get(payload, "response_headers")
	if !node.Exists() || !node.IsObject() {
		return
	}
	node.ForEach(func(key, value gjson.Result) bool {
		name := strings.TrimSpace(key.String())
		if name == "" {
			return true
		}
		if value.IsArray() {
			for _, item := range value.Array() {
				v := item.String()
				if v != "" {
					headers.Add(name, v)
				}
			}
			return true
		}
		// Defensive fallback: accept a bare string too, in case a
		// non-standard producer ever sends a single unwrapped value.
		v := strings.TrimSpace(value.String())
		if v != "" {
			headers.Add(name, v)
		}
		return true
	})
}

// parseResponseHeadersFlatShape reads "quota_headers" (flat string values
// -- see sanitizeUsageQuotaHeaders' `filtered` map) into headers, in place.
// Skips any key response_headers already populated, so that field keeps
// precedence on a collision.
func parseResponseHeadersFlatShape(payload string, headers http.Header) {
	node := gjson.Get(payload, "quota_headers")
	if !node.Exists() || !node.IsObject() {
		return
	}
	node.ForEach(func(key, value gjson.Result) bool {
		name := strings.TrimSpace(key.String())
		if name == "" {
			return true
		}
		if len(headers.Values(name)) > 0 {
			return true
		}
		v := strings.TrimSpace(value.String())
		if v != "" {
			headers.Add(name, v)
		}
		return true
	})
}
