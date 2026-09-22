package ui

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// 学校返回的 announcement 是带内联样式的 HTML，里面完全可能混着 <script> 和
// 事件属性。它只能在 Go 层被剥成纯文本：原始 HTML 不进浏览器，前端照旧 esc()，
// 不给校方内容开任何 HTML 注入通道。
func TestBookingTypeAnnouncementIsStrippedToPlainText(t *testing.T) {
	raw := `{"name":"共享琴房","availableTimePeriod":4397794590720,` +
		`"samePersonMaxReservationPerDay":4,"lastReservationDayBeforeAppointment":3,` +
		`"blacklistValidDuration":1,"announcement":"<p style=\"text-align:center;\">` +
		`<strong>使用须知</strong></p><ol><li>每人每日可预约4个时段；</li>` +
		`<li>超时未签<img src=x onerror=alert(1)>；</li></ol>` +
		`<script>alert(2)</script>&nbsp;第二次预约会被拉黑。"}`

	var bt bookingType
	if err := json.Unmarshal([]byte(raw), &bt); err != nil {
		t.Fatalf("解码学校返回的类型失败: %v", err)
	}
	if bt.Name != "共享琴房" {
		t.Fatalf("场地类型名没解出来: %+v", bt)
	}
	if bt.Blacklist != 1 {
		t.Fatalf("爽约黑名单时长没解出来: %+v", bt)
	}

	if schoolTagRe.MatchString(bt.Announcement) {
		t.Fatalf("announcement 里还留着 HTML 标签: %q", bt.Announcement)
	}
	for _, gone := range []string{"onerror", "alert(2)", "<script"} {
		if strings.Contains(bt.Announcement, gone) {
			t.Fatalf("script / 事件属性没清掉，出现了 %q: %q", gone, bt.Announcement)
		}
	}
	for _, keep := range []string{"使用须知", "每人每日可预约4个时段；", "超时未签；", "第二次预约会被拉黑。"} {
		if !strings.Contains(bt.Announcement, keep) {
			t.Fatalf("正文内容被吃掉了，少了 %q: %q", keep, bt.Announcement)
		}
	}
	if !strings.Contains(bt.Announcement, "\n") {
		t.Fatalf("列表项没有换行，粘成一坨: %q", bt.Announcement)
	}
}

