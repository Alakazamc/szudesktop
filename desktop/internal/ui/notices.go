package ui

// Public school notices only. Personal services must use a separate authenticated adapter.
import (
	"errors"
	"fmt"
	"io"
	"strconv"

	xhtml "golang.org/x/net/html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

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
var noticePath = regexp.MustCompile(`/info/\d+/\d+\.htm$`)
var noticeDates = []struct {
	pattern *regexp.Regexp
	order   [3]int
}{
	{regexp.MustCompile(`\b(20\d{2})[.\-/](\d{1,2})[.\-/](\d{1,2})\b`), [3]int{1, 2, 3}},
	{regexp.MustCompile(`\b(20\d{2})\s+(\d{1,2})[/.-](\d{1,2})\b`), [3]int{1, 2, 3}},
	{regexp.MustCompile(`\b(\d{1,2})\s*/?\s+(20\d{2})[.\-](\d{1,2})\b`), [3]int{2, 3, 1}},
	{regexp.MustCompile(`\b(\d{1,2})[.\-/](\d{1,2})\s+(20\d{2})\b`), [3]int{3, 1, 2}},
	{regexp.MustCompile(`\b(\d{1,2})\s+(\d{1,2})月\s+(20\d{2})\b`), [3]int{3, 2, 1}},
	{regexp.MustCompile(`(20\d{2})年\s*(\d{1,2})月\s*(\d{1,2})日`), [3]int{1, 2, 3}},
}

func noticeDateFrom(text string) (string, string) {
	for _, p := range noticeDates {
		m := p.pattern.FindStringSubmatch(text)
		if len(m) == 0 {
			continue
		}
		y, _ := strconv.Atoi(m[p.order[0]])
		month, _ := strconv.Atoi(m[p.order[1]])
		day, _ := strconv.Atoi(m[p.order[2]])
		date := fmt.Sprintf("%04d-%02d-%02d", y, month, day)
		if _, err := time.Parse("2006-01-02", date); err == nil {
			return date, m[0]
		}
	}
	return "", ""
}
func noticeNodeText(n *xhtml.Node, skipLinks bool) string {
	if n.Type == xhtml.ElementNode && (n.Data == "script" || n.Data == "style" || n.Data == "noscript" || (skipLinks && n.Data == "a")) {
		return ""
	}
	if n.Type == xhtml.TextNode {
		return n.Data + " "
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(noticeNodeText(c, skipLinks))
	}
	return strings.Join(strings.Fields(b.String()), " ") + " "
}
func noticeNodeAttr(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func noticeArticleURL(n *xhtml.Node, base *url.URL) string {
	if n.Type != xhtml.ElementNode || n.Data != "a" {
		return ""
	}
	ref, err := url.Parse(noticeNodeAttr(n, "href"))
	if err != nil {
		return ""
	}
	link := base.ResolveReference(ref)
	if (link.Scheme != "https" && link.Scheme != "http") || link.Host != base.Host || link.User != nil || !noticePath.MatchString(link.Path) {
		return ""
	}
	link.Scheme = "https"
	link.RawQuery = ""
	link.Fragment = ""
	return link.String()
}
func noticeArticleCount(n *xhtml.Node, base *url.URL) int {
	paths := map[string]bool{}
	var visit func(*xhtml.Node)
	visit = func(v *xhtml.Node) {
		if link := noticeArticleURL(v, base); link != "" {
			paths[link] = true
		}
		for c := v.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(n)
	return len(paths)
}
func noticeMarkedDate(n *xhtml.Node) string {
	// Prefer publication-date elements over dates quoted in an article excerpt.
	if n.Type == xhtml.ElementNode {
		class := strings.ToLower(noticeNodeAttr(n, "class"))
		marked := n.Data == "time" || strings.Contains(class, "time") || strings.Contains(class, "date") || strings.Contains(class, "news_meta") || strings.Contains(class, "fright") || class == "sj"
		excluded := strings.Contains(class, "desc") || strings.Contains(class, "title") || strings.Contains(class, "excerpt") || strings.Contains(class, "links")
		if marked && !excluded {
			if date, _ := noticeDateFrom(noticeNodeText(n, false)); date != "" {
				return date
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if date := noticeMarkedDate(c); date != "" {
			return date
		}
	}
	return ""
}
func parseNotices(body, base string) []campusNotice {
	root, err := xhtml.Parse(strings.NewReader(body))
	if err != nil {
		return nil
	}
	u, err := url.Parse(base)
	if err != nil {
		return nil
	}
	items := []campusNotice{}
	seen := map[string]bool{}
	var visit func(*xhtml.Node)
	visit = func(n *xhtml.Node) {
		if len(items) >= 30 {
			return
		}
		if link := noticeArticleURL(n, u); link != "" && !seen[link] {
			text := strings.TrimSpace(noticeNodeText(n, false))
			date := noticeMarkedDate(n)
			plainDate, match := noticeDateFrom(text)
			if date == "" {
				date = plainDate
			}
			if date == "" {
				// HTML parsing also handles the optional closing </li> used by some colleges.
				for row, depth := n.Parent, 0; row != nil && depth < 6; row, depth = row.Parent, depth+1 {
					if row.Data == "body" || row.Data == "html" || row.Data == "ul" || row.Data == "ol" || row.Data == "nav" || row.Data == "main" {
						break
					}
					if noticeArticleCount(row, u) != 1 {
						break
					}
					date = noticeMarkedDate(row)
					if date == "" {
						date, _ = noticeDateFrom(noticeNodeText(row, true))
					}
					if date != "" {
						break
					}
				}
			}
			title := strings.TrimSpace(noticeNodeAttr(n, "title"))
			if title == "" {
				title = strings.TrimSpace(strings.Replace(text, match, "", 1))
			}
			if date != "" && len([]rune(title)) >= 4 && len([]rune(title)) <= 180 {
				items = append(items, campusNotice{title, link, date})
				seen[link] = true
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(root)
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
		writeAPIError(w, 400, errors.New("请选择已收录的学院或学校部门"))
		return
	}
	if !source.Readable {
		writeAPIError(w, 503, errors.New(source.Note))
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
	req.Header.Set("User-Agent", "szuDesktop/0.5 (+https://github.com/SzuDesktopTeam/szudesktop)")
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
