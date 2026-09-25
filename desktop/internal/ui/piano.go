package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 深大某学院「琴房管理预约系统」。前端页面在 8080，接口在 60837，都是校园网内网地址。
// 和 booking.go 的 bookingPublic 一样：根地址是默认值，学校若更换需随版本更新。
// 做成字段（而非常量）是为了让测试能把 base 指到 httptest 假服务器。
const pianoDefaultBase = "http://192.168.197.131:60837/api/web/"

// 琴房系统把 token 放在 URL query 里传（它自己的前端就是这么设计的，我们只是转发）。
// 这意味着 token 可能出现在对方的访问日志里——是对方系统的设计，我们无法改变，
// 因此这里只缓存 token、绝不缓存卡号密码。
var (
	errPianoUnreachable = errors.New("连不上琴房系统。它在校园网内网：请确认当前在校园网内；若本机开着 Clash/TUN 等代理，需给 192.168.197.0/24 加直连规则，否则流量会被代理转出校园网")
	errPianoNeedLogin   = errors.New("琴房登录已失效，请重新登录")
	errPianoFormat      = errors.New("琴房系统返回的数据格式变化，本次未显示，请到原系统页面核对")
)

// pianoService 只做「登录换 token + 只读查询」。
// 预约(roomReserve/insert)、取消(cancel)、开门(udp/openRoom) 等写操作一律不提供通路，
// 与 booking.go「办理在官方页面完成」同一立场。
// 卡号密码不落盘：只在登录请求里出现一次，换回的 token 仅存内存。
type pianoService struct {
	base   string
	mu     sync.Mutex
	token  string
	client *http.Client
}

func newPianoService() *pianoService {
	return &pianoService{base: pianoDefaultBase, client: &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (p *pianoService) currentToken() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.token
}
func (p *pianoService) setToken(t string) {
	p.mu.Lock()
	p.token = t
	p.mu.Unlock()
}

// pianoEnvelope 是琴房系统统一的响应包装：{code, success, message, data, count?}。
// 列表类接口把总数放在与 data 同级的 count 里。
type pianoEnvelope struct {
	Code    int             `json:"code"`
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
	Count   *int            `json:"count"`
}

// call 发一个请求。body 非 nil 时以 JSON POST，否则 GET；withToken 时把缓存的
// token 拼到 query。返回解析后的信封；code==401 时返回 errPianoNeedLogin。
func (p *pianoService) call(ctx context.Context, path string, body any, withToken bool) (*pianoEnvelope, error) {
	u := p.base + path
	q := url.Values{}
	if withToken {
		tok := p.currentToken()
		if tok == "" {
			return nil, errPianoNeedLogin
		}
		q.Set("token", tok)
	}
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	var rdr io.Reader
	method := http.MethodGet
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rdr = bytes.NewReader(b)
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := p.client.Do(req)
	if err != nil {
		return nil, errPianoUnreachable
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, ehallMaxBody+1))
	if err != nil || len(data) > ehallMaxBody {
		return nil, errPianoFormat
	}
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("琴房系统返回 HTTP %d", res.StatusCode)
	}
	var env pianoEnvelope
	if json.Unmarshal(data, &env) != nil {
		return nil, errPianoFormat
	}
	if env.Code == 401 {
		return nil, errPianoNeedLogin
	}
	if env.Code != 200 || !env.Success {
		msg := strings.TrimSpace(env.Message)
		if msg == "" {
			msg = "未知错误"
		}
		return nil, fmt.Errorf("琴房系统：%s", msg)
	}
	return &env, nil
}

// login 用卡号+密码换 token。凭据只在这一次请求里出现，成功后只留 token。
func (p *pianoService) login(ctx context.Context, cardNo, pwd string) error {
	cardNo = strings.TrimSpace(cardNo)
	if cardNo == "" || pwd == "" {
		return errors.New("请填写卡号和密码")
	}
	env, err := p.call(ctx, "user/login", map[string]string{"cardNo": cardNo, "pwd": pwd}, false)
	if err != nil {
		return err
	}
	var data struct {
		Token string `json:"token"`
	}
	if json.Unmarshal(env.Data, &data) != nil || data.Token == "" {
		return errPianoFormat
	}
	p.setToken(data.Token)
	return nil
}

func (p *pianoService) logout() { p.setToken("") }
func (p *pianoService) loggedIn() bool { return p.currentToken() != "" }

// 下面这些 view 结构是「归一化后的展示形状」。服务端真实键名是从前端打包产物里
// 推出来的（Element 表格的 prop 即 JSON key），为防键名/类型漂移导致整页解码失败，
// 先解成 map 再按别名取值、文本消毒，最后收敛成固定形状；取不到的字段给空串，
// 前端显示「—」，等于如实说「这项未知」。
type pianoRoomView struct {
	Name    string `json:"name"`
	Device  string `json:"device"`
	Manager string `json:"manager"`
	Desc    string `json:"desc"`
}
type pianoReserveView struct {
	Room string `json:"room"`
	Time string `json:"time"`
	Sign string `json:"sign"`
}