// 场地列表里内嵌的 type 与单独查 /boothType/info 键集相同，所以同一条解码路径
// 必须把 announcement 一起剥干净——不能只在 availability 那条路上生效。
func TestBookingRoomsAnnouncementIsSanitizedThroughRoomList(t *testing.T) {
	body := `{"status":200,"data":{"list":[{"id":1,"typeId":1,"name":"测试会议室",` +
		`"campus":"粤海","community":"时光","status":true,` +
		`"type":{"id":1,"name":"会议室","availableTimePeriod":4397794590720,` +
		`"samePersonMaxReservationPerDay":4,"lastReservationDayBeforeAppointment":3,` +
		`"blacklistValidDuration":1,"announcement":"<p>须知<b>加粗</b></p><script>x</script>"}}],` +
		`"total":1}}`

	var result struct {
		Data struct {
			List  []bookingRoom `json:"list"`
			Total *int          `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("解码场地列表失败: %v", err)
	}
	if len(result.Data.List) != 1 {
		t.Fatalf("场地没解出来: %d 条", len(result.Data.List))
	}
	got := result.Data.List[0].Type.Announcement
	if schoolTagRe.MatchString(got) {
		t.Fatalf("场地列表里的 announcement 没剥干净: %q", got)
	}
	if !strings.Contains(got, "须知加粗") {
		t.Fatalf("正文被吃掉了: %q", got)
	}
	if strings.Contains(got, "x") && !strings.Contains(got, "须") {
		t.Fatalf("script 内容漏进来了: %q", got)
	}
}

// 空 / 只有空格的 announcement 不能变成一堆空白行。
func TestBookingAnnouncementWhitespaceStaysEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", "<p></p>", "&nbsp;"} {
		var bt bookingType
		if err := json.Unmarshal([]byte(`{"announcement":"`+in+`"}`), &bt); err != nil {
			t.Fatalf("解码失败: %v", err)
		}
		if strings.TrimSpace(bt.Announcement) != "" {
			t.Fatalf("%q 应剥成空，得到 %q", in, bt.Announcement)
		}
	}
}

// 学校用 div / h3 / table 排版时也必须分段。只认 </p></li> 会让整段须知挤成一行，
// 用户看到的是「第一段标题正文」这种连在一起的文本。
func TestPlainTextFromSchoolHTMLBreaksBlocksAndDropsComments(t *testing.T) {
	in := `<div>第一段</div><h3>标题</h3>正文<br>第二行<BR/>第三行` +
		`<!-- 删掉我 --><table><tr><td>A</td><td>B</td></tr></table>`
	got := plainTextFromSchoolHTML(in)
	if schoolTagRe.MatchString(got) {
		t.Fatalf("还留着 HTML 标签: %q", got)
	}
	if strings.Contains(got, "删掉我") {
		t.Fatalf("HTML 注释没被丢掉: %q", got)
	}
	for _, want := range []string{"第一段", "标题", "正文", "第二行", "第三行", "A", "B"} {
		if !strings.Contains(got, want) {
			t.Fatalf("正文被吃掉了，少了 %q: %q", want, got)
		}
	}
	if strings.Contains(got, "第一段标题") {
		t.Fatalf("块级标签没有换行，段落粘在一起: %q", got)
	}
	if strings.Contains(got, "第二行第三行") {
		t.Fatalf("大写 / 自闭合的 <BR> 没有换成换行: %q", got)
	}
}

// validate 是 availability 与场地列表共用的边界。掩码超过 48 位、单日格数或可提前
// 天数越界，都说明学校字段变了，不能再当成可信数字用。
func TestBookingTypeValidateBoundaries(t *testing.T) {
	if !(bookingType{Max: 1, Days: 1, Mask: 1}).validate() {
		t.Fatal("边界内的最小合法值应通过校验")
	}
	for _, bad := range []bookingType{
		{Max: 0, Days: 1, Mask: 1},
		{Max: 49, Days: 1, Mask: 1},
		{Max: 1, Days: 0, Mask: 1},
		{Max: 1, Days: 32, Mask: 1},
		{Max: 1, Days: 1, Mask: uint64(1) << 48},
		{Max: 1, Days: 1, Mask: uint64(1) << 63},
	} {
		if bad.validate() {
			t.Fatalf("越界的类型不该通过校验: %+v", bad)
		}
	}
}

// 列表这条路上，单个场地的规则数字异常不能连累其它场地：返回 200，越界数字收敛成
// 零值（前端显示「—」），场地本身照旧出现。硬校验只属于 availability。
func TestBookingRoomsSanitizesImplausibleTypeInsteadOfFailing(t *testing.T) {
	s := bookingFixture(t)
	// availableTimePeriod = 1<<50，掩码超过 48 位；单日格数与可提前天数也都是 0。
	body := `{"status":200,"data":{"list":[{"id":1,"typeId":1,"name":"测试会议室","status":true,` +
		`"type":{"id":1,"name":"会议室","availableTimePeriod":1125899906842624,` +
		`"samePersonMaxReservationPerDay":0,"lastReservationDayBeforeAppointment":0,` +
		`"blacklistValidDuration":0}}],"total":1}}`
	s.booking.client.Transport = calendarTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	mux := http.NewServeMux()
	s.routes(mux, fstest.MapFS{})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/booking/rooms", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("一个场地的规则越界不该让整份列表失败，得到 %d: %s", w.Code, w.Body.String())
	}
	var got struct {
		Rooms []bookingRoom `json:"rooms"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("解析返回失败: %v", err)
	}
	if len(got.Rooms) != 1 {
		t.Fatalf("场地数量不对: %d", len(got.Rooms))
	}
	if got.Rooms[0].Name != "测试会议室" || got.Rooms[0].Type.Name != "会议室" {
		t.Fatalf("场地本身不该被丢掉: %+v", got.Rooms[0])
	}
	if t2 := got.Rooms[0].Type; t2.Mask != 0 || t2.Max != 0 || t2.Days != 0 {
		t.Fatalf("越界的规则数字应被收敛成零值，得到 %+v", t2)
	}
}
