package management

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPIHome/internal/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPIHome/internal/cluster"
	"github.com/router-for-me/CLIProxyAPIHome/internal/config"
	"github.com/router-for-me/CLIProxyAPIHome/internal/home"
	"gorm.io/gorm"
)

func TestPatchAuthFileFieldsPreservesInterleavedCurrentState(t *testing.T) {
	for _, writer := range []string{"refresh", "reconnect", "disable", "quota", "operator"} {
		for _, mixed := range []bool{false, true} {
			name := writer + "/priority-only"
			if mixed {
				name = writer + "/mixed-fields-and-policy"
			}
			t.Run(name, func(t *testing.T) {
				handler, engine, db, closeRepo := newConcurrencyManagementTestServer(t)
				defer closeRepo()
				ctx := context.Background()
				const id = "11111111-1111-4111-8111-111111111111"
				auth := &coreauth.Auth{
					ID: id, Index: id, Provider: "claude", Status: coreauth.StatusActive,
					Attributes: map[string]string{"priority": "3", "note": "original"},
					Metadata: map[string]any{"type": "claude", "email": "fixture@example.com", "priority": 3,
						"access_token": "fake-old-access", "refresh_token": "fake-old-refresh", "note": "original",
						"nested": map[string]any{"keep": "original"}, "headers": map[string]any{"X-Keep": "original"}},
				}
				if writer == "refresh" {
					// This provider uses the injectable transport; the same controller finalizes all providers.
					auth.Provider = "antigravity"
					auth.Metadata["type"] = "antigravity"
				}
				if _, errUpsert := handler.repo.UpsertAuth(ctx, auth, "fixture"); errUpsert != nil {
					t.Fatal(errUpsert)
				}
				engine.PATCH("/auth-files/status", handler.PatchAuthFileStatus)
				patch := `{"priority":17}`
				if mixed {
					patch = `{"priority":17,"note":"requested","nested.edit":"requested","headers":{"X-Edit":"requested"}}`
				}
				var current *coreauth.Auth
				var currentVersion int64
				interleaved := false
				// Intercept the completed selector query: its snapshot is stale before the PATCH transaction starts.
				const hook = "test:priority-after-selector-read"
				if errRegister := db.Callback().Query().After("gorm:query").Register(hook, func(tx *gorm.DB) {
					if _, ok := tx.Statement.Dest.(*[]cluster.AuthRecord); !ok || interleaved {
						return
					}
					interleaved = true
					switch writer {
					case "refresh":
						refreshAtomicPriorityFixture(t, handler.repo, db, auth)
					case "reconnect":
						session, errSession := cluster.NewOAuthSessionRecord("claude", "reconnect", map[string]any{
							"reconnect_credential_id": id, "reconnect_email": "fixture@example.com"}, time.Now())
						if errSession != nil {
							t.Fatal(errSession)
						}
						if errStore := handler.repo.UpsertOAuthSession(ctx, session); errStore != nil {
							t.Fatal(errStore)
						}
						fresh := &coreauth.Auth{Provider: "claude", Metadata: map[string]any{
							"email": "fixture@example.com", "access_token": "fake-reconnected-access", "refresh_token": "fake-reconnected-refresh",
							"expired": time.Now().Add(time.Hour).Format(time.RFC3339)}}
						if errComplete := handler.repo.CompleteOAuthSessionWithAuths(ctx, session.State, []*coreauth.Auth{fresh}); errComplete != nil {
							t.Fatal(errComplete)
						}
					case "disable":
						response := httptest.NewRecorder()
						engine.ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/auth-files/status", strings.NewReader(`{"name":"`+id+`","disabled":true}`)))
						if response.Code != http.StatusOK {
							t.Fatalf("disable status = %d", response.Code)
						}
					case "operator":
						response := httptest.NewRecorder()
						engine.ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/auth-files/fields", strings.NewReader(`{"id":"`+id+`","priority":31,"note":"newer","nested.keep":"newer","headers":{"X-Keep":"newer"}}`)))
						if response.Code != http.StatusOK {
							t.Fatalf("operator status = %d", response.Code)
						}
					default:
						// Quota collection itself is outside this test; persist its cooldown through the real state mutator.
						_, _, _, errMutate := handler.repo.MutateAuth(ctx, id, writer, func(latest *coreauth.Auth) bool {
							latest.Unavailable = true
							latest.Status = coreauth.StatusError
							latest.StatusMessage = "quota cooldown"
							latest.NextRetryAfter = time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
							latest.LastError = &coreauth.Error{Code: "quota_exceeded", HTTPStatus: 429}
							latest.ModelStates = map[string]*coreauth.ModelState{"test-model": {Unavailable: true, NextRetryAfter: latest.NextRetryAfter}}
							return true
						})
						if errMutate != nil {
							t.Fatal(errMutate)
						}
					}
					var record *cluster.AuthRecord
					var errGet error
					current, record, errGet = handler.repo.GetAuth(ctx, id)
					if errGet != nil {
						t.Fatal(errGet)
					}
					currentVersion = record.Version
				}); errRegister != nil {
					t.Fatal(errRegister)
				}
				t.Cleanup(func() {
					if errRemove := db.Callback().Query().Remove(hook); errRemove != nil {
						t.Error(errRemove)
					}
				})
				body := `{"id":"` + id + `",` + patch[1:]
				if mixed {
					body = strings.TrimSuffix(body, "}") + `,"max_in_flight":2,"version":0}`
				}
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/auth-files/fields", strings.NewReader(body)))
				if response.Code != http.StatusOK || !interleaved {
					t.Fatalf("PATCH status = %d, interleaved = %t", response.Code, interleaved)
				}
				after, record, errGet := handler.repo.GetAuth(ctx, id)
				if errGet != nil {
					t.Fatal(errGet)
				}
				if after.Attributes["priority"] != "17" || record.Version != currentVersion+1 {
					t.Fatal("priority or auth event version did not advance once")
				}
				if _, errPatch := applyOAuthFieldPatch(current, mustRawFields(t, patch)); errPatch != nil {
					t.Fatal(errPatch)
				}
				current.UpdatedAt = after.UpdatedAt
				current.StateVersion = after.StateVersion
				assertAtomicPriorityAuthEqual(t, current, after)
				policy := mustManagementPolicy(t, handler.repo, id)
				if mixed {
					if policy.MaxInFlight == nil || *policy.MaxInFlight != 2 || policy.Version != 1 {
						t.Fatal("mixed request did not persist policy atomically")
					}
				} else if policy.Version != 0 {
					t.Fatal("priority-only request changed policy")
				}
			})
		}
	}
}

