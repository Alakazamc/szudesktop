package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeVPNServer(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		bad  bool
	}{
		{name: "default", in: "", want: defaultVPNServer},
		{name: "hostname", in: "ssl.szu.edu.cn", want: "ssl.szu.edu.cn:443"},
		{name: "host and port", in: "svpn.szu.edu.cn:8443", want: "svpn.szu.edu.cn:8443"},
		{name: "https URL", in: "https://ssl.szu.edu.cn/", want: "ssl.szu.edu.cn:443"},
		{name: "page path rejected", in: "https://ssl.szu.edu.cn/portal", bad: true},
		{name: "bad port rejected", in: "ssl.szu.edu.cn:99999", bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeVPNServer(tt.in)
			if tt.bad {
				if err == nil {
					t.Fatalf("normalizeVPNServer(%q) unexpectedly succeeded: %q", tt.in, got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("normalizeVPNServer(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
			}
		})
	}
}

func TestSocksAddressAlwaysUsesLoopback(t *testing.T) {
	got, err := socksAddress(7891)
	if err != nil {
		t.Fatal(err)
	}
	if got != "127.0.0.1:7891" {
		t.Fatalf("socksAddress(7891) = %q", got)
	}
	for _, port := range []int{-1, 80, 65536} {
		if _, err := socksAddress(port); err == nil {
			t.Fatalf("socksAddress(%d) unexpectedly succeeded", port)
		}
	}
}

func TestVPNLogRingKeepsNewestEntries(t *testing.T) {
	m := &vpnManager{}
	for i := 0; i < maxVPNLogs+7; i++ {
		m.appendLog("info", "entry")
	}
	if len(m.logs) != maxVPNLogs {
		t.Fatalf("got %d logs, want %d", len(m.logs), maxVPNLogs)
	}
}

func TestVPNStatusDoesNotExposePassword(t *testing.T) {
	s := &Server{vpn: newVPNManager()}
	rec := httptest.NewRecorder()
	s.handleVPNStatus(rec, httptest.NewRequest(http.MethodGet, "/api/vpn/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for key := range body {
		switch strings.ToLower(key) {
		case "password", "passwd", "pwd", "token", "twfid":
			t.Fatalf("sensitive field leaked: %s", key)
		}
	}
	if body["socks_addr"] != "127.0.0.1:7891" {
		t.Fatalf("unexpected socks_addr: %v", body["socks_addr"])
	}
}

func TestVPNProxyRejectsEnableBeforeConnection(t *testing.T) {
	s := &Server{vpn: newVPNManager()}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/vpn/proxy", strings.NewReader(`{"enabled":true}`))
	s.handleVPNProxy(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("Content-Type = %q", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ok"] != false || body["message"] == "" {
		t.Fatalf("unexpected body: %#v", body)
	}
}

func TestVPNConnectRejectsBadInputWithoutNetwork(t *testing.T) {
	s := &Server{vpn: newVPNManager()}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/vpn/connect", strings.NewReader(`{
		"server":"https://ssl.szu.edu.cn/not-a-server",
		"socks_port":7891,
		"username":"123456",
		"password":"not-real"
	}`))
	s.handleVPNConnect(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "服务器") {
		t.Fatalf("unclear error: %s", rec.Body.String())
	}
}
