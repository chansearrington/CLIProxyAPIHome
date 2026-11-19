package management

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPIHome/internal/cluster"
	"github.com/router-for-me/CLIProxyAPIHome/internal/fleet"
)

func TestFleetPostponePersistsAndEnforcesOncePerTarget(t *testing.T) {
	db, cleanup := openManagementLogTestDB(t)
	defer cleanup()
	repo := cluster.NewRepository(db)
	ctx := context.Background()
	start := time.Date(2030, 1, 2, 8, 0, 0, 0, time.UTC)
	state := fleet.State{Schema: 1, Policy: fleet.Policy{IntervalSeconds: 3600}, Rollout: &fleet.Rollout{Release: fleet.Release{Version: "8.0.30"}, Phase: "upgrade", Window: &fleet.Window{ID: "fake-window", WindowStart: start, Nodes: []string{"00000000-0000-4000-8000-000000000001"}}}}
	raw, errEncode := json.Marshal(state)
	if errEncode != nil {
		t.Fatal(errEncode)
	}
	if swapped, errSwap := repo.FleetSwap(ctx, nil, false, raw); errSwap != nil || !swapped {
		t.Fatalf("seed: %v %v", swapped, errSwap)
	}
	handler := NewHandler(repo, nil, "home", 8327)
	router := gin.New()
	router.POST("/fleet/postpone", handler.PostponeFleet)
	for _, tc := range []struct {
		body string
		code int
	}{
		{`{}`, http.StatusBadRequest},
		{`{"target_version":"8.0.31"}`, http.StatusConflict},
		{`{"target_version":"8.0.30","node_id":"unregistered"}`, http.StatusConflict},
		{`{"target_version":"8.0.30"}`, http.StatusOK},
		{`{"target_version":"8.0.30"}`, http.StatusConflict},
		{`{"target_version":"8.0.30","force":true}`, http.StatusOK},
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/fleet/postpone", bytes.NewBufferString(tc.body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		if recorder.Code != tc.code {
			t.Fatalf("%s: status %d: %s", tc.body, recorder.Code, recorder.Body.String())
		}
	}
	stored, errStatus := fleet.New(repo).Status(ctx)
	if errStatus != nil || stored.Schema != 1 || stored.Rollout.Release.Version != "8.0.30" || !stored.Rollout.Window.WindowStart.Equal(start.Add(48*time.Hour)) || !stored.PostponedTargets["8.0.30"] || len(stored.Notices) != 3 {
		t.Fatalf("postponement not durably stored: %+v %v", stored, errStatus)
	}
}

func TestFleetPolicyPersistsAndRejectsInvalidChanges(t *testing.T) {
	db, cleanup := openManagementLogTestDB(t)
	defer cleanup()
	handler := NewHandler(cluster.NewRepository(db), nil, "home", 8327)
	router := gin.New()
	router.GET("/fleet", handler.GetFleet)
	router.PUT("/fleet/policy", handler.PutFleetPolicy)
	for _, tc := range []struct {
		body string
		code int
	}{{`{"enabled":true,"interval_seconds":1}`, http.StatusBadRequest}, {`{"enabled":false,"interval_seconds":3600,"nodes":[]}`, http.StatusOK}} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPut, "/fleet/policy", bytes.NewBufferString(tc.body))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		if recorder.Code != tc.code {
			t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
		}
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/fleet", nil))
	var state fleet.State
	if errDecode := json.Unmarshal(recorder.Body.Bytes(), &state); errDecode != nil {
		t.Fatal(errDecode)
	}
	if state.Policy.IntervalSeconds != 3600 || state.Schema != 1 {
		t.Fatal("policy readback mismatch")
	}
}
