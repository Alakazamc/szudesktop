package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/SzuDesktopTeam/szudesktop/internal/version"
)

func TestHealthReadinessNeedsNoBusinessServices(t *testing.T) {
	// Deliberately leave credentials, network detection and sessions uninitialized.
	// Desktop readiness must work before (or without) any of those services.
	s := &Server{}
	mux := http.NewServeMux()
	s.routes(mux, fstest.MapFS{})
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:12345/api/health", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("health returned %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		OK      bool   `json:"ok"`
		App     string `json:"app"`
		Version string `json:"app_version"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.OK || body.App != "szuDesktop" || body.Version != version.Current {
		t.Fatalf("unexpected health identity: %+v", body)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("health must not be cached")
	}
	for _, tc := range []struct {
		method, address, origin string
		status                  int
	}{
		{http.MethodPost, "http://127.0.0.1:12345/api/health", "", http.StatusMethodNotAllowed},
		{http.MethodGet, "http://example.com/api/health", "", http.StatusForbidden},
		{http.MethodGet, "http://127.0.0.1:12345/api/health", "https://example.com", http.StatusForbidden},
	} {
		request := httptest.NewRequest(tc.method, tc.address, nil)
		request.Header.Set("Origin", tc.origin)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != tc.status {
			t.Fatalf("guard %s %s: got %d want %d", tc.method, tc.address, response.Code, tc.status)
		}
	}
}
