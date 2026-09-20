package ui

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const undergradTimetablePath = "/jwapp/sys/wdkb/modules/xskcb/xskcb.do"
const undergradTermPath = "/jwapp/sys/wdkb/modules/jshkcb/dqxnxq.do"
const undergradTimetableHome = "https://ehall.szu.edu.cn/jwapp/sys/wdkb/*default/index.do"

type undergradCourse struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Class       string `json:"class"`
	Teacher     string `json:"teacher"`
	Arrangement string `json:"arrangement"`
}

// Preserve YPSJDD verbatim. Its combined week/period/location grammar must not
// be guessed from the graduate system or promoted to a verified weekly grid.
func parseUndergradTimetable(data []byte) ([]undergradCourse, error) {
	page, err := parseEhallPage(data, "xskcb")
	if err != nil {
		return nil, err
	}
	if page.Total != nil && *page.Total != len(page.Rows) {
		return nil, errors.New("学校返回的课表不完整，请到本科我的课表核对")
	}
	courses := []undergradCourse{}
	for _, row := range page.Rows {
		name := str(row, "KCM")
		if name == "" {
			return nil, errors.New("学校课表课程名称无法识别，未显示不完整课表")
		}
		courses = append(courses, undergradCourse{Name: name, Code: str(row, "KCH"), Class: str(row, "KXH"), Teacher: str(row, "SKJS"), Arrangement: str(row, "YPSJDD")})
	}
	return courses, nil
}
func (s *Server) handleUndergradTimetable(w http.ResponseWriter, r *http.Request) {
	term := strings.TrimSpace(r.URL.Query().Get("term"))
	validTerm := regexp.MustCompile(`^\d{4}-\d{4}-[123]$`)
	if term != "" && !validTerm.MatchString(term) {
		writeAPIError(w, 400, errors.New("学期格式应为 2026-2027-1"))
		return
	}
	v, err := s.sessionStore().Load()
	if err != nil {
		writeSessionLoadError(w, err)
		return
	}
	c := s.makeEhallClient(v.Cookie)
	if term == "" {
		body, err := c.postForm(undergradTermPath, url.Values{})
		if err != nil {
			writeSchoolError(w, err)
			return
		}
		rows, err := ehallRows(body, "dqxnxq")
		if err != nil {
			writeSchoolError(w, err)
			return
		}
		if len(rows) != 1 || !validTerm.MatchString(str(rows[0], "DM")) {
			writeAPIError(w, 502, errors.New("未能确认学校当前学期，请在本科课表中填写官方学期后重试"))
			return
		}
		term = str(rows[0], "DM")
	}
	form := allRowsForm(500)
	form.Set("XNXQDM", term)
	body, err := c.postForm(undergradTimetablePath, form)
	if err != nil {
		writeSchoolError(w, err)
		return
	}
	courses, err := parseUndergradTimetable(body)
	if err != nil {
		writeSchoolError(w, err)
		return
	}
	writeJSON(w, map[string]any{"level": "undergrad", "term": term, "courses": courses, "fetched_at": time.Now().UTC(), "source": undergradTimetableHome})
}