func assertAtomicPriorityAuthEqual(t *testing.T, want, got *coreauth.Auth) {
	t.Helper()
	encode := func(auth *coreauth.Auth) any {
		raw, errMarshal := json.Marshal(auth)
		if errMarshal != nil {
			t.Fatal(errMarshal)
		}
		var value any
		if errDecode := json.Unmarshal(raw, &value); errDecode != nil {
			t.Fatal(errDecode)
		}
		return value
	}
	if !reflect.DeepEqual(encode(want), encode(got)) {
		t.Fatal("PATCH overwrote unrequested current auth state")
	}
}

func TestPatchAuthFileFieldsPriorityValidationAndPolicyAtomicity(t *testing.T) {
	for _, tc := range []struct {
		name, fields, wantAttribute string
		wantStatus                  int
		wantPriority                *int
	}{
		{name: "zero", fields: `"priority":0`, wantStatus: 200, wantPriority: atomicPriorityInt(0)},
		{name: "unset", fields: `"priority":null`, wantStatus: 200},
		{name: "negative", fields: `"priority":-2`, wantAttribute: "-2", wantStatus: 200, wantPriority: atomicPriorityInt(-2)},
		{name: "numeric string", fields: `"priority":" 12 "`, wantAttribute: "12", wantStatus: 200, wantPriority: atomicPriorityInt(12)},
		{name: "omitted", fields: `"note":"only-note"`, wantAttribute: "9", wantStatus: 200, wantPriority: atomicPriorityInt(9)},
		{name: "fraction", fields: `"priority":1.5`, wantStatus: 400},
		{name: "exponent", fields: `"priority":1e2`, wantStatus: 400},
		{name: "boolean", fields: `"priority":true`, wantStatus: 400},
		{name: "object", fields: `"priority":{}`, wantStatus: 400},
		{name: "array", fields: `"priority":[]`, wantStatus: 400},
		{name: "text", fields: `"priority":"not-an-integer"`, wantStatus: 400},
		{name: "overflow", fields: `"priority":9223372036854775808`, wantStatus: 400},
		{name: "nested", fields: `"priority.child":4`, wantStatus: 400},
		{name: "invalid sibling", fields: `"priority":12,"excluded_models":[false]`, wantStatus: 400},
		{name: "policy conflict", fields: `"priority":12,"version":99`, wantStatus: 409},
		{name: "invalid policy", fields: `"priority":12,"max_in_flight":-1`, wantStatus: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler, engine, _, closeRepo := newConcurrencyManagementTestServer(t)
			defer closeRepo()
			seedOAuthAuth(t, handler.repo, "priority-validation")
			_, _, _, errSeed := handler.repo.MutateAuth(t.Context(), "priority-validation", "fixture", func(auth *coreauth.Auth) bool {
				auth.Metadata["priority"] = 9
				auth.Attributes = map[string]string{"priority": "9"}
				return true
			})
			if errSeed != nil {
				t.Fatal(errSeed)
			}
			before, beforeRecord, errBefore := handler.repo.GetAuth(t.Context(), "priority-validation")
			if errBefore != nil {
				t.Fatal(errBefore)
			}
			body := `{"id":"priority-validation","max_in_flight":2,` + tc.fields + `}`
			if tc.name == "invalid policy" {
				body = `{"id":"priority-validation",` + tc.fields + `}`
			}
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, httptest.NewRequest(http.MethodPatch, "/auth-files/fields", strings.NewReader(body)))
			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tc.wantStatus)
			}
			after, record, errGet := handler.repo.GetAuth(t.Context(), "priority-validation")
			if errGet != nil {
				t.Fatal(errGet)
			}
			policy := mustManagementPolicy(t, handler.repo, "priority-validation")
			if tc.wantStatus != http.StatusOK {
				assertAtomicPriorityAuthEqual(t, before, after)
				if record.Version != beforeRecord.Version || policy.Version != 0 || policy.MaxInFlight != nil {
					t.Fatal("rejected PATCH changed auth or policy")
				}
				return
			}
			if after.Attributes["priority"] != tc.wantAttribute || !reflect.DeepEqual(priorityFromAuth(after), tc.wantPriority) {
				t.Fatal("priority did not round-trip")
			}
			if policy.MaxInFlight == nil || *policy.MaxInFlight != 2 || policy.Version != 1 {
				t.Fatal("valid PATCH did not persist policy")
			}
		})
	}
}

