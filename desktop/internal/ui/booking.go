package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const bookingPublic = "http://swzx.szu.edu.cn/venue-api"

// 公开空位查询不需要登录。被重定向或拿到 401 说明这张网根本到不了学校的公开接口，
// 不能显示成「没有空位」。
var errBookingUnreachable = errors.New("学校预约服务没有返回公开数据。公开空位查询需要在校园网内，结果请到官方页面核对")
var errBookingFormat = errors.New("学校预约数据格式变化，本次未显示为成功，请到官方页面核对")

// bookingService 只做只读查询。办理预约在学校官方页面完成，
// 这里不留任何写操作通路，也不保存用户粘贴的 Cookie（STATUS.md F23）。
type bookingService struct {
	client *http.Client
}
type bookingType struct {
	Name         string `json:"name"`
	Mask         uint64 `json:"availableTimePeriod"`
	Max          int    `json:"samePersonMaxReservationPerDay"`
	Days         int    `json:"lastReservationDayBeforeAppointment"`
	Blacklist    int    `json:"blacklistValidDuration"`
	Announcement string `json:"announcement"`
}

// validate 把「学校字段是否还在我们的假设范围里」收成一处，供场地列表与单场地空位
// 两条路径共用。之前只有 availability 做了这组边界检查，列表路径没做，于是越界的
// Mask/Max/Days 会一路渲染到界面上（前端要对 64 位掩码做算术，越界值直接不可信）。
func (t bookingType) validate() bool {
	return t.Days >= 1 && t.Days <= 31 && t.Max >= 1 && t.Max <= 48 && t.Mask>>48 == 0
}

// UnmarshalJSON 在解码时就剥净 announcement。场地列表里内嵌的 type 与单独查
// /boothType/info 键集相同，两条路径都走这里，不存在某条漏掉的情况。
func (t *bookingType) UnmarshalJSON(b []byte) error {
	type raw bookingType // 剥掉方法，避免递归
	var r raw
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*t = bookingType(r)
	t.Announcement = plainTextFromSchoolHTML(t.Announcement)
	return nil
}

var (
	// schoolTagRe 匹配任意 HTML 标签；剥完标签的文本里不该再出现它。
	schoolTagRe = regexp.MustCompile(`</?[A-Za-z][^<>]*>`)
	// schoolScriptRe 连内容一起丢掉，避免只剥标签却把脚本代码留在正文里。
	schoolScriptRe = regexp.MustCompile(`(?is)<(script|style)\b[^>]*>.*?</(script|style)\s*>`)
	// schoolCommentRe 去掉 HTML 注释，否则注释文字会原样混进正文。
	schoolCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)
	// schoolBrRe 换行标签，大小写与 `>` / `/>` / ` />` 几种写法都要认。
	schoolBrRe = regexp.MustCompile(`(?i)<br\s*/?>`)
	// schoolBlockCloseRe 块级元素结束标签一律换行。只换 </p></li> 是不够的：学校用
	// <div>/<h3>/<tr> 排版时，整段文字会被挤成一行、失去原有的分段。
	schoolBlockCloseRe = regexp.MustCompile(`(?i)</(p|div|li|ul|ol|tr|td|th|table|thead|tbody|section|article|header|footer|blockquote|pre|figure|h[1-6]|dd|dt|dl)\s*>`)
)

// plainTextFromSchoolHTML 把学校返回的富文本转成纯文本：块级标签和列表项换成
// 换行，script / style 连内容一起丢，注释丢掉，实体解码，行内空白收敛。
//
// 为什么必须在服务端做：这些文本来自学校，直接渲染等于给外部内容开 HTML 通道。
// 剥成纯文本后前端照旧 esc()，两层都不出问题。
func plainTextFromSchoolHTML(s string) string {
	if s == "" {
		return ""
	}
	s = schoolScriptRe.ReplaceAllString(s, "")
	s = schoolCommentRe.ReplaceAllString(s, "")
	s = schoolBrRe.ReplaceAllString(s, "\n")
	s = schoolBlockCloseRe.ReplaceAllString(s, "\n")
	s = schoolTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	// 实体解码后可能又冒出换行或新的标签字符，再收敛一次空白。
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			if blank || len(out) == 0 {
				continue
			}
			blank = true
			out = append(out, "")
			continue
		}
		blank = false
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

type bookingRoom struct {
	ID          int         `json:"id"`
	TypeID      int         `json:"typeId"`
	Name        string      `json:"name"`
	Campus      string      `json:"campus"`
	Community   string      `json:"community"`
	Description string      `json:"description"`
	Enabled     bool        `json:"status"`
	Type        bookingType `json:"type"`
}
type bookingSlot struct {
	Index int    `json:"index"`
	Start string `json:"start"`
	End   string `json:"end"`
	State string `json:"state"`
}
type bookingDay struct {
	Room      bookingRoom   `json:"room"`
	Date      string        `json:"date"`
	Slots     []bookingSlot `json:"slots"`
	FetchedAt time.Time     `json:"fetched_at"`
}

