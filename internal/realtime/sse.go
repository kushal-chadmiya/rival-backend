package realtime

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// ServeSSE streams task events to a connected client.
func ServeSSE(w http.ResponseWriter, r *http.Request, hub *Hub, userID string, isAdmin bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	events := hub.Subscribe(userID, isAdmin)
	defer hub.Unsubscribe(events)

	_, _ = fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"ok\"}\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprintf(w, "event: heartbeat\ndata: {}\n\n")
			flusher.Flush()
		case event, ok := <-events:
			if !ok {
				return
			}
			payload, err := event.Marshal()
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(w, "event: task\ndata: %s\n\n", payload)
			flusher.Flush()
		}
	}
}

// TokenFromQuery extracts an access token from the SSE query string.
func TokenFromQuery(r *http.Request) string {
	return r.URL.Query().Get("access_token")
}

// ContextWithCancel returns a context that cancels when the client disconnects.
func ContextWithCancel(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithCancel(r.Context())
}
