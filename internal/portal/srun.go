package portal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Alakazamc/szunet/internal/crypto"
)

// DefaultSrunHost 是深大教学区的深澜认证门户。
// 其他用深澜系统的学校换成自己的门户地址即可。
const DefaultSrunHost = "https://net.szu.edu.cn"

// 深澜协议里两个固定参数，照门户页面的默认值抄。
const (
	srunN    = "200"
	srunType = "1"
)

// SrunClient 是深澜（SRun）认证客户端，用于教学区 / 办公区 / 图书馆。
type SrunClient struct {
	Host     string // 认证门户地址
	Username string // 6 位校园卡号
	Password string // 统一身份认证密码
	ServerIP string // 可选：直接指定服务器 IP，绕过域名解析
	AcID     string // 可选：留空则自动从门户页面抓取

	http *http.Client
}

// NewSrunClient 创建一个深澜认证客户端。
func NewSrunClient(host, username, password string) *SrunClient {
	if host == "" {
		host = DefaultSrunHost
	}
	c := &SrunClient{
		Host:     strings.TrimRight(host, "/"),
		Username: username,
		Password: password,
	}
	c.http = newHTTPClient(c.Host, "", 10*time.Second)
	return c
}

// SetServerIP 指定认证服务器的 IP，用于域名解析不通的情况。
func (c *SrunClient) SetServerIP(ip string) {
	c.ServerIP = ip
	c.http = newHTTPClient(c.Host, ip, 10*time.Second)
}

type srunChallengeResp struct {
	Challenge string `json:"challenge"`
	ClientIP  string `json:"client_ip"`
	Error     string `json:"error"`
	ErrorMsg  string `json:"error_msg"`
	Res       string `json:"res"`
}

type srunPortalResp struct {
	Error    string `json:"error"`
	ErrorMsg string `json:"error_msg"`
	SucMsg   string `json:"suc_msg"`
	OnlineIP string `json:"online_ip"`
	ClientIP string `json:"client_ip"`
	Res      string `json:"res"`
}

type srunUserInfo struct {
	Username string `json:"username"`
	Password string `json:"password"`
	IP       string `json:"ip"`
	AcID     string `json:"acid"`
	EncVer   string `json:"enc_ver"`
}

