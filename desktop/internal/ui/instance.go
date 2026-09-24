package ui

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type instanceRecord struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}
type desktopInstance struct {
	instanceRecord
	path    string
	release func()
}

// The OS lock survives stale discovery files, but is released after a crash.
func acquireInstance(dir string, open bool) (*desktopInstance, bool, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, false, err
	}
	release, owned, err := tryInstanceLock(filepath.Join(dir, "desktop-instance.lock"))
	if err != nil {
		return nil, false, err
	}
	path := filepath.Join(dir, "desktop-instance.json")
	if owned {
		token := make([]byte, 32)
		if _, err = rand.Read(token); err != nil {
			release()
			return nil, false, err
		}
		return &desktopInstance{instanceRecord: instanceRecord{Token: hex.EncodeToString(token)}, path: path, release: release}, false, nil
	}
	// The first process may still be starting and publishing its endpoint.
	for i := 0; i < 40; i++ {
		if data, err := os.ReadFile(path); err == nil {
			var record instanceRecord
			if json.Unmarshal(data, &record) == nil && activateInstance(record, open) == nil {
				// Return the authenticated endpoint for an enclosing desktop shell.
				// This borrowed record owns neither the lock nor the running service.
				return &desktopInstance{instanceRecord: record}, true, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, false, errors.New("应用正在启动或退出，请稍后再打开；不会另外启动一份后台服务")
}
func (i *desktopInstance) publish(address string) error {
	i.URL = address
	data, err := json.Marshal(i.instanceRecord)
	if err != nil {
		return err
	}
	return os.WriteFile(i.path, data, 0600)
}
func (i *desktopInstance) close() { _ = os.Remove(i.path); i.release() }
func activateInstance(record instanceRecord, open bool) error {
	u, err := url.Parse(record.URL)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.Port() == "" {
		return errors.New("无效的本机地址")
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() || len(record.Token) != 64 {
		return errors.New("无效的本机实例")
	}
	body, _ := json.Marshal(map[string]any{"token": record.Token, "open": open})
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Post(record.URL+"/api/instance", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("实例响应 %d", response.StatusCode)
	}
	return nil
}
func (s *Server) handleInstance(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
		Open  bool   `json:"open"`
	}
	if s.instance == nil || json.NewDecoder(r.Body).Decode(&in) != nil || subtle.ConstantTimeCompare([]byte(in.Token), []byte(s.instance.Token)) != 1 {
		http.Error(w, "实例校验失败", http.StatusForbidden)
		return
	}
	s.windows.reopen(time.Now())
	if in.Open {
		if err := openBrowser(s.instance.URL); err != nil {
			writeAPIError(w, 500, err)
			return
		}
	}
	writeJSON(w, map[string]bool{"ok": true})
}
