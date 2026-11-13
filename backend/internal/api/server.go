// Package api exposes the HTTP REST API and the WebSocket alert endpoint.
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"geofencing/backend/internal/store"
	"geofencing/backend/internal/ws"
)

// Server hosts all HTTP handlers.
type Server struct {
	store   store.Store
	hub     *ws.Hub
	limiter *rateLimiter
	logger  *log.Logger
}

// NewServer wires the API against the given store and alert hub.
func NewServer(st store.Store, hub *ws.Hub, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}
	return &Server{
		store:   st,
		hub:     hub,
		limiter: newRateLimiter(rateLimitPerMinute()),
		logger:  logger,
	}
}

// Handler returns the fully composed HTTP handler (CORS, logging,
// recovery and all routes).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ws/alerts", s.handleWebSocket)

	mux.HandleFunc("POST /geofences", s.createGeofence)
	mux.HandleFunc("GET /geofences", s.listGeofences)

	mux.HandleFunc("POST /vehicles", s.createVehicle)
	mux.HandleFunc("GET /vehicles", s.listVehicles)
	mux.HandleFunc("POST /vehicles/location", s.limit(s.updateVehicleLocation))
	mux.HandleFunc("GET /vehicles/location/{vehicle_id}", s.getVehicleLocation)

	mux.HandleFunc("POST /alerts/configure", s.configureAlert)
	mux.HandleFunc("GET /alerts", s.listAlerts)

	mux.HandleFunc("GET /violations/history", s.listViolations)

	return s.recover(s.logRequest(s.cors(mux)))
}

// --- response helpers ---

// writeJSON renders payload as JSON and injects the required `time_ns`
// field (elapsed nanoseconds measured from start) into the top-level object.
func (s *Server) writeJSON(w http.ResponseWriter, start time.Time, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		s.logger.Printf("api: marshal response: %v", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	// Re-encode through a map so time_ns is guaranteed to be present even if
	// the handler struct forgot to declare it.
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err == nil {
		obj["time_ns"] = strconv.FormatInt(time.Since(start).Nanoseconds(), 10)
		if body, err = json.Marshal(obj); err != nil {
			s.logger.Printf("api: marshal response: %v", err)
			http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		s.logger.Printf("api: write response: %v", err)
	}
}

// fail writes a JSON error body including time_ns.
func (s *Server) fail(w http.ResponseWriter, start time.Time, status int, msg string) {
	s.writeJSON(w, start, status, map[string]string{"error": msg})
}

// decodeJSON reads and strictly decodes a JSON request body.
func decodeJSON(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
