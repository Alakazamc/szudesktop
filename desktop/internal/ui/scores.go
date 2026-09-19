package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Each application must be verified separately; same host does not imply permission.
const (
	undergradScorePath = "/jwapp/sys/cjcx/modules/cjcx/xscjcx.do"
	gradScorePath      = "/gsapp/sys/xscjglapp/modules/xscjcx/xscjcx_dqx.do"
	scorePageSize      = 200
)

type courseScore struct {
	Term     string   `json:"term,omitempty"`
	Name     string   `json:"name"`
	Credit   *float64 `json:"credit,omitempty"`
	Score    string   `json:"score,omitempty"`
	GPA      *float64 `json:"gpa,omitempty"`
	Category string   `json:"category,omitempty"`
	Teacher  string   `json:"teacher,omitempty"`
	Identity string   `json:"identity"`
}
type scoreResult struct {
	Level   string        `json:"level"`
	Label   string        `json:"label"`
	Items   []courseScore `json:"items"`
	Total   *int          `json:"total,omitempty"` // only the server's explicit totalSize
	Fetched int           `json:"fetched"`
	Full    bool          `json:"full"`
	Note    string        `json:"note,omitempty"`
}
type scoreApp struct{ Level, Label, Path, Dataset string }

func selectScoreApp(level string) (scoreApp, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "undergrad", "undergraduate", "本科":
		return scoreApp{"undergrad", "本科", undergradScorePath, "xscjcx"}, nil
	case "graduate", "grad", "研究生":
		return scoreApp{"graduate", "研究生", gradScorePath, "xscjcx_dqx"}, nil
	default:
		return scoreApp{}, errors.New("请选择本科或研究生")
	}
}
func readUndergradScore(c *ehallClient) (*scoreResult, error) {
	app, _ := selectScoreApp("undergrad")
	return readScore(c, app)
}
func readGradScore(c *ehallClient) (*scoreResult, error) {
	app, _ := selectScoreApp("graduate")
	return readScore(c, app)
}
func readScore(c *ehallClient, app scoreApp) (*scoreResult, error) {
	body, err := c.postForm(app.Path, allRowsForm(scorePageSize))
	if err != nil {
		return nil, err
	}
	page, err := parseEhallPage(body, app.Dataset)
	if err != nil {
		return nil, err
	}
	out := &scoreResult{Level: app.Level, Label: app.Label, Items: []courseScore{}, Total: page.Total}
	seen := map[string]bool{}
	for _, row := range page.Rows {
		item := courseScore{Term: str(row, "XNXQDM", "XNXQ"), Name: str(row, "KCM", "KCMC"), Category: str(row, "KCXZDM_DISPLAY", "KCLXDM_DISPLAY"), Teacher: str(row, "JSXM")}
		if app.Level == "graduate" {
			item.Name = str(row, "KCMC", "KCM")
			item.Score = str(row, "DYBFZCJ", "ZCJ", "CJ")
		} else {
			item.Score = str(row, "ZCJ", "CJ")
		}
		if item.Name == "" {
			return nil, fmt.Errorf("有%s记录缺少课程名称，可能接口已改版。请到官方系统核对，不要以本结果为准", app.Label)
		}
		item.Credit, err = optionalNumber(row, "XF", 100)
		if err != nil {
			return nil, err
		}
		item.GPA, err = optionalNumber(row, "JD", 5)
		if err != nil {
			return nil, err
		}
		// No XFJD fallback: its meaning has not been verified. Missing != zero.
		item.Identity = str(row, "JXBID")
		if item.Identity == "" {
			item.Identity = str(row, "KCH", "KCDM") + "|" + item.Name + "|" + item.Term
		}
		// Only discard identical records, retaining retakes or differing grades.
		encoded, _ := json.Marshal(item)
		if seen[string(encoded)] {
			continue
		}
		seen[string(encoded)] = true
		out.Items = append(out.Items, item)
	}
	out.Fetched = len(out.Items)
	out.Full = page.Total != nil && *page.Total == len(page.Rows)
	if page.Total == nil {
		out.Note = "仅查询第一页，学校未提供可确认的总数；不能据此认定成绩已取全，请到官方系统核对。"
	} else if !out.Full {
		out.Note = "仅显示第一页，尚未取全；请到官方系统查看完整成绩。"
	}
	sort.SliceStable(out.Items, func(i, j int) bool {
		if out.Items[i].Term != out.Items[j].Term {
			return out.Items[i].Term > out.Items[j].Term
		}
		return out.Items[i].Name < out.Items[j].Name
	})
	return out, nil
}
func optionalNumber(row map[string]any, key string, max float64) (*float64, error) {
	raw, exists := row[key]
	if !exists || raw == nil {
		return nil, nil
	}
	var n float64
	switch v := raw.(type) {
	case float64:
		n = v
	case string:
		v = strings.TrimSpace(v)
		if v == "" || v == "null" {
			return nil, nil
		}
		var err error
		n, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, errors.New("学校成绩数值格式异常，请到官方系统核对")
		}
	default:
		return nil, errors.New("学校成绩数值格式异常，请到官方系统核对")
	}
	if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > max {
		return nil, errors.New("学校成绩数值超出可识别范围，请到官方系统核对")
	}
	return &n, nil
}

func str(row map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := row[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			if s := strings.TrimSpace(t); s != "" && s != "null" {
				return s
			}
		case float64:
			return strconv.FormatFloat(t, 'f', -1, 64)
		}
	}
	return ""
}