// Status 查询账号当前是否在线。
func (c *SrunClient) Status() (*OnlineStatus, error) {
	u := fmt.Sprintf("%s/cgi-bin/rad_user_info?callback=_&_=%d", c.Host, time.Now().Unix())
	body, err := c.get(u)
	if err != nil {
		return nil, fmt.Errorf("查询在线状态失败: %w", err)
	}

	var resp struct {
		Error    string `json:"error"`
		UserName string `json:"user_name"`
		OnlineIP string `json:"online_ip"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析在线状态失败: %w", err)
	}

	return &OnlineStatus{
		Online:   resp.Error == "ok",
		Username: resp.UserName,
		IP:       resp.OnlineIP,
		Raw:      truncate(string(body), 300),
	}, nil
}

// Login 执行一次完整的深澜认证。
//
// 流程：要 challenge → 定 acid → 算 HMAC-MD5 密码 → 算加密用户信息
// → 算 SHA1 校验和 → 发登录请求。
func (c *SrunClient) Login() (*Result, error) {
	token, ip, err := c.challenge()
	if err != nil {
		return nil, err
	}

	acID := c.resolveAcIDOrDefault()

	pwd := crypto.HMACMD5Hex(c.Password, token)

	info, err := c.encodeUserInfo(token, ip, acID)
	if err != nil {
		return nil, err
	}

	// 校验和：按固定顺序把「token+字段」拼起来算 SHA1。
	// 顺序错了服务端会报 sign error，所以这里不要动。
	chksum := crypto.SHA1Hex(
		token + c.Username +
			token + pwd +
			token + acID +
			token + ip +
			token + srunN +
			token + srunType +
			token + info,
	)

	q := url.Values{}
	q.Set("callback", "_")
	q.Set("action", "login")
	q.Set("username", c.Username)
	q.Set("password", "{MD5}"+pwd)
	q.Set("os", "Mac OS")
	q.Set("name", "Macintosh")
	q.Set("double_stack", "0")
	q.Set("info", info)
	q.Set("chksum", chksum)
	q.Set("ac_id", acID)
	q.Set("ip", ip)
	q.Set("n", srunN)
	q.Set("type", srunType)

	body, err := c.get(c.Host + "/cgi-bin/srun_portal?" + q.Encode())
	if err != nil {
		return nil, fmt.Errorf("发送登录请求失败: %w", err)
	}

	var resp srunPortalResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析登录响应失败: %w", err)
	}

	raw := truncate(string(body), 400)

	// error 为 ok 就是成功；suc_msg 里带 already_online 说明本来就在线，
	// 这同样证明协议走通了，算成功，不主动去踢掉原有会话。
	if resp.Error == "ok" {
		msg := "认证成功"
		if strings.Contains(resp.SucMsg, "already_online") {
			msg = "该账号本来就在线，无需重复认证"
		}
		return &Result{OK: true, Message: msg, Raw: raw}, nil
	}

	return &Result{OK: false, Message: friendlySrunError(resp), Raw: raw}, nil
}

// Logout 注销当前会话（相当于把自己踢下线）。
func (c *SrunClient) Logout() (*Result, error) {
	_, ip, err := c.challenge()
	if err != nil {
		return nil, err
	}

	acID := c.resolveAcIDOrDefault()

	q := url.Values{}
	q.Set("callback", "_")
	q.Set("action", "logout")
	q.Set("username", c.Username)
	q.Set("ac_id", acID)
	q.Set("ip", ip)

	body, err := c.get(c.Host + "/cgi-bin/srun_portal?" + q.Encode())
	if err != nil {
		return nil, fmt.Errorf("发送注销请求失败: %w", err)
	}

	var resp srunPortalResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析注销响应失败: %w", err)
	}

	raw := truncate(string(body), 400)
	if resp.Error == "ok" {
		return &Result{OK: true, Message: "已注销下线", Raw: raw}, nil
	}
	return &Result{OK: false, Message: "注销失败：" + resp.Error + " " + resp.ErrorMsg, Raw: raw}, nil
}

// challenge 向服务端要一个一次性随机串。
// 这个串是后面所有加密的密钥，每次请求都不同，所以认证请求无法重放。
// 顺带把服务端认定的本机 IP 拿回来。
func (c *SrunClient) challenge() (token, ip string, err error) {
	u := fmt.Sprintf("%s/cgi-bin/get_challenge?callback=_&username=%s&ip=",
		c.Host, url.QueryEscape(c.Username))

	body, err := c.get(u)
	if err != nil {
		return "", "", fmt.Errorf("获取 challenge 失败: %w", err)
	}

	var resp srunChallengeResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", "", fmt.Errorf("解析 challenge 响应失败: %w", err)
	}
	if resp.Challenge == "" {
		return "", "", fmt.Errorf("服务端没有返回 challenge（error=%s）", resp.Error)
	}
	return resp.Challenge, resp.ClientIP, nil
}

// resolveAcID 从门户页面里抓认证真正要用的 acid。
//
// 这里有个经典的坑：URL 上那个 ac_id（比如 srun_portal_pc?ac_id=1）
// 只是页面入口的编号，不是认证参数。真正的 acid 藏在页面内嵌的一段 JS
// 配置里，而且不同楼栋可能不一样。用错了会报 "Unknow ac-type:"。
//
// 更阴的是：已经在线的时候重复登录会提前短路返回 already_online，
// 把这个错误盖住，让人误以为 ac_id 没问题。
func (c *SrunClient) resolveAcID() (string, error) {
	resp, err := c.http.Get(c.Host + "/srun_portal_pc?ac_id=1&theme=proyx")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`acid\s*[:=]\s*['"]?(\d+)`)
	m := re.FindSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("门户页面里没找到 acid 配置")
	}
	return string(m[1]), nil
}

// resolveAcIDOrDefault 优先用调用方指定的 acid，其次从页面抓，最后兜底。
func (c *SrunClient) resolveAcIDOrDefault() string {
	if c.AcID != "" {
		return c.AcID
	}
	if id, err := c.resolveAcID(); err == nil {
		return id
	}
	return "1"
}

// encodeUserInfo 构造登录请求里的 info 字段。
//
// 三步：用户信息转 JSON → 用 challenge 加密（XXTEA 变体）
// → 用深澜那副打乱的 Base64 字母表编码，最后加上 {SRBX1} 前缀。
func (c *SrunClient) encodeUserInfo(token, ip, acID string) (string, error) {
	info := srunUserInfo{
		Username: c.Username,
		Password: c.Password,
		IP:       ip,
		AcID:     acID,
		EncVer:   "srun_bx1",
	}

	// 字段顺序必须固定，所以用 struct 而不是 map。
	//
	// 另外这里必须关掉 HTML 转义：Go 的 json 默认会把 & < > 转成 \u0026 之类，
	// 而浏览器里的 JSON.stringify 不会。密码里含这几个字符时，
	// 两边算出来的密文不一致，服务端就会报解密失败。
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(info); err != nil {
		return "", fmt.Errorf("编码用户信息失败: %w", err)
	}
	raw := bytes.TrimRight(buf.Bytes(), "\n")

	encoded := crypto.Encode(string(raw), token)
	return "{SRBX1}" + crypto.Base64WithAlphaSet([]byte(encoded), crypto.SrunAlphaSet), nil
}

// get 发一个 GET 请求，并返回 JSONP 里的 JSON 部分。
func (c *SrunClient) get(rawURL string) ([]byte, error) {
	resp, err := c.http.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	return parseJSONP(body)
}

// friendlySrunError 把服务端返回的错误码翻成能看懂的话，并给出常见原因。
// 这份对照关系来自官方 FAQ 和实际报错记录。
func friendlySrunError(resp srunPortalResp) string {
	code := strings.ToLower(resp.Error + " " + resp.ErrorMsg)

	switch {
	case strings.Contains(code, "already_online"):
		return "该账号已在线"
	case strings.Contains(code, "ldap"):
		return "认证失败：密码不对（ldap auth error）。另外注意密码不要超过 16 位"
	case strings.Contains(code, "userid"):
		return "认证失败：账号不对（Rad:userid error）。账号是 6 位数的校园卡号"
	case strings.Contains(code, "ac-type"):
		return "认证失败：ac_id 用错了（Unknow ac-type）。可以加 --ac-id 手动指定"
	case strings.Contains(code, "sign"):
		return "认证失败：校验和不对（sign error）。加密环节出错，请把原始返回发给作者排查"
	case strings.Contains(code, "decrypt"):
		return "认证失败：服务端解不开用户信息（decrypt error）。通常是加密实现有出入"
	case strings.Contains(code, "not_online"):
		return "账号当前不在线"
	case strings.Contains(code, "login_error"):
		return "认证失败：账号或密码不对（教学区用的是统一身份认证密码）"
	default:
		return fmt.Sprintf("认证失败：%s %s", resp.Error, resp.ErrorMsg)
	}
}
