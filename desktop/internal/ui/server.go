// Package ui 把 szuDesktop 的页面和本地服务打包在一起。
//
// 页面本身是纯静态的 HTML（desktop/ 目录），用 go:embed 编译进二进制。
// 这样发出去就是一个文件：没有安装包、没有运行时依赖、没有外部资源。
// 页面里的「一键登录」通过本机回环地址上的 HTTP 接口驱动 szunet 内核。
package ui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Alakazamc/szunet/internal/credential"
	"github.com/Alakazamc/szunet/internal/diagnose"
	"github.com/Alakazamc/szunet/internal/portal"
)

// 页面和字体全部嵌进来。embed 的路径相对本包目录，
// 所以 assets/ 必须和这个文件放在一起（desktop/internal/ui/assets）。
// 同步方式：改完 desktop/index.html 后跑一次 desktop/sync-assets.py。
//
//go:embed all:assets
var assetsFS embed.FS

// Options 是起服务时可调的开关。
type Options struct {
	Addr      string // 监听地址，默认 127.0.0.1:0（随机端口）
	User      string // 覆盖保存的账号
	Password  string // 覆盖保存的密码
	AutoLogin bool   // 启动后自动登录一次
	KeepAlive bool   // 常驻盯着网络，掉线自动补登
	Interval  int    // KeepAlive 的检查间隔（秒），默认 30
	SrunHost  string
	DrcomHost string
	NoOpen    bool // 不自动开浏览器
}

// Server 是本地服务。
type Server struct {
	opts  Options
	store credential.Store

	mu       sync.Mutex
	lastErr  string
	lastZone portal.Zone

	alive      bool // 常驻监控是否在跑
	lastOK     time.Time
	relogin    int // 累计自动补登次数
	cancelKeep context.CancelFunc
}

// New 创建一个还没开始监听的 Server。
func New(opts Options) *Server {
	if opts.SrunHost == "" {
		opts.SrunHost = portal.DefaultSrunHost
	}
	if opts.DrcomHost == "" {
		opts.DrcomHost = portal.DefaultDrcomHost
	}
	return &Server{opts: opts, store: credential.Default()}
}

// creds 按「命令行参数 > 已保存的凭据」的顺序取账号密码。
func (s *Server) creds() (string, string, error) {
	user, pass := s.opts.User, s.opts.Password
	if user != "" && pass != "" {
		return user, pass, nil
	}
	c, err := s.store.Load()
	if err != nil {
		return "", "", fmt.Errorf("还没有保存账号密码。" +
			"先跑 `szunet config set -u 你的卡号 -p 你的密码` 存一次，或者用 --user / --password 临时指定")
	}
	if user == "" {
		user = c.Username
	}
	if pass == "" {
		pass = c.Password
	}
	return user, pass, nil
}

// Run 起服务，顺便按需自动登录，然后在浏览器里打开页面。
func (s *Server) Run() error {
	ln, err := net.Listen("tcp", s.opts.Addr)
	if err != nil {
		return fmt.Errorf("端口被占用或没有权限: %w", err)
	}

	sub, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	s.routes(mux, sub)

	url := "http://" + ln.Addr().String()
	fmt.Printf("szuDesktop 已启动: %s\n", url)

	if s.opts.AutoLogin {
		go func() {
			time.Sleep(300 * time.Millisecond) // 先让服务起来，再打日志
			res := s.doLogin()
			if res.OK {
				fmt.Printf("[自动登录] %s\n", res.Message)
			} else {
				fmt.Printf("[自动登录失败] %s\n", res.Message)
			}
		}()
	}

	if s.opts.KeepAlive {
		if s.opts.Interval <= 0 {
			s.opts.Interval = 30
		}
		s.startKeepAlive()
		fmt.Printf("[保持在线] 每 %d 秒检查一次，掉线自动补登\n", s.opts.Interval)
	}

	if !s.opts.NoOpen {
		go func() {
			time.Sleep(200 * time.Millisecond)
			if err := openBrowser(url); err != nil {
				fmt.Printf("浏览器没打开，自己复制上面的地址: %v\n", err)
			}
		}()
	}

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	return srv.Serve(ln)
}

