package ui

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

type workspaceSnapshot struct {
	Version  int             `json:"version"`
	Revision uint64          `json:"revision"`
	Data     json.RawMessage `json:"data"`
}
type workspaceStore struct {
	mu   sync.Mutex
	path string
}

func newWorkspaceStore() *workspaceStore {
	dir := os.Getenv("SZUNET_CONFIG_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			dir = filepath.Join(home, ".szunet")
		}
	}
	return &workspaceStore{path: filepath.Join(dir, "workspace-v1.json")}
}
func (s *workspaceStore) read() (workspaceSnapshot, error) {
	empty := workspaceSnapshot{Version: 1, Data: json.RawMessage(`null`)}
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return empty, nil
	}
	if err != nil {
		return empty, err
	}
	var v workspaceSnapshot
	if err = json.Unmarshal(b, &v); err != nil {
		return empty, errors.New("本机存档无法读取，原文件已保留，请先备份后再处理")
	}
	if v.Version != 1 {
		return empty, errors.New("存档版本不兼容，原文件已保留")
	}
	return v, nil
}
func (s *Server) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	s.workspace.mu.Lock()
	defer s.workspace.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.workspace.path), 0700); err != nil {
		writeAPIError(w, 500, err)
		return
	}
	unlock, err := lockWorkspace(s.workspace.path + ".lock")
	if err != nil {
		writeAPIError(w, 503, errors.New("存档正在被另一个窗口使用，请稍后重试"))
		return
	}
	defer unlock()
	current, err := s.workspace.read()
	if err != nil {
		writeAPIError(w, 500, err)
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, current)
		return
	}
	var incoming workspaceSnapshot
	dec := json.NewDecoder(io.LimitReader(r.Body, 512<<10))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&incoming); err != nil || incoming.Version != 1 || len(incoming.Data) == 0 || string(incoming.Data) == "null" {
		writeAPIError(w, 400, errors.New("存档格式不正确"))
		return
	}
	var trailing any
	if dec.Decode(&trailing) != io.EOF {
		writeAPIError(w, 400, errors.New("存档不能包含额外内容"))
		return
	}
	if incoming.Revision != current.Revision {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusConflict)
		writeJSON(w, map[string]any{"ok": false, "message": "另一个窗口更新了存档，请重新加载后继续", "revision": current.Revision})
		return
	}
	incoming.Revision++
	b, err := json.Marshal(incoming)
	if err != nil {
		writeAPIError(w, 400, err)
		return
	}
	if err = os.MkdirAll(filepath.Dir(s.workspace.path), 0700); err != nil {
		writeAPIError(w, 500, err)
		return
	}
	f, err := os.CreateTemp(filepath.Dir(s.workspace.path), ".workspace-*.tmp")
	if err != nil {
		writeAPIError(w, 500, err)
		return
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp, s.workspace.path)
	}
	if err != nil {
		writeAPIError(w, 500, errors.New("存档未写入，原存档保留，请检查磁盘空间或权限"))
		return
	}
	writeJSON(w, incoming)
}
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]bool{"ok": true})
	if s.shutdown != nil {
		go s.shutdown()
	}
}
