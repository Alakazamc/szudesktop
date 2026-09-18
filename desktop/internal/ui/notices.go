package ui

// Public school notices only. Personal services must use a separate authenticated adapter.
import (
	"errors"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

type noticeSource struct{ Name, URL string }

var noticeSources = map[string]noticeSource{
	"undergrad": {"教务部", "https://jwb.szu.edu.cn/index/jwtz.htm"},
	"graduate":  {"研究生院", "https://gra.szu.edu.cn/"},
}

type campusNotice struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Date  string `json:"date,omitempty"`
}
type noticeResult struct {
	Source    string         `json:"source"`
	URL       string         `json:"url"`
	Items     []campusNotice `json:"items"`
	FetchedAt time.Time      `json:"fetched_at"`
	Stale     bool           `json:"stale"`
	Message   string         `json:"message,omitempty"`
}
type noticeCache struct {
	sync.Mutex
	results map[string]noticeResult
	retry   map[string]time.Time
}

var publicNotices = noticeCache{results: map[string]noticeResult{}, retry: map[string]time.Time{}}
var noticeAnchor = regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a\s*>`)
var noticeAttr = regexp.MustCompile(`(?is)\b(href|title)\s*=\s*["']([^"']*)["']`)
var noticeTags = regexp.MustCompile(`(?s)<[^>]*>`)
var noticePath = regexp.MustCompile(`/info/\d+/\d+\.htm$`)
var noticeDate = regexp.MustCompile(`\b(20\d{2})[.\-/](\d{2})[.\-/](\d{2})\b`)
var noticeSplitDate = regexp.MustCompile(`\b(\d{2})\s*/?\s+(20\d{2})[.\-](\d{2})\b`)

func parseNotices(body, base string) []campusNotice {
	u, _ := url.Parse(base)
	items := []campusNotice{}
	seen := map[string]bool{}
	for _, a := range noticeAnchor.FindAllStringSubmatch(body, -1) {
		attrs := map[string]string{}
		for _, m := range noticeAttr.FindAllStringSubmatch(a[1], -1) {
			attrs[strings.ToLower(m[1])] = html.UnescapeString(m[2])
		}
		ref, err := url.Parse(attrs["href"])
		if err != nil || ref == nil {
			continue
		}
		link := u.ResolveReference(ref)
		if link.Scheme != "https" || link.Host != u.Host || link.User != nil || !noticePath.MatchString(link.Path) {
			continue
		}
		link.RawQuery = ""
		link.Fragment = ""
		if seen[link.String()] {
			continue
		}
		text := strings.Join(strings.Fields(html.UnescapeString(noticeTags.ReplaceAllString(a[2], " "))), " ")
		title := strings.TrimSpace(attrs["title"])
		date := ""
		if m := noticeDate.FindStringSubmatch(text); len(m) > 0 {
			date = m[1] + "-" + m[2] + "-" + m[3]
			text = strings.Replace(text, m[0], "", 1)
		} else if m := noticeSplitDate.FindStringSubmatch(text); len(m) > 0 {
			date = m[2] + "-" + m[3] + "-" + m[1]
			text = strings.Replace(text, m[0], "", 1)
		}
		if title == "" {
			title = strings.TrimSpace(text)
		}
		// Undated homepage resources and navigation are not news.
		if len([]rune(title)) < 4 || len([]rune(title)) > 180 || date == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", date); err != nil {
			continue
		}
		items = append(items, campusNotice{title, link.String(), date})
		seen[link.String()] = true
		if len(items) >= 30 {
			break
		}
	}
	return items
}

var noticeClient = &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
	if len(via) >= 3 || req.URL.Scheme != "https" || req.URL.Host != via[0].URL.Host {
		return errors.New("unexpected notice redirect")
	}
	return nil
}}

func (s *Server) handleCampusNotices(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("source")
	source, ok := noticeSources[id]
	if !ok {
		writeAPIError(w, 400, errors.New("请选择教务部或研究生院"))
		return
	}
	// Serialize refreshes and bound source polling even after a failure.
	publicNotices.Lock()
	defer publicNotices.Unlock()
	cached, exists := publicNotices.results[id]
	if exists && time.Since(cached.FetchedAt) < 10*time.Minute {
		writeJSON(w, cached)
		return
	}
	if time.Now().Before(publicNotices.retry[id]) {
		if exists {
			cached.Stale = true
			cached.Message = "学校网站暂时无法读取，显示上次读取的内容"
			writeJSON(w, cached)
		} else {
			writeAPIError(w, 503, errors.New("学校网站暂时无法读取，请稍后重试或查看官方原页"))
		}
		return
	}
	publicNotices.retry[id] = time.Now().Add(time.Minute)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, source.URL, nil)
	if err != nil {
		writeAPIError(w, 500, err)
		return
	}
	req.Header.Set("User-Agent", "szuDesktop/0.5 (+https://github.com/Alakazamc/szudesktop)")
	res, err := noticeClient.Do(req)
	var items []campusNotice
	if err == nil {
		defer res.Body.Close()
		if res.StatusCode == 200 {
			data, readErr := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
			if readErr == nil && len(data) <= 2<<20 {
				items = parseNotices(string(data), source.URL)
			}
		}
	}
	if len(items) == 0 {
		if exists {
			cached.Stale = true
			cached.Message = "学校网站暂时无法读取或页面结构变化，显示上次读取的内容"
			writeJSON(w, cached)
		} else {
			writeAPIError(w, 503, errors.New("暂时未能读取学校公告，请稍后重试或查看官方原页"))
		}
		return
	}
	result := noticeResult{Source: source.Name, URL: source.URL, Items: items, FetchedAt: time.Now()}
	publicNotices.results[id] = result
	writeJSON(w, result)
}
