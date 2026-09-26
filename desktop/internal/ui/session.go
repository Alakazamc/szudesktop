package ui

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/SzuDesktopTeam/szudesktop/internal/credential"
)

// 学校系统（ehall）个人业务的本地接口。
//
// 设计要点：
//
//   - 会话由用户从浏览器交过来，落在系统安全存储里（和校园网密码同一规格），
//     不写明文、不进日志、不进发布包。用户可以随时单独清除。
//   - 所有读取都是只读。写操作（比如提交预约）必须由用户确认，本模块不代提交。
//   - 会话失效要报得明明白白，不能退化成「暂无数据」。
//
// 校园网凭据和学校系统会话是两件事：
// 前者是联网用的账号密码，后者是浏览器里的登录状态。
// 所以分两个接口、两份存储，用户清其中一个不会影响另一个。

const (
	sessionSaveTimeout = 25 * time.Second
	// 粘贴的 Cookie 长度上限。正常 ehall 会话也就几百到一两千字符，
	// 给足余量但仍然拦住明显异常的超长输入。
	maxCookieLen = 8 << 10
)

// sessionStore 在测试里替换成内存实现。
func (s *Server) sessionStore() credential.SessionStore {
	if s.session != nil {
		return s.session
	}
	return credential.DefaultSession()
}

// sanitizeCookie 把用户粘进来的东西收拾干净。
//
// 用户从浏览器复制时经常带上这些：
//   - 整行 "Cookie: a=b; c=d"（带了前缀）
//   - 从 document.cookie 复制的、值里带换行
//   - 顺手粘进了别的请求头
//
// 这里只做机械清理，不猜、不构造。清完还是空就报错。
func sanitizeCookie(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "Cookie:")
	s = strings.TrimPrefix(s, "cookie:")
	// 用户可能把整个请求头块粘进来，只取到第一个空行为止。
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("请把浏览器里的 Cookie 复制完整")
	}
	if len(s) > maxCookieLen {
		return "", errors.New("这段内容太长了，看着不像 Cookie。请只复制 Cookie 的值")
	}
	// 至少要有一个 name=value，否则八成是粘错了东西。
	if !strings.Contains(s, "=") {
		return "", errors.New("这段内容里没有 Cookie 的键值对，请确认复制的是 Cookie 值")
	}
	return s, nil
}

type sessionStatusResp struct {
	Saved     bool   `json:"saved"`
	StoreDesc string `json:"store_desc"`
	Note      string `json:"note,omitempty"`
	// 只回报长度，不回报内容——状态接口不该把会话本身吐出去。
	CookieLen int `json:"cookie_len,omitempty"`
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	store := s.sessionStore()
	switch r.Method {
	case http.MethodGet:
		v, err := store.Load()
		if err != nil {
			if !errors.Is(err, credential.ErrSessionNotFound) {
				writeAPIError(w, 503, err)
				return
			}
			writeJSON(w, sessionStatusResp{Saved: false, StoreDesc: store.Describe()})
			return
		}
		writeJSON(w, sessionStatusResp{
			Saved:     true,
			StoreDesc: store.Describe(),
			Note:      v.Note,
			CookieLen: len(v.Cookie),
		})

	case http.MethodPost:
		var req struct {
			Cookie string `json:"cookie"`
			Note   string `json:"note"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAPIError(w, 400, errors.New("请求格式不对"))
			return
		}
		cookie, err := sanitizeCookie(req.Cookie)
		if err != nil {
			writeAPIError(w, 400, err)
			return
		}
		if err := store.Save(credential.Session{Cookie: cookie, Note: strings.TrimSpace(req.Note)}); err != nil {
			writeAPIError(w, 500, err)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "store_desc": store.Describe()})

	case http.MethodDelete:
		if err := store.Delete(); err != nil {
			writeAPIError(w, 500, err)
			return
		}
		writeJSON(w, map[string]any{"ok": true})

	default:
		writeAPIError(w, 405, errors.New("不支持的方法"))
	}
}

// Probe only the selected business. Lack of access is not an expired session.
func (s *Server) handleSessionCheck(w http.ResponseWriter, r *http.Request) {
	app, err := selectScoreApp(r.URL.Query().Get("level"))
	if err != nil {
		writeAPIError(w, 400, err)
		return
	}
	v, err := s.sessionStore().Load()
	if err != nil {
		writeSessionLoadError(w, err)
		return
	}
	c := s.makeEhallClient(v.Cookie)
	body, err := c.postForm(app.Path, allRowsForm(1))
	if err == nil {
		_, err = ehallRows(body, app.Dataset)
	}
	if err != nil {
		writeSchoolError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "message": app.Label + "成绩业务可访问；其他业务权限需分别验证"})
}
func (s *Server) makeEhallClient(cookie string) *ehallClient {
	if s.ehallFactory != nil {
		return s.ehallFactory(cookie)
	}
	return newEhallClient(cookie, sessionSaveTimeout)
}
func writeSessionLoadError(w http.ResponseWriter, err error) {
	if errors.Is(err, credential.ErrSessionNotFound) {
		writeAPIError(w, 409, errors.New("还没有学校系统登录状态，请在学习工具中保存"))
		return
	}
	writeAPIError(w, 503, err)
}
func writeSchoolError(w http.ResponseWriter, err error) {
	status := 502
	if errors.Is(err, errSessionInvalid) {
		status = 401
	}
	if errors.Is(err, errSessionPermission) {
		status = 403
	}
	writeAPIError(w, status, err)
}

// handleScores 读取成绩。level=undergrad|graduate。
//
// 只读。不缓存到磁盘：成绩属于个人信息，没必要在本机多留一份。
func (s *Server) handleScores(w http.ResponseWriter, r *http.Request) {
	app, err := selectScoreApp(r.URL.Query().Get("level"))
	if err != nil {
		writeAPIError(w, 400, err)
		return
	}
	// 优先统一身份认证会话，回落到粘来的 ehall Cookie（迁移期间两条路并存）。
	c, err := s.schoolClient()
	if err != nil {
		writeSchoolClientError(w, err)
		return
	}
	result, err := readScore(c, app)
	if err != nil {
		writeSchoolError(w, err)
		return
	}
	writeJSON(w, result)
}

// allRowsForm 是「不加过滤条件、取第一页」的通用查询参数。
// ehall 这套框架用 querySetting 传过滤条件，空数组表示不过滤。
func allRowsForm(size int) url.Values {
	if size <= 0 {
		size = 1
	}
	return url.Values{
		"querySetting": {"[]"},
		"pageSize":     {strconv.Itoa(size)},
		"pageNumber":   {"1"},
	}
}
