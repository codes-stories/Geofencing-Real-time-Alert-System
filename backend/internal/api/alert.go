package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"geofencing/backend/internal/models"
	"geofencing/backend/internal/store"
)

type configureAlertRequest struct {
	GeofenceID string `json:"geofence_id"`
	VehicleID  string `json:"vehicle_id"`
	EventType  string `json:"event_type"`
}

type configureAlertResponse struct {
	AlertID    string `json:"alert_id"`
	GeofenceID string `json:"geofence_id"`
	VehicleID  string `json:"vehicle_id"`
	EventType  string `json:"event_type"`
	Status     string `json:"status"`
}

type alertRuleResponse struct {
	AlertID       string    `json:"alert_id"`
	GeofenceID    string    `json:"geofence_id"`
	GeofenceName  string    `json:"geofence_name"`
	VehicleID     string    `json:"vehicle_id"`
	VehicleNumber string    `json:"vehicle_number"`
	EventType     string    `json:"event_type"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// configureAlert handles POST /alerts/configure.
func (s *Server) configureAlert(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req configureAlertRequest
	if err := decodeJSON(r, &req); err != nil {
		s.fail(w, start, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	req.GeofenceID = strings.TrimSpace(req.GeofenceID)
	req.VehicleID = strings.TrimSpace(req.VehicleID)
	req.EventType = strings.TrimSpace(req.EventType)

	if req.GeofenceID == "" {
		s.fail(w, start, http.StatusBadRequest, "geofence_id is required")
		return
	}
	if req.EventType == "" {
		s.fail(w, start, http.StatusBadRequest, "event_type is required")
		return
	}
	if !models.ValidEventTypes(req.EventType) {
		s.fail(w, start, http.StatusBadRequest, "event_type must be one of: entry, exit, both")
		return
	}

	ctx := r.Context()

	if _, err := s.store.GetGeofence(ctx, req.GeofenceID); err != nil {
		if isNotFound(err) {
			s.fail(w, start, http.StatusNotFound, "geofence not found")
			return
		}
		s.logger.Printf("api: get geofence: %v", err)
		s.fail(w, start, http.StatusInternalServerError, "failed to load geofence")
		return
	}
	if req.VehicleID != "" {
		if _, err := s.store.GetVehicle(ctx, req.VehicleID); err != nil {
			if isNotFound(err) {
				s.fail(w, start, http.StatusNotFound, "vehicle not found")
				return
			}
			s.logger.Printf("api: get vehicle: %v", err)
			s.fail(w, start, http.StatusInternalServerError, "failed to load vehicle")
			return
		}
	}

	cfg := &models.AlertConfig{
		ID:         store.NewID("alert_"),
		GeofenceID: req.GeofenceID,
		VehicleID:  req.VehicleID,
		EventType:  req.EventType,
		Status:     models.StatusActive,
		CreatedAt:  time.Now().UTC().Truncate(time.Second),
	}
	if err := s.store.CreateAlertConfig(ctx, cfg); err != nil {
		if errors.Is(err, store.ErrConflict) {
			s.fail(w, start, http.StatusConflict, "an identical alert rule already exists")
			return
		}
		s.logger.Printf("api: create alert config: %v", err)
		s.fail(w, start, http.StatusInternalServerError, "failed to create alert rule")
		return
	}

	s.writeJSON(w, start, http.StatusOK, configureAlertResponse{
		AlertID:    cfg.ID,
		GeofenceID: cfg.GeofenceID,
		VehicleID:  cfg.VehicleID,
		EventType:  cfg.EventType,
		Status:     cfg.Status,
	})
}

// listAlerts handles GET /alerts?geofence_id=&vehicle_id=.
func (s *Server) listAlerts(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	q := r.URL.Query()
	geofenceID := strings.TrimSpace(q.Get("geofence_id"))
	vehicleID := strings.TrimSpace(q.Get("vehicle_id"))

	configs, err := s.store.ListAlertConfigs(r.Context(), geofenceID, vehicleID)
	if err != nil {
		s.logger.Printf("api: list alert configs: %v", err)
		s.fail(w, start, http.StatusInternalServerError, "failed to list alert rules")
		return
	}

	out := make([]alertRuleResponse, 0, len(configs))
	for _, c := range configs {
		out = append(out, alertRuleResponse{
			AlertID:       c.ID,
			GeofenceID:    c.GeofenceID,
			GeofenceName:  c.GeofenceName,
			VehicleID:     c.VehicleID,
			VehicleNumber: c.VehicleNumber,
			EventType:     c.EventType,
			Status:        c.Status,
			CreatedAt:     c.CreatedAt,
		})
	}

	page, total, paged, err := paginate(r.URL.Query(), out)
	if err != nil {
		s.fail(w, start, http.StatusBadRequest, err.Error())
		return
	}
	resp := map[string]any{"alerts": page}
	if paged {
		resp["total_count"] = total
	}
	s.writeJSON(w, start, http.StatusOK, resp)
}
