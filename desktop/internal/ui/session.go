package ui

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Alakazamc/szudesktop/internal/credential"
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

// handleSessionCheck 用保存的会话真打一次学校系统，验证它还有效。
//
// 这一步就是「会话桥最小验证」：用户刚粘完就能知道对不对，
// 而不是等到用成绩功能时才发现会话是坏的。
//
// ⚠️ 探活刻意选「本科成绩」这个接口，判断依据是响应能不能解析出 rows。
// 不拿「请求返回 200」当有效——ehall 在会话失效时可能仍返回 200 加一段登录页，
// 那正是「读不到却报成功」的坑。这里只看有没有真的拿到结构化数据。
func (s *Server) handleSessionCheck(w http.ResponseWriter, r *http.Request) {
	v, err := s.sessionStore().Load()
	if err != nil {
		writeAPIError(w, 409, errors.New("还没有保存学校系统登录状态"))
		return
	}
	c := newEhallClient(v.Cookie, sessionSaveTimeout)
	// 只取一页一条，够用来判断会话通不通，不拉走用户的整份成绩。
	body, err := c.postForm(undergradScorePath, allRowsForm(1))
	if err != nil {
		status := 502
		if errors.Is(err, errSessionInvalid) {
			status = 401
		}
		writeAPIError(w, status, err)
		return
	}
	if _, err := ehallRows(body, "xscjcx"); err != nil {
		status := 502
		if errors.Is(err, errSessionInvalid) {
			status = 401
		}
		writeAPIError(w, status, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "message": "登录状态可用"})
}

type scoreResp struct {
	*scoreResult
	Stale bool `json:"stale,omitempty"`
}

// handleScores 读取成绩。level=undergrad|graduate。
//
// 只读。不缓存到磁盘：成绩属于个人信息，没必要在本机多留一份。
func (s *Server) handleScores(w http.ResponseWriter, r *http.Request) {
	level := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("level")))
	if level == "" {
		level = "undergrad"
	}

	var reader scoreReader
	switch level {
	case "undergrad", "undergraduate", "本科":
		reader = scoreReaderFunc(readUndergradScore)
	case "graduate", "grad", "研究生":
		reader = scoreReaderFunc(readGradScore)
	default:
		writeAPIError(w, 400, errors.New("请选择本科或研究生"))
		return
	}

	v, err := s.sessionStore().Load()
	if err != nil {
		writeAPIError(w, 409, errors.New("还没有学校系统的登录状态。请先在浏览器登录 ehall，再把 Cookie 粘贴到设置里"))
		return
	}

	client := newEhallClient(v.Cookie, sessionSaveTimeout)
	result, err := reader.Read(client)
	if err != nil {
		status := 502
		if errors.Is(err, errSessionInvalid) {
			status = 401
		}
		writeAPIError(w, status, err)
		return
	}
	writeJSON(w, result)
}

// scoreReaderFunc 让普通函数满足 scoreReader 接口，方便测试替身。
type scoreReaderFunc func(*ehallClient) (*scoreResult, error)

func (f scoreReaderFunc) Read(c *ehallClient) (*scoreResult, error) { return f(c) }

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
