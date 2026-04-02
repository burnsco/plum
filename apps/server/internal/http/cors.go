package httpapi

import (
	"net/http"
	"strings"
)

const loopbackHTTPAnyPortOrigin = "__plum_loopback_http_any_port__"

var defaultAllowedOrigins = []string{
	"http://localhost:3000",
	"http://127.0.0.1:3000",
	"http://localhost:4173",
	"http://127.0.0.1:4173",
	"http://localhost:5173",
	"http://127.0.0.1:5173",
}

func AllowedOriginsFromEnv(raw string) map[string]struct{} {
	origins := make(map[string]struct{})
	values := defaultAllowedOrigins
	if trimmed := strings.TrimSpace(raw); trimmed != "" {
		values = strings.Split(trimmed, ",")
	} else {
		origins[loopbackHTTPAnyPortOrigin] = struct{}{}
	}
	for _, value := range values {
		origin := strings.TrimSpace(value)
		if origin == "" {
			continue
		}
		origins[origin] = struct{}{}
	}
	return origins
}

func CORSMiddleware(allowedOrigins map[string]struct{}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			allowed := OriginAllowed(r, allowedOrigins)

			headers := w.Header()
			headers.Add("Vary", "Origin")
			headers.Add("Vary", "Access-Control-Request-Method")
			headers.Add("Vary", "Access-Control-Request-Headers")
			headers.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, PATCH, DELETE")
			headers.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if allowed {
				headers.Set("Access-Control-Allow-Origin", origin)
				headers.Set("Access-Control-Allow-Credentials", "true")
			}

			if r.Method == http.MethodOptions {
				if origin != "" && !allowed {
					http.Error(w, "origin not allowed", http.StatusForbidden)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
