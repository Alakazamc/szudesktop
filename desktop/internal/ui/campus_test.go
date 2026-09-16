package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewCampusGatewayValidatesFixedHTTPURL(t *testing.T) {
	valid, err := newCampusGateway("http://10.20.30.40:8080/api")
	if err != nil {
		t.Fatal(err)
	}
	if valid.baseURL != "http://10.20.30.40:8080/api" {
		t.Fatalf("baseURL = %q", valid.baseURL)
	}
	for _, raw := range []string{
		"ftp://campus-api.example.edu",
		"campus-api.example.edu",
		"https://user:pass@campus-api.example.edu",
		"https://campus-api.example.edu?token=secret",
		"https://campus-api.example.edu#fragment",
	} {
		if _, err := newCampusGateway(raw); err == nil {
			t.Fatalf("newCampusGateway(%q) unexpectedly succeeded", raw)
		}
	}
}

func TestCampusStatusDoesNotExposeSecretsOrProxyArbitraryURL(t *testing.T) {
	g, err := newCampusGateway("https://campus-api.example.edu")
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{campus: g}
	rec := httptest.NewRecorder()
	s.handleCampusStatus(rec, httptest.NewRequest(http.MethodGet, "/api/campus/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["base_url"] != "https://campus-api.example.edu" {
		t.Fatalf("unexpected base_url: %#v", body["base_url"])
	}
	if strings.Contains(rec.Body.String(), "password") || strings.Contains(rec.Body.String(), "token") {
		t.Fatalf("status contains secret-like field: %s", rec.Body.String())
	}
	if _, ok := body["proxy_url"]; ok {
		t.Fatal("campus status must not expose an arbitrary proxy URL")
	}
}
