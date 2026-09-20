package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ehall 是学校一站式服务大厅（ehall.szu.edu.cn）。
//
// 教务（jwapp）、研究生（gsapp）、场馆预约（qljfwapp）这些应用都在同一个域下，
// 会话是否可复用还取决于 Cookie 路径和账号业务权限，须分别验证。
// 社区静音舱在另一站点，不能由 ehall 的验证推断其可用。
//
// 会话由用户自己从浏览器里取出来交给我们（见 credential.Session）。
// 这里只做「带着这份会话去请求」，不实现登录、不保存统一身份认证密码。
const (
	ehallHost      = "ehall.szu.edu.cn"
	ehallBaseURL   = "https://" + ehallHost
	ehallUserAgent = "szuDesktop/0.5 (+https://github.com/Alakazamc/szudesktop)"

	// 单次请求超时。学校服务器偶尔很慢，但也不能无限挂着。
	ehallTimeout = 20 * time.Second
	// 响应体上限，防止异常页面把内存吃光。
	ehallMaxBody = 4 << 20
)

// errSessionInvalid 表示会话不能用（过期、退出登录、或粘错了）。
//
// 单独分出来是因为它需要给用户一句明确的话——
// 不能说成「网络错误」，否则用户会一直重试而不知道要重新登录。
var errSessionPermission = errors.New("当前账号没有所选业务的访问权限，请核对培养层次或在官方系统确认权限")
var errUnsafeEhallURL = errors.New("学校系统连接地址不安全，已停止发送登录状态")

var errSessionInvalid = errors.New("学校系统登录状态已失效，请重新在浏览器登录后再复制一次")

// ehallClient 用一份会话请求 ehall。
type ehallClient struct {
	cookie string
	// base 是站点根地址。生产环境永远是 https://ehall.szu.edu.cn，
	// 留成字段只为让测试能指向本地假服务，不必联网。
	base string
	http *http.Client
}

// newEhallClient 造一个客户端。
//
// ⚠️ Proxy 显式设为 nil，和 portal 那边同一个原因：
// 系统上开着代理或加速器时，请求会被抓走，导致读不到校园系统。
// 访问 ehall 必须直连。
func newEhallClient(cookie string, timeout time.Duration) *ehallClient {
	if timeout <= 0 {
		timeout = ehallTimeout
	}
	return &ehallClient{
		cookie: cookie,
		base:   ehallBaseURL,
		http: &http.Client{
			Timeout:   timeout,
			Transport: &http.Transport{Proxy: nil},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// 会话失效时 ehall 会 302 到统一身份认证登录页。
				// 这里直接拦下来转成明确错误，比跟着跳到最后拿到一个登录页 HTML 更好判断。
				if req.URL.Scheme != "https" || req.URL.User != nil {
					return errUnsafeEhallURL
				}
				if len(via) >= 3 {
					return errors.New("跳转次数过多")
				}
				// 被跳到别的域名（统一身份认证在另一个域）就是会话没了。
				if !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
					return errSessionInvalid
				}
				if req.URL.Query().Get("login") != "" || strings.Contains(req.URL.Path, "/login") {
					return errSessionInvalid
				}
				if !ehallPathAllowed(req.URL.Path) {
					return errUnsafeEhallURL
				}
				return nil
			},
		},
	}
}

