package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORS(t *testing.T) {
	for _, tc := range []struct{ name, configured, origin, expected string }{
		{"disabled", "", "http://localhost:5173", ""},
		{"wildcard disabled", "*", "http://localhost:5173", ""},
		{"allowed", "http://localhost:5173", "http://localhost:5173", "http://localhost:5173"},
		{"different port", "http://localhost:5173", "http://localhost:5174", ""},
		{"no origin", "http://localhost:5173", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewHandlerWithOptions(Options{CORSOrigin: tc.configured})
			for _, route := range []struct{ method, path, body string }{
				{"OPTIONS", "/api/explain", ""}, {"OPTIONS", "/api/simulate", ""},
				{"GET", "/api/scenario", ""}, {"POST", "/api/explain", goldenJSON},
				{"POST", "/api/simulate", `{"decisions":[]}`},
			} {
				r := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
				r.Header.Set("Origin", tc.origin)
				if route.method == "OPTIONS" {
					r.Header.Set("Access-Control-Request-Method", "POST")
					r.Header.Set("Access-Control-Request-Headers", "content-type")
				}
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, r)
				if got := w.Header().Get("Access-Control-Allow-Origin"); got != tc.expected {
					t.Errorf("origin=%q, want %q", got, tc.expected)
				}
				if route.method == "OPTIONS" {
					if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
						t.Fatalf("preflight not intercepted: %d", w.Code)
					}
					if tc.expected != "" && (w.Header().Get("Access-Control-Allow-Headers") != "Content-Type" || !strings.Contains(w.Header().Get("Access-Control-Allow-Methods"), "POST")) {
						t.Fatal("missing preflight headers")
					}
				}
				if tc.expected != "" && !strings.Contains(strings.Join(w.Header().Values("Vary"), ","), "Origin") {
					t.Fatal("missing Vary: Origin")
				}
				if w.Header().Get("Access-Control-Allow-Credentials") != "" {
					t.Fatal("credentials must not be enabled")
				}
			}
		})
	}
}
