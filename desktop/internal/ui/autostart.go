package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Alakazamc/szudesktop/internal/autostart"
)

// autostartBackend 是可替换的开机自启操作。
//
// 生产环境直接读写注册表；测试必须注入假的，否则跑一次 go test
// 就会改掉开发机和 CI 机器上真实的开机启动项。
type autostartBackend struct {
	status func() autostart.State
	set    func(on bool) error
}

func realAutostart() autostartBackend {
	return autostartBackend{
		status: autostart.Status,
		set: func(on bool) error {
			if on {
				// 桌面版登记界面程序自己：开机静默起服务，不弹窗口。
				return autostart.Enable(false)
			}
			return autostart.Disable()
		},
	}
}

// handleAutostart 报告并切换「登录 Windows 时自动运行」。
//
// GET 读当前状态；POST {"enabled":true|false} 打开或关掉。
// 状态读不出来时如实说读不出来，不显示成「未开启」——
// 那会让用户以为开关没生效，反复点。
func (s *Server) handleAutostart(w http.ResponseWriter, r *http.Request) {
	backend := realAutostart()
	if s.autostartTest != nil {
		backend = *s.autostartTest
	}

	if r.Method == http.MethodGet {
		writeJSON(w, backend.status())
		return
	}

	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, errors.New("请求不是合法的 JSON"))
		return
	}
	if req.Enabled == nil {
		writeAPIError(w, http.StatusBadRequest, errors.New("要写清楚 enabled 是 true 还是 false"))
		return
	}
	if err := backend.set(*req.Enabled); err != nil {
		writeAPIError(w, http.StatusInternalServerError, fmt.Errorf("改动开机自启失败: %w", err))
		return
	}
	writeJSON(w, backend.status())
}
