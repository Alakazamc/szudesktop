package ui

import (
	"context"
	"errors"
	"net/http"
	"slices"
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
	previous := bundledCalendar()
	content := "version one"
	fetch := func(_ context.Context, address string) ([]byte, error) {
		if address != schoolCalendarURL {
			return []byte(content), nil
		}
		body := ""
		for _, image := range previous.Images {
			body += `<img class="img_vsb_content" orisrc="` + image + `">`
		}
		return []byte(body), nil
	}
	calls := 0
	recognize := func(_ context.Context, b []byte) (string, error) {
		calls++
		switch (calls - 1) % 4 {
		case 0:
			return "深圳大学2026一2027学年第一学期校历说明 学期为2026年8月28日至2027年1月22日 老生 开始上课：8月31日", nil
		case 2:
			return "深圳大学2025一2026学年第二学期校历说明 学期为2026年3月4日至7月17日 开始上课：3月9日", nil
		default:
			return "校历网格", nil
		}
	}
	got, err := refreshCalendar(context.Background(), previous, fetch, recognize)
	if err != nil || got.Stale || time.Since(got.CheckedAt) > time.Minute || calls != 4 {
		t.Fatalf("%+v %v OCR=%d", got, err, calls)
	}
	got, err = refreshCalendar(context.Background(), got, fetch, recognize)
	if err != nil || calls != 4 {
		t.Fatal("unchanged content must not repeat OCR", err, calls)
	}
	content = "version two, same image URL"
	got, err = refreshCalendar(context.Background(), got, fetch, recognize)
	if err != nil || calls != 8 {
		t.Fatal("same-URL replacement was missed", err, calls)
	}
	offline := func(context.Context, string) ([]byte, error) { return nil, errors.New("offline") }
	kept, err := refreshCalendar(context.Background(), got, offline, recognize)
	if err == nil || kept.Terms[0].WeekStart != got.Terms[0].WeekStart {
		t.Fatal("lost cached dates")
	}
	content = "unrecognizable replacement"
	failed := func(context.Context, []byte) (string, error) { return "", errors.New("OCR unavailable") }
	kept, err = refreshCalendar(context.Background(), got, fetch, failed)
	if err == nil || kept.ImageHashes[0] != got.ImageHashes[0] {
		t.Fatal("failed OCR adopted new content")
	}
}

// calendarPage 拼出校历页的 img 列表，供取数替身按 previous.Images 返回。
func calendarPage(images []string) string {
	body := ""
	for _, image := range images {
		body += `<img class="img_vsb_content" orisrc="` + image + `">`
	}
	return body
}

// 复现 issue #9：用户语言不是中文的 Windows 上，TryCreateFromUserProfileLanguages()
// 给出英文识别器，中文校历被读成拉丁乱码。四张图一个汉字都没有，却逐张被当成
// 「没有校历说明的网格页」跳过，最后误报成「学校改了校历格式」——把本机的语言
// 环境问题指向了错误的方向。
func TestCalendarOCRWithoutChineseReportsHonestly(t *testing.T) {
	previous := bundledCalendar()
	content := "garbage"
	fetch := func(_ context.Context, address string) ([]byte, error) {
		if address != schoolCalendarURL {
			return []byte(content), nil
		}
		return []byte(calendarPage(previous.Images)), nil
	}
	garbage := "OCR languages: en-US\n2026-2027 2026 28 2027 22 E\nJEJ (3 h 9 5 h 29 H"
	recognize := func(context.Context, []byte) (string, error) { return garbage, nil }

	got, err := refreshCalendar(context.Background(), previous, fetch, recognize)
	if err == nil {
		t.Fatal("全是拉丁乱码时必须报错，不能当成新版校历收下")
	}
	if !strings.Contains(err.Error(), "未能识别中文") {
		t.Fatalf("要如实指出本机 OCR 认不出中文，不能报成学校改了格式：%v", err)
	}
	if !slices.Equal(got.ImageHashes, previous.ImageHashes) {
		t.Fatal("识别失败必须保留上一次的校历")
	}
}

// 防线不能误伤现在能用的机器：网格页可能全是数字、一个汉字都没有，只要说明页
// 认出了中文，就要照常成功，不能因为某张图没有汉字就中断整个刷新。
func TestCalendarOCRGridWithoutChineseStillSucceeds(t *testing.T) {
	previous := bundledCalendar()
	content := "mixed"
	fetch := func(_ context.Context, address string) ([]byte, error) {
		if address != schoolCalendarURL {
			return []byte(content), nil
		}
		return []byte(calendarPage(previous.Images)), nil
	}
	calls := 0
	recognize := func(_ context.Context, b []byte) (string, error) {
		calls++
		switch (calls - 1) % 4 {
		case 0:
			return "深圳大学2026一2027学年第一学期校历说明 学期为2026年8月28日至2027年1月22日 老生 开始上课：8月31日", nil
		case 2:
			return "深圳大学2025一2026学年第二学期校历说明 学期为2026年3月4日至7月17日 开始上课：3月9日", nil
		default:
			return "1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18", nil
		}
	}
	got, err := refreshCalendar(context.Background(), previous, fetch, recognize)
	if err != nil || got.Stale || len(got.Terms) != 2 {
		t.Fatalf("说明页有汉字、网格页没有时必须照常成功：%+v %v", got, err)
	}
}

func TestHasCJKText(t *testing.T) {
	for _, text := range []string{
		"深圳大学2026一2027学年第一学期校历说明",
		"校历网格",
		"2026-08-30 学年第一学期",
	} {
		if !hasCJKText(text) {
			t.Fatalf("含汉字却判成没有：%q", text)
		}
	}
	for _, text := range []string{
		"",
		"OCR languages: en-US",
		"2026-2027 2026 28 2027 22 E",
		"JEJ (3 h 9 5 h 29 H",
		"1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18",
	} {
		if hasCJKText(text) {
			t.Fatalf("没有汉字却判成有：%q", text)
		}
	}
}
