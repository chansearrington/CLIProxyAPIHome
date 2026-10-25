package auth

import (
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestModelErrorDisplayStatus(t *testing.T) {
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name        string
		state       *ModelState
		wantBlocked bool
	}{
		{
			name: "404 without retry",
			state: &ModelState{
				Status: StatusError, StatusMessage: "retired model",
				LastError: &Error{HTTPStatus: http.StatusNotFound, Message: "retired model"},
			},
		},
		{
			name: "499 unavailable without retry",
			state: &ModelState{
				Status: StatusError, StatusMessage: "request canceled", Unavailable: true,
				LastError: &Error{HTTPStatus: 499, Message: "request canceled"},
			},
		},
		{
			name: "future retry",
			state: &ModelState{
				Status: StatusError, StatusMessage: "temporarily unavailable", Unavailable: true,
				NextRetryAfter: now.Add(time.Minute),
				LastError:      &Error{HTTPStatus: http.StatusBadGateway, Message: "temporarily unavailable"},
			},
			wantBlocked: true,
		},
		{
			name: "quota exceeded",
			state: &ModelState{
				Status: StatusError, StatusMessage: "quota exceeded", Unavailable: true,
				NextRetryAfter: now.Add(time.Hour),
				LastError:      &Error{HTTPStatus: http.StatusTooManyRequests, Message: "quota exceeded"},
				Quota:          QuotaState{Exceeded: true, Scope: quotaScopeModel, NextRecoverAt: now.Add(time.Hour)},
			},
			wantBlocked: true,
		},
		{
			name: "expired retry",
			state: &ModelState{
				Status: StatusError, StatusMessage: "old failure", Unavailable: true,
				NextRetryAfter: now.Add(-time.Minute),
				LastError:      &Error{HTTPStatus: http.StatusBadGateway, Message: "old failure"},
			},
		},
		{
			name: "retry exactly now",
			state: &ModelState{
				Status: StatusError, StatusMessage: "old failure", Unavailable: true,
				NextRetryAfter: now,
				LastError:      &Error{HTTPStatus: http.StatusBadGateway, Message: "old failure"},
			},
		},
		{
			name: "future retry but available",
			state: &ModelState{
				Status: StatusError, StatusMessage: "non-blocking failure",
				NextRetryAfter: now.Add(time.Hour),
				LastError:      &Error{HTTPStatus: http.StatusNotFound, Message: "non-blocking failure"},
			},
		},
		{
			name: "disabled model",
			state: &ModelState{
				Status: StatusDisabled, StatusMessage: "model disabled",
				LastError: &Error{HTTPStatus: http.StatusForbidden, Message: "model disabled"},
			},
			wantBlocked: true,
		},
	}
	for _, tc := range cases {
		for _, path := range []string{"success", "refresh clear", "persisted adoption", "persisted refresh clear"} {
			t.Run(tc.name+"/"+path, func(t *testing.T) {
				state := tc.state.Clone()
				state.UpdatedAt = now.Add(-time.Hour)
				auth := &Auth{
					ID: "fake-status-account", Status: StatusError,
					StatusMessage: state.StatusMessage, LastError: cloneError(state.LastError),
					ModelStates: map[string]*ModelState{"old-model": state},
				}
				if blocked, _, _ := isAuthBlockedForModel(auth, "old-model", now); blocked != tc.wantBlocked {
					t.Fatalf("routing precondition: blocked = %v, want %v", blocked, tc.wantBlocked)
				}
				before := state.Clone()
				manager := NewManager(nil, nil, nil)
				switch path {
				case "success":
					manager.applyResultTransition(auth, Result{Success: true, Model: "healthy-model"}, "healthy-model", now, false)
				case "refresh clear":
					clearRefreshDispatchBlock(auth, now)
				case "persisted adoption", "persisted refresh clear":
					if path == "persisted refresh clear" {
						auth.RuntimeRefreshBlocked = true
						auth.LastError = &Error{Code: refreshTransientErrorCode, Message: refreshTransientErrorMsg}
						auth.StatusMessage = refreshTransientErrorMsg
					}
					manager.auths[auth.ID] = auth
					persisted := &Auth{ID: auth.ID, Status: StatusActive}
					var current bool
					auth, current = manager.adoptPersistedResultState(Result{AuthID: auth.ID, Model: "healthy-model"}, "healthy-model", persisted, now)
					if !current || auth == nil {
						t.Fatal("persisted state was not adopted")
					}
				}
				wantStatus, wantMessage := StatusActive, ""
				if tc.wantBlocked {
					wantStatus, wantMessage = StatusError, state.StatusMessage
				}
				if auth.Status != wantStatus || auth.StatusMessage != wantMessage {
					t.Errorf("account display = %q/%q, want %q/%q", auth.Status, auth.StatusMessage, wantStatus, wantMessage)
				}
				retained := auth.ModelStates["old-model"]
				if retained == nil || !reflect.DeepEqual(retained.LastError, before.LastError) || retained.Status != before.Status || retained.StatusMessage != before.StatusMessage || !retained.UpdatedAt.Equal(before.UpdatedAt) {
					t.Errorf("model diagnostics changed: got %#v, want %#v", retained, before)
				}
				if path == "success" || path == "persisted adoption" {
					if !reflect.DeepEqual(auth.LastError, before.LastError) {
						t.Error("display-only correction replaced account LastError")
					}
				}
				if blocked, _, _ := isAuthBlockedForModel(auth, "old-model", now); blocked != tc.wantBlocked {
					t.Errorf("routing changed: blocked = %v, want %v", blocked, tc.wantBlocked)
				}
			})
		}
	}
}

