package ui

import (
	"context"
	"crypto/des"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

const graduateRoot = ehallBaseURL + "/yjsxkapp/sys/xsxkapp"
const graduateHome = graduateRoot + "/*default/index.do"
const graduatePublicPath = graduateRoot + "/xsxkHome/loadPublicInfo.do"
const graduateProfilePath = graduateRoot + "/xsxkHome/loadStdInfo.do"
const graduateTablePath = graduateRoot + "/xsxkCourse/loadKbxx.do"

// Academic credentials and cookies live only in this process. They are separate
// from both the network password and the manually imported grade session.
type academicService struct {
	mu            sync.Mutex
	client        *http.Client
	challenge     string
	vtoken        string
	captcha       []byte
	expires       time.Time
	authenticated bool
}

func newAcademicService() *academicService { return &academicService{} }

func writeAcademicError(w http.ResponseWriter, err error) {
	if errors.Is(err, errSessionInvalid) {
		writeAPIError(w, 401, errors.New("教务登录已失效，请重新登录"))
		return
	}
	writeSchoolError(w, err)
}

func newAcademicClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: nil},
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if r.URL.Scheme == "https" && r.URL.Hostname() == "authserver.szu.edu.cn" {
				return errSessionInvalid
			}
			if len(via) >= 5 || !academicURLAllowed(r.URL) {
				return errors.New("学校登录跳转到未支持的地址")
			}
			return nil
		},
	}
}

func academicURLAllowed(u *url.URL) bool {
	if u.Scheme != "https" || u.User != nil || u.Hostname() != ehallHost || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	switch u.Path {
	case "/yjsxkapp/sys/xsxkapp/*default/index.do", "/yjsxkapp/sys/xsxkapp/login/4/vcode.do", "/yjsxkapp/sys/xsxkapp/login/vcode/image.do", "/yjsxkapp/sys/xsxkapp/login/check/login.do", "/yjsxkapp/sys/xsxkapp/xsxkHome/loadPublicInfo.do", "/yjsxkapp/sys/xsxkapp/xsxkHome/loadStdInfo.do", "/yjsxkapp/sys/xsxkapp/xsxkCourse/loadKbxx.do":
		return true
	}
	return false
}

func academicRequest(ctx context.Context, client *http.Client, address string, form url.Values) ([]byte, error) {
	u, err := url.Parse(address)
	if err != nil || !academicURLAllowed(u) {
		return nil, errors.New("不支持的学校系统地址")
	}
	method := http.MethodGet
	var body io.Reader
	if form != nil {
		method = http.MethodPost
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, address, body)
	if err != nil {
		return nil, errors.New("无法创建学校请求")
	}
	req.Header.Set("User-Agent", ehallUserAgent)
	req.Header.Set("Referer", graduateHome)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
		req.Header.Set("Origin", ehallBaseURL)
	}
	res, err := client.Do(req)
	// Do not propagate URL-bearing errors: challenge tokens may be in the URL.
	if err != nil {
		if errors.Is(err, errSessionInvalid) {
			return nil, errSessionInvalid
		}
		return nil, errors.New("学校系统暂时无法连接，请检查校园网后重试")
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		return nil, errSessionInvalid
	}
	if res.StatusCode == 403 {
		return nil, errSessionPermission
	}
	if res.StatusCode != 200 {
		return nil, errors.New("学校系统暂时未能完成请求，请稍后重试")
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, ehallMaxBody+1))
	if err != nil || len(b) > ehallMaxBody {
		return nil, errors.New("学校响应无法完整读取")
	}
	return b, nil
}

// The official graduate login encodes UTF-16BE blocks with DES E-E-E using
// the three published protocol keys. This is wire compatibility, not storage
// encryption; transport still requires HTTPS. No school JS is bundled.
func graduatePassword(password string) string {
	words := utf16.Encode([]rune(password))
	b := make([]byte, ((len(words)+3)/4)*8)
	for i, v := range words {
		binary.BigEndian.PutUint16(b[i*2:], v)
	}
	for _, key := range []byte{'1', '2', '3'} {
		// The school's encoder orders PC-1 columns differently from standard
		// DES. Remap the input key so the standard library gets identical bits.
		pc1 := []int{57, 49, 41, 33, 25, 17, 9, 1, 58, 50, 42, 34, 26, 18, 10, 2, 59, 51, 43, 35, 27, 19, 11, 3, 60, 52, 44, 36, 63, 55, 47, 39, 31, 23, 15, 7, 62, 54, 46, 38, 30, 22, 14, 6, 61, 53, 45, 37, 29, 21, 13, 5, 28, 20, 12, 4}
		raw := []byte{0, key, 0, 0, 0, 0, 0, 0}
		mapped := make([]byte, 8)
		for i, dst := range pc1 {
			src := 8*(7-i%8) + i/8
			bit := (raw[src/8] >> uint(7-src%8)) & 1
			mapped[(dst-1)/8] |= bit << uint(7-(dst-1)%8)
		}
		cipher, _ := des.NewCipher(mapped)
		for i := 0; i < len(b); i += 8 {
			cipher.Encrypt(b[i:i+8], b[i:i+8])
		}
	}
	return strings.ToUpper(hex.EncodeToString(b))
}

func (a *academicService) reset() {
	if a.client != nil {
		a.client.CloseIdleConnections()
	}
	a.client = nil
	a.challenge = ""
	a.vtoken = ""
	a.captcha = nil
	a.expires = time.Time{}
	a.authenticated = false
}

