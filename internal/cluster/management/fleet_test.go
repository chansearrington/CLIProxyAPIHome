package management

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPIHome/internal/cluster"
	"github.com/router-for-me/CLIProxyAPIHome/internal/fleet"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