// postForm 向 ehall 发一个表单 POST，返回响应体。
func (c *ehallClient) postForm(path string, form url.Values) ([]byte, error) {
	if strings.TrimSpace(c.cookie) == "" {
		return nil, errors.New("还没有学校系统的登录状态")
	}
	base, parseErr := url.Parse(c.base)
	if parseErr != nil || base.Scheme != "https" || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || !ehallPathAllowed(path) {
		return nil, errUnsafeEhallURL
	}
	body := form.Encode()
	req, err := http.NewRequest(http.MethodPost, c.base+path, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("User-Agent", ehallUserAgent)
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	// Cookie 只放请求头里，不进日志、不进错误信息。
	req.Header.Set("Cookie", c.cookie)
	req.Header.Set("Origin", c.base)
	req.Header.Set("Referer", c.base+"/")

	res, err := c.http.Do(req)
	if err != nil {
		if errors.Is(err, errSessionInvalid) {
			return nil, errSessionInvalid
		}
		if errors.Is(err, errUnsafeEhallURL) {
			return nil, errUnsafeEhallURL
		}
		return nil, errors.New("连不上学校系统，请检查网络后重试")
	}
	defer res.Body.Close()

	data, err := io.ReadAll(io.LimitReader(res.Body, ehallMaxBody+1))
	if err != nil {
		return nil, fmt.Errorf("读取学校系统响应失败：%w", err)
	}
	if len(data) > ehallMaxBody {
		return nil, errors.New("学校系统返回的内容异常大，已中止")
	}
	if res.StatusCode == http.StatusUnauthorized {
		return nil, errSessionInvalid
	}
	if res.StatusCode == http.StatusForbidden {
		return nil, errSessionPermission
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("学校系统返回 HTTP %d", res.StatusCode)
	}
	return data, nil
}

func ehallPathAllowed(path string) bool {
	return path == undergradScorePath || path == gradScorePath || path == undergradTimetablePath || path == undergradTermPath
}

// ehallRows 从 ehall 的响应里取出数据行。
//
// ehall 这套框架的返回格式很固定：
//
//	{"code":"0","datas":{"<数据集名>":{"rows":[...]}},"msg":"成功"}
//
// 但失败时可能把话写在 code/msg 里，或者干脆给个 HTML（会话失效）。
// 三种情况都要分开报，不能一律当「没有数据」——
// 那正是「读不到却报空」的坑。
type ehallPage struct {
	Rows  []map[string]any
	Total *int
}

func ehallRows(data []byte, dataset string) ([]map[string]any, error) {
	p, err := parseEhallPage(data, dataset)
	if err != nil {
		return nil, err
	}
	return p.Rows, nil
}
func parseEhallPage(data []byte, dataset string) (*ehallPage, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, errors.New("学校系统返回了空内容")
	}
	// 会话失效时可能直接返回登录页 HTML，此时 JSON 解析会失败。
	if trimmed[0] != '{' && trimmed[0] != '[' {
		if bytes.Contains(trimmed, []byte("统一身份认证")) || bytes.Contains(trimmed, []byte("<html")) {
			return nil, errSessionInvalid
		}
		return nil, errors.New("学校系统返回的不是预期格式，可能是登录状态失效或页面已改版")
	}

	var envelope struct {
		Code  json.RawMessage            `json:"code"`
		Msg   string                     `json:"msg"`
		Datas map[string]json.RawMessage `json:"datas"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return nil, errors.New("学校系统返回的内容无法解析，可能登录状态已失效或系统已改版")
	}
	if code := string(bytes.Trim(envelope.Code, `"`)); code != "" && code != "0" {
		if strings.Contains(envelope.Msg, "无权限") || strings.Contains(envelope.Msg, "权限不足") {
			return nil, errSessionPermission
		}
		if maybeSessionExpired(envelope.Msg) {
			return nil, errSessionInvalid
		}
		return nil, errors.New("学校系统拒绝了这次请求，请到官方页面核对")
	}
	raw, ok := envelope.Datas[dataset]
	if !ok {
		return nil, fmt.Errorf("学校系统返回的数据里没有 %s，页面结构可能已改版", dataset)
	}
	var payload struct {
		Rows  json.RawMessage `json:"rows"`
		Total json.RawMessage `json:"totalSize"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("解析 %s 数据失败，页面结构可能已改版", dataset)
	}
	rowsJSON := bytes.TrimSpace(payload.Rows)
	if len(rowsJSON) == 0 || rowsJSON[0] != '[' {
		return nil, errors.New("学校系统缺少有效的成绩列表，页面结构可能已改版")
	}
	p := &ehallPage{}
	if err := json.Unmarshal(rowsJSON, &p.Rows); err != nil {
		return nil, errors.New("学校成绩列表格式不正确")
	}
	for _, row := range p.Rows {
		if row == nil {
			return nil, errors.New("学校成绩列表包含无效记录")
		}
	}
	if len(payload.Total) > 0 && !bytes.Equal(bytes.TrimSpace(payload.Total), []byte("null")) {
		var total int
		if json.Unmarshal(payload.Total, &total) != nil || total < 0 {
			return nil, errors.New("学校成绩总数格式不正确")
		}
		if total < len(p.Rows) {
			return nil, errors.New("学校成绩条数与总数不一致，请重新读取")
		}
		p.Total = &total
	}
	return p, nil
}

// maybeSessionExpired 判断服务端这句话是不是在说「你没登录」。
func maybeSessionExpired(msg string) bool {
	for _, hint := range []string{"登录", "未认证", "认证失败", "会话", "超时"} {
		if strings.Contains(msg, hint) {
			return true
		}
	}
	return false
}
