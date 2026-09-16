package ui

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

// campusGateway 只保存未来校内后端的固定入口。
// 当前版本不做任意 URL 转发：等后端接口契约确定后，再按接口白名单补本地代理。
type campusGateway struct {
	baseURL string
}

func newCampusGateway(raw string) (*campusGateway, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return &campusGateway{}, nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil, errors.New("校内服务地址必须是 http:// 或 https:// 开头的完整地址")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("校内服务地址不能带账号、查询参数或片段")
	}
	return &campusGateway{baseURL: raw}, nil
}

func (g *campusGateway) status() map[string]any {
	if g == nil || g.baseURL == "" {
		return map[string]any{
			"configured":  false,
			"state":       "not_configured",
			"state_label": "未配置",
			"message":     "后续可在这里接入仅校园网可访问的后端服务",
		}
	}
	return map[string]any{
		"configured":  true,
		"state":       "configured",
		"state_label": "已配置",
		"base_url":    g.baseURL,
		"message":     "地址已由本地程序托管；具体接口接入后再启用本地代理",
	}
}

func (s *Server) handleCampusStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		writeAPIError(w, 405, errors.New("这个接口只接受 GET"))
		return
	}
	writeJSON(w, s.campus.status())
}
