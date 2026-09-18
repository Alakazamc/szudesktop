package ui

import (
	"testing"
	"time"
)

func TestWindowCloseReloadAndMultipleWindows(t *testing.T) {
	now := time.Now()
	s := newWindowSessions()
	if s.shouldExit(now) {
		t.Fatal("headless server exited")
	}
	s.touch("a", false, now)
	s.touch("b", false, now)
	s.touch("a", true, now)
	if s.shouldExit(now.Add(windowGrace)) {
		t.Fatal("closed with another window alive")
	}
	s.touch("b", true, now.Add(windowGrace))
	if s.shouldExit(now.Add(2*windowGrace - time.Second)) {
		t.Fatal("no reload grace")
	}
	s.touch("replacement", false, now.Add(2*windowGrace))
	if s.shouldExit(now.Add(2 * windowGrace)) {
		t.Fatal("reload closed service")
	}
	s.touch("replacement", true, now.Add(2*windowGrace))
	if !s.shouldExit(now.Add(3 * windowGrace)) {
		t.Fatal("last close did not exit")
	}
}
func TestWindowLostHeartbeatAndResume(t *testing.T) {
	now := time.Now()
	s := newWindowSessions()
	s.touch("a", false, now)
	for i := 0; i <= int(windowTTL/time.Second); i += 2 {
		if s.shouldExit(now.Add(time.Duration(i) * time.Second)) {
			t.Fatal("exited before expiry grace")
		}
	}
	if s.shouldExit(now.Add(windowTTL + 2*time.Second)) {
		t.Fatal("no expiry grace")
	}
	if !s.shouldExit(now.Add(windowTTL + windowGrace + 2*time.Second)) {
		t.Fatal("lost window stayed alive")
	}
	s = newWindowSessions()
	s.touch("a", false, now)
	s.shouldExit(now)
	if s.shouldExit(now.Add(8 * time.Hour)) {
		t.Fatal("system sleep killed live window")
	}
	if s.shouldExit(now.Add(8*time.Hour + 10*time.Second)) {
		t.Fatal("resume gave no heartbeat grace")
	}
}
func TestReopenDuringCloseGrace(t *testing.T) {
	now := time.Now()
	s := newWindowSessions()
	s.touch("a", false, now)
	s.touch("a", true, now)
	s.shouldExit(now)
	s.reopen(now.Add(9 * time.Second))
	if s.shouldExit(now.Add(11 * time.Second)) {
		t.Fatal("reopening lost the server")
	}
	s.touch("b", false, now.Add(20*time.Second))
	if s.shouldExit(now.Add(21 * time.Second)) {
		t.Fatal("replacement window not kept alive")
	}
}

func TestClosedWindowIgnoresLateHeartbeat(t *testing.T) {
	now := time.Now()
	s := newWindowSessions()
	s.touch("a", false, now)
	s.touch("a", true, now)
	s.touch("a", false, now.Add(time.Second))
	if !s.shouldExit(now.Add(windowGrace)) {
		t.Fatal("delayed heartbeat resurrected a closed window")
	}
}
