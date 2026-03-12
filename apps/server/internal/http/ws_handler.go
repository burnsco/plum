package httpapi

import (
	"net/http"

	"plum/internal/ws"
)

func ServeWebSocket(hub *ws.Hub, allowedOrigins map[string]struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if UserFromContext(r.Context()) == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !OriginAllowed(r, allowedOrigins) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		if err := ws.ServeWS(hub, w, r, func(req *http.Request) bool {
			return OriginAllowed(req, allowedOrigins)
		}); err != nil {
			return
		}
	}
}
