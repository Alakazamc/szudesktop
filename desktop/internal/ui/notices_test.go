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

func TestCollegeNoticeDateLayouts(t *testing.T) {
	cases := []struct{ name, markup, date string }{
		{"sibling", `<li><a href="/info/100/1.htm" title="学院公开通知">学院公开通知</a><span class="news_meta">2026-09-18</span></li>`, "2026-09-18"},
		{"optional li closing", `<ul><li><a href="/info/100/1.htm">第一条学院通知</a><span>2026-09-18</span><li><a href="/info/100/2.htm">第二条学院通知</a><span>2026-09-17</span></ul>`, "2026-09-18"},
		{"year above month day", `<li><a href="/info/100/1.htm" title="人工智能学院通知"><div class="sj"><p>2026</p><p>09/17</p></div>人工智能学院通知</a></li>`, "2026-09-17"},
		{"month day above year", `<a href="/info/100/1.htm" title="化学学院通知"><div class="date"><b>07-02</b><span>2025</span></div>化学学院通知</a>`, "2025-07-02"},
		{"Chinese month", `<a href="/info/100/1.htm" title="体育学院通知"><div class="time"><span>10</span><b>04月</b><em>2026</em></div>体育学院通知</a>`, "2026-04-10"},
		{"publication not excerpt date", `<div class="event-list-item"><a href="/info/100/1.htm" title="生命学院通知">生命学院通知</a><div class="date-excerpt">活动在 2026-03-01 举行</div><div class="fbtime">发布时间：2026-03-19</div><a href="/info/100/1.htm">详情</a></div>`, "2026-03-19"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseNotices(c.markup, "https://fe.szu.edu.cn/")
			if len(got) == 0 || got[0].Date != c.date {
				t.Fatalf("got %+v", got)
			}
		})
	}
	markup := `<ul><li><a href="/info/100/1.htm">没有日期的通知</a></li><li><a href="/info/100/2.htm">旁边有日期通知</a><span>2026-09-18</span></li></ul>`
	got := parseNotices(markup, "https://fe.szu.edu.cn/")
	if len(got) != 1 || !strings.HasSuffix(got[0].URL, "/2.htm") {
		t.Fatalf("borrowed neighbour's date: %+v", got)
	}
}

func TestCollegeNoticeCachesAreSeparate(t *testing.T) {
	old := noticeClient
	publicNotices.results = map[string]noticeResult{}
	publicNotices.retry = map[string]time.Time{}
	t.Cleanup(func() {
		noticeClient = old
		publicNotices.results = map[string]noticeResult{}
		publicNotices.retry = map[string]time.Time{}
	})
	calls := 0
	noticeClient = &http.Client{Transport: noticeTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Cookie") != "" {
			t.Fatal("public request carried a cookie")
		}
		body := `<li><a href="/info/100/1.htm" title="` + r.URL.Host + `学院公告">学院公告</a><span>2026-09-18</span></li>`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	server := &Server{}
	for _, id := range []string{"college-fe", "college-law", "college-fe"} {
		w := httptest.NewRecorder()
		server.handleCampusNotices(w, httptest.NewRequest("GET", "/api/campus/notices?source="+id, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), noticeSources[id].Name) {
			t.Fatal(id, w.Body.String())
		}
	}
	if calls != 2 {
		t.Fatal("sources must have independent caches", calls)
	}
	w := httptest.NewRecorder()
	server.handleCampusNotices(w, httptest.NewRequest("GET", "/api/campus/notices?source=college-csse", nil))
	if w.Code != 503 || calls != 2 {
		t.Fatal("unavailable source must not pretend to have an empty feed")
	}
}
