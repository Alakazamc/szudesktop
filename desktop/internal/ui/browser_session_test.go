package ui

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestBrowserCookiesStayOnSchoolHostAndPath(t *testing.T) {
	c := newCasClient()
	if err := setBrowserCookies(c, []browserCookie{{"SESSION", "private-value", "/jwapp/"}}); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		address string
		count   int
	}{
		{"https://ehall.szu.edu.cn/jwapp/x", 1},
		{"https://ehall.szu.edu.cn/gsapp/x", 0},
		{"https://example.org/jwapp/x", 0},
		{"http://ehall.szu.edu.cn/jwapp/x", 0},
	} {
		u, _ := url.Parse(row.address)
		if got := len(c.Jar.Cookies(u)); got != row.count {
			t.Fatalf("cookie scope %s: %d", row.address, got)
		}
	}
	if err := setBrowserCookies(c, []browserCookie{{"bad\nname", "v", "/"}}); err == nil {
		t.Fatal("invalid header accepted")
	}
}

func TestBrowserSessionRequiresBusinessResponse(t *testing.T) {
	for _, row := range []struct {
		business, body string
		ok             bool
	}{
		{"undergrad", `<html>统一身份认证</html>`, false},
		{"undergrad", `{"code":"0"}`, false},
		{"undergrad", `{"code":"0","datas":{"dqxnxq":{"rows":[{"DM":"2026-2027-1"}]}}}`, true},
		{"graduate", `{"CODE":"0"}`, false},
		{"graduate", `{"XM":"测试学生"}`, true},
	} {
		t.Run(row.business+row.body, func(t *testing.T) {
			client := newCasClient()
			client.Transport = ehallTestTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != ehallHost {
					t.Fatal("unexpected destination")
				}
				body := row.body
				if row.ok && r.URL.Path == undergradTimetablePath {
					body = `{"code":"0","datas":{"xskcb":{"rows":[],"totalSize":0}}}`
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			err := validateBrowserSession(context.Background(), client, row.business)
			if (err == nil) != row.ok {
				t.Fatalf("business validation: %v", err)
			}
		})
	}
}
