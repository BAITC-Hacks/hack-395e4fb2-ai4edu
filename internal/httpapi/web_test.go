package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func webFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range map[string]string{
		"index.html":    "<!doctype html><html><body>Test SPA</body></html>",
		"assets/app.js": "console.log('asset served');",
		"api/unknown":   "This static file must not override the API",
	} {
		filename := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestWebFilesAndFallback(t *testing.T) {
	handler := NewHandlerWithOptions(Options{WebDir: webFixture(t)})
	for _, path := range []string{"/", "/index.html", "/some/route", "/history?from=home", "/assets/"} {
		got := call(handler, "GET", path, "")
		if got.Code != 200 || got.Body.String() != "<!doctype html><html><body>Test SPA</body></html>" || !strings.HasPrefix(got.Header().Get("Content-Type"), "text/html") {
			t.Errorf("GET %s: %d %v %s", path, got.Code, got.Header(), got.Body.String())
		}
	}
	asset := call(handler, "GET", "/assets/app.js", "")
	if asset.Code != 200 || asset.Body.String() != "console.log('asset served');" || !strings.Contains(asset.Header().Get("Content-Type"), "javascript") {
		t.Fatalf("asset not served: %d %v %s", asset.Code, asset.Header(), asset.Body.String())
	}
	head := call(handler, "HEAD", "/history", "")
	if head.Code != 200 || head.Body.Len() != 0 || !strings.HasPrefix(head.Header().Get("Content-Type"), "text/html") {
		t.Fatal("HEAD must return HTML headers without a body")
	}
}

func TestWebPreservesAPI(t *testing.T) {
	api := NewHandler()
	web := NewHandlerWithOptions(Options{WebDir: webFixture(t)})
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/api/unknown", ""}, {"POST", "/api/unknown", ""},
		{"GET", "/api/../history", ""}, {"GET", "//api/unknown", ""},
		{"GET", "/api", ""}, {"GET", "/api/scenario", ""},
		{"HEAD", "/api/scenario", ""}, {"POST", "/api/scenario", ""},
		{"POST", "/api/simulate", goldenJSON}, {"GET", "/api/simulate", ""},
		{"POST", "/api/simulate", `{"decisions":[]}`}, {"POST", "/api/simulate", `{"decisions":`},
		{"POST", "/api/explain", goldenJSON},
		{"POST", "/api/recommend", `{"mode":"improve","decisions":[]}`},
		{"OPTIONS", "/api/unknown", ""}, {"OPTIONS", "/history", ""},
		{"POST", "/history", ""},
	} {
		want := call(api, tc.method, tc.path, tc.body)
		got := call(web, tc.method, tc.path, tc.body)
		if got.Code != want.Code || got.Body.String() != want.Body.String() || got.Header().Get("Content-Type") != want.Header().Get("Content-Type") || got.Header().Get("Allow") != want.Header().Get("Allow") {
			t.Errorf("static handler changed %s %s: got %d %s, want %d %s", tc.method, tc.path, got.Code, got.Body.String(), want.Code, want.Body.String())
		}
	}
	if got := call(web, "GET", "/api/unknown", ""); got.Code != 404 || got.Body.String() != "404 page not found\n" {
		t.Fatal("unknown API path must never serve index.html or a static API file")
	}
}

func TestWebDisabledOrMissing(t *testing.T) {
	for _, root := range []string{"", filepath.Join(t.TempDir(), "missing"), t.TempDir()} {
		handler := NewHandlerWithOptions(Options{WebDir: root})
		for _, path := range []string{"/", "/some/route", "/assets/app.js", "/api/unknown"} {
			if got := call(handler, "GET", path, ""); got.Code != 404 {
				t.Errorf("WEB_DIR=%q GET %s: %d, want 404", root, path, got.Code)
			}
		}
		if got := call(handler, "GET", "/api/scenario", ""); got.Code != 200 {
			t.Errorf("WEB_DIR=%q prevented API access: %d", root, got.Code)
		}
	}
}

func TestWebCannotEscapeDirectory(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "web")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("SPA"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "private.txt"), []byte("outside WEB_DIR"), 0644); err != nil {
		t.Fatal(err)
	}
	handler := NewHandlerWithOptions(Options{WebDir: root})
	for _, path := range []string{"/../private.txt", "/%2e%2e/private.txt", "/assets/../../private.txt"} {
		if got := call(handler, "GET", path, ""); got.Code != 200 || got.Body.String() != "SPA" {
			t.Errorf("traversal %s escaped fallback: %d %s", path, got.Code, got.Body.String())
		}
	}
	for name, target := range map[string]string{"linked.txt": filepath.Join(parent, "private.txt"), "linked-dir": parent} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"/linked.txt", "/linked-dir/private.txt"} {
		if got := call(handler, "GET", path, ""); got.Code != 403 || strings.Contains(got.Body.String(), "outside WEB_DIR") {
			t.Errorf("symlink %s was not refused: %d %s", path, got.Code, got.Body.String())
		}
	}
}

func TestWebConditionalGET(t *testing.T) {
	handler := NewHandlerWithOptions(Options{WebDir: webFixture(t)})
	initial := call(handler, "GET", "/assets/app.js", "")
	request := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	request.Header.Set("If-Modified-Since", initial.Header().Get("Last-Modified"))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotModified || response.Body.Len() != 0 {
		t.Fatal("unchanged asset must support conditional GET")
	}
}