// routes 注册路由。
//
// ⚠️ 静态资源这条规则别动，改了会踩一个很隐蔽的坑。
//
// 同一个页面有两种打开方式，对相对路径的解析基准不一样：
//
//  1. 双击页面（file://）：基准是 desktop/ 目录，旁边就躺着 assets/，
//     所以页面里写 assets/art/m1.png 是对的。
//  2. 起服务访问 http://127.0.0.1:PORT/：二进制里嵌的根已经是 assets/ 那一层了
//     （上面 fs.Sub 把前缀剥了），再收到 /assets/art/m1.png 会去找
//     assets/assets/art/m1.png —— 404。
//
// 症状特别难看：本地开页面一切正常，一跑起来满屏破图，但浏览器控制台
// 一句错都不报，看着像"图坏了"。这次就是这么找了两小时的。
//
// 解法：/assets/ 前缀在服务端统一剥掉再交给静态服务，两条路都通，
// 页面里那套相对路径一个字符都不用改。
func (s *Server) routes(mux *http.ServeMux, static fs.FS) {
	fileServer := http.FileServer(http.FS(static))

	// /assets/xxx -> 剥掉前缀 -> 当 xxx 处理
	// 剥完 r.URL.Path 就是 assets 里那一层的路径，正好对上 embed 的根
	assetsPrefix := http.StripPrefix("/assets", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	}))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == "/" || p == "/index.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			data, err := fs.ReadFile(static, "index.html")
			if err != nil {
				http.Error(w, "页面没有嵌进来: "+err.Error(), 500)
				return
			}
			_, _ = w.Write(data)
			return
		case p == "/assets" || strings.HasPrefix(p, "/assets/"):
			assetsPrefix.ServeHTTP(w, r)
			return
		}
		fileServer.ServeHTTP(w, r)
	})

	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/diag", s.handleDiag)
	mux.HandleFunc("/api/credential", s.handleCredential)
	mux.HandleFunc("/api/keepalive", s.handleKeepAlive)
}

// handleKeepAlive 开关常驻保持在线。前端那个「自动重连」开关接的就是这里。
//
// GET  看当前状态
// POST {"on":true|false} 切换
func (s *Server) handleKeepAlive(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.mu.Lock()
		on, n := s.alive, s.relogin
		s.mu.Unlock()
		writeJSON(w, map[string]any{"on": on, "relogins": n, "interval": s.checkInterval()})
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "不支持的方法", 405)
		return
	}

	var req struct {
		On bool `json:"on"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求格式不对", 400)
		return
	}

	// started 表示「这次调用有没有真的改状态」，state 改完才发响应，
	// 所以前端拿到 started=false 就知道本来就是这个状态，不是没生效。
	started := false
	if req.On {
		started = s.startKeepAlive()
	} else {
		s.mu.Lock()
		running := s.alive
		s.mu.Unlock()
		if running {
			s.stopKeepAlive()
			started = true
		}
	}

	s.mu.Lock()
	on, n := s.alive, s.relogin
	s.mu.Unlock()
	writeJSON(w, map[string]any{
		"on":       on,
		"relogins": n,
		"interval": s.checkInterval(),
		"started":  started,
	})
}

func (s *Server) checkInterval() int {
	if s.opts.Interval <= 0 {
		return 30
	}
	return s.opts.Interval
}

func (s *Server) stopKeepAlive() {
	s.mu.Lock()
	if s.cancelKeep != nil {
		s.cancelKeep()
		s.cancelKeep = nil
	}
	s.alive = false
	s.mu.Unlock()
}

/* ---------- 接口 ---------- */

type statusResp struct {
	Zone       string   `json:"zone"`
	ZoneLabel  string   `json:"zone_label"`
	InternetOK bool     `json:"internet_ok"`
	Online     bool     `json:"online"`
	OnlineIP   string   `json:"online_ip"`
	Username   string   `json:"username"`
	Saved      bool     `json:"saved"`       // 有没有存过凭据
	StoreDesc  string   `json:"store_desc"`  // 凭据存在哪
	LastError  string   `json:"last_error"`
	Advices    []string `json:"advices"`
	KeepAlive  bool     `json:"keep_alive"`  // 有没有开着常驻保持
	Relogins   int      `json:"relogins"`    // 累计自动补登次数
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	user, pass, credErr := s.creds()

	det := portal.Detect()
	out := statusResp{
		Zone:       string(det.Zone),
		ZoneLabel:  det.Zone.Label(),
		InternetOK: det.InternetOK,
		StoreDesc:  s.store.Describe(),
	}
	if credErr == nil {
		out.Saved = true
		out.Username = user
		if det.Zone != portal.ZoneOnline && det.Zone != portal.ZoneOutside {
			var st *portal.OnlineStatus
			var err error
			switch det.Zone {
			case portal.ZoneTeaching:
				st, err = portal.NewSrunClient(s.opts.SrunHost, user, pass).Status()
			case portal.ZoneDorm:
				st, err = portal.NewDrcomClient(s.opts.DrcomHost, user, pass).Status()
			}
			if err == nil && st != nil {
				out.Online = st.Online
				out.OnlineIP = st.IP
			}
		}
	} else {
		out.LastError = credErr.Error()
	}

	s.mu.Lock()
	if s.lastErr != "" {
		out.LastError = s.lastErr
	}
	out.Advices = detectAdvices(det)
	out.KeepAlive = s.alive
	out.Relogins = s.relogin
	s.mu.Unlock()

	writeJSON(w, out)
}

type loginResp struct {
	OK      bool   `json:"ok"`
	Zone    string `json:"zone"`
	Message string `json:"message"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	res := s.doLogin()
	writeJSON(w, loginResp{OK: res.OK, Message: res.Message})
}

