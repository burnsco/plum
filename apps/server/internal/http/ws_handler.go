package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"plum/internal/transcoder"
	"plum/internal/ws"
)

// webSocketOriginAllowed enforces Origin for browser/cookie clients. Bearer-authenticated
// native clients may omit Origin; if they send Origin, it must still be allowed.
func webSocketOriginAllowed(r *http.Request, allowedOrigins map[string]struct{}) bool {
	if AuthViaBearerFromContext(r.Context()) {
		if strings.TrimSpace(r.Header.Get("Origin")) == "" {
			return true
		}
	}
	return OriginAllowed(r, allowedOrigins)
}

func ServeWebSocket(hub *ws.Hub, sessions *transcoder.PlaybackSessionManager, allowedOrigins map[string]struct{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			logWebSocketHandshakeRejected(r, "unauthorized", 0)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !webSocketOriginAllowed(r, allowedOrigins) {
			logWebSocketHandshakeRejected(r, "origin_not_allowed", user.ID)
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		if err := ws.ServeWS(hub, w, r, ws.ServeOptions{
			CheckOrigin: func(req *http.Request) bool {
				return webSocketOriginAllowed(req, allowedOrigins)
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
			// Upgrade may have failed after writing the HTTP error response; do not log as handler error.
			return
		}
	}
}

func logWebSocketHandshakeRejected(r *http.Request, reason string, userID int) {
	hasSessionAuth := sessionIDFromCookie(r) != "" || bearerSessionToken(r) != ""
	if reason == "unauthorized" && !hasSessionAuth {
		return
	}

	slog.Info("ws handshake rejected",
		"reason", reason,
		"origin", strings.TrimSpace(r.Header.Get("Origin")),
		"remote_ip", clientIP(r),
		"host", strings.TrimSpace(r.Host),
		"user_id", userID,
		"session_auth", hasSessionAuth,
	)
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
		state, err := sessions.Attach(command.SessionID, client.User().ID, client.ID())
		if err != nil {
			slog.Debug("attach playback session failed", "session_id", command.SessionID, "client_id", client.ID(), "user_id", client.User().ID, "error", err)
			return
		}
		if state != nil {
			msg := map[string]any{
				"type":            "playback_session_update",
				"sessionId":       state.SessionID,
				"delivery":        state.Delivery,
				"mediaId":         state.MediaID,
				"revision":        state.Revision,
				"audioIndex":      state.AudioIndex,
				"status":          state.Status,
				"streamUrl":       state.StreamURL,
				"durationSeconds": state.DurationSeconds,
				"error":           state.Error,
			}
			if state.IntroSkipMode != "" {
				msg["intro_skip_mode"] = state.IntroSkipMode
			}
			if state.IntroStartSeconds != nil {
				msg["intro_start_seconds"] = *state.IntroStartSeconds
			}
			if state.IntroEndSeconds != nil {
				msg["intro_end_seconds"] = *state.IntroEndSeconds
			}
			if state.BurnEmbeddedSubtitleStreamIndex != nil {
				msg["burnEmbeddedSubtitleStreamIndex"] = *state.BurnEmbeddedSubtitleStreamIndex
			}
			payload, marshalErr := json.Marshal(msg)
			if marshalErr != nil {
				slog.Debug("attach playback session marshal failed", "session_id", command.SessionID, "client_id", client.ID(), "user_id", client.User().ID, "error", marshalErr)
				return
			}
			if !client.Send(payload) {
				slog.Debug("attach playback session replay dropped", "session_id", command.SessionID, "client_id", client.ID(), "user_id", client.User().ID)
				return
			}
			slog.Debug("attach playback session replay",
				"session_id", command.SessionID,
				"client_id", client.ID(),
				"user_id", client.User().ID,
				"status", state.Status,
				"revision", state.Revision,
			)
		}
	case "detach_playback_session":
		sessions.Detach(command.SessionID, client.User().ID, client.ID())
	}
}
