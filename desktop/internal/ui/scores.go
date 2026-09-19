package ui

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// 成绩读取。两套系统各走各的接口：
//
//   - 本科：jwapp/sys/cjcx（教务）
//   - 研究生：gsapp/sys/xscjglapp（研究生院）
//
// 两者共用同一份 ehall 会话，所以只需要用户交一次登录状态。
//
// ⚠️ 这里只做只读。在线写操作（提交预约、评教之类）不在本模块范围内。

const (
	undergradScorePath = "/jwapp/sys/cjcx/modules/cjcx/xscjcx.do"
	gradScorePath      = "/gsapp/sys/xscjglapp/modules/xscjcx/xscjcx_dqx.do"
)

// 一次最多拉多少条。个人成绩单不会超过这个量级；
// 真超了也只是少显示，不会静默截断后当成全部——下面会显式告诉用户。
const scorePageSize = 200

// courseScore 是一门课的读取结果。
//
// 字段名保持和学校一致（用注释写清楚中文），
// 免得以后对照原始返回时来回猜。
type courseScore struct {
	Term     string  `json:"term,omitempty"`     // 学期，如 2025-2026-1
	Name     string  `json:"name"`               // 课程名称
	Credit   float64 `json:"credit,omitempty"`   // 学分
	Score    string  `json:"score,omitempty"`    // 成绩原文（可能是等级或百分制）
	GPA      float64 `json:"gpa,omitempty"`      // 官方绩点（有才填）
	Category string  `json:"category,omitempty"` // 课程性质/类别
	Teacher  string  `json:"teacher,omitempty"`  // 任课教师（有才填）
	Identity string  `json:"identity"`           // 教学班标识，用来去重
}

// scoreResult 是一次读取的完整结果。
type scoreResult struct {
	Level   string        `json:"level"`              // undergrad / graduate
	Label   string        `json:"label"`              // 给人看的层次名
	Items   []courseScore `json:"items"`              // 课程列表
	Total   int           `json:"total"`              // 系统里一共有多少门
	Fetched int           `json:"fetched"`            // 本次取回多少门
	Full    bool          `json:"full"`               // fetched == total 才为 true
	Note    string        `json:"note,omitempty"`     // 需要额外说明的话
	APIPath string        `json:"api_path,omitempty"` // 实际请求的接口，排查用
}

// scoreReader 抽象「怎么取成绩」，测试里换成假实现。
type scoreReader interface {
	Read(c *ehallClient) (*scoreResult, error)
}

// readUndergradScore 读本科成绩。
func readUndergradScore(c *ehallClient) (*scoreResult, error) {
	// 空 querySetting 表示不加过滤条件，取回全部。
	form := url.Values{
		"querySetting": {"[]"},
		"pageSize":     {strconv.Itoa(scorePageSize)},
		"pageNumber":   {"1"},
	}
	body, err := c.postForm(undergradScorePath, form)
	if err != nil {
		return nil, err
	}
	rows, err := ehallRows(body, "xscjcx")
	if err != nil {
		return nil, err
	}

	out := &scoreResult{Level: "undergrad", Label: "本科", APIPath: undergradScorePath}
	seen := map[string]bool{}
	for _, row := range rows {
		item := courseScore{
			Term:     str(row, "XNXQDM", "XNXQ", "JXJHH"),
			Name:     str(row, "KCM", "KCMC"),
			Credit:   num(row, "XF", "XFJD"),
			Score:    str(row, "ZCJ", "CJ", "DYBFZCJ"),
			GPA:      num(row, "JD", "JDZS", "XFJD"),
			Category: str(row, "KCXZDM_DISPLAY", "KCXZ", "KCLBDM_DISPLAY"),
			Teacher:  str(row, "JSXM", "JSRXM", "SKJS"),
		}
		item.Identity = str(row, "JXBID", "KCH", "KCDM")
		if item.Identity == "" {
			item.Identity = item.Name + "|" + item.Term
		}
		if item.Name == "" || seen[item.Identity] {
			continue
		}
		seen[item.Identity] = true
		out.Items = append(out.Items, item)
	}
	return finishScoreResult(out, len(rows), "本科成绩"),
		checkSuspiciouslyEmpty(out, rows, "本科")
}

