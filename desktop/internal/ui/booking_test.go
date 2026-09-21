package ui

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// bookingFixture 造一个只读的预约服务，学校的公开接口由替身应答。
//
// 替身对三件事零容忍：请求带 Cookie、请求不是 GET、目标不是学校的公开地址。
// 预约的办理在学校官方页面完成，本应用不留写操作通路，也不保存用户粘贴的
// 预约 Cookie（STATUS.md F23）。`/venue-api/boothReservation/*` 没有列进
// switch，任何代码走到那里都会撞上 default 分支而失败。
func bookingFixture(t *testing.T) *Server {
	t.Helper()
	b := newBookingService()
	b.client.Transport = calendarTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Cookie") != "" {
			t.Fatal("只读查询携带了 Cookie")
		}
		if r.Method != http.MethodGet {
			t.Fatal("预约查询发出了非 GET 请求：" + r.Method)
		}
		if r.URL.Scheme != "http" || r.URL.Host != "swzx.szu.edu.cn" {
			t.Fatal("公开查询的目标被改成了 " + r.URL.Scheme + "://" + r.URL.Host)
		}
		var data any
		switch r.URL.Path {
		case "/venue-api/booth/list":
			data = map[string]any{"list": []any{map[string]any{"id": 1, "typeId": 1, "name": "测试会议室", "status": true}}, "total": 1}
		case "/venue-api/booth/info/1":
			data = map[string]any{"id": 1, "typeId": 1, "name": "测试会议室", "status": true}
		case "/venue-api/boothType/info/1":
			data = map[string]any{"availableTimePeriod": uint64(1)<<28 | uint64(1)<<29 | uint64(1)<<30 | uint64(1)<<31, "samePersonMaxReservationPerDay": 4, "lastReservationDayBeforeAppointment": 3}
		case "/venue-api/booth/1/available-time":
			times := make([]int, 48)
			times[28] = 1
			times[29] = -1
			times[31] = 7
			data = []any{map[string]any{"date": r.URL.Query().Get("startDate"), "times": times}}
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		body, _ := json.Marshal(map[string]any{"status": 200, "data": data})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Request: r}, nil
	})
	return &Server{booking: b}
}

func TestBookingPublicSlotsAreReadOnly(t *testing.T) {
	s := bookingFixture(t)
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	day, err := s.booking.availability(context.Background(), 1, "2026-09-21", now)
	if err != nil || len(day.Slots) != 4 {
		t.Fatal("slot parse", err)
	}
	for i, want := range []string{"available", "closed", "occupied", "unknown"} {
		if day.Slots[i].State != want {
			t.Fatal("official state mapping", day.Slots)
		}
	}
	if day.Slots[0].Start != "14:00" || day.Slots[0].End != "14:30" {
		t.Fatal("half-hour index mapping")
	}
	if _, err = s.booking.availability(context.Background(), 1, "2026-09-23", now); err == nil {
		t.Fatal("beyond booking window accepted")
	}
	if bookingToday(time.Date(2026, 9, 19, 17, 0, 0, 0, time.UTC)).Format("2006-01-02") != "2026-09-20" {
		t.Fatal("not Shenzhen date")
	}
}

func TestBookingFailuresNeverBecomeEmptyOrSuccessful(t *testing.T) {
	for _, body := range []string{`<html>登录</html>`, `{"status":200,"data":null}`, `{"status":200,"data":{"list":[],"total":null}}`, `{"status":200,"encoding":1,"data":"opaque"}`} {
		s := bookingFixture(t)
		s.booking.client.Transport = calendarTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		})
		if _, err := s.booking.availability(context.Background(), 1, "2026-09-22", time.Now()); err == nil {
			t.Fatal("unrecognized response accepted", body)
		}
	}
}

// 被网关拦下、重定向到登录页时，不能显示成「这天没有空位」。
func TestBookingRedirectIsNotReportedAsNoAvailability(t *testing.T) {
	s := bookingFixture(t)
	s.booking.client.Transport = calendarTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 302,
			Header:     http.Header{"Location": {"https://auth.szu.edu.cn/login"}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    r,
		}, nil
	})
	_, err := s.booking.availability(context.Background(), 1, "2026-09-22", time.Now())
	if err == nil {
		t.Fatal("被重定向到登录页却报成查询成功")
	}
	if !strings.Contains(err.Error(), "校园网") {
		t.Fatalf("错误没有告诉用户需要校园网：%v", err)
	}
}

// 预约只保留只读的场地与空位查询。这四个端点里 commit 会真的向学校提交预约，
// session 会收用户粘贴的 Cookie，而界面早已不调用它们；留着就等于发布包里
// 带一条没人验收过的写操作通路（STATUS.md F23）。
func TestBookingWriteEndpointsAreNotServed(t *testing.T) {
	s := &Server{booking: newBookingService()}
	mux := http.NewServeMux()
	s.routes(mux, fstest.MapFS{})

	for _, path := range []string{"/api/booking/session", "/api/booking/history", "/api/booking/prepare", "/api/booking/commit"} {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
			r := httptest.NewRequest(method, "http://127.0.0.1"+path, strings.NewReader("{}"))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, r)
			if w.Code != http.StatusNotFound {
				t.Fatalf("%s %s 仍在提供服务，得到 %d（应为 404）", method, path, w.Code)
			}
		}
	}
}

// 只读的两个端点必须还在：删死代码不能顺手删掉已实测可用的功能。
func TestBookingReadOnlyEndpointsStillServed(t *testing.T) {
	s := bookingFixture(t)
	mux := http.NewServeMux()
	s.routes(mux, fstest.MapFS{})

	for _, path := range []string{"/api/booking/rooms", "/api/booking/availability"} {
		r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1"+path+"?room=1&date=2026-09-22", nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code == http.StatusNotFound {
			t.Fatalf("%s 被误删了", path)
		}
	}
}
