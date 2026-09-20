package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"
)

type timetableEntry struct {
	Name      string `json:"name"`
	Code      string `json:"code,omitempty"`
	Class     string `json:"class,omitempty"`
	Teacher   string `json:"teacher,omitempty"`
	Room      string `json:"room,omitempty"`
	Weeks     string `json:"weeks,omitempty"`
	WeekMask  string `json:"week_mask,omitempty"`
	Day       int    `json:"day"`
	Start     int    `json:"start"`
	End       int    `json:"end"`
	TimeStart string `json:"time_start,omitempty"`
	TimeEnd   string `json:"time_end,omitempty"`
	Scheme    string `json:"scheme,omitempty"`
}

type timetableResult struct {
	Level       string           `json:"level"`
	Term        string           `json:"term"`
	Round       string           `json:"round,omitempty"`
	Entries     []timetableEntry `json:"entries"`
	Unscheduled []timetableEntry `json:"unscheduled"`
	FetchedAt   time.Time        `json:"fetched_at"`
	Source      string           `json:"source"`
}

func timetableRows(raw json.RawMessage) ([]map[string]any, error) {
	trim := bytes.TrimSpace(raw)
	var rows []map[string]any
	if len(trim) == 0 || trim[0] != '[' || json.Unmarshal(trim, &rows) != nil {
		return nil, errors.New("学校课表列表格式变化，请到官方系统核对")
	}
	for _, row := range rows {
		if row == nil {
			return nil, errors.New("学校课表含有无法识别的记录")
		}
	}
	return rows, nil
}

func parseGraduateTimetable(b []byte) (*timetableResult, error) {
	var payload struct {
		Results  json.RawMessage `json:"results"`
		Selected json.RawMessage `json:"xkjgList"`
	}
	if json.Unmarshal(b, &payload) != nil {
		return nil, errors.New("学校未返回有效课表，请重新登录后读取")
	}
	rows, err := timetableRows(payload.Results)
	if err != nil {
		return nil, err
	}
	selected, err := timetableRows(payload.Selected)
	if err != nil {
		return nil, err
	}
	result := &timetableResult{Level: "graduate", Entries: []timetableEntry{}, Unscheduled: []timetableEntry{}, Source: graduateHome, FetchedAt: time.Now().UTC()}
	arranged := map[string]bool{}
	for _, row := range rows {
		entry := timetableEntry{Name: str(row, "KCMC"), Code: str(row, "KCDM"), Class: str(row, "BJMC"), Teacher: str(row, "JSXM"), Room: str(row, "JASMC"), Weeks: str(row, "ZCMC"), WeekMask: str(row, "ZCBH"), TimeStart: str(row, "KSSJ"), TimeEnd: str(row, "JSSJ"), Scheme: str(row, "JCFAMC")}
		entry.Day, _ = strconv.Atoi(str(row, "XQ"))
		entry.Start, _ = strconv.Atoi(str(row, "KSJCDM"))
		entry.End, _ = strconv.Atoi(str(row, "JSJCDM"))
		if entry.Name == "" || entry.Day < 1 || entry.Day > 7 || entry.Start < 1 || entry.End < entry.Start || entry.End > 30 {
			return nil, errors.New("部分课程的日期或节次无法识别，未显示不完整课表；请到官方系统核对")
		}
		arranged[str(row, "BJDM")] = true
		result.Entries = append(result.Entries, entry)
	}
	for _, row := range selected {
		id := str(row, "BJDM")
		if id == "" || str(row, "KCMC") == "" {
			return nil, errors.New("学校已选课程记录不完整，请到官方系统核对")
		}
		if !arranged[id] {
			result.Unscheduled = append(result.Unscheduled, timetableEntry{Name: str(row, "KCMC"), Code: str(row, "KCDM"), Teacher: str(row, "JSXM")})
		}
	}
	sort.SliceStable(result.Entries, func(i, j int) bool {
		a, b := result.Entries[i], result.Entries[j]
		if a.Day != b.Day {
			return a.Day < b.Day
		}
		return a.Start < b.Start
	})
	return result, nil
}

func (s *Server) handleTimetable(w http.ResponseWriter, r *http.Request) {
	a := s.academic
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.authenticated || a.client == nil {
		writeAPIError(w, 409, errors.New("请先登录研究生教务，再读取课表"))
		return
	}
	if _, err := readGraduateProfile(r.Context(), a.client); err != nil {
		if errors.Is(err, errSessionInvalid) {
			a.reset()
		}
		writeAcademicError(w, err)
		return
	}
	b, err := academicRequest(r.Context(), a.client, graduateTablePath, nil)
	if err != nil {
		writeAcademicError(w, err)
		return
	}
	result, err := parseGraduateTimetable(b)
	if err != nil {
		writeAcademicError(w, err)
		return
	}
	b, err = academicRequest(r.Context(), a.client, graduatePublicPath, nil)
	if err != nil {
		writeAcademicError(w, err)
		return
	}
	var public struct {
		Round map[string]any `json:"lcxx"`
	}
	if json.Unmarshal(b, &public) != nil || str(public.Round, "XNXQDM") == "" {
		writeAPIError(w, 502, errors.New("未能确认这份课表所属学期，请在学校系统核对"))
		return
	}
	result.Term = str(public.Round, "XNXQDM")
	result.Round = str(public.Round, "MC")
	writeJSON(w, result)
}