func newBookingService() *bookingService {
	return &bookingService{client: &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// Every call chooses one fixed origin and path, and every call is a GET.
// The HTTP client has no cookie jar and no write path exists to attach one to.
func (b *bookingService) request(ctx context.Context, path string, query url.Values, target any) error {
	u := bookingPublic + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", ehallUserAgent)
	res, err := b.client.Do(req)
	if err != nil {
		return errors.New("无法连接学校预约服务；公开空位查询需要校园网")
	}
	defer res.Body.Close()
	if res.StatusCode == 401 || (res.StatusCode >= 300 && res.StatusCode < 400) {
		return errBookingUnreachable
	}
	if res.StatusCode == 403 {
		return errSessionPermission
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("学校预约服务返回 HTTP %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, ehallMaxBody+1))
	if err != nil || len(data) > ehallMaxBody {
		return errBookingFormat
	}
	var envelope struct {
		Status   int             `json:"status"`
		Data     json.RawMessage `json:"data"`
		Encoding int             `json:"encoding"`
	}
	if json.Unmarshal(data, &envelope) != nil {
		return errBookingFormat
	}
	if envelope.Status == 401 {
		return errBookingUnreachable
	}
	if envelope.Status == 403 {
		return errSessionPermission
	}
	if envelope.Status != 200 {
		return fmt.Errorf("学校预约服务返回状态 %d，本次未显示为成功，请到官方页面核对", envelope.Status)
	}
	if envelope.Encoding != 0 {
		return errBookingFormat
	}
	if target != nil && json.Unmarshal(envelope.Data, target) != nil {
		return errBookingFormat
	}
	return nil
}
func bookingToday(now time.Time) time.Time {
	x := now.In(time.FixedZone("Asia/Shanghai", 8*60*60))
	return time.Date(x.Year(), x.Month(), x.Day(), 0, 0, 0, 0, x.Location())
}
func bookingClock(index int) string { return fmt.Sprintf("%02d:%02d", index/2, (index%2)*30) }

func (b *bookingService) availability(ctx context.Context, id int, date string, now time.Time) (*bookingDay, error) {
	if id <= 0 {
		return nil, errors.New("请选择场地")
	}
	var room bookingRoom
	if err := b.request(ctx, "/booth/info/"+strconv.Itoa(id), nil, &room); err != nil {
		return nil, err
	}
	if room.ID != id || room.Name == "" || room.TypeID <= 0 {
		return nil, errBookingFormat
	}
	if err := b.request(ctx, "/boothType/info/"+strconv.Itoa(room.TypeID), nil, &room.Type); err != nil {
		return nil, err
	}
	if room.Type.Days < 1 || room.Type.Days > 31 || room.Type.Max < 1 || room.Type.Max > 48 || room.Type.Mask>>48 != 0 {
		return nil, errBookingFormat
	}
	today := bookingToday(now)
	day, err := time.ParseInLocation("2006-01-02", date, today.Location())
	if err != nil || day.Before(today) || !day.Before(today.AddDate(0, 0, room.Type.Days)) {
		return nil, errors.New("日期超出学校当前开放预约范围")
	}
	var days []struct {
		Date  string `json:"date"`
		Times []int  `json:"times"`
	}
	if err = b.request(ctx, fmt.Sprintf("/booth/%d/available-time", id), url.Values{"startDate": {date}, "endDate": {date}}, &days); err != nil {
		return nil, err
	}
	var times []int
	for _, d := range days {
		if d.Date == date {
			if times != nil {
				return nil, errBookingFormat
			}
			times = d.Times
		}
	}
	if len(times) != 48 {
		return nil, errBookingFormat
	}
	out := &bookingDay{Room: room, Date: date, Slots: []bookingSlot{}, FetchedAt: now.UTC()}
	for i, state := range times {
		if room.Type.Mask&(uint64(1)<<i) == 0 {
			continue
		}
		label := "unknown"
		switch state {
		case 1:
			label = "available"
		case 0:
			label = "occupied"
		case -1:
			label = "closed"
		}
		if !room.Enabled {
			label = "closed"
		} else if day.Add(time.Duration(i) * 30 * time.Minute).Before(now) {
			label = "past"
		}
		out.Slots = append(out.Slots, bookingSlot{Index: i, Start: bookingClock(i), End: bookingClock(i + 1), State: label})
	}
	return out, nil
}

func writeBookingError(w http.ResponseWriter, err error) {
	code := 502
	if errors.Is(err, errSessionPermission) {
		code = 403
	}
	writeAPIError(w, code, err)
}
func (s *Server) handleBookingRooms(w http.ResponseWriter, r *http.Request) {
	var result struct {
		List  []bookingRoom `json:"list"`
		Total *int          `json:"total"`
	}
	if err := s.booking.request(r.Context(), "/booth/list", url.Values{"current": {"1"}, "pageSize": {"100"}}, &result); err != nil {
		writeBookingError(w, err)
		return
	}
	if result.List == nil || result.Total == nil || *result.Total != len(result.List) {
		writeBookingError(w, errBookingFormat)
		return
	}
	for i := range result.List {
		room := &result.List[i]
		if room.ID <= 0 || room.Name == "" {
			writeBookingError(w, errBookingFormat)
			return
		}
		// 列表这条路上，字段异常只收敛不拒绝。availability 会硬校验 type，因为掩码错
		// 一格就会显示错误的空位；而列表只用 type 展示规则，整份数据因一个场地被扣下
		// 会让用户什么都看不到。收敛成零值后前端显示「—」，等于如实说「这项未知」。
		if !room.Type.validate() {
			room.Type = bookingType{Name: room.Type.Name, Announcement: room.Type.Announcement}
		}
	}
	writeJSON(w, map[string]any{"rooms": result.List, "today": bookingToday(time.Now()).Format("2006-01-02"), "fetched_at": time.Now().UTC()})
}
func (s *Server) handleBookingAvailability(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("room"))
	result, err := s.booking.availability(r.Context(), id, r.URL.Query().Get("date"), time.Now())
	if err != nil {
		writeBookingError(w, err)
		return
	}
	writeJSON(w, result)
}
