package ui

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestCalendarDates(t *testing.T) {
	for _, tc := range []struct{ text, week, end string }{
		{"深圳大学2026一2027学年第一学期校历说明 学期为2026年8月28日至2027年1月22日 新生开始上课：9月28日 老生 开始上课：8月31日", "2026-08-30", "2027-01-22"},
		{"深圳大学2025一2026学年第二学期校历说明 学期为2026年3月4日至7月17日 开始上课： 本科生上课周数： 第 一 至 十 七 周（3月9日、7月3日）", "2026-03-08", "2026-07-17"},
	} {
		got, err := parseAcademicTerm(tc.text, "official-image")
		if err != nil || got.WeekStart != tc.week || got.End != tc.end {
			t.Fatalf("%+v %v", got, err)
		}
	}
	for _, text := range []string{
		"深圳大学2026一2027学年第一学期校历说明 学期为2026年8月28日至2027年1月22日 开始上课：9月28日",
		"深圳大学2026一2027学年第一学期校历说明 学期为2026年8月28日至2027年1月22日 老生 开始上课：8月32日",
		"深圳大学2026一2027学年第一学期校历说明 学期为2026年8月28日至2027年1月22日 老生 开始上课：8月30日",
		"深圳大学2026一2027学年第一学期校历说明 学期为2026年8月28日至2027年1月22日 未提供上课日期",
	} {
		if _, err := parseAcademicTerm(text, ""); err == nil {
			t.Fatal("adopted ambiguous date", text)
		}
	}
}

func TestCalendarImages(t *testing.T) {
	html := `<img class="img_vsb_content" orisrc="/__local/a.png"><img orisrc="https://evil.example/a.png" class="img_vsb_content"><img src="logo.png"><img orisrc="/__local/a.png" class="img_vsb_content"><img class="img_vsb_content" orisrc="//evil.example/a.png">`
	got := parseCalendarImages(html)
	if len(got) != 1 || got[0] != "https://www.szu.edu.cn/__local/a.png" {
		t.Fatal(got)
	}
}

type calendarTransport func(*http.Request) (*http.Response, error)

func (f calendarTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCalendarUpdateAndOffline(t *testing.T) {
	original := noticeClient
	defer func() { noticeClient = original }()
	previous := bundledCalendar()
	noticeClient = &http.Client{Transport: calendarTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != schoolCalendarURL {
			t.Fatal("unnecessary download", r.URL)
		}
		body := ""
		for _, image := range previous.Images {
			body += `<img class="img_vsb_content" orisrc="` + image + `">`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	got, err := updateCalendar(context.Background(), previous)
	if err != nil || got.Stale || time.Since(got.CheckedAt) > time.Minute {
		t.Fatalf("%+v %v", got, err)
	}
	noticeClient = &http.Client{Transport: calendarTransport(func(*http.Request) (*http.Response, error) { return nil, errors.New("offline") })}
	got, err = updateCalendar(context.Background(), got)
	if err == nil || got.Terms[0].WeekStart != previous.Terms[0].WeekStart {
		t.Fatal("lost cached dates")
	}
}