func (s *Server) handleAcademicSession(w http.ResponseWriter, r *http.Request) {
	a := s.academic
	a.mu.Lock()
	defer a.mu.Unlock()
	if r.Method == http.MethodDelete {
		a.reset()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
	writeJSON(w, map[string]any{"authenticated": a.authenticated, "level": "graduate", "storage": "仅本次运行，关闭应用即清除"})
}

func (s *Server) handleAcademicChallenge(w http.ResponseWriter, r *http.Request) {
	a := s.academic
	a.mu.Lock()
	defer a.mu.Unlock()
	// Starting a new account must never leave the previous account active.
	a.reset()
	a.client = newAcademicClient()
	_, err := academicRequest(r.Context(), a.client, graduateHome, nil)
	if err != nil {
		writeAcademicError(w, err)
		return
	}
	b, err := academicRequest(r.Context(), a.client, graduateRoot+"/login/4/vcode.do", nil)
	var result struct {
		Code json.RawMessage `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err == nil && (json.Unmarshal(b, &result) != nil || stringCode(result.Code) != "1" || result.Data.Token == "") {
		err = errors.New("学校未能提供登录验证码，请稍后重试")
	}
	if err != nil {
		a.reset()
		writeAcademicError(w, err)
		return
	}
	a.vtoken = result.Data.Token
	a.captcha, err = academicRequest(r.Context(), a.client, graduateRoot+"/login/vcode/image.do?vtoken="+url.QueryEscape(a.vtoken), nil)
	if err != nil || !strings.HasPrefix(http.DetectContentType(a.captcha), "image/") {
		a.reset()
		writeAPIError(w, 502, errors.New("学校验证码图片未能加载"))
		return
	}
	nonce := make([]byte, 16)
	if _, err = rand.Read(nonce); err != nil {
		a.reset()
		writeAPIError(w, 500, errors.New("无法创建本次登录"))
		return
	}
	a.challenge = hex.EncodeToString(nonce)
	a.expires = time.Now().Add(5 * time.Minute)
	writeJSON(w, map[string]string{"challenge": a.challenge, "image": "/api/academic/captcha?id=" + a.challenge, "message": "请输入学校验证码。账号密码仅本次使用。"})
}

func (s *Server) handleAcademicCaptcha(w http.ResponseWriter, r *http.Request) {
	a := s.academic
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.challenge == "" || r.URL.Query().Get("id") != a.challenge || time.Now().After(a.expires) {
		http.Error(w, "验证码已过期，请刷新", 410)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(a.captcha))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(a.captcha)
}

func stringCode(b json.RawMessage) string { return strings.Trim(string(b), `"`) }

func (s *Server) handleAcademicLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Challenge string `json:"challenge"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		Captcha   string `json:"captcha"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Username) == "" || in.Password == "" || len(in.Username) > 80 || len(in.Password) > 128 || strings.TrimSpace(in.Captcha) == "" {
		writeAPIError(w, 400, errors.New("请填写账号、密码和验证码"))
		return
	}
	a := s.academic
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.challenge == "" || in.Challenge != a.challenge || time.Now().After(a.expires) {
		writeAPIError(w, 409, errors.New("本次验证码已失效，请刷新验证码后登录"))
		return
	}
	a.challenge = ""
	a.captcha = nil
	a.authenticated = false
	form := url.Values{"loginName": {strings.TrimSpace(in.Username)}, "loginPwd": {graduatePassword(in.Password)}, "verifyCode": {strings.TrimSpace(in.Captcha)}, "vtoken": {a.vtoken}}
	in.Password = ""
	a.vtoken = ""
	b, err := academicRequest(r.Context(), a.client, graduateRoot+"/login/check/login.do", form)
	form.Del("loginPwd")
	if err != nil {
		a.reset()
		writeAcademicError(w, err)
		return
	}
	var result struct {
		Code json.RawMessage `json:"code"`
	}
	if err == nil {
		if json.Unmarshal(b, &result) != nil {
			err = errors.New("学校登录响应格式发生变化，请使用官方页面")
		} else {
			switch stringCode(result.Code) {
			case "1":
			case "2":
				err = errors.New("学校未接受账号或密码；研究生选课系统请使用学号及对应密码")
			case "3":
				err = errors.New("验证码不正确，请刷新后重试")
			case "4":
				err = errors.New("学校要求先处理密码问题，请在官方系统操作")
			default:
				err = errors.New("学校未完成登录，请在官方页面核对是否需要额外验证")
			}
		}
	}
	if err != nil {
		a.reset()
		writeAPIError(w, 401, err)
		return
	}
	// A success code is followed by a business read, not merely cookie presence.
	if _, err = readGraduateProfile(r.Context(), a.client); err != nil {
		a.reset()
		writeAcademicError(w, err)
		return
	}
	a.authenticated = true
	writeJSON(w, map[string]any{"ok": true, "authenticated": true, "message": "研究生教务已登录，登录状态仅保留到关闭应用"})
}

func readGraduateProfile(ctx context.Context, client *http.Client) (map[string]any, error) {
	b, err := academicRequest(ctx, client, graduateProfilePath, nil)
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if json.Unmarshal(b, &v) != nil || v == nil {
		return nil, errSessionInvalid
	}
	if str(v, "CODE") == "0" {
		return nil, errSessionPermission
	}
	// The official page requires student metadata before enabling the timetable.
	if str(v, "XM") == "" {
		return nil, errors.New("学校尚未返回有效学生信息，请重新登录")
	}
	return v, nil
}
