package ui

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"time"
)

const bookingPublic = "http://swzx.szu.edu.cn/venue-api"
const bookingSecure = "https://swzx.webvpn.szu.edu.cn/venue-api"

var errBookingSession = errors.New("预约登录状态不可用，请先在官方 WebVPN 完成登录，再更新预约专用 Cookie")
var errBookingFormat = errors.New("学校预约数据格式变化，本次未显示为成功，请到官方页面核对")

type bookingService struct {
	mu      sync.Mutex
	client  *http.Client
	cookie  string // Only memory; never sent to the public HTTP endpoint.
	pending *bookingPending
}
type bookingType struct {
	Mask         uint64 `json:"availableTimePeriod"`
	Max          int    `json:"samePersonMaxReservationPerDay"`
	Days         int    `json:"lastReservationDayBeforeAppointment"`
	Announcement string `json:"announcement"`
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
type bookingInput struct {
	RoomID int    `json:"boothId"`
	Phone  string `json:"phone"`
	Grade  string `json:"grade"`
	Date   string `json:"date"`
	Times  []int  `json:"timeList"`
	Agree  bool   `json:"agree"`
}
type bookingPending struct {
	Input   bookingInput
	Token   string
	Expires time.Time
}
type bookingRecord struct {
	ID     int         `json:"id"`
	Date   string      `json:"date"`
	Start  string      `json:"reservationStartTime"`
	End    string      `json:"reservationEndTime"`
	Status *int        `json:"status"`
	Room   bookingRoom `json:"booth"`
}
type bookingHistory struct {
	List  []bookingRecord `json:"list"`
	Total *int            `json:"total"`
}

func newBookingService() *bookingService {
	return &bookingService{client: &http.Client{Timeout: 20 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

// Every call chooses one fixed origin and path. The HTTP client has no cookie jar.
func (b *bookingService) request(ctx context.Context, path string, query url.Values, input any, authenticated bool, target any) error {
	base := bookingPublic
	if authenticated {
		if b.cookie == "" {
			return errBookingSession
		}
		base = bookingSecure
	}
	method := http.MethodGet
	var body io.Reader
	if input != nil {
		if !authenticated || path != "/boothReservation/add" {
			return errors.New("不支持的预约操作")
		}
		data, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
		method = http.MethodPost
	}
	u := base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", ehallUserAgent)
	if authenticated {
		req.Header.Set("Cookie", b.cookie)
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://swzx.webvpn.szu.edu.cn")
		req.Header.Set("Referer", "https://swzx.webvpn.szu.edu.cn/")
	}
	res, err := b.client.Do(req)
	if err != nil {
		return errors.New("无法连接学校预约服务；公开空位查询需要校园网，登录业务需要可用的 WebVPN 会话")
	}
	defer res.Body.Close()
	if res.StatusCode == 401 || (res.StatusCode >= 300 && res.StatusCode < 400) {
		return errBookingSession
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
		if authenticated {
			return errBookingSession
		}
		return errBookingFormat
	}
	if envelope.Status == 401 {
		return errBookingSession
	}
	if envelope.Status == 403 {
		return errSessionPermission
	}
	if envelope.Status != 200 {
		return errors.New("学校未接受这次预约请求，请在官方页面查看具体原因")
	}
	if envelope.Encoding != 0 {
		return errBookingFormat
	}
	if input != nil && bytes.Equal(bytes.TrimSpace(envelope.Data), []byte("false")) {
		return errors.New("学校没有确认预约成功")
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

func (b *bookingService) availability(ctx context.Context, id int, date string, now time.Time, authenticated bool) (*bookingDay, error) {
	if id <= 0 {
		return nil, errors.New("请选择场地")
	}
	var room bookingRoom
	if err := b.request(ctx, "/booth/info/"+strconv.Itoa(id), nil, nil, authenticated, &room); err != nil {
		return nil, err
	}
	if room.ID != id || room.Name == "" || room.TypeID <= 0 {
		return nil, errBookingFormat
	}
	if err := b.request(ctx, "/boothType/info/"+strconv.Itoa(room.TypeID), nil, nil, authenticated, &room.Type); err != nil {
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
	if err = b.request(ctx, fmt.Sprintf("/booth/%d/available-time", id), url.Values{"startDate": {date}, "endDate": {date}}, nil, authenticated, &days); err != nil {
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

func (b *bookingService) history(ctx context.Context, page int) (*bookingHistory, error) {
	var h bookingHistory
	if err := b.request(ctx, "/boothReservation/my", url.Values{"current": {strconv.Itoa(page)}, "pageSize": {"30"}}, nil, true, &h); err != nil {
		return nil, err
	}
	if h.List == nil || h.Total == nil || *h.Total < len(h.List) {
		return nil, errBookingFormat
	}
	for _, r := range h.List {
		if r.ID <= 0 || r.Date == "" || r.Room.Name == "" || r.Start == "" || r.End == "" || r.Status == nil {
			return nil, errBookingFormat
		}
	}
	return &h, nil
}

func writeBookingError(w http.ResponseWriter, err error) {
	code := 502
	if errors.Is(err, errBookingSession) {
		code = 401
	} else if errors.Is(err, errSessionPermission) {
		code = 403
	}
	writeAPIError(w, code, err)
}
func (s *Server) handleBookingRooms(w http.ResponseWriter, r *http.Request) {
	var result struct {
		List  []bookingRoom `json:"list"`
		Total *int          `json:"total"`
	}
	if err := s.booking.request(r.Context(), "/booth/list", url.Values{"current": {"1"}, "pageSize": {"100"}}, nil, false, &result); err != nil {
		writeBookingError(w, err)
		return
	}
	if result.List == nil || result.Total == nil || *result.Total != len(result.List) {
		writeBookingError(w, errBookingFormat)
		return
	}
	for _, room := range result.List {
		if room.ID <= 0 || room.Name == "" {
			writeBookingError(w, errBookingFormat)
			return
		}
	}
	writeJSON(w, map[string]any{"rooms": result.List, "today": bookingToday(time.Now()).Format("2006-01-02"), "fetched_at": time.Now().UTC()})
}
func (s *Server) handleBookingAvailability(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.URL.Query().Get("room"))
	result, err := s.booking.availability(r.Context(), id, r.URL.Query().Get("date"), time.Now(), false)
	if err != nil {
		writeBookingError(w, err)
		return
	}
	writeJSON(w, result)
}
func (s *Server) handleBookingSession(w http.ResponseWriter, r *http.Request) {
	b := s.booking
	b.mu.Lock()
	defer b.mu.Unlock()
	if r.Method == http.MethodDelete {
		b.cookie = ""
		b.pending = nil
		writeJSON(w, map[string]bool{"authenticated": false})
		return
	}
	if r.Method == http.MethodPost {
		b.cookie = ""
		b.pending = nil
		var input struct {
			Cookie string `json:"cookie"`
		}
		if json.NewDecoder(r.Body).Decode(&input) != nil {
			writeAPIError(w, 400, errors.New("请粘贴预约专用 Cookie"))
			return
		}
		cookie, err := sanitizeCookie(input.Cookie)
		if err != nil {
			writeAPIError(w, 400, err)
			return
		}
		b.cookie = cookie
		if _, err = b.history(r.Context(), 1); err != nil {
			b.cookie = ""
			writeBookingError(w, err)
			return
		}
	}
	writeJSON(w, map[string]bool{"authenticated": b.cookie != ""})
}
func (s *Server) handleBookingHistory(w http.ResponseWriter, r *http.Request) {
	b := s.booking
	b.mu.Lock()
	defer b.mu.Unlock()
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 || page > 100 {
		page = 1
	}
	h, err := b.history(r.Context(), page)
	if err != nil {
		if errors.Is(err, errBookingSession) {
			b.cookie = ""
			b.pending = nil
		}
		writeBookingError(w, err)
		return
	}
	writeJSON(w, map[string]any{"records": h.List, "total": h.Total, "page": page})
}
func validateBooking(input bookingInput, day *bookingDay) error {
	grade, err := strconv.Atoi(input.Grade)
	if !input.Agree || err != nil || grade <= 1000 || grade >= 3000 || !regexp.MustCompile(`^1\d{10}$`).MatchString(input.Phone) {
		return errors.New("请填写有效的手机号、入学年份并阅读使用须知")
	}
	if len(input.Times) == 0 || len(input.Times) > day.Room.Type.Max {
		return errors.New("所选时段超过学校单日上限或没有选择时段")
	}
	free := map[int]bool{}
	for _, slot := range day.Slots {
		free[slot.Index] = slot.State == "available"
	}
	for _, index := range input.Times {
		if !free[index] {
			return errors.New("所选时段已不可预约，请刷新后重新选择")
		}
		free[index] = false
	}
	return nil
}
func (s *Server) handleBookingPrepare(w http.ResponseWriter, r *http.Request) {
	b := s.booking
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pending = nil
	var input bookingInput
	if json.NewDecoder(r.Body).Decode(&input) != nil {
		writeAPIError(w, 400, errors.New("预约内容格式不正确"))
		return
	}
	if _, err := b.history(r.Context(), 1); err != nil {
		writeBookingError(w, err)
		return
	}
	day, err := b.availability(r.Context(), input.RoomID, input.Date, time.Now(), true)
	if err != nil {
		writeBookingError(w, err)
		return
	}
	if err = validateBooking(input, day); err != nil {
		writeAPIError(w, 400, err)
		return
	}
	var token [24]byte
	if _, err = rand.Read(token[:]); err != nil {
		writeAPIError(w, 500, errors.New("无法准备预约"))
		return
	}
	sort.Ints(input.Times)
	b.pending = &bookingPending{Input: input, Token: hex.EncodeToString(token[:]), Expires: time.Now().Add(2 * time.Minute)}
	labels := []string{}
	for _, i := range input.Times {
		labels = append(labels, bookingClock(i)+"–"+bookingClock(i+1))
	}
	writeJSON(w, map[string]any{"token": b.pending.Token, "room": day.Room.Name, "date": input.Date, "times": labels})
}
func (s *Server) handleBookingCommit(w http.ResponseWriter, r *http.Request) {
	b := s.booking
	b.mu.Lock()
	defer b.mu.Unlock()
	var input struct {
		Token string `json:"token"`
	}
	if json.NewDecoder(r.Body).Decode(&input) != nil || b.pending == nil || input.Token != b.pending.Token || time.Now().After(b.pending.Expires) {
		writeAPIError(w, 409, errors.New("预约确认已失效，请重新核对"))
		return
	}
	pending := b.pending
	b.pending = nil // Consume before contacting the school, including on timeout.
	day, err := b.availability(r.Context(), pending.Input.RoomID, pending.Input.Date, time.Now(), true)
	if err != nil {
		writeBookingError(w, err)
		return
	}
	if err = validateBooking(pending.Input, day); err != nil {
		writeAPIError(w, 409, err)
		return
	}
	p := pending.Input
	payload := map[string]any{"boothId": p.RoomID, "phone": p.Phone, "grade": p.Grade, "date": p.Date, "timeList": p.Times}
	if err = b.request(r.Context(), "/boothReservation/add", nil, payload, true, nil); err != nil {
		writeAPIError(w, 502, errors.New("未能确认提交结果。请先查看「我的预约」或学校原页，确认没有生成记录后再重新预约；不要直接重复提交。"))
		return
	}
	writeJSON(w, map[string]any{"ok": true, "message": "学校已接受预约请求，请查看我的预约核对结果"})
}
