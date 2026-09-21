package ui

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Alakazamc/szudesktop/internal/autostart"
)

// 这些测试一律注入替身，绝不碰真实注册表：
// 跑一次测试就改掉开发机或 CI 机器的开机启动项是不可接受的。
type autostartRecorder struct {
	state  autostart.State
	setErr error
	calls  []bool
}

func (r *autostartRecorder) backend() *autostartBackend {
	return &autostartBackend{
		status: func() autostart.State { return r.state },
		set: func(on bool) error {
			r.calls = append(r.calls, on)
			if r.setErr != nil {
				return r.setErr
			}
			r.state = autostart.State{Supported: true, Enabled: on, Detail: "已开启"}
			return nil
		},
	}
}

func autostartRequest(rec *autostartRecorder, method, origin, contentType, body string) *httptest.ResponseRecorder {
	s := &Server{store: &guardTestStore{}, autostartTest: rec.backend()}
	mux := http.NewServeMux()
	s.routes(mux, fstest.MapFS{})
	req := httptest.NewRequest(method, "http://127.0.0.1:1234/api/autostart", strings.NewReader(body))
	req.Header.Set("Origin", origin)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func TestAutostartRouteRejectsForeignAndMalformedRequests(t *testing.T) {
	for _, tc := range []struct {
		name, method, origin, contentType, body string
		want                                    int
	}{
		{"cross origin toggle", "POST", "https://example.com", "application/json", `{"enabled":true}`, http.StatusForbidden},
		{"unsupported method", "PUT", "", "application/json", `{"enabled":true}`, http.StatusMethodNotAllowed},
		{"missing enabled", "POST", "", "application/json", `{}`, http.StatusBadRequest},
		{"broken json", "POST", "", "application/json", `{"enabled":`, http.StatusBadRequest},
		{"form body", "POST", "", "text/plain", `enabled=true`, http.StatusUnsupportedMediaType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &autostartRecorder{state: autostart.State{Supported: true, Detail: "未开启"}}
			w := autostartRequest(rec, tc.method, tc.origin, tc.contentType, tc.body)
			if w.Code != tc.want {
				t.Fatalf("status=%d body=%s, want %d", w.Code, w.Body.String(), tc.want)
			}
			if len(rec.calls) != 0 {
				t.Fatalf("被拒绝的请求改动了开机自启: %v", rec.calls)
			}
		})
	}
}

func TestAutostartToggleReportsRealResult(t *testing.T) {
	rec := &autostartRecorder{state: autostart.State{Supported: true, Detail: "未开启"}}

	w := autostartRequest(rec, "POST", "", "application/json", `{"enabled":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("打开失败: %d %s", w.Code, w.Body.String())
	}
	var got autostart.State
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if !got.Enabled {
		t.Fatal("打开后必须回报已开启，不能只说操作完成")
	}

	w = autostartRequest(rec, "POST", "", "application/json", `{"enabled":false}`)
	if w.Code != http.StatusOK {
		t.Fatalf("关掉失败: %d %s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if got.Enabled {
		t.Fatal("关掉后仍回报已开启")
	}
	if len(rec.calls) != 2 || rec.calls[0] != true || rec.calls[1] != false {
		t.Fatalf("开关调用顺序不对: %v", rec.calls)
	}
}

func TestAutostartFailureIsNotReportedAsSuccess(t *testing.T) {
	rec := &autostartRecorder{
		state:  autostart.State{Supported: true, Detail: "未开启"},
		setErr: errors.New("注册表被组策略锁定"),
	}
	w := autostartRequest(rec, "POST", "", "application/json", `{"enabled":true}`)
	if w.Code == http.StatusOK {
		t.Fatalf("写失败却回报成功: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "注册表被组策略锁定") {
		t.Fatalf("失败原因没有告诉用户: %s", w.Body.String())
	}
}

// 读不到状态时不能显示成「未开启」，否则用户会以为开关没生效，反复点。
func TestAutostartUnknownStateIsNotShownAsDisabled(t *testing.T) {
	rec := &autostartRecorder{state: autostart.State{
		Supported: true,
		Detail:    "状态未知：打不开注册表启动项",
		Error:     "打不开注册表启动项",
	}}
	w := autostartRequest(rec, "GET", "", "", "")
	if w.Code != http.StatusOK {
		t.Fatalf("读取状态失败: %d", w.Code)
	}
	var got autostart.State
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("响应不是合法 JSON: %v", err)
	}
	if got.Error == "" {
		t.Fatal("读不到状态时必须带上原因，不能默默显示未开启")
	}
	if got.Enabled {
		t.Fatal("读不到状态却被报成已开启")
	}
	if !strings.Contains(got.Detail, "状态未知") {
		t.Fatalf("界面文案没有说明状态未知: %q", got.Detail)
	}
}