func atomicPriorityInt(value int) *int { return &value }

type atomicPriorityRefreshTransport struct{}

func (atomicPriorityRefreshTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header),
		Body: io.NopCloser(strings.NewReader(`{"access_token":"fake-refreshed-access","refresh_token":"fake-refreshed-refresh","expires_in":3600}`))}, nil
}

func (transport atomicPriorityRefreshTransport) RoundTripperFor(*coreauth.Auth) http.RoundTripper {
	return transport
}

func refreshAtomicPriorityFixture(t *testing.T, repo *cluster.Repository, db *gorm.DB, auth *coreauth.Auth) {
	t.Helper()
	runtime, errRuntime := home.NewRuntime(&config.Config{})
	if errRuntime != nil {
		t.Fatal(errRuntime)
	}
	defer runtime.Stop()
	runtime.SetClusterAdapter(cluster.NewRuntimeAdapter(repo, ""))
	runtime.CoreManager().SetRoundTripperProvider(atomicPriorityRefreshTransport{})
	if _, errRegister := runtime.CoreManager().Register(coreauth.WithSkipPersist(t.Context()), auth.Clone()); errRegister != nil {
		t.Fatal(errRegister)
	}
	node := cluster.NodeIdentity{IP: "192.0.2.17", Port: 9308, StartedAt: time.Now().UTC()}
	if errCreate := db.Create(&cluster.ClusterNodeRecord{IP: node.IP, Port: node.Port, StartedAt: node.StartedAt,
		IsMaster: true, LastSeenAt: node.StartedAt}).Error; errCreate != nil {
		t.Fatal(errCreate)
	}
	controller := cluster.NewRefreshController(cluster.NewCoordinator(repo, node, cluster.CoordinatorOptions{}), runtime, repo, nil)
	if _, errRefresh := controller.RefreshNow(t.Context(), auth.ID); errRefresh != nil {
		t.Fatal(errRefresh)
	}
	current, _, errGet := repo.GetAuth(t.Context(), auth.ID)
	if errGet != nil {
		t.Fatal(errGet)
	}
	if current.Metadata["access_token"] != "fake-refreshed-access" || current.Metadata["refresh_token"] != "fake-refreshed-refresh" {
		t.Fatal("real refresh finalization did not persist fake rotated grant")
	}
}
