package ui

import (
	"encoding/json"
	"strings"
	"testing"
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
