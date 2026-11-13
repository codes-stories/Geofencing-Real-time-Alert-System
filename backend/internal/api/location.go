package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"geofencing/backend/internal/models"
	"geofencing/backend/internal/store"
)

type updateLocationRequest struct {
	VehicleID string   `json:"vehicle_id"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Timestamp string   `json:"timestamp"`
}

type currentLocationResponse struct {
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Timestamp time.Time `json:"timestamp"`
}

type geofenceEntryResponse struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Status       string `json:"status,omitempty"`
	Category     string `json:"category,omitempty"`
}

// transition is an entry or exit detected between two location updates.
type transition struct {
	geofence  models.Geofence
	eventType string
}

// updateVehicleLocation handles POST /vehicles/location.
//
// It stores the new position, recomputes which geofences contain the
// vehicle, diffs the result against the previous membership to detect
// entry/exit transitions, persists them as violation history and pushes
// matching real-time alerts over WebSocket (asynchronously).
func (s *Server) updateVehicleLocation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req updateLocationRequest
	if err := decodeJSON(r, &req); err != nil {
		s.fail(w, start, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	if strings.TrimSpace(req.VehicleID) == "" {
		s.fail(w, start, http.StatusBadRequest, "vehicle_id is required")
		return
	}
	if req.Latitude == nil {
		s.fail(w, start, http.StatusBadRequest, "latitude is required")
		return
	}
	if req.Longitude == nil {
		s.fail(w, start, http.StatusBadRequest, "longitude is required")
		return
	}
	lat, lng := *req.Latitude, *req.Longitude
	if lat < -90 || lat > 90 {
		s.fail(w, start, http.StatusBadRequest, "latitude must be between -90 and 90")
		return
	}
	if lng < -180 || lng > 180 {
		s.fail(w, start, http.StatusBadRequest, "longitude must be between -180 and 180")
		return
	}
	ts, err := parseTimestamp(req.Timestamp)
	if err != nil {
		s.fail(w, start, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()

	vehicle, err := s.store.GetVehicle(ctx, req.VehicleID)
	if err != nil {
		if isNotFound(err) {
			s.fail(w, start, http.StatusNotFound, "vehicle not found")
			return
		}
		s.logger.Printf("api: get vehicle: %v", err)
		s.fail(w, start, http.StatusInternalServerError, "failed to load vehicle")
		return
	}

	loc := models.Location{Latitude: lat, Longitude: lng, Timestamp: ts}
	if err := s.store.SaveLocation(ctx, vehicle.ID, loc); err != nil {
		s.logger.Printf("api: save location: %v", err)
		s.fail(w, start, http.StatusInternalServerError, "failed to update location")
		return
	}

	current, err := s.store.InsideGeofences(ctx, lat, lng)
	if err != nil {
		s.logger.Printf("api: geofence containment query: %v", err)
		s.fail(w, start, http.StatusInternalServerError, "failed to evaluate geofences")
		return
	}

	prevIDs, err := s.store.GetGeofenceMembership(ctx, vehicle.ID)
	if err != nil {
		s.logger.Printf("api: get geofence membership: %v", err)
		s.fail(w, start, http.StatusInternalServerError, "failed to load geofence state")
		return
	}

	// Diff previous membership against the current containment set.
	prevSet := make(map[string]bool, len(prevIDs))
	for _, id := range prevIDs {
		prevSet[id] = true
	}
	curIDs := make([]string, 0, len(current))
	curSet := make(map[string]bool, len(current))
	for _, g := range current {
		curIDs = append(curIDs, g.ID)
		curSet[g.ID] = true
	}

	var transitions []transition
	for _, g := range current {
		if !prevSet[g.ID] {
			transitions = append(transitions, transition{geofence: g, eventType: models.EventTypeEntry})
		}
	}
	for _, id := range prevIDs {
		if curSet[id] {
			continue
		}
		g, err := s.store.GetGeofence(ctx, id)
		if err != nil {
			// The geofence disappeared between updates; skip the exit event.
			s.logger.Printf("api: load geofence %s for exit event: %v", id, err)
			continue
		}
		transitions = append(transitions, transition{geofence: *g, eventType: models.EventTypeExit})
	}

	if err := s.store.SetGeofenceMembership(ctx, vehicle.ID, curIDs); err != nil {
		s.logger.Printf("api: set geofence membership: %v", err)
		s.fail(w, start, http.StatusInternalServerError, "failed to save geofence state")
		return
	}

	// Persist history and dispatch real-time alerts for each transition.
	s.processTransitions(r.Context(), vehicle, loc, transitions)

	currentGeofences := make([]geofenceEntryResponse, 0, len(current))
	for _, g := range current {
		currentGeofences = append(currentGeofences, geofenceEntryResponse{
			GeofenceID:   g.ID,
			GeofenceName: g.Name,
			Status:       "inside",
		})
	}

	s.writeJSON(w, start, http.StatusOK, map[string]any{
		"vehicle_id":        vehicle.ID,
		"location_updated":  true,
		"current_geofences": currentGeofences,
	})
}

// processTransitions stores each entry/exit event and, when an alert rule
// matches (or the geofence is a restricted zone), records the alert and
// broadcasts it to WebSocket clients without blocking the HTTP response.
func (s *Server) processTransitions(ctx context.Context, vehicle *models.Vehicle, loc models.Location, transitions []transition) {
	for _, t := range transitions {
		violation := &models.Violation{
			ID:            store.NewID("viol_"),
			VehicleID:     vehicle.ID,
			VehicleNumber: vehicle.VehicleNumber,
			GeofenceID:    t.geofence.ID,
			GeofenceName:  t.geofence.Name,
			EventType:     t.eventType,
			Latitude:      loc.Latitude,
			Longitude:     loc.Longitude,
			Timestamp:     loc.Timestamp,
		}
		if err := s.store.CreateViolation(ctx, violation); err != nil {
			s.logger.Printf("api: store violation: %v", err)
		}

		if !s.shouldAlert(ctx, t.geofence, vehicle, t.eventType) {
			continue
		}

		event := models.AlertEvent{
			EventID:   store.NewID("evt_"),
			EventType: t.eventType,
			Timestamp: loc.Timestamp,
			Vehicle: models.VehicleRef{
				VehicleID:     vehicle.ID,
				VehicleNumber: vehicle.VehicleNumber,
				DriverName:    vehicle.DriverName,
			},
			Geofence: models.GeofenceRef{
				GeofenceID:   t.geofence.ID,
				GeofenceName: t.geofence.Name,
				Category:     t.geofence.Category,
			},
			Location: models.LocationRef{
				Latitude:  loc.Latitude,
				Longitude: loc.Longitude,
			},
		}

		if err := s.store.CreateAlertEvent(ctx, event); err != nil {
			s.logger.Printf("api: store alert event: %v", err)
		}

		// Deliver asynchronously so the HTTP response is never delayed.
		go s.hub.Broadcast(event)
	}
}

// shouldAlert reports whether a real-time alert must be generated for the
// transition. An alert fires when:
//   - a configured rule matches the geofence, the vehicle (or all
//     vehicles) and the event type; or
//   - the geofence is a restricted zone (always monitored).
func (s *Server) shouldAlert(ctx context.Context, g models.Geofence, v *models.Vehicle, eventType string) bool {
	if g.Category == models.CategoryRestrictedZone {
		return true
	}
	configs, err := s.store.ListAlertConfigs(ctx, g.ID, "")
	if err != nil {
		s.logger.Printf("api: load alert configs: %v", err)
		return false
	}
	for _, c := range configs {
		if c.VehicleID != "" && c.VehicleID != v.ID {
			continue
		}
		if c.EventType == models.EventTypeBoth || c.EventType == eventType {
			return true
		}
	}
	return false
}

// getVehicleLocation handles GET /vehicles/location/{vehicle_id}.
func (s *Server) getVehicleLocation(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	vehicleID := strings.TrimSpace(r.PathValue("vehicle_id"))
	if vehicleID == "" {
		s.fail(w, start, http.StatusBadRequest, "vehicle_id is required")
		return
	}

	ctx := r.Context()
	vehicle, err := s.store.GetVehicle(ctx, vehicleID)
	if err != nil {
		if isNotFound(err) {
			s.fail(w, start, http.StatusNotFound, "vehicle not found")
			return
		}
		s.logger.Printf("api: get vehicle: %v", err)
		s.fail(w, start, http.StatusInternalServerError, "failed to load vehicle")
		return
	}

	resp := map[string]any{
		"vehicle_id":        vehicle.ID,
		"vehicle_number":    vehicle.VehicleNumber,
		"current_location":  nil,
		"current_geofences": []geofenceEntryResponse{},
	}

	loc, err := s.store.GetLatestLocation(ctx, vehicle.ID)
	if err != nil {
		if !isNotFound(err) {
			s.logger.Printf("api: get latest location: %v", err)
			s.fail(w, start, http.StatusInternalServerError, "failed to load location")
			return
		}
		// No location recorded yet: respond with a null current_location.
		s.writeJSON(w, start, http.StatusOK, resp)
		return
	}

	inside, err := s.store.InsideGeofences(ctx, loc.Latitude, loc.Longitude)
	if err != nil {
		s.logger.Printf("api: geofence containment query: %v", err)
		s.fail(w, start, http.StatusInternalServerError, "failed to evaluate geofences")
		return
	}

	entries := make([]geofenceEntryResponse, 0, len(inside))
	for _, g := range inside {
		entries = append(entries, geofenceEntryResponse{
			GeofenceID:   g.ID,
			GeofenceName: g.Name,
			Category:     g.Category,
		})
	}

	resp["current_location"] = currentLocationResponse{
		Latitude:  loc.Latitude,
		Longitude: loc.Longitude,
		Timestamp: loc.Timestamp,
	}
	resp["current_geofences"] = entries

	s.writeJSON(w, start, http.StatusOK, resp)
}
