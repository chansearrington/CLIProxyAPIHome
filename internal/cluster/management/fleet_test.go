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
	last := stored.History[len(stored.History)-1]
	if !last.Force || last.Caller == "" {
		t.Fatal("postpone force/caller not audited")
	}
	if errStatus != nil || stored.Schema != 1 || stored.Rollout.Release.Version != "8.0.30" || !stored.Rollout.Window.WindowStart.Equal(start.Add(48*time.Hour)) || !stored.PostponedTargets["8.0.30"] || len(stored.Notices) != 1 {
		t.Fatalf("postponement not durably stored: %+v %v", stored, errStatus)
	}
}

func TestFleetStatusMasksHookAndPolicyForceIsAudited(t *testing.T) {
	db, cleanup := openManagementLogTestDB(t)
	defer cleanup()
	repo := cluster.NewRepository(db)
	s := fleet.New(repo)
	endpoint := "https://hooks.example.invalid/private-path?fake-private-value=test-only"
	if errPolicy := s.SetPolicy(context.Background(), fleet.Policy{IntervalSeconds: 3600, AnnouncementURL: endpoint}); errPolicy != nil {
		t.Fatal(errPolicy)
	}
	handler := NewHandler(repo, nil, "home", 8327)
	router := gin.New()
	router.GET("/fleet", handler.GetFleet)
	router.PUT("/fleet/policy", handler.PutFleetPolicy)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/fleet", nil))
	var state fleet.State
	if errDecode := json.Unmarshal(recorder.Body.Bytes(), &state); errDecode != nil {
		t.Fatal(errDecode)
	}
	if state.Policy.AnnouncementURL != "https://hooks.example.invalid" || bytes.Contains(recorder.Body.Bytes(), []byte("private-path")) {
		t.Fatal("GET exposed webhook path/query")
	}
	stored, errStatus := s.Status(context.Background())
	if errStatus != nil || stored.Policy.AnnouncementURL != endpoint {
		t.Fatal("mask changed stored hook")
	}
	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPut, "/fleet/policy", bytes.NewBufferString(`{"interval_seconds":3600,"force":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "192.0.2.10:1234"
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatal(recorder.Body.String())
	}
	stored, _ = s.Status(context.Background())
	last := stored.History[len(stored.History)-1]
	if !last.Force || last.Caller != "192.0.2.10:1234" {
		t.Fatal("policy force/caller not audited")
	}
}

func TestFleetReviewAuditsCallerAndRejectsInactiveHold(t *testing.T) {
	db, cleanup := openManagementLogTestDB(t)
	defer cleanup()
	repo := cluster.NewRepository(db)
	state := fleet.State{Schema: 1, Policy: fleet.Policy{IntervalSeconds: 3600}, Latest: &fleet.Release{Version: "8.0.30"}}
	raw, errEncode := json.Marshal(state)
	if errEncode != nil {
		t.Fatal(errEncode)
	}
	if swapped, errSwap := repo.FleetSwap(context.Background(), nil, false, raw); errSwap != nil || !swapped {
		t.Fatal(errSwap)
	}
	h := NewHandler(repo, nil, "home", 8327)
	router := gin.New()
	router.POST("/fleet/releases/:version/:action", h.ReviewFleetRelease)
	for _, tc := range []struct {
		action string
		code   int
	}{{"hold", http.StatusConflict}, {"approve", http.StatusOK}, {"hold", http.StatusOK}, {"hold", http.StatusConflict}} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/fleet/releases/8.0.30/"+tc.action, nil)
		request.RemoteAddr = "192.0.2.11:1234"
		router.ServeHTTP(recorder, request)
		if recorder.Code != tc.code {
			t.Fatalf("%s: %d %s", tc.action, recorder.Code, recorder.Body.String())
		}
	}
	stored, _ := fleet.New(repo).Status(context.Background())
	for _, event := range stored.History {
		if event.Operation == "review" && event.Caller != "192.0.2.11:1234" {
			t.Fatal("review caller not audited")
		}
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