func (s *Server) doLogin() portal.Result {
	user, pass, err := s.creds()
	if err != nil {
		return s.remember(err.Error())
	}

	det := portal.Detect()
	s.mu.Lock()
	s.lastZone = det.Zone
	s.mu.Unlock()

	switch det.Zone {
	case portal.ZoneOnline:
		s.clearErr()
		return portal.Result{OK: true, Message: "已经能上外网，不用再认证"}

	case portal.ZoneTeaching:
		res, err := portal.NewSrunClient(s.opts.SrunHost, user, pass).Login()
		if err != nil {
			return s.remember(err.Error())
		}
		if !res.OK {
			return s.remember(res.Message)
		}
		s.clearErr()
		return *res

	case portal.ZoneDorm:
		res, err := portal.NewDrcomClient(s.opts.DrcomHost, user, pass).Login()
		if err != nil {
			return s.remember(err.Error())
		}
		if !res.OK {
			return s.remember(res.Message)
		}
		s.clearErr()
		return *res

	default:
		return s.remember("判断不出你在哪个区。确认一下是不是连着校园网（SZU_WLAN）")
	}
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	user, pass, err := s.creds()
	if err != nil {
		writeJSON(w, loginResp{OK: false, Message: err.Error()})
		return
	}
	det := portal.Detect()
	var res *portal.Result
	var e error
	switch det.Zone {
	case portal.ZoneTeaching:
		res, e = portal.NewSrunClient(s.opts.SrunHost, user, pass).Logout()
	case portal.ZoneDorm:
		res, e = portal.NewDrcomClient(s.opts.DrcomHost, user, pass).Logout()
	default:
		writeJSON(w, loginResp{OK: false, Message: "不在校园网里，没有可注销的会话"})
		return
	}
	if e != nil {
		writeJSON(w, loginResp{OK: false, Message: e.Error()})
		return
	}
	writeJSON(w, loginResp{OK: res.OK, Zone: string(det.Zone), Message: res.Message})
}

type diagResp struct {
	Zone       string   `json:"zone"`
	ZoneLabel  string   `json:"zone_label"`
	InternetOK bool     `json:"internet_ok"`
	DormPortal bool     `json:"dorm_portal_ok"`
	TeachPortal bool    `json:"teaching_portal_ok"`
	DNSOK      bool     `json:"dns_ok"`
	Online     *bool    `json:"online,omitempty"`
	Advices    []string `json:"advices"`
	Notes      []string `json:"notes"`
}

func (s *Server) handleDiag(w http.ResponseWriter, r *http.Request) {
	user, pass, _ := s.creds()
	rep := diagnose.Run(user, pass, s.opts.SrunHost, s.opts.DrcomHost)

	out := diagResp{
		Zone:        string(rep.Detect.Zone),
		ZoneLabel:   rep.Detect.Zone.Label(),
		InternetOK:  rep.Detect.InternetOK,
		DormPortal:  rep.Detect.DormPortalOK,
		TeachPortal: rep.Detect.TeachPortalOK,
		DNSOK:       rep.Detect.SrunDNSOK,
		Advices:     rep.Advices,
		Notes:       rep.Detect.Notes,
	}
	if rep.Online != nil {
		on := rep.Online.Online
		out.Online = &on
	}
	writeJSON(w, out)
}

type credReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleCredential(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c, err := s.store.Load()
		if err != nil {
			writeJSON(w, map[string]any{"saved": false, "store_desc": s.store.Describe()})
			return
		}
		writeJSON(w, map[string]any{
			"saved":      true,
			"username":   c.Username,
			"store_desc": s.store.Describe(),
		})

	case http.MethodPost:
		var req credReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "请求格式不对", 400)
			return
		}
		if req.Username == "" || req.Password == "" {
			http.Error(w, "账号和密码都不能空", 400)
			return
		}
		if err := s.store.Save(credential.Credentials{Username: req.Username, Password: req.Password}); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		s.clearErr()
		writeJSON(w, map[string]any{"ok": true, "store_desc": s.store.Describe()})

	case http.MethodDelete:
		if err := s.store.Delete(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})

	default:
		http.Error(w, "不支持的方法", 405)
	}
}

/* ---------- 保持在线 ---------- */

// startKeepAlive 开常驻监控。
//
// 关键点：状态在这里「同步」改好，循环才丢到后台跑。
// 之前是 `go s.keepAlive()`，改状态的动作排在 goroutine 里，
// 接口可能在它被调度到之前就把旧状态返回去了，表现为「刚点开又说没开」。
// 返回 false 说明本来就在跑，没重复起。
func (s *Server) startKeepAlive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.alive {
		return false
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelKeep = cancel
	s.alive = true
	s.lastErr = "" // 刚打开，别让上一轮的旧报错挂在页面上
	go s.keepLoop(ctx)
	return true
}

// keepLoop 是常驻监控的循环体。由 startKeepAlive 在后台拉起，别直接调。
//
// 只在「探测到校园网、但外网不通」时动手，两种情况跳过：
//   - 已经能上外网：什么都不用做
//   - 两个门户都连不上（人在校外，或者校园网整体故障）：登录也没用，别白试
//
// 连续失败不会死循环——每轮最多尝试一次，失败就等下一轮。
func (s *Server) keepLoop(ctx context.Context) {
	defer func() {
		s.mu.Lock()
		// 只有自己还是「当前那一个」时才清状态，避免误伤后来重启的循环
		if s.cancelKeep == nil {
			s.alive = false
		}
		s.mu.Unlock()
	}()

	ticker := time.NewTicker(time.Duration(s.checkInterval()) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		det := portal.Detect()
		if det.InternetOK {
			s.mu.Lock()
			s.lastOK = time.Now()
			s.mu.Unlock()
			continue
		}
		// 不在校园网里，登录无意义
		if det.Zone == portal.ZoneOutside || det.Zone == portal.ZoneUnknown {
			continue
		}

		fmt.Printf("[保持在线] 外网不通（%s），尝试补登……\n", det.Zone.Label())
		res := s.doLogin()
		if res.OK {
			s.mu.Lock()
			s.relogin++
			n := s.relogin
			s.mu.Unlock()
			fmt.Printf("[保持在线] 补登成功（累计第 %d 次）: %s\n", n, res.Message)
		} else {
			fmt.Printf("[保持在线] 补登失败: %s\n", res.Message)
		}
	}
}

/* ---------- 小工具 ---------- */

func (s *Server) remember(msg string) portal.Result {
	s.mu.Lock()
	s.lastErr = msg
	s.mu.Unlock()
	return portal.Result{OK: false, Message: msg}
}

func (s *Server) clearErr() {
	s.mu.Lock()
	s.lastErr = ""
	s.mu.Unlock()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// detectAdvices 只根据网络侧的探测结果给建议，不查账号。
func detectAdvices(det *portal.DetectResult) []string {
	switch det.Zone {
	case portal.ZoneOnline:
		return []string{"当前能正常上外网，不用做任何事"}
	case portal.ZoneTeaching:
		return []string{"你在教学区，走深澜认证。账号是 6 位校园卡号，密码是统一身份认证密码"}
	case portal.ZoneDorm:
		return []string{"你在宿舍区，走 Dr.COM 认证。先去自助服务确认套餐没到期"}
	default:
		return []string{"两个认证门户都连不上，先确认是不是在校外"}
	}
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

var _ = log.Println