func TestModelErrorDisplayChoosesLatestBlockingState(t *testing.T) {
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	for _, path := range []string{"refresh clear", "persisted refresh clear"} {
		t.Run(path, func(t *testing.T) {
			blocking := &ModelState{
				Status: StatusError, StatusMessage: "blocked model", Unavailable: true,
				NextRetryAfter: now.Add(time.Hour), UpdatedAt: now.Add(-time.Hour),
				LastError: &Error{HTTPStatus: http.StatusBadGateway, Message: "blocked model"},
			}
			newerBlocking := blocking.Clone()
			newerBlocking.UpdatedAt = now.Add(-time.Minute)
			newerBlocking.StatusMessage = "newest blocked model"
			newerBlocking.LastError.Message = newerBlocking.StatusMessage
			stale := &ModelState{
				Status: StatusError, StatusMessage: "newer non-blocking failure", UpdatedAt: now,
				LastError: &Error{HTTPStatus: http.StatusNotFound, Message: "newer non-blocking failure"},
			}
			auth := &Auth{
				ID: "fake-mixed-account", Status: StatusError, LastError: cloneError(stale.LastError),
				ModelStates: map[string]*ModelState{"blocked": blocking, "newer-blocked": newerBlocking, "stale": stale, "nil": nil},
			}
			if path == "refresh clear" {
				clearRefreshDispatchBlock(auth, now)
			} else {
				manager := NewManager(nil, nil, nil)
				auth.RuntimeRefreshBlocked = true
				manager.auths[auth.ID] = auth
				auth, _ = manager.adoptPersistedResultState(Result{AuthID: auth.ID}, "healthy-model", &Auth{ID: auth.ID, Status: StatusActive}, now)
			}
			if auth.Status != StatusError || auth.StatusMessage != newerBlocking.StatusMessage || !reflect.DeepEqual(auth.LastError, newerBlocking.LastError) {
				t.Fatalf("display did not choose newest blocking model: %#v", auth)
			}
		})
	}
}

func TestModelErrorDisplayPreservesDisabledAccount(t *testing.T) {
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name     string
		disabled bool
		status   Status
	}{
		{name: "disabled flag and status", disabled: true, status: StatusDisabled},
		{name: "disabled flag", disabled: true, status: StatusError},
		{name: "disabled status", status: StatusDisabled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			auth := &Auth{
				ID: "fake-disabled-account", Disabled: tc.disabled, Status: tc.status,
				StatusMessage: "account disabled", LastError: &Error{HTTPStatus: http.StatusNotFound, Message: "retired model"},
				ModelStates: map[string]*ModelState{
					"old-model": {Status: StatusError, LastError: &Error{HTTPStatus: http.StatusNotFound, Message: "retired model"}},
				},
			}
			before := auth.Clone()
			manager := NewManager(nil, nil, nil)
			manager.applyResultTransition(auth, Result{Success: true, Model: "healthy-model"}, "healthy-model", now, false)
			if auth.Disabled != before.Disabled || auth.Status != before.Status || auth.StatusMessage != before.StatusMessage || !reflect.DeepEqual(auth.LastError, before.LastError) {
				t.Fatalf("late success changed disabled account status or diagnostics: %#v", auth)
			}
			if blocked, _, _ := isAuthBlockedForModel(auth, "healthy-model", now); !blocked {
				t.Fatal("late success unblocked a disabled account")
			}
		})
	}
}

func TestQuotaCooldownClearPreservesNonBlockingModelErrorBehavior(t *testing.T) {
	now := time.Date(2026, time.October, 25, 12, 0, 0, 0, time.UTC)
	stale := &ModelState{
		Status: StatusError, StatusMessage: "retired model", UpdatedAt: now.Add(-time.Hour),
		LastError: &Error{HTTPStatus: http.StatusNotFound, Message: "retired model"},
	}
	before := stale.Clone()
	auth := &Auth{
		ID: "fake-cooldown-account", Status: StatusError, StatusMessage: stale.StatusMessage,
		LastError: cloneError(stale.LastError), Unavailable: true, NextRetryAfter: now.Add(time.Hour),
		ModelStates: map[string]*ModelState{
			"stale": stale,
			"quota": {
				Status: StatusError, Unavailable: true, NextRetryAfter: now.Add(time.Hour),
				LastError: &Error{HTTPStatus: http.StatusTooManyRequests, Message: "quota exceeded"},
				Quota:     QuotaState{Exceeded: true, Scope: quotaScopeModel, NextRecoverAt: now.Add(time.Hour)},
			},
		},
	}
	transition := clearQuotaCooldownState(auth, []string{"quota"}, now, false)
	if !transition.changed() || !reflect.DeepEqual(transition.clearedModels, []string{"quota"}) {
		t.Fatalf("quota clear did not run: %#v", transition)
	}
	// Baseline behavior: aggregation clears availability, but the broad error
	// check preserves the old account diagnostics and error status.
	if auth.Unavailable || !auth.NextRetryAfter.IsZero() || auth.Quota.Exceeded {
		t.Fatalf("quota clear availability changed: %#v", auth)
	}
	if auth.Status != StatusError || auth.StatusMessage != before.StatusMessage || !reflect.DeepEqual(auth.LastError, before.LastError) {
		t.Fatalf("quota clear broad-error behavior changed: %#v", auth)
	}
	if !reflect.DeepEqual(auth.ModelStates["stale"], before) || !hasModelError(auth, now) {
		t.Fatal("quota clear changed the non-blocking error or the broad error check")
	}
}
