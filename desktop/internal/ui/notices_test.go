package ui

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type noticeTransport func(*http.Request) (*http.Response, error)

func (f noticeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNoticeCacheAndFailureState(t *testing.T) {
	old := noticeClient
	t.Cleanup(func() {
		noticeClient = old
		publicNotices.results = map[string]noticeResult{}
		publicNotices.retry = map[string]time.Time{}
	})
	publicNotices.results = map[string]noticeResult{}
	publicNotices.retry = map[string]time.Time{}
	calls := 0
	body := `<a href="../info/1053/1.htm" title="本科教务公告">2026-09-18</a>`
	noticeClient = &http.Client{Transport: noticeTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	s := &Server{}
	get := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.handleCampusNotices(w, httptest.NewRequest("GET", "/api/campus/notices?source=undergrad", nil))
		return w
	}
	if w := get(); w.Code != 200 || !strings.Contains(w.Body.String(), "本科教务公告") {
		t.Fatal(w.Code, w.Body.String())
	}
	get()
	if calls != 1 {
		t.Fatal("cache did not prevent another fetch")
	}
	entry := publicNotices.results["undergrad"]
	entry.FetchedAt = time.Now().Add(-time.Hour)
	publicNotices.results["undergrad"] = entry
	publicNotices.retry["undergrad"] = time.Time{}
	body = "<html>layout changed</html>"
	w := get()
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"stale":true`) {
		t.Fatal("failed refresh must preserve stale content", w.Body.String())
	}
	get()
	if calls != 2 {
		t.Fatal("failed source was not throttled")
	}
	publicNotices.results = map[string]noticeResult{}
	w = get()
	if w.Code != 503 {
		t.Fatal("no cache failure must be explicit", w.Code)
	}
	w = httptest.NewRecorder()
	s.handleCampusNotices(w, httptest.NewRequest("GET", "/api/campus/notices?source=https://example.org", nil))
	if w.Code != 400 {
		t.Fatal("unknown source accepted")
	}
}

func TestParseSchoolNotices(t *testing.T) {
	markup := `<a href="../info/1053/1.htm" title="教务 &amp; 公告"><h3>教务公告<span><b>08</b>/ 2026-06</span></h3></a><a href="../info/1053/1.htm">重复 2026.06.08</a><a href="https://evil.test/info/1053/2.htm">外站 2026.06.08</a><a href="javascript:alert(1)">恶意 2026.06.08</a><a href="../info/1053/3.htm">研究生培养通知 2026.09.18</a><a href="../info/1053/4.htm">无日期页面</a>`
	items := parseNotices(markup, "https://jwb.szu.edu.cn/index/jwtz.htm")
	if len(items) != 2 || items[0].Title != "教务 & 公告" || items[0].Date != "2026-06-08" || items[1].Title != "研究生培养通知" || items[1].Date != "2026-09-18" {
		t.Fatalf("unexpected: %+v", items)
	}
}
