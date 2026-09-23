package httpapi

import "net/http"

func cors(next http.Handler, allowedOrigin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Explicit origin only: an empty setting or wildcard never enables CORS.
		if allowedOrigin != "" && allowedOrigin != "*" {
			w.Header().Add("Vary", "Origin")
			if r.Header.Get("Origin") == allowedOrigin {
				w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
				if r.Method == http.MethodOptions {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				}
			}
		}
		// Preflight must be handled before the method-qualified ServeMux routes.
		if r.Method == http.MethodOptions {
			w.Header().Add("Vary", "Access-Control-Request-Method")
			w.Header().Add("Vary", "Access-Control-Request-Headers")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