// pickStr 按候选键顺序取第一个非空字符串并消毒。值不是字符串（比如数字）时转成文本。
func pickStr(m map[string]json.RawMessage, keys ...string) string {
	for _, k := range keys {
		raw, ok := m[k]
		if !ok || len(raw) == 0 || string(raw) == "null" {
			continue
		}
		var s string
		if json.Unmarshal(raw, &s) == nil {
			s = strings.TrimSpace(s)
			if s != "" {
				return pianoText(s)
			}
			continue
		}
		var f float64
		if json.Unmarshal(raw, &f) == nil {
			return strconv.FormatFloat(f, 'f', -1, 64)
		}
		var b bool
		if json.Unmarshal(raw, &b) == nil {
			if b {
				return "是"
			}
			return "否"
		}
	}
	return ""
}

// pianoText 把学校返回的文本收敛成安全纯文本：复用 booking.go 里已测过的
// plainTextFromSchoolHTML（剥标签、丢 script/style、解实体、收敛空白），再截断。
// 只把尖括号换成空格是不够的——标签名会作为正文留下来。
func pianoText(s string) string {
	s = plainTextFromSchoolHTML(s)
	if r := []rune(s); len(r) > 200 {
		s = string(r[:200]) + "…"
	}
	return s
}

// signLabel 把 isSign 的各种可能取值收敛成可读状态。
func signLabel(m map[string]json.RawMessage) string {
	raw, ok := m["isSign"]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		if b {
			return "已签到"
		}
		return "未签到"
	}
	var n float64
	if json.Unmarshal(raw, &n) == nil {
		if n == 1 {
			return "已签到"
		}
		if n == 0 {
			return "未签到"
		}
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	return pickStr(m, "isSign")
}

func decodeRows(env *pianoEnvelope) ([]map[string]json.RawMessage, error) {
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return nil, nil
	}
	var rows []map[string]json.RawMessage
	if json.Unmarshal(env.Data, &rows) != nil {
		return nil, errPianoFormat
	}
	return rows, nil
}

// rooms 拉琴房列表（只读）。pageIndex 从 1 起。
func (p *pianoService) rooms(ctx context.Context, pageIndex, pageSize int) ([]pianoRoomView, int, error) {
	if pageIndex < 1 {
		pageIndex = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	env, err := p.call(ctx, "room/findPage", map[string]any{"pageIndex": pageIndex, "pageSize": pageSize}, true)
	if err != nil {
		return nil, 0, err
	}
	rows, err := decodeRows(env)
	if err != nil {
		return nil, 0, err
	}
	out := make([]pianoRoomView, 0, len(rows))
	for _, m := range rows {
		out = append(out, pianoRoomView{
			Name:    pickStr(m, "roomName", "name"),
			Device:  pickStr(m, "deviceNo"),
			Manager: pickStr(m, "userName", "custodian"),
			Desc:    pickStr(m, "description"),
		})
	}
	count := len(out)
	if env.Count != nil {
		count = *env.Count
	}
	return out, count, nil
}

// myReserves 拉「我的预约」（只读）。
func (p *pianoService) myReserves(ctx context.Context) ([]pianoReserveView, error) {
	env, err := p.call(ctx, "roomReserve/myReserve", map[string]any{"pageIndex": 1, "pageSize": 50}, true)
	if err != nil {
		return nil, err
	}
	rows, err := decodeRows(env)
	if err != nil {
		return nil, err
	}
	out := make([]pianoReserveView, 0, len(rows))
	for _, m := range rows {
		out = append(out, pianoReserveView{
			Room: pickStr(m, "roomName", "name"),
			Time: pickStr(m, "reserveTime", "time"),
			Sign: signLabel(m),
		})
	}
	return out, nil
}

/* ---------- 接口 ---------- */

func writePianoError(w http.ResponseWriter, err error) {
	code := 502
	switch {
	case errors.Is(err, errPianoNeedLogin):
		code = 401
	case errors.Is(err, errPianoUnreachable):
		code = 502
	default:
		code = 502
	}
	writeAPIError(w, code, err)
}

func (s *Server) handlePianoStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"logged_in": s.piano.loggedIn(), "base": s.piano.base})
}

func (s *Server) handlePianoLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CardNo string `json:"cardNo"`
		Pwd    string `json:"pwd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAPIError(w, 400, errors.New("请求格式不对"))
		return
	}
	if err := s.piano.login(r.Context(), body.CardNo, body.Pwd); err != nil {
		writePianoError(w, err)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handlePianoLogout(w http.ResponseWriter, r *http.Request) {
	s.piano.logout()
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handlePianoRooms(w http.ResponseWriter, r *http.Request) {
	pageIndex, _ := strconv.Atoi(r.URL.Query().Get("pageIndex"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	rooms, count, err := s.piano.rooms(r.Context(), pageIndex, pageSize)
	if err != nil {
		writePianoError(w, err)
		return
	}
	writeJSON(w, map[string]any{"rooms": rooms, "count": count})
}

func (s *Server) handlePianoMy(w http.ResponseWriter, r *http.Request) {
	list, err := s.piano.myReserves(r.Context())
	if err != nil {
		writePianoError(w, err)
		return
	}
	writeJSON(w, map[string]any{"list": list})
}
