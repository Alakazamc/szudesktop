package ui

import (
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestEmbeddedModulesIgnoreSystemMIMEOverride(t *testing.T) {
	// Windows applications can register .mjs as text/plain in HKCR. Browsers
	// reject that MIME type for ES modules even when the file content is valid.
	original := mime.TypeByExtension(".mjs")
	if err := mime.AddExtensionType(".mjs", "text/plain"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mime.AddExtensionType(".mjs", original); err != nil {
			t.Error(err)
		}
	})
	const script = "export const ready = true;"
	static := fstest.MapFS{"garden/app.mjs": {Data: []byte(script)}}
	mux := http.NewServeMux()
	(&Server{}).routes(mux, static)
	for _, path := range []string{"/assets/garden/app.mjs", "/garden/app.mjs"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusOK || response.Body.String() != script {
				t.Fatalf("static module: %d %q", response.Code, response.Body.String())
			}
			kind, _, err := mime.ParseMediaType(response.Header().Get("Content-Type"))
			if err != nil || (kind != "text/javascript" && kind != "application/javascript") {
				t.Fatalf("module MIME must not depend on the Windows registry: %q (%v)", response.Header().Get("Content-Type"), err)
			}
		})
	}
}
