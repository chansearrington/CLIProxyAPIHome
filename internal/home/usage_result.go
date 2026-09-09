package home

import (
	"context"
	"net/http"
	"strings"

	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
	"github.com/tidwall/gjson"
)

// RecordUsagePayload applies downstream usage status to the scheduler auth state.
func (r *Runtime) RecordUsagePayload(ctx context.Context, payload string) {
	// Validate input data before converting it into runtime state.
	if r == nil || r.coreManager == nil {
		return
	}
	payload = strings.TrimSpace(payload)
	if payload == "" || !gjson.Valid(payload) {
		return
	}

	authIndex := strings.TrimSpace(gjson.Get(payload, "auth_index").String())
	if authIndex == "" {
		return
	}

	provider := strings.TrimSpace(gjson.Get(payload, "provider").String())
	model := strings.TrimSpace(gjson.Get(payload, "model").String())
	if coreauth.CanonicalModelID(model) == "" {
		return
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

	result := coreauth.NewUsageResultWithHeaders(authIndex, provider, model, statusCode, body, headers)
	result.AccessTokenSHA256 = strings.TrimSpace(gjson.Get(payload, "access_token_sha256").String())
	r.coreManager.MarkResult(ctx, result)
}

// parseResponseHeaders reads the optional "response_headers" object off a
// usage payload into an http.Header. The node marshals http.Header as a
// standard JSON object whose keys are canonical header names and whose
// values are JSON arrays (Go's encoding/json shape for map[string][]string),
// e.g. {"Anthropic-Ratelimit-Unified-Status":["rejected"]} -- never a bare
// string, even for a single value. Returns nil when the field is absent or
// not a JSON object, so callers fall back to body-only parsing.
func parseResponseHeaders(payload string) http.Header {
	node := gjson.Get(payload, "response_headers")
	if !node.Exists() || !node.IsObject() {
		return nil
	}
	headers := make(http.Header)
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
	if len(headers) == 0 {
		return nil
	}
	return headers
}
