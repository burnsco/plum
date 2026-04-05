package httputil

import (
	"net/http"
	"time"
)

// ClearStreamWriteDeadline removes the server-level response write deadline so long-lived
// streams (HLS, direct file, subtitles) are not cut off by http.Server.WriteTimeout.
func ClearStreamWriteDeadline(w http.ResponseWriter) {
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})
}
