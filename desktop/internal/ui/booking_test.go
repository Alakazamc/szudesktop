package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func bookingFixture(t *testing.T) (*Server, *int, *bool) {
	t.Helper()
	b := newBookingService()
	writes := 0
	occupied := false
	b.client.Transport = calendarTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Scheme == "http" && r.Header.Get("Cookie") != "" {
			t.Fatal("cookie leaked onto public HTTP request")
		}
		if r.Header.Get("Cookie") != "" && (r.URL.Scheme != "https" || r.URL.Host != "swzx.webvpn.szu.edu.cn") {
			t.Fatal("private destination changed")
		}
		var data any
		switch r.URL.Path {
		case "/venue-api/booth/info/1":
			data = map[string]any{"id": 1, "typeId": 1, "name": "测试会议室", "status": true}
		case "/venue-api/boothType/info/1":
			data = map[string]any{"availableTimePeriod": uint64(1)<<28 | uint64(1)<<29 | uint64(1)<<30 | uint64(1)<<31, "samePersonMaxReservationPerDay": 4, "lastReservationDayBeforeAppointment": 3}
		case "/venue-api/booth/1/available-time":
			times := make([]int, 48)
			times[28] = 1
			times[29] = -1
			times[31] = 7
			if occupied {
				times[28] = 0
			}
			data = []any{map[string]any{"date": r.URL.Query().Get("startDate"), "times": times}}
		case "/venue-api/boothReservation/my":
			data = map[string]any{"list": []any{}, "total": 0}
		case "/venue-api/boothReservation/add":
			writes++
			data = true
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		body, _ := json.Marshal(map[string]any{"status": 200, "data": data})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Request: r}, nil
	})
	return &Server{booking: b}, &writes, &occupied
}
func bookingCall(handler http.HandlerFunc, method, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://127.0.0.1/api/booking", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	protectAPI(handler, method)(w, r)
	return w
}
func TestBookingPublicSlotsAndPrivateIsolation(t *testing.T) {
	s, _, _ := bookingFixture(t)
	s.booking.cookie = "test=secret"
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	day, err := s.booking.availability(context.Background(), 1, "2026-09-21", now, false)
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
	if _, err = s.booking.availability(context.Background(), 1, "2026-09-23", now, false); err == nil {
		t.Fatal("beyond booking window accepted")
	}
	if bookingToday(time.Date(2026, 9, 19, 17, 0, 0, 0, time.UTC)).Format("2006-01-02") != "2026-09-20" {
		t.Fatal("not Shenzhen date")
	}
}
func TestBookingPrepareCommitOnceAndAvailabilityRace(t *testing.T) {
	for _, race := range []bool{false, true} {
		t.Run(fmt.Sprint(race), func(t *testing.T) {
			s, writes, occupied := bookingFixture(t)
			login := bookingCall(s.handleBookingSession, "POST", `{"cookie":"test=secret"}`)
			if login.Code != 200 || strings.Contains(login.Body.String(), "secret") {
				t.Fatal("session not validated safely")
			}
			date := bookingToday(time.Now()).AddDate(0, 0, 1).Format("2006-01-02")
			p := bookingCall(s.handleBookingPrepare, "POST", fmt.Sprintf(`{"boothId":1,"phone":"13800000000","grade":"2025","date":%q,"timeList":[28],"agree":true}`, date))
			if p.Code != 200 || *writes != 0 {
				t.Fatal("prepare must never write", p.Code, p.Body.String())
			}
			var reply struct {
				Token string `json:"token"`
			}
			json.Unmarshal(p.Body.Bytes(), &reply)
			*occupied = race
			payload := fmt.Sprintf(`{"token":%q}`, reply.Token)
			c := bookingCall(s.handleBookingCommit, "POST", payload)
			if race {
				if c.Code != 409 || *writes != 0 {
					t.Fatal("occupied slot submitted")
				}
			} else if c.Code != 200 || *writes != 1 {
				t.Fatal("explicit commit failed", c.Code)
			}
			if bookingCall(s.handleBookingCommit, "POST", payload).Code != 409 || *writes > 1 {
				t.Fatal("commit replay sent to school")
			}
			bookingCall(s.handleBookingSession, "DELETE", "")
			if s.booking.cookie != "" || s.booking.pending != nil {
				t.Fatal("clear retained personal state")
			}
		})
	}
}
func TestBookingFailuresNeverBecomeEmptyOrSuccessful(t *testing.T) {
	for _, body := range []string{`<html>登录</html>`, `{"status":200,"data":null}`, `{"status":200,"data":{"list":[],"total":null}}`, `{"status":200,"encoding":1,"data":"opaque"}`} {
		s, _, _ := bookingFixture(t)
		s.booking.cookie = "test=secret"
		s.booking.client.Transport = calendarTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
		})
		if _, err := s.booking.history(context.Background(), 1); err == nil {
			t.Fatal("unrecognized response accepted", body)
		}
	}
	s, _, _ := bookingFixture(t)
	s.booking.cookie = "test=secret"
	s.booking.pending = &bookingPending{Token: "once", Expires: time.Now().Add(time.Minute), Input: bookingInput{RoomID: 1, Date: bookingToday(time.Now()).AddDate(0, 0, 1).Format("2006-01-02"), Times: []int{28}, Phone: "13800000000", Grade: "2025", Agree: true}}
	base := s.booking.client.Transport
	writes := 0
	s.booking.client.Transport = calendarTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == http.MethodPost {
			writes++
			return nil, errors.New("test network timeout")
		}
		return base.RoundTrip(r)
	})
	w := bookingCall(s.handleBookingCommit, "POST", `{"token":"once"}`)
	if w.Code != 502 || !strings.Contains(w.Body.String(), "不要直接重复提交") || writes != 1 {
		t.Fatal("uncertain write falsely succeeded")
	}
	if bookingCall(s.handleBookingCommit, "POST", `{"token":"once"}`).Code != 409 || writes != 1 {
		t.Fatal("uncertain write retried")
	}
}
func TestBookingRejectsUnconfirmedAndDuplicateSlots(t *testing.T) {
	day := &bookingDay{Room: bookingRoom{Type: bookingType{Max: 4}}, Slots: []bookingSlot{{Index: 28, State: "available"}}}
	p := bookingInput{Phone: "13800000000", Grade: "2025", Times: []int{28}}
	if validateBooking(p, day) == nil {
		t.Fatal("missing rule consent accepted")
	}
	p.Agree = true
	p.Times = []int{28, 28}
	if validateBooking(p, day) == nil {
		t.Fatal("duplicate slots accepted")
	}
}
