package ui

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Alakazamc/szudesktop/internal/credential"
)

/* ---------- 会话存取 ---------- */

type memSessionStore struct {
	value   credential.Session
	saved   int
	deleted int
	failOn  string
}

func (s *memSessionStore) Save(v credential.Session) error {
	if s.failOn == "save" {
		return errors.New("写入失败")
	}
	s.value = v
	s.saved++
	return nil
}
func (s *memSessionStore) Load() (credential.Session, error) {
	if s.value.Cookie == "" {
		return credential.Session{}, credential.ErrSessionNotFound
	}
	return s.value, nil
}
func (s *memSessionStore) Delete() error    { s.value = credential.Session{}; s.deleted++; return nil }
func (s *memSessionStore) Describe() string { return "test memory" }

func TestSessionStatusNeverEchoesTheCookie(t *testing.T) {
	// 会话比密码还敏感：能直接登进学校系统。
	// 状态接口只准回报「有没有、多长」，绝不能把内容吐回去。
	store := &memSessionStore{value: credential.Session{Cookie: "JSESSIONID=super-secret-value", Note: "ehall"}}
	s := &Server{session: store}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1234/api/session", nil)
	s.handleSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "super-secret-value") {
		t.Fatalf("会话内容被回显了: %s", rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["saved"] != true {
		t.Fatalf("saved = %#v", body["saved"])
	}
	if body["cookie_len"].(float64) != float64(len("JSESSIONID=super-secret-value")) {
		t.Fatalf("cookie_len = %#v", body["cookie_len"])
	}
}

func TestSessionSaveAcceptsBrowserCopyPasteForms(t *testing.T) {
	// 用户从浏览器复制时形态各异，这些都是真实会遇到的样子，都得认。
	cases := []struct {
		name, input, want string
		ok                bool
	}{
		{"纯 cookie", "JSESSIONID=abc; route=1", "JSESSIONID=abc; route=1", true},
		{"带 Cookie: 前缀", "Cookie: JSESSIONID=abc", "JSESSIONID=abc", true},
		{"带换行的整块头", "Cookie: JSESSIONID=abc\r\nUser-Agent: Chrome", "JSESSIONID=abc", true},
		{"首尾空白", "   JSESSIONID=abc  ", "JSESSIONID=abc", true},
		{"空的", "", "", false},
		{"只有空白", "   ", "", false},
		{"没有键值对", "这台电脑好卡啊", "", false},
		{"超长", strings.Repeat("a", maxCookieLen+1) + "=1", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sanitizeCookie(tc.input)
			if tc.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.ok {
				if err == nil {
					t.Fatalf("expected rejection, got %q", got)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestSessionDeleteOnlyClearsSessionNotCampusPassword(t *testing.T) {
	// 校园网密码和 ehall 会话是两套东西。
	// 清会话不该动到校园网凭据，否则用户会发现「退出学校系统后网也登不上了」。
	session := &memSessionStore{value: credential.Session{Cookie: "JSESSIONID=abc"}}
	creds := &guardTestStore{value: credential.Credentials{Username: "2024xxxx", Password: "not-real"}}
	s := &Server{session: session, store: creds}

	rec := httptest.NewRecorder()
	s.handleSession(rec, httptest.NewRequest(http.MethodDelete, "http://127.0.0.1:1234/api/session", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if session.deleted != 1 {
		t.Fatal("会话没有被清除")
	}
	if creds.writes != 0 || creds.value.Username != "2024xxxx" {
		t.Fatal("清除会话影响了校园网凭据")
	}
}

/* ---------- 会话失效的四种情形 ---------- */

func TestEhallRowsDistinguishesExpiredFromEmpty(t *testing.T) {
	// 这是本模块最关键的一组判断：读不到 ≠ 没有数据。
	cases := []struct {
		name    string
		body    string
		wantErr error
		wantLen int
	}{
		{
			name:    "正常有数据",
			body:    `{"code":"0","msg":"成功","datas":{"xscjcx":{"rows":[{"KCM":"高等数学","ZCJ":"95"}]}}}`,
			wantLen: 1,
		},
		{
			name:    "正常但没有数据",
			body:    `{"code":"0","msg":"成功","datas":{"xscjcx":{"rows":[]}}}`,
			wantLen: 0,
		},
		{
			name:    "会话失效返回登录页",
			body:    `<html><head><title>统一身份认证</title></head></html>`,
			wantErr: errSessionInvalid,
		},
		{
			name:    "服务端说没登录",
			body:    `{"code":"1","msg":"用户未登录，请重新登录","datas":{}}`,
			wantErr: errSessionInvalid,
		},
		{
			name:    "换了个数据集名（页面改版）",
			body:    `{"code":"0","msg":"成功","datas":{"somethingElse":{"rows":[]}}}`,
			wantLen: -1, // 期望报错
		},
		{
			name:    "空响应",
			body:    ``,
			wantLen: -1,
		},
		{
			name:    "被网关挡住",
			body:    `{"code":"1","msg":"系统繁忙","datas":{}}`,
			wantLen: -1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := ehallRows([]byte(tc.body), "xscjcx")
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if tc.wantLen == -1 {
				if err == nil {
					t.Fatalf("expected error, got %d rows", len(rows))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(rows) != tc.wantLen {
				t.Fatalf("rows = %d, want %d", len(rows), tc.wantLen)
			}
		})
	}
}

func TestScoreParseNeverSilentlyReportsEmpty(t *testing.T) {
	// 服务端给了行，但我们认不出课程名字段 —— 这必须是错误，不是「暂无成绩」。
	// 让用户误以为「查过了没成绩」是本项目最不能犯的错。
	rows := []map[string]any{
		{"WEIRD_FIELD_1": "高等数学", "WEIRD_FIELD_2": "95"},
	}
	out := &scoreResult{Level: "undergrad", Label: "本科"}
	out.Items = nil
	err := checkSuspiciouslyEmpty(out, rows, "本科")
	if err == nil {
		t.Fatal("认不出字段时应当报错，而不是当成没有成绩")
	}
	if !strings.Contains(err.Error(), "不要以本结果为准") {
		t.Fatalf("错误信息应当告诉用户去官方系统核对: %v", err)
	}

	// 服务端本来就给了空列表，这才是真「暂无成绩」，允许通过。
	if err := checkSuspiciouslyEmpty(&scoreResult{}, nil, "本科"); err != nil {
		t.Fatalf("空列表是合法结果: %v", err)
	}
}

/* ---------- 成绩字段解析 ---------- */

func TestParseUndergradAndGraduateFields(t *testing.T) {
	// 两个系统的字段名不一样，这里用各自的真实字段名焊死，
	// 以后谁改错了立刻红。
	undergrad := map[string]any{
		"JXBID": "ABC123", "KCM": "数据结构", "XF": "3.0",
		"ZCJ": "92", "JD": "4.0", "XNXQDM": "2025-2026-1",
	}
	if got := str(undergrad, "KCM", "KCMC"); got != "数据结构" {
		t.Fatalf("本科课程名 = %q", got)
	}
	if got := num(undergrad, "XF"); got != 3.0 {
		t.Fatalf("本科学分 = %v", got)
	}

	grad := map[string]any{
		"KCMC": "机器学习", "XF": 2.5, "DYBFZCJ": "88", "JXBID": "G999",
	}
	if got := str(grad, "KCMC", "KCM"); got != "机器学习" {
		t.Fatalf("研究生课程名 = %q", got)
	}
	if got := num(grad, "XF"); got != 2.5 {
		t.Fatalf("研究生学分 = %v", got)
	}
	// 数字型的成绩也要能取出来（接口有时给 number 有时给 string）
	if got := str(grad, "DYBFZCJ"); got != "88" {
		t.Fatalf("研究生成绩 = %q", got)
	}
}

func TestStrSkipsJSONNullAndBlank(t *testing.T) {
	row := map[string]any{"A": "null", "B": "   ", "C": "真正的内容"}
	if got := str(row, "A", "B", "C"); got != "真正的内容" {
		t.Fatalf("应当跳过 null 和空白，得到 %q", got)
	}
	if got := str(row, "A", "B"); got != "" {
		t.Fatalf("全是空值时应返回空串，得到 %q", got)
	}
}

/* ---------- 接口不泄露会话 ---------- */

func TestScoreEndpointRejectsUnknownLevelAndMissingSession(t *testing.T) {
	s := &Server{session: &memSessionStore{}}

	rec := httptest.NewRecorder()
	s.handleScores(rec, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1234/api/scores?level=phd", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("未知层次应当 400，得到 %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.handleScores(rec, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:1234/api/scores?level=undergrad", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("没有会话时应当明确提示，得到 %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "登录状态") {
		t.Fatalf("提示里应当说清是登录状态的问题: %s", rec.Body.String())
	}
}

func TestScoreRoutesAreGuardedAndReadOnly(t *testing.T) {
	s := &Server{session: &memSessionStore{}}
	mux := http.NewServeMux()
	s.routes(mux, fstest.MapFS{})

	cases := []struct {
		name, method, path string
		want               int
	}{
		{"本机读取成绩", http.MethodGet, "/api/scores?level=undergrad", http.StatusConflict},
		{"跨来源读成绩被拒", http.MethodGet, "/api/scores?level=undergrad", http.StatusForbidden},
		{"成绩接口不接受 POST", http.MethodPost, "/api/scores?level=undergrad", http.StatusMethodNotAllowed},
		{"会话探活不接受 GET", http.MethodGet, "/api/session/check", http.StatusMethodNotAllowed},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "http://127.0.0.1:1234"+tc.path, nil)
			if i == 1 {
				req.Header.Set("Origin", "https://example.com")
				req.Header.Set("Sec-Fetch-Site", "cross-site")
			}
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("%s %s = %d, want %d", tc.method, tc.path, rec.Code, tc.want)
			}
		})
	}
}

/* ---------- 对着假 ehall 服务跑完整读取流程 ---------- */

// 用假服务器替掉真实 ehall。接口路径与字段名都照 tools4szu 里记录的真实形态搭，
// 这样解析逻辑能真的跑一遍，而不是只测到几个小函数。
func newFakeEhall(t *testing.T, handler func(dataset string, form url.Values) (int, string)) (*ehallClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie := r.Header.Get("Cookie"); cookie == "" {
			w.WriteHeader(http.StatusFound)
			w.Header().Set("Location", "https://sso.szu.edu.cn/login")
			return
		}
		_ = r.ParseForm()
		dataset := "xscjcx"
		if strings.Contains(r.URL.Path, "gsapp") {
			dataset = "xscjcx_dqx"
		}
		status, body := handler(dataset, r.Form)
		w.Header().Set("Content-Type", "application/json; charset=UTF-8")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	c := newEhallClient("JSESSIONID=test-only", 5*time.Second)
	// 让客户端打到假服务器上，其余配置保持真实。
	c.base = srv.URL
	return c, srv
}

// ehall 真实域名下的路径拼接保留在一个可替换的 base 上，
// 这样测试不用联网，生产代码也不用为测试留后门。
func (c *ehallClient) withBase(base string) *ehallClient { c.base = base; return c }

func TestReadUndergradScoreEndToEnd(t *testing.T) {
	c, _ := newFakeEhall(t, func(dataset string, form url.Values) (int, string) {
		if dataset != "xscjcx" {
			t.Fatalf("本科成绩不该请求到 %s", dataset)
		}
		return 200, `{"code":"0","msg":"成功","datas":{"xscjcx":{"rows":[
			{"JXBID":"b1","KCM":"高等数学","XF":"5.0","ZCJ":"92","JD":"4.0","XNXQDM":"2024-2025-1","KCXZDM_DISPLAY":"必修"},
			{"JXBID":"b2","KCM":"大学物理","XF":"3.0","ZCJ":"85","JD":"3.5","XNXQDM":"2024-2025-2"},
			{"JXBID":"b2","KCM":"大学物理","XF":"3.0","ZCJ":"85","XNXQDM":"2024-2025-2"}
		]}}}`
	})
	result, err := readUndergradScore(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("同一教学班应当去重，得到 %d 门", len(result.Items))
	}
	if result.Items[0].Term != "2024-2025-2" {
		t.Fatalf("应当按学期倒序，第一个是 %q", result.Items[0].Term)
	}
	if result.Label != "本科" || result.Level != "undergrad" {
		t.Fatalf("层次标注不对: %+v", result)
	}
	if !result.Full {
		t.Fatal("全部取回时 Full 应为 true")
	}
}

func TestReadGraduateScoreUsesItsOwnDataset(t *testing.T) {
	// 研究生走 gsapp，数据集名也不一样。走错就会读成空。
	c, _ := newFakeEhall(t, func(dataset string, form url.Values) (int, string) {
		if dataset != "xscjcx_dqx" {
			t.Fatalf("研究生成绩不该请求到 %s", dataset)
		}
		return 200, `{"code":"0","msg":"成功","datas":{"xscjcx_dqx":{"rows":[
			{"JXBID":"g1","KCMC":"机器学习","XF":2.5,"DYBFZCJ":"88","JD":"3.5","XNXQDM":"2025-2026-1"}
		]}}}`
	})
	result, err := readGradScore(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].Name != "机器学习" {
		t.Fatalf("研究生课程没读对: %+v", result.Items)
	}
	if result.Items[0].Credit != 2.5 {
		t.Fatalf("学分 = %v", result.Items[0].Credit)
	}
}

func TestReadScoreReportsExpiredSessionInsteadOfEmpty(t *testing.T) {
	// 会话过期时必须报错。如果这里退化成「暂无成绩」，
	// 用户会以为学校系统里没有他的成绩 —— 这是最不能犯的错。
	c, _ := newFakeEhall(t, func(dataset string, form url.Values) (int, string) {
		return 200, `<html><head><title>统一身份认证</title></head><body>请登录</body></html>`
	})
	_, err := readUndergradScore(c)
	if !errors.Is(err, errSessionInvalid) {
		t.Fatalf("会话失效应当返回 errSessionInvalid，得到 %v", err)
	}
}

func TestReadScoreReportsRewrittenAPIInsteadOfEmpty(t *testing.T) {
	// 接口改版：字段名换了。必须报错并让用户去官方核对。
	c, _ := newFakeEhall(t, func(dataset string, form url.Values) (int, string) {
		return 200, `{"code":"0","msg":"成功","datas":{"xscjcx":{"rows":[{"NEWNAME":"高等数学","NEWSCORE":"92"}]}}}`
	})
	_, err := readUndergradScore(c)
	if err == nil {
		t.Fatal("字段认不出时必须报错")
	}
	if !strings.Contains(err.Error(), "不要以本结果为准") {
		t.Fatalf("错误信息应当让用户去官方核对: %v", err)
	}
}

func TestReadScorePassesThroughTrulyEmptyResult(t *testing.T) {
	// 服务端明确说没有成绩，这才是真正的「暂无」。
	c, _ := newFakeEhall(t, func(dataset string, form url.Values) (int, string) {
		return 200, `{"code":"0","msg":"成功","datas":{"xscjcx":{"rows":[]}}}`
	})
	result, err := readUndergradScore(c)
	if err != nil {
		t.Fatalf("空列表是合法结果: %v", err)
	}
	if len(result.Items) != 0 || !result.Full {
		t.Fatalf("空结果应当如实返回: %+v", result)
	}
}

func TestNoSessionCookieNeverReachesTheSchool(t *testing.T) {
	// 没会话就不该发请求出去，更不该把「连不上」说成「没有数据」。
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	c := newEhallClient("", 5*time.Second).withBase(srv.URL)
	if _, err := c.postForm(undergradScorePath, allRowsForm(10)); err == nil {
		t.Fatal("没有会话时应当直接报错")
	}
	if called {
		t.Fatal("没有会话时不应该把请求发到学校系统")
	}
}
