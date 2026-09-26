package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakePiano 起一个假琴房系统，记录收到的请求，便于断言载荷与 token 传递方式。
type fakePiano struct {
	srv      *httptest.Server
	gotLogin map[string]any
	gotTok   []string // 每次带 token 的请求记下的 token 值
	loginTok string
	failNext bool
}

func newFakePiano(t *testing.T) *fakePiano {
	f := &fakePiano{loginTok: "tok-123"}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/user/login":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.gotLogin = body
			if body["cardNo"] == "" || body["pwd"] == "" {
				_, _ = w.Write([]byte(`{"code":4000,"success":false,"message":"卡号不能为空"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":200,"success":true,"data":{"token":"` + f.loginTok + `"}}`))
		case "/room/findPage":
			f.gotTok = append(f.gotTok, r.URL.Query().Get("token"))
			if f.failNext {
				_, _ = w.Write([]byte(`{"code":401,"success":false,"message":"token过期"}`))
				return
			}
			_, _ = w.Write([]byte(`{"code":200,"success":true,"count":2,"data":[` +
				`{"id":7,"roomName":"<b>文华</b>301","deviceNo":"D-01","userName":"张老师","description":"  有钢琴  "},` +
				`{"roomName":"文华302"}]}`))
		case "/roomReserve/myReserve":
			f.gotTok = append(f.gotTok, r.URL.Query().Get("token"))
			_, _ = w.Write([]byte(`{"code":200,"success":true,"data":[` +
				`{"roomName":"文华301","reserveTime":"2026-09-25 10:00-11:00","isSign":1},` +
				`{"roomName":"文华302","reserveTime":"2026-09-26 14:00-15:00","isSign":false}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakePiano) service() *pianoService {
	p := newPianoService()
	p.base = f.srv.URL + "/"
	return p
}

func TestPianoLoginSendsCardNoAndPwdAndKeepsToken(t *testing.T) {
	f := newFakePiano(t)
	p := f.service()
	if err := p.login(context.Background(), " 2099000001 ", "secret"); err != nil {
		t.Fatal(err)
	}
	if f.gotLogin["cardNo"] != "2099000001" || f.gotLogin["pwd"] != "secret" {
		t.Fatalf("登录载荷不对，应是 {cardNo,pwd} 且卡号去空白：%v", f.gotLogin)
	}
	if p.currentToken() != "tok-123" {
		t.Fatalf("token 没存下：%q", p.currentToken())
	}
	if !p.loggedIn() {
		t.Fatal("loggedIn 应为 true")
	}
}

func TestPianoRoomsSendsTokenInQueryAndMapsFields(t *testing.T) {
	f := newFakePiano(t)
	p := f.service()
	if err := p.login(context.Background(), "1", "2"); err != nil {
		t.Fatal(err)
	}
	rooms, count, err := p.rooms(context.Background(), 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.gotTok) != 1 || f.gotTok[0] != "tok-123" {
		t.Fatalf("token 必须走 query 且等于登录拿到的：%v", f.gotTok)
	}
	if count != 2 || len(rooms) != 2 {
		t.Fatalf("count/列表长度不对：count=%d len=%d", count, len(rooms))
	}
	if rooms[0].Name != "文华301" {
		t.Fatalf("HTML 标签应被剥掉：got %q", rooms[0].Name)
	}
	if rooms[0].Device != "D-01" || rooms[0].Manager != "张老师" {
		t.Fatalf("device/manager 映射不对：%+v", rooms[0])
	}
	if rooms[0].Desc != "有钢琴" {
		t.Fatalf("desc 应收敛空白：got %q", rooms[0].Desc)
	}
	if rooms[1].Device != "" {
		t.Fatalf("缺失字段应为空串（前端显示—）：got %q", rooms[1].Device)
	}
}

func TestPianoRoomsRequiresLogin(t *testing.T) {
	f := newFakePiano(t)
	p := f.service()
	if _, _, err := p.rooms(context.Background(), 1, 20); err != errPianoNeedLogin {
		t.Fatalf("未登录应报需登录，got %v", err)
	}
}

func TestPianoFailedAccountSwitchClearsOldToken(t *testing.T) {
	p := newFakePiano(t).service()
	if err := p.login(context.Background(), "first", "fake-password"); err != nil {
		t.Fatal(err)
	}
	if err := p.login(context.Background(), "second", ""); err == nil {
		t.Fatal("empty password accepted")
	}
	if p.loggedIn() {
		t.Fatal("previous account remained available after failed switch")
	}
}

func TestPianoRooms401BecomesNeedLogin(t *testing.T) {
	f := newFakePiano(t)
	p := f.service()
	_ = p.login(context.Background(), "1", "2")
	f.failNext = true
	if _, _, err := p.rooms(context.Background(), 1, 20); err != errPianoNeedLogin {
		t.Fatalf("服务端 401 应转成需登录，got %v", err)
	}
}

func TestPianoUnreachableWhenServerDown(t *testing.T) {
	f := newFakePiano(t)
	p := f.service()
	if err := p.login(context.Background(), "1", "2"); err != nil {
		t.Fatal(err)
	}
	f.srv.Close() // 已登录后再关掉假服务器，模拟「不在校园网/被代理截走」
	if _, _, err := p.rooms(context.Background(), 1, 20); err != errPianoUnreachable {
		t.Fatalf("连不上应报不可达（且提示代理/校园网），got %v", err)
	}
	if !strings.Contains(errPianoUnreachable.Error(), "校园网") {
		t.Fatal("不可达提示应提到校园网")
	}
}

func TestPianoMyReservesMapsSignStates(t *testing.T) {
	f := newFakePiano(t)
	p := f.service()
	_ = p.login(context.Background(), "1", "2")
	list, err := p.myReserves(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("len=%d", len(list))
	}
	if list[0].Room != "文华301" || list[0].Time != "2026-09-25 10:00-11:00" || list[0].Sign != "已签到" {
		t.Fatalf("第一条映射不对：%+v", list[0])
	}
	if list[1].Sign != "未签到" {
		t.Fatalf("isSign=false 应显示未签到：%+v", list[1])
	}
}

func TestPianoTextSanitizesAndTruncates(t *testing.T) {
	if got := pianoText("a<script>x</script>b"); got != "ab" {
		t.Fatalf("script 连内容一起丢，got %q", got)
	}
	if got := pianoText("<b>文华</b>301"); got != "文华301" {
		t.Fatalf("标签应整段剥掉，got %q", got)
	}
	long := strings.Repeat("字", 300)
	if got := pianoText(long); len([]rune(got)) != 201 || !strings.HasSuffix(got, "…") {
		t.Fatalf("截断不对：len=%d", len([]rune(got)))
	}
}
