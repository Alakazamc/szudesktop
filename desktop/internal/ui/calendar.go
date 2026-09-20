package ui

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

const schoolCalendarURL = "https://www.szu.edu.cn/xxgk/xl.htm"

type academicTerm struct {
	Name         string `json:"name"`
	Start        string `json:"start"`
	End          string `json:"end"`
	WeekStart    string `json:"week_start"`
	ClassesStart string `json:"classes_start"`
	Image        string `json:"image"`
}

type calendarResult struct {
	Terms       []academicTerm `json:"terms"`
	Images      []string       `json:"images"`
	ImageHashes []string       `json:"image_hashes,omitempty"`
	CheckedAt   time.Time      `json:"checked_at"`
	Source      string         `json:"source"`
	Stale       bool           `json:"stale"`
	Message     string         `json:"message,omitempty"`
}

// Dates and Sunday week boundaries visually checked against both official grids.
// This snapshot makes first launch useful offline; a failed refresh stays explicit.
func bundledCalendar() calendarResult {
	images := []string{
		"https://www.szu.edu.cn/__local/1/F8/00/9B90CC3ED3B321E2F55DFE6A653_03D5B7D4_4DB31.png",
		"https://www.szu.edu.cn/__local/C/9B/2C/71247060C10DF03836096D18ECA_8B9B2DDF_398DC.png",
		"https://www.szu.edu.cn/__local/0/DF/6D/FB22CB68D90B9A4FD8817F3DA71_94B43F98_2C7A0.png",
		"https://www.szu.edu.cn/__local/2/B1/AC/28B9110830A713D1459CD320A9C_A03128E3_209D3.png",
	}
	return calendarResult{Source: schoolCalendarURL, Images: images, Stale: true,
		Message: "使用随应用核实的校历（2026-09-20），等待联网更新",
		Terms: []academicTerm{
			{"2026–2027 学年第一学期", "2026-08-28", "2027-01-22", "2026-08-30", "2026-08-31", images[0]},
			{"2025–2026 学年第二学期", "2026-03-04", "2026-07-17", "2026-03-08", "2026-03-09", images[2]},
		},
	}
}

var calendarImages = regexp.MustCompile(`(?is)<img\b[^>]*class=["'][^"']*img_vsb_content[^"']*["'][^>]*>`)
var calendarOriginal = regexp.MustCompile(`(?i)\borisrc=["']([^"']+)["']`)
var calendarTitle = regexp.MustCompile(`(20\d{2})[一—–-](20\d{2})学年第([一二])学期`)
var calendarRange = regexp.MustCompile(`学期为(20\d{2})年(\d{1,2})月(\d{1,2})日至(?:(20\d{2})年)?(\d{1,2})月(\d{1,2})日`)
var calendarClass = regexp.MustCompile(`开始上课[：:·]*(\d{1,2})月(\d{1,2})日`)
var calendarWeeks = regexp.MustCompile(`第一至十[七八九]周[（(](\d{1,2})月(\d{1,2})日`)

func parseCalendarImages(body string) []string {
	var images []string
	for _, tag := range calendarImages.FindAllString(body, 8) {
		m := calendarOriginal.FindStringSubmatch(tag)
		if len(m) != 2 {
			continue
		}
		u, err := url.Parse(m[1])
		if err != nil {
			continue
		}
		base, _ := url.Parse(schoolCalendarURL)
		u = base.ResolveReference(u)
		if u.Scheme != "https" || u.Host != base.Host || u.User != nil || !strings.HasPrefix(u.Path, "/__local/") {
			continue
		}
		if !slices.Contains(images, u.String()) {
			images = append(images, u.String())
		}
	}
	return images
}

func parseAcademicTerm(text, image string) (academicTerm, error) {
	text = strings.Join(strings.Fields(text), "")
	fail := errors.New("校历图片格式变化，未自动采用未经确认的日期")
	title, span := calendarTitle.FindStringSubmatch(text), calendarRange.FindStringSubmatch(text)
	if !strings.Contains(text, "校历说明") || len(title) == 0 || len(span) == 0 {
		return academicTerm{}, fail
	}
	year, _ := strconv.Atoi(title[1])
	next, _ := strconv.Atoi(title[2])
	if next != year+1 {
		return academicTerm{}, fail
	}
	if span[4] == "" {
		span[4] = span[1]
	}
	date := func(y, m, d string) (time.Time, error) {
		yy, _ := strconv.Atoi(y)
		mm, _ := strconv.Atoi(m)
		dd, _ := strconv.Atoi(d)
		return time.Parse("2006-01-02", fmt.Sprintf("%04d-%02d-%02d", yy, mm, dd))
	}
	start, e1 := date(span[1], span[2], span[3])
	end, e2 := date(span[4], span[5], span[6])
	if e1 != nil || e2 != nil || end.Sub(start) < 90*24*time.Hour || end.Sub(start) > 180*24*time.Hour || start.Year() < year || end.Year() > next {
		return academicTerm{}, fail
	}
	// Autumn notes distinguish freshmen from continuing students. Never use the
	// freshmen start date as the campus-wide first teaching week.
	section := text
	if i := strings.Index(text, "老生"); i >= 0 {
		section = text[i:]
	}
	classes := calendarClass.FindStringSubmatch(section)
	if len(classes) == 0 {
		classes = calendarWeeks.FindStringSubmatch(section)
	}
	if len(classes) == 0 {
		return academicTerm{}, fail
	}
	first, err := date(span[1], classes[1], classes[2])
	if err != nil || first.Weekday() != time.Monday || first.Before(start) || first.Sub(start) > 10*24*time.Hour {
		return academicTerm{}, fail
	}
	return academicTerm{title[1] + "–" + title[2] + " 学年第" + title[3] + "学期", start.Format("2006-01-02"), end.Format("2006-01-02"), first.AddDate(0, 0, -1).Format("2006-01-02"), first.Format("2006-01-02"), image}, nil
}

