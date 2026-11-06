package auth

import (
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestUsageResultKeepsStructuredCodeAndClaudeHeaders(t *testing.T) {
	reset := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	headers := claudeHeaders(map[string]string{
		"Anthropic-Ratelimit-Unified-5h-Status": "rejected",
		"Anthropic-Ratelimit-Unified-5h-Reset":  strconv.FormatInt(reset.Unix(), 10),
	})
	result := NewUsageResultWithCodeAndHeaders("fixture-auth", "claude", "fixture-model", http.StatusTooManyRequests, "limited", ErrorCodeStreamTruncated, headers)
	if result.Error == nil || result.Error.Code != ErrorCodeStreamTruncated || !isStreamTruncatedResultError(result.Error) {
		t.Fatal("lost the structured downstream classification")
	}
	if !result.CredentialScope || result.ResetAt == nil || !result.ResetAt.Equal(reset) {
		t.Fatal("lost the Claude reset scope while retaining the failure classification")
	}
}
