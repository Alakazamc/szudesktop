package ui

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestGraduatePasswordCompatibility(t *testing.T) {
	// Non-secret test vectors checked against the school's public login encoder.
	for input, want := range map[string]string{"test": "D8D35E5019288C41", "abcd1234": "A9CF2704230383D1C1BB5938DF9F2190", "x": "4B4072F73C901FDD", "中文A": "F7CA2C5D3E57CE18"} {
		if got := graduatePassword(input); got != want {
			t.Fatalf("protocol vector mismatch: %q", input)
		}
	}
}

func TestAcademicLoginReadAndClear(t *testing.T) {
	a := newAcademicService()
	a.client = newAcademicClient()
	a.challenge = "test-challenge"
	a.vtoken = "test-token"
	a.expires = time.Now().Add(time.Minute)
	calls := 0
	a.client.Transport = calendarTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := ""
		switch r.URL.Path {
		case "/yjsxkapp/sys/xsxkapp/login/check/login.do":
			r.ParseForm()
			if r.Form.Get("loginPwd") != "D8D35E5019288C41" || r.Form.Get("vtoken") != "test-token" {
				t.Fatal("incorrect official login protocol")
			}
			body = `{"code":"1"}`
		case "/yjsxkapp/sys/xsxkapp/xsxkHome/loadStdInfo.do":
			body = `{"XM":"test-student"}`
		case "/yjsxkapp/sys/xsxkapp/xsxkCourse/loadKbxx.do":
			body = `{"results":[],"xkjgList":[]}`
		case "/yjsxkapp/sys/xsxkapp/xsxkHome/loadPublicInfo.do":
			body = `{"lcxx":{"XNXQDM":"2026-2027-1","MC":"测试学期"}}`
		default:
			t.Fatal("unexpected request", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	s := &Server{academic: a}
	request := func(handler http.HandlerFunc, method, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "http://127.0.0.1/api/academic", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		protectAPI(handler, method)(rec, req)
		return rec
	}
	payload := `{"username":"test-student","password":"test","captcha":"0000","challenge":"test-challenge"}`
	login := request(s.handleAcademicLogin, "POST", payload)
	if login.Code != 200 || !a.authenticated || strings.Contains(login.Body.String(), "test-token") {
		t.Fatal("login did not complete safely", login.Code)
	}
	count := calls
	replay := request(s.handleAcademicLogin, "POST", payload)
	if replay.Code != 409 || calls != count {
		t.Fatal("replayed login made another school request")
	}
	table := request(s.handleTimetable, "GET", "")
	if table.Code != 200 || !strings.Contains(table.Body.String(), `"entries":[]`) || strings.Contains(table.Body.String(), "test-student") {
		t.Fatal("read-only timetable", table.Code)
	}
	status := request(s.handleAcademicSession, "GET", "")
	if strings.Contains(status.Body.String(), "test-") {
		t.Fatal("status revealed private fields")
	}
	request(s.handleAcademicSession, "DELETE", "")
	if a.client != nil || a.authenticated || a.vtoken != "" || a.challenge != "" {
		t.Fatal("clear retained authentication")
	}
	if request(s.handleTimetable, "GET", "").Code != 409 {
		t.Fatal("cleared account still readable")
	}
}

func TestAcademicLoginExpiredAndCrossOrigin(t *testing.T) {
	a := newAcademicService()
	a.challenge = "expired"
	a.expires = time.Now().Add(-time.Minute)
	s := &Server{academic: a}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "http://127.0.0.1/api/academic/login", strings.NewReader(`{"username":"test","password":"test","captcha":"0000","challenge":"expired"}`))
	s.handleAcademicLogin(rec, req)
	if rec.Code != 409 {
		t.Fatal("expired challenge accepted")
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "http://127.0.0.1/api/academic/login", strings.NewReader(`{}`))
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Content-Type", "application/json")
	protectAPI(s.handleAcademicLogin, "POST")(rec, req)
	if rec.Code != 403 {
		t.Fatal("cross-origin school login accepted")
	}
}

func TestAcademicDestinations(t *testing.T) {
	for _, address := range []string{"http://ehall.szu.edu.cn/yjsxkapp/a", "https://ehall.szu.edu.cn.evil.example/yjsxkapp/a", "https://ehall.szu.edu.cn:8443/yjsxkapp/a", "https://ehall.szu.edu.cn/other", "https://user@ehall.szu.edu.cn/yjsxkapp/a"} {
		u, _ := url.Parse(address)
		if academicURLAllowed(u) {
			t.Fatal("unsafe destination accepted")
		}
	}
	client := newAcademicClient()
	called := false
	client.Transport = calendarTransport(func(r *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://outside.example/private"}}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})
	_, err := academicRequest(context.Background(), client, graduateProfilePath, nil)
	if !called || err == nil || strings.Contains(err.Error(), "outside.example") {
		t.Fatal("redirect must stop without echoing URL")
	}
}

func TestGraduateTimetableAndUnscheduled(t *testing.T) {
	got, err := parseGraduateTimetable([]byte(`{"results":[{"KCMC":"示例课程","XQ":2,"KSJCDM":3,"JSJCDM":4,"BJDM":"one","ZCMC":"1-16周(单)","JASMC":"示例教室","JSXM":"示例教师"}],"xkjgList":[{"KCMC":"示例课程","BJDM":"one"},{"KCMC":"待排课程","BJDM":"two"}]}`))
	if err != nil || len(got.Entries) != 1 || len(got.Unscheduled) != 1 || got.Entries[0].Weeks != "1-16周(单)" {
		t.Fatalf("%+v %v", got, err)
	}
	empty, err := parseGraduateTimetable([]byte(`{"results":[],"xkjgList":[]}`))
	if err != nil || len(empty.Entries) != 0 {
		t.Fatal(err)
	}
	for _, raw := range []string{`{}`, `{"results":null,"xkjgList":[]}`, `{"results":[],"xkjgList":null}`, `{"results":[{}],"xkjgList":[]}`, `{"results":[null],"xkjgList":[]}`, `{"results":[],"xkjgList":[{}]}`, `<html>登录</html>`} {
		if _, err := parseGraduateTimetable([]byte(raw)); err == nil {
			t.Fatal("malformed response treated as empty", raw)
		}
	}
}