type calendarService struct {
	mu      sync.Mutex
	result  calendarResult
	path    string
	nextTry time.Time
}

func newCalendarService(dir string) *calendarService {
	s := &calendarService{result: bundledCalendar(), path: filepath.Join(dir, "calendar-v1.json")}
	if b, err := os.ReadFile(s.path); err == nil {
		var cached calendarResult
		if json.Unmarshal(b, &cached) == nil && cached.Source == schoolCalendarURL && len(cached.Terms) > 0 && len(cached.Images) > 0 {
			s.result = cached
		}
	}
	return s
}

func fetchCalendar(ctx context.Context, address string) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	res, err := noticeClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("calendar HTTP %d", res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, (6<<20)+1))
	if err != nil || len(b) > 6<<20 {
		return nil, errors.New("calendar download failed")
	}
	return b, nil
}

func updateCalendar(ctx context.Context, previous calendarResult) (calendarResult, error) {
	return refreshCalendar(ctx, previous, fetchCalendar, recognizeCalendar)
}

func refreshCalendar(ctx context.Context, previous calendarResult, fetch func(context.Context, string) ([]byte, error), recognize func(context.Context, []byte) (string, error)) (calendarResult, error) {
	b, err := fetch(ctx, schoolCalendarURL)
	if err != nil {
		return previous, errors.New("学校校历暂时无法访问，保留上次校历")
	}
	images := parseCalendarImages(string(b))
	if len(images) == 0 {
		return previous, errors.New("官方校历页面结构变化，保留上次校历")
	}
	// Schools can overwrite an image without changing its URL. Hash the image
	// content on each daily check; only changed content needs local OCR.
	contents := make([][]byte, len(images))
	hashes := make([]string, len(images))
	for i, address := range images {
		contents[i], err = fetch(ctx, address)
		if err != nil {
			return previous, errors.New("校历图片未能完整下载，保留上次校历")
		}
		hashes[i] = fmt.Sprintf("%x", sha256.Sum256(contents[i]))
	}
	if !slices.Equal(images, previous.Images) || !slices.Equal(hashes, previous.ImageHashes) {
		var terms []academicTerm
		// Official page pairs the explanatory sheet with its grid. OCR only adopts
		// explicit dates from the explanatory sheets, never guesses grid digits.
		for i, address := range images {
			text, err := recognize(ctx, contents[i])
			if err != nil {
				return previous, errors.New("发现校历图片更新，但本机未能识别；请查看官方校历或手动设置")
			}
			if !strings.Contains(strings.Join(strings.Fields(text), ""), "校历说明") {
				continue
			}
			term, err := parseAcademicTerm(text, address)
			if err != nil {
				return previous, err
			}
			terms = append(terms, term)
		}
		if len(terms) == 0 || len(terms)*2 != len(images) {
			return previous, errors.New("新版校历未能完整识别，请查看官方校历或手动设置")
		}
		previous.Terms, previous.Images, previous.ImageHashes = terms, images, hashes
	}
	previous.CheckedAt = time.Now().UTC()
	previous.Stale = false
	previous.Message = ""
	return previous, nil
}

func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
	c := s.calendar
	c.mu.Lock()
	defer c.mu.Unlock()
	if r.URL.Query().Get("refresh") != "1" {
		result := c.result
		if time.Since(result.CheckedAt) > 24*time.Hour {
			result.Stale = true
		}
		writeJSON(w, result)
		return
	}
	if time.Now().Before(c.nextTry) {
		writeJSON(w, c.result)
		return
	}
	c.nextTry = time.Now().Add(time.Minute)
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	defer cancel()
	result, err := updateCalendar(ctx, c.result)
	if err != nil {
		c.result.Stale = true
		c.result.Message = err.Error()
	} else {
		c.result = result
		if err = os.MkdirAll(filepath.Dir(c.path), 0700); err == nil {
			b, _ := json.Marshal(result)
			err = os.WriteFile(c.path, b, 0600)
		}
		if err != nil {
			c.result.Message = "已更新，本机缓存未能保存；下次启动会重新查询"
		}
	}
	writeJSON(w, c.result)
}
