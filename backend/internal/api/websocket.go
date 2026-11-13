package api

import (
	"net/http"

	"geofencing/backend/internal/ws"
)

// handleWebSocket upgrades the connection on GET /ws/alerts and hands it to
// the alert hub. All further communication is push-only (server → client).
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade already wrote an HTTP error response.
		s.logger.Printf("api: websocket upgrade failed: %v", err)
		return
	}
	s.hub.Register(conn)
	s.logger.Printf("api: websocket client connected (%d total)", s.hub.ClientCount())
}
