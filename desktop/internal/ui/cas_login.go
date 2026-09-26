package ui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SzuDesktopTeam/szudesktop/internal/credential"
)

// 深大统一身份认证（authserver.szu.edu.cn，Apereo CAS）的应用内登录。
//
// 与研究生选课系统那套（academic_login.go）是同一个形状：挑战号 + 验证码 +
// 账号密码，全部由用户在应用内填写，登录态只留在进程内存里，关闭应用即清除。
// 差别在于本科业务挂在 ehall 的统一身份认证后面，登录成功后要拿着 CAS 发的
// ticket 跳回 ehall 才会落下 JSESSIONID，之后本科课表和成绩共用这一条会话。
const (
	casHost        = "authserver.szu.edu.cn"
	casBaseURL     = "https://" + casHost
	casLoginPath   = "/authserver/login"
	casCaptchaPath = "/authserver/getCaptcha.htl"
	// 登录成功后要回到的 ehall 业务页。它同时是 CAS 的 service 参数。
	casServicePath = "/jwapp/sys/wdkb/*default/index.do"
	// 验证登录是否真的成立：读一次本科业务接口，而不是只看有没有 Cookie。
	casProbePath = "/jwapp/sys/wdkb/modules/jshkcb/dqxnxq.do"
)

// 测试注入点：非空时用 httptest 假服务器顶替真实学校地址，
// 并放行 http 与本地回环。生产环境下两者为零值。
var casTestBase, casTestEhall string

func casRoot() string {
	if casTestBase != "" {
		return casTestBase
	}
	return casBaseURL
}

func ehallRoot() string {
	if casTestEhall != "" {
		return casTestEhall
	}
	return ehallBaseURL
}

func casServiceTarget() string { return ehallRoot() + casServicePath }
func casProbeTarget() string   { return ehallRoot() + casProbePath }

var (
	casSaltRe      = regexp.MustCompile(`id="pwdEncryptSalt"\s+value="([^"]*)"`)
	casExecutionRe = regexp.MustCompile(`name="execution"\s+value="([^"]*)"`)
	casLtRe        = regexp.MustCompile(`name="lt"\s+id="lt"\s+value="([^"]*)"`)
	// 页面上出现这个容器才说明本次登录被要求输入图形验证码。
	// 真正决定要不要验证码的是页面里的 needCaptcha 变量：空字符串表示不需要。
	// captchaDiv 在 HTML 里恒存在（其中一个还带 hide class，由 JS 按需显示），
	// 所以拿容器判断会永远多要一个验证码框——这是实测踩到的坑。
	casNeedCaptchaRe = regexp.MustCompile(`var\s+needCaptcha\s*=\s*"([^"]*)"`)
	// needCaptcha 变量缺席时（页面结构变了）的后备信号：账号密码表单里有没有验证码输入框。
	casCaptchaInputRe = regexp.MustCompile(`<input[^>]*id="captcha"[^>]*name="captcha"`)
)

// casService 的字段语义与 academicService 一致，方便对照审查。
type casService struct {
	mu            sync.Mutex
	client        *http.Client
	challenge     string
	salt          string
	execution     string
	lt            string
	captcha       []byte
	expires       time.Time
	authenticated bool
}

func newCasService() *casService { return &casService{} }

func writeCasError(w http.ResponseWriter, err error) {
	if errors.Is(err, errSessionInvalid) {
		writeAPIError(w, 401, errors.New("统一身份认证登录已失效，请重新登录"))
		return
	}
	writeSchoolError(w, err)
}

// newCasClient 允许在 authserver 与 ehall 之间跟随跳转——CAS 的 ticket 必须
// 跳回 ehall 才会落下会话，这一点和研究生那套刻意拦住 authserver 正好相反。
func newCasClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Timeout: 25 * time.Second, Transport: &http.Transport{Proxy: nil},
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 8 {
				return errors.New("学校登录跳转次数过多")
			}
			if !casURLAllowed(r.URL) {
				return errors.New("学校登录跳转到未支持的地址")
			}
			return nil
		},
	}
}

