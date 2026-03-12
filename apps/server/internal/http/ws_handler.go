package httpapi

import (
	"encoding/json"
	"log"
	"net/http"

	"plum/internal/transcoder"
	"plum/internal/ws"
)

func ServeWebSocket(hub *ws.Hub, sessions *transcoder.PlaybackSessionManager, allowedOrigins map[string]struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !OriginAllowed(r, allowedOrigins) {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		if err := ws.ServeWS(hub, w, r, ws.ServeOptions{
			CheckOrigin: func(req *http.Request) bool {
				return OriginAllowed(req, allowedOrigins)
			},
			User: user,
			OnClose: func(client *ws.Client) {
				if sessions == nil || client.User() == nil {
					return
				}
				sessions.HandleDisconnect(client.User().ID, client.ID())
			},
			OnText: func(client *ws.Client, payload []byte) {
				if sessions == nil || client.User() == nil {
					return
				}
				handlePlaybackSessionCommand(sessions, client, payload)
			},
		}); err != nil {
			return
		}
	}
}

func handlePlaybackSessionCommand(sessions *transcoder.PlaybackSessionManager, client *ws.Client, payload []byte) {
	var command struct {
		Action    string `json:"action"`
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(payload, &command); err != nil {
		return
	}

	switch command.Action {
	case "attach_playback_session":
		if err := sessions.Attach(command.SessionID, client.User().ID, client.ID()); err != nil {
			log.Printf("attach playback session session=%s client=%s user=%d error=%v", command.SessionID, client.ID(), client.User().ID, err)
		}
	case "detach_playback_session":
		sessions.Detach(command.SessionID, client.User().ID, client.ID())
	}
}