// readGradScore 读研究生成绩。
func readGradScore(c *ehallClient) (*scoreResult, error) {
	form := url.Values{
		"querySetting": {"[]"},
		"pageSize":     {strconv.Itoa(scorePageSize)},
		"pageNumber":   {"1"},
	}
	body, err := c.postForm(gradScorePath, form)
	if err != nil {
		return nil, err
	}
	rows, err := ehallRows(body, "xscjcx_dqx")
	if err != nil {
		return nil, err
	}

	out := &scoreResult{Level: "graduate", Label: "研究生", APIPath: gradScorePath}
	seen := map[string]bool{}
	for _, row := range rows {
		item := courseScore{
			Term:     str(row, "XNXQDM", "XNXQ"),
			Name:     str(row, "KCMC", "KCM"),
			Credit:   num(row, "XF"),
			Score:    str(row, "DYBFZCJ", "ZCJ", "CJ"),
			GPA:      num(row, "JD", "JDZS"),
			Category: str(row, "KCLXDM_DISPLAY", "CJFZDM_DISPLAY", "KCXZDM_DISPLAY"),
		}
		item.Identity = str(row, "JXBID", "KCH", "KCDM")
		if item.Identity == "" {
			item.Identity = item.Name + "|" + item.Term
		}
		// 研究生成绩里也可能混入非百分制记录，成绩原文照实给，不替用户换算。
		if item.Name == "" || seen[item.Identity] {
			continue
		}
		seen[item.Identity] = true
		out.Items = append(out.Items, item)
	}
	return finishScoreResult(out, len(rows), "研究生成绩"),
		checkSuspiciouslyEmpty(out, rows, "研究生")
}

// finishScoreResult 补齐统计字段并排序。
//
// ⚠️ Full 的判断只看「服务端还有没有下一页」，不看去重前后差多少。
// 早先这里拿「去重后的门数」和「原始行数」比，结果只要学校返回里
// 有重复行（同一教学班出现两次很常见），就会被算成「没取全」，
// 给用户看一句莫名其妙的「系统共 N 门，本次取回 M 门」。
func finishScoreResult(out *scoreResult, rawRows int, what string) *scoreResult {
	out.Fetched = len(out.Items)
	out.Total = rawRows
	// 一次就取满了 pageSize 说明可能还有下一页，那才是真的「没取全」。
	out.Full = rawRows < scorePageSize
	sort.SliceStable(out.Items, func(i, j int) bool {
		if out.Items[i].Term != out.Items[j].Term {
			return out.Items[i].Term > out.Items[j].Term
		}
		return out.Items[i].Name < out.Items[j].Name
	})
	if !out.Full {
		out.Note = fmt.Sprintf("%s本次取回 %d 门，可能还有更多。可以按学期分批读取。", what, out.Fetched)
	}
	return out
}

// checkSuspiciouslyEmpty 是这套代码里最重要的一道防线。
//
// 接口返回 200 且结构正常，但一门课都没有，有两种可能：
//  1. 学生确实还没有成绩；
//  2. 接口改版了/字段改名了/权限不够，我们其实没读到。
//
// 这两种情况对用户的意义完全相反，绝不能混成一句「暂无成绩」。
// 拿不准的时候必须报错让人去看官方页面，而不是让用户以为「我查过了，没成绩」。
func checkSuspiciouslyEmpty(out *scoreResult, rows []map[string]any, what string) error {
	if len(rows) == 0 {
		return nil // 服务端明确给了空列表，可以如实说「暂无」
	}
	if len(out.Items) > 0 {
		return nil
	}
	return fmt.Errorf("学校系统返回了 %d 条%s记录，但没能识别出课程名称字段，可能接口已改版。请先到官方系统核对，不要以本结果为准", len(rows), what)
}

/* ---------- 取值小工具 ---------- */

// str 按顺序找第一个非空字段，转成字符串。
//
// 一所学校的不同系统字段名经常不一致（KCM / KCMC），
// 硬写一个名字容易在另一个系统上取空。
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

// num 按顺序找第一个能转成数字的字段。
func num(row map[string]any, keys ...string) float64 {
	for _, k := range keys {
		v, ok := row[k]
		if !ok || v == nil {
			continue
		}
		switch t := v.(type) {
		case float64:
			return t
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
				return f
			}
		}
	}
	return 0
}