func casURLAllowed(u *url.URL) bool {
	if u.User != nil {
		return false
	}
	// 测试模式：只认注入进来的两个假源。
	if casTestBase != "" {
		return strings.HasPrefix(u.String(), casTestBase+"/") || strings.HasPrefix(u.String(), casTestEhall+"/")
	}
	if u.Scheme != "https" {
		return false
	}
	switch u.Hostname() {
	case casHost:
		return u.Port() == "" || u.Port() == "443"
	case ehallHost:
		return (u.Port() == "" || u.Port() == "443") && (ehallPathAllowed(u.Path) || u.Path == casServicePath)
	}
	return false
}

// casRequest 只发往白名单内的地址。form 为 nil 时是 GET。
func casRequest(ctx context.Context, client *http.Client, address string, form url.Values, referer string) ([]byte, error) {
	u, err := url.Parse(address)
	if err != nil || !casURLAllowed(u) {
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
	req.Header.Set("Referer", referer)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
		req.Header.Set("Origin", u.Scheme+"://"+u.Host)
	}
	res, err := client.Do(req)
	if err != nil {
		// 跳转被我们主动截断时错误里会带上 URL，不能原样透出。
		if errors.Is(err, errSessionInvalid) {
			return nil, errSessionInvalid
		}
		return nil, errors.New("学校系统暂时无法连接，请检查校园网后重试")
	}
	defer res.Body.Close()
	switch res.StatusCode {
	case 200, 302:
	case 401:
		return nil, errSessionInvalid
	case 403:
		return nil, errSessionPermission
	default:
		return nil, errors.New("学校系统暂时未能完成请求，请稍后重试")
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, ehallMaxBody+1))
	if err != nil || len(b) > ehallMaxBody {
		return nil, errors.New("学校响应无法完整读取")
	}
	return b, nil
}

func (c *casService) reset() {
	if c.client != nil {
		c.client.CloseIdleConnections()
	}
	c.client = nil
	c.challenge = ""
	c.salt = ""
	c.execution = ""
	c.lt = ""
	c.captcha = nil
	c.expires = time.Time{}
	c.authenticated = false
}

// casEhallClient 复用 CAS 登录时积累的 cookiejar。
// 不能走 newEhallClient：那会另起一个没有会话的 http.Client。
func (c *casService) ehallClient() *ehallClient {
	base := ehallBaseURL
	if casTestEhall != "" {
		base = casTestEhall
	}
	httpClient := c.client
	if httpClient == nil {
		httpClient = &http.Client{Timeout: ehallTimeout, Transport: &http.Transport{Proxy: nil}}
	}
	return &ehallClient{base: base, http: httpClient, usesJar: true}
}

// schoolClient 决定本科业务用哪条会话：统一身份认证登录过就用它，
// 否则回落到用户粘来的 ehall Cookie。两条路并存期间谁都不能坏。
// 会话失效时要能区分「先去登录」和「重新粘一次」，所以错误信息分开。
func (s *Server) schoolClient() (*ehallClient, error) {
	// s.cas 可能为 nil：测试按需直接构造 Server，只填自己那几个字段。
	if s.cas != nil {
		c := s.cas
		c.mu.Lock()
		authenticated := c.authenticated && c.client != nil
		client := c.ehallClient()
		c.mu.Unlock()
		if authenticated {
			return client, nil
		}
	}
	v, err := s.sessionStore().Load()
	if err != nil {
		return nil, err
	}
	return s.makeEhallClient(v.Cookie), nil
}

// handleCasSession 的 DELETE 已经把 cas 状态清掉，schoolClient 会自动回落到 Cookie，
// 所以「清除统一身份认证登录」不需要额外动 Cookie，反之亦然。
func writeSchoolClientError(w http.ResponseWriter, err error) {
	if errors.Is(err, credential.ErrSessionNotFound) {
		writeAPIError(w, 409, errors.New("还没有学校系统登录状态：请先在上方用学号密码登录统一身份认证"))
		return
	}
	writeSessionLoadError(w, err)
}

