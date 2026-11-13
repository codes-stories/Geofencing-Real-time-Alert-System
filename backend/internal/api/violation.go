package api

import (
	"net/http"
	"strings"
	"time"

	"geofencing/backend/internal/models"
)

// listViolations handles
// GET /violations/history?vehicle_id=&geofence_id=&start_date=&end_date=&limit=&offset=.
func (s *Server) listViolations(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	q := r.URL.Query()

	filter := models.ViolationFilter{
		VehicleID:  strings.TrimSpace(q.Get("vehicle_id")),
		GeofenceID: strings.TrimSpace(q.Get("geofence_id")),
	}

	limit, err := parseLimit(q.Get("limit"))
	if err != nil {
		s.fail(w, start, http.StatusBadRequest, err.Error())
		return
	}
	offset, err := parseOffset(q.Get("offset"))
	if err != nil {
		s.fail(w, start, http.StatusBadRequest, err.Error())
		return
	}
	filter.Limit = limit
	filter.Offset = offset

	if filter.StartDate, err = parseOptionalTime("start_date", q.Get("start_date")); err != nil {
		s.fail(w, start, http.StatusBadRequest, err.Error())
		return
	}
	if filter.EndDate, err = parseOptionalTime("end_date", q.Get("end_date")); err != nil {
		s.fail(w, start, http.StatusBadRequest, err.Error())
		return
	}
	if filter.StartDate != nil && filter.EndDate != nil && filter.EndDate.Before(*filter.StartDate) {
		s.fail(w, start, http.StatusBadRequest, "end_date must be after start_date")
		return
	}

	violations, total, err := s.store.ListViolations(r.Context(), filter)
	if err != nil {
		s.logger.Printf("api: list violations: %v", err)
		s.fail(w, start, http.StatusInternalServerError, "failed to list violations")
		return
	}

	s.writeJSON(w, start, http.StatusOK, map[string]any{
		"violations":  violations,
		"total_count": total,
	})
}
