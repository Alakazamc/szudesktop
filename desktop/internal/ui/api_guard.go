package ui

import (
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// protectAPI rejects browser requests from other origins, including other local
// ports. Host validation also prevents DNS rebinding to this loopback service.
// Non-browser local clients may omit Origin; POST still requires JSON.
func protectAPI(next http.HandlerFunc, methods ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		ip := net.ParseIP(strings.Trim(host, "[]"))
		if !strings.EqualFold(host, "localhost") && (ip == nil || !ip.IsLoopback()) {
			http.Error(w, "只接受本机地址", http.StatusForbidden)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			http.Error(w, "不接受其他页面的请求", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Scheme != "http" || !strings.EqualFold(u.Host, r.Host) || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
				http.Error(w, "请求来源不匹配", http.StatusForbidden)
				return
			}
		}
		allowed := false
		for _, method := range methods {
			if r.Method == method {
				allowed = true
				break
			}
		}
		if !allowed {
			w.Header().Set("Allow", strings.Join(methods, ", "))
			http.Error(w, "不支持的方法", http.StatusMethodNotAllowed)
			return
		}
		if r.Method == http.MethodPost {
			mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mediaType != "application/json" {
				http.Error(w, "请求必须使用 JSON", http.StatusUnsupportedMediaType)
				return
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next(w, r)
	}
}
