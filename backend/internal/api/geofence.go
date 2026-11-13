package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"geofencing/backend/internal/geo"
	"geofencing/backend/internal/models"
	"geofencing/backend/internal/store"
)

// --- request / response payloads ---

type createGeofenceRequest struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Coordinates [][]float64 `json:"coordinates"`
	Category    string      `json:"category"`
}

type geofenceResponse struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Coordinates [][]float64 `json:"coordinates"`
	Category    string      `json:"category"`
	CreatedAt   time.Time   `json:"created_at"`
}

type createGeofenceResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// createGeofence handles POST /geofences.
func (s *Server) createGeofence(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req createGeofenceRequest
	if err := decodeJSON(r, &req); err != nil {
		s.fail(w, start, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.Category = strings.TrimSpace(req.Category)

	if req.Name == "" {
		s.fail(w, start, http.StatusBadRequest, "name is required")
		return
	}
	if req.Category == "" {
		s.fail(w, start, http.StatusBadRequest, "category is required")
		return
	}
	if !models.ValidCategories(req.Category) {
		s.fail(w, start, http.StatusBadRequest,
			"category must be one of: delivery_zone, restricted_zone, toll_zone, customer_area")
		return
	}
	if len(req.Coordinates) == 0 {
		s.fail(w, start, http.StatusBadRequest, "coordinates is required")
		return
	}
	if err := geo.ValidatePolygon(req.Coordinates); err != nil {
		s.fail(w, start, http.StatusBadRequest, err.Error())
		return
	}

	g := &models.Geofence{
		ID:          store.NewID("geo_"),
		Name:        req.Name,
		Description: req.Description,
		Coordinates: req.Coordinates,
		Category:    req.Category,
		Status:      models.StatusActive,
		CreatedAt:   time.Now().UTC().Truncate(time.Second),
	}

	if err := s.store.CreateGeofence(r.Context(), g); err != nil {
		s.logger.Printf("api: create geofence: %v", err)
		s.fail(w, start, http.StatusInternalServerError, "failed to create geofence")
		return
	}

	s.writeJSON(w, start, http.StatusOK, createGeofenceResponse{
		ID:     g.ID,
		Name:   g.Name,
		Status: g.Status,
	})
}

// listGeofences handles GET /geofences?category=.
func (s *Server) listGeofences(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	category := strings.TrimSpace(r.URL.Query().Get("category"))
	if category != "" && !models.ValidCategories(category) {
		s.fail(w, start, http.StatusBadRequest,
			"category must be one of: delivery_zone, restricted_zone, toll_zone, customer_area")
		return
	}

	geofences, err := s.store.ListGeofences(r.Context(), category)
	if err != nil {
		s.logger.Printf("api: list geofences: %v", err)
		s.fail(w, start, http.StatusInternalServerError, "failed to list geofences")
		return
	}

	out := make([]geofenceResponse, 0, len(geofences))
	for _, g := range geofences {
		out = append(out, geofenceResponse{
			ID:          g.ID,
			Name:        g.Name,
			Description: g.Description,
			Coordinates: g.Coordinates,
			Category:    g.Category,
			CreatedAt:   g.CreatedAt,
		})
	}

	page, total, paged, err := paginate(r.URL.Query(), out)
	if err != nil {
		s.fail(w, start, http.StatusBadRequest, err.Error())
		return
	}
	resp := map[string]any{"geofences": page}
	if paged {
		resp["total_count"] = total
	}
	s.writeJSON(w, start, http.StatusOK, resp)
}

// handleHealth handles GET /health (used by Docker healthchecks).
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if err := s.store.Ping(r.Context()); err != nil {
		s.fail(w, start, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	s.writeJSON(w, start, http.StatusOK, map[string]string{"status": "ok"})
}

// isNotFound reports whether err is the store's not-found sentinel.
func isNotFound(err error) bool { return errors.Is(err, store.ErrNotFound) }
