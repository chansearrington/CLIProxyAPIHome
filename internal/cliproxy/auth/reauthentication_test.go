package auth

import (
	"testing"
	"time"
)

func TestReauthenticationReenablesWithoutClearingQuota(t *testing.T) {
	now := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	for _, scope := range []string{"none", "credential", "model"} {
		t.Run(scope, func(t *testing.T) {
			auth := &Auth{Disabled: true, Status: StatusDisabled, Unavailable: true, StatusMessage: "unauthorized",
				LastError: &Error{Code: "unauthorized", HTTPStatus: 401}, LastRefreshError: &Error{Code: "authentication_error", HTTPStatus: 401}}
			quota := QuotaState{Exceeded: true, NextRecoverAt: now.Add(time.Hour)}
			if scope == "credential" {
				auth.Quota = quota
			}
			if scope == "model" {
				auth.ModelStates = map[string]*ModelState{"limited": {Status: StatusError, Unavailable: true, NextRetryAfter: quota.NextRecoverAt, Quota: quota, LastError: &Error{HTTPStatus: 429}}}
			}
			ApplyReauthenticationSuccessState(auth, now)
			if auth.Disabled || auth.LastRefreshError != nil || auth.RuntimeRefreshBlocked {
				t.Fatal("authentication block remained")
			}
			if scope == "none" && (auth.Unavailable || auth.Status != StatusActive || auth.LastError != nil) {
				t.Fatal("account did not recover")
			}
			if scope == "credential" && (!auth.Quota.Exceeded || auth.Quota.NextRecoverAt != quota.NextRecoverAt || !auth.Unavailable) {
				t.Fatal("credential quota was cleared")
			}
			if scope == "model" && (!auth.ModelStates["limited"].Quota.Exceeded || auth.ModelStates["limited"].NextRetryAfter != quota.NextRecoverAt) {
				t.Fatal("model quota was cleared")
			}
		})
	}
}