func (s *Server) handleCasSession(w http.ResponseWriter, r *http.Request) {
	c := s.cas
	c.mu.Lock()
	defer c.mu.Unlock()
	if r.Method == http.MethodDelete {
		c.reset()
		writeJSON(w, map[string]bool{"ok": true})
		return
	}
	writeJSON(w, map[string]any{"authenticated": c.authenticated, "level": "undergraduate", "storage": "仅本次运行，关闭应用即清除"})
}

// handleCasChallenge 取登录页，解析加密盐与 execution，并按页面实际情况取验证码。
func (s *Server) handleCasChallenge(w http.ResponseWriter, r *http.Request) {
	c := s.cas
	c.mu.Lock()
	defer c.mu.Unlock()
	// 重新开始一次登录必须先把上一次的账号状态清掉。
	c.reset()
	c.client = newCasClient()
	page, err := casRequest(r.Context(), c.client, casRoot()+casLoginPath+"?service="+url.QueryEscape(casServiceTarget()), nil, casServiceTarget())
	if err != nil {
		c.reset()
		writeCasError(w, err)
		return
	}
	body := string(page)
	parsed := parseCasLoginPage(body)
	c.salt = parsed.Salt
	c.execution = parsed.Execution
	c.lt = parsed.Lt
	if c.salt == "" {
		c.reset()
		writeAPIError(w, 502, errors.New("学校登录页没有返回加密参数，请稍后重试"))
		return
	}
	// 只在页面确实要求图形验证码时才去取图：多数情况下本次登录不需要验证码，
	// 硬取一张只会让用户多填一个框。
	if parsed.NeedCaptcha {
		img, err := casRequest(r.Context(), c.client, casRoot()+casCaptchaPath+"?"+strconv.FormatInt(time.Now().UnixMilli(), 10), nil, casRoot()+casLoginPath)
		if err != nil || len(img) == 0 || !strings.HasPrefix(http.DetectContentType(img), "image/") {
			c.reset()
			writeAPIError(w, 502, errors.New("学校验证码图片未能加载，请稍后重试"))
			return
		}
		c.captcha = img
	}
	nonce := make([]byte, 16)
	if _, err = rand.Read(nonce); err != nil {
		c.reset()
		writeAPIError(w, 500, errors.New("无法创建本次登录"))
		return
	}
	c.challenge = hex.EncodeToString(nonce)
	c.expires = time.Now().Add(5 * time.Minute)
	out := map[string]any{"challenge": c.challenge, "message": "请填写统一身份认证的学号和密码。账号密码仅本次使用。"}
	if c.captcha != nil {
		out["image"] = "/api/cas/captcha?id=" + c.challenge
	}
	writeJSON(w, out)
}

func (s *Server) handleCasCaptcha(w http.ResponseWriter, r *http.Request) {
	c := s.cas
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.challenge == "" || r.URL.Query().Get("id") != c.challenge || time.Now().After(c.expires) {
		http.Error(w, "验证码已过期，请刷新", 410)
		return
	}
	if c.captcha == nil {
		http.Error(w, "本次登录不需要验证码", 404)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(c.captcha))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(c.captcha)
}

// casLoginPage 是登录页里我们需要的全部信息。
type casLoginPage struct {
	Salt        string
	Execution   string
	Lt          string
	NeedCaptcha bool
}

