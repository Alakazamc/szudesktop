package ui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"
)

const windowGrace = 10 * time.Second
const windowTTL = 3 * time.Minute

type windowSessions struct {
	mu           sync.Mutex
	active       map[string]time.Time
	closed       map[string]time.Time
	seen         bool
	emptySince   time.Time
	lastCheck    time.Time
	pendingUntil time.Time
}

func newWindowSessions() *windowSessions {
	return &windowSessions{active: make(map[string]time.Time), closed: make(map[string]time.Time)}
}
func (s *windowSessions) touch(id string, closing bool, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seen = true
	if closing {
		delete(s.active, id)
		s.closed[id] = now
	} else {
		if _, closed := s.closed[id]; closed {
			return
		}
		s.active[id] = now
		s.emptySince = time.Time{}
	}
	if len(s.active) == 0 && s.emptySince.IsZero() {
		s.emptySince = now
	}
}
func (s *windowSessions) reopen(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pendingUntil = now.Add(30 * time.Second)
}
func (s *windowSessions) shouldExit(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Give browser heartbeats time to recover after system sleep or suspension.
	if !s.lastCheck.IsZero() && now.Sub(s.lastCheck) > 30*time.Second {
		for id := range s.active {
			s.active[id] = now
		}
		if !s.emptySince.IsZero() {
			s.emptySince = now
		}
	}
	s.lastCheck = now
	for id, closed := range s.closed {
		if now.Sub(closed) > 2*windowTTL {
			delete(s.closed, id)
		}
	}
	for id, last := range s.active {
		if now.Sub(last) > windowTTL {
			delete(s.active, id)
		}
	}
	if len(s.active) > 0 {
		s.emptySince = time.Time{}
		return false
	}
	if !s.seen {
		return false
	} // Headless diagnostics have no browser to keep alive.
	if s.emptySince.IsZero() {
		s.emptySince = now
	}
	return !now.Before(s.pendingUntil) && now.Sub(s.emptySince) >= windowGrace
}
func (s *Server) handleWindow(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID      string `json:"id"`
		Closing bool   `json:"closing"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || len(in.ID) < 16 || len(in.ID) > 80 {
		http.Error(w, "无效的窗口", 400)
		return
	}
	s.windows.touch(in.ID, in.Closing, time.Now())
	writeJSON(w, map[string]bool{"ok": true})
}
func (s *Server) watchWindows(done <-chan struct{}) {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-done:
			return
		case now := <-tick.C:
			if s.windows.shouldExit(now) {
				s.shutdown()
				return
			}
		}
	}
}

// A live stream ends even when a browser closes without sending pagehide.
// Each connection owns a separate lease so an old disconnect cannot close a
// newer EventSource reconnection using the same page identifier.
func (s *Server) handleWindowStream(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if len(id) < 16 || len(id) > 80 {
		http.Error(w, "无效的窗口", 400)
		return
	}
	if _, ok := w.(http.Flusher); !ok {
		http.Error(w, "不支持窗口连接", 500)
		return
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		http.Error(w, "无法建立窗口连接", 500)
		return
	}
	key := id + "/" + hex.EncodeToString(nonce)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	control := http.NewResponseController(w)
	defer control.SetWriteDeadline(time.Time{})
	s.windows.touch(key, false, time.Now())
	defer func() { s.windows.touch(key, true, time.Now()) }()
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for {
		_ = control.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := io.WriteString(w, ": alive\n\n"); err != nil {
			return
		}
		if err := control.Flush(); err != nil {
			return
		}
		s.windows.touch(key, false, time.Now())
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
		}
	}
}