// parseCasLoginPage 从登录页 HTML 里抽出登录所需的参数。
// 抽成纯函数是为了能拿学校真实页面直接验，不必真登录一次。
//
// lt 在真实页面上就是空值，而浏览器自己也从不设置它——login.js 没有任何额外
// 请求去取 lt，直接用原生表单提交隐藏域里的空值。所以这里「给空传空」是照抄
// 浏览器行为，不是图省事。
//
// 是否需要验证码看 needCaptcha 变量，不看 captchaDiv 容器：容器在 HTML 里恒存在
// （真实页面上有两个，其中一个还带 hide class，由 JS 按需显示），拿它判断会永远
// 多要一个验证码框——这是拿真实页面实测踩到的坑。
func parseCasLoginPage(body string) casLoginPage {
	var p casLoginPage
	if m := casSaltRe.FindStringSubmatch(body); m != nil {
		p.Salt = m[1]
	}
	if m := casExecutionRe.FindStringSubmatch(body); m != nil {
		p.Execution = m[1]
	}
	if m := casLtRe.FindStringSubmatch(body); m != nil {
		p.Lt = m[1]
	}
	if m := casNeedCaptchaRe.FindStringSubmatch(body); m != nil {
		// 变量在场：非空才要验证码。空字符串是学校的「本次不需要」。
		p.NeedCaptcha = m[1] != ""
	} else {
		// 变量缺席（页面结构变了）：退回结构判断，看账号密码表单里有没有验证码输入框。
		p.NeedCaptcha = casCaptchaInputRe.MatchString(body)
	}
	return p
}

// casLoginForm 拼出登录表单。抽成纯函数是为了能在不发一个请求的情况下
// 钉死协议形状——字段名和取值直接决定学校收不收。
func casLoginForm(username, encryptedPassword, lt, execution, captcha string) url.Values {
	form := url.Values{
		"username":   {strings.TrimSpace(username)},
		"password":   {encryptedPassword},
		"lt":         {lt},
		"execution":  {execution},
		"_eventId":   {"submit"},
		"cllt":       {"userNameLogin"},
		"dllt":       {"generalLogin"},
		"rememberMe": {"false"},
	}
	if captcha := strings.TrimSpace(captcha); captcha != "" {
		form.Set("captcha", captcha)
	}
	return form
}

// handleCasLogin 提交账号密码。口令按学校 encrypt.js 的规则加密后才上网。
func (s *Server) handleCasLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Challenge string `json:"challenge"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		Captcha   string `json:"captcha"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil || strings.TrimSpace(in.Username) == "" || in.Password == "" || len(in.Username) > 80 || len(in.Password) > 128 {
		writeAPIError(w, 400, errors.New("请填写学号和密码"))
		return
	}
	c := s.cas
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.challenge == "" || in.Challenge != c.challenge || time.Now().After(c.expires) {
		writeAPIError(w, 409, errors.New("本次登录已失效，请重新获取登录页"))
		return
	}
	// 验证码是条件性的：页面没要就不校验，页面要了就必须填。
	if c.captcha != nil && strings.TrimSpace(in.Captcha) == "" {
		writeAPIError(w, 400, errors.New("请填写学校验证码"))
		return
	}
	encrypted, err := casEncryptPassword(in.Password, c.salt)
	in.Password = ""
	if err != nil {
		c.reset()
		writeCasError(w, err)
		return
	}
	form := casLoginForm(in.Username, encrypted, c.lt, c.execution, in.Captcha)
	// 密码字段用完即从表单里删掉，后面的错误处理也不再接触它。
	defer form.Del("password")
	_, err = casRequest(r.Context(), c.client, casRoot()+casLoginPath, form, casRoot()+casLoginPath+"?service="+url.QueryEscape(casServiceTarget()))
	form.Del("password")
	if err != nil {
		c.reset()
		writeCasError(w, err)
		return
	}
	// 拿着 CAS 发的 ticket 跳回 ehall，会话才真正建立。
	if _, err = casRequest(r.Context(), c.client, casServiceTarget(), nil, casRoot()+casLoginPath); err != nil {
		c.reset()
		writeCasError(w, err)
		return
	}
	// 用一次真实业务读验证会话，而不是只看有没有 cookie。
	err = validateUndergradSession(r.Context(), c.client)
	if err != nil {
		c.reset()
		writeCasError(w, err)
		return
	}
	c.challenge = ""
	c.captcha = nil
	c.authenticated = true
	writeJSON(w, map[string]any{"ok": true, "authenticated": true, "message": "统一身份认证已登录，本科课表与成绩可以读取了；登录状态仅保留到关闭应用"})
}
