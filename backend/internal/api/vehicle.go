package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"geofencing/backend/internal/models"
	"geofencing/backend/internal/store"
)

type createVehicleRequest struct {
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
	VehicleType   string `json:"vehicle_type"`
	Phone         string `json:"phone"`
}

type createVehicleResponse struct {
	ID            string `json:"id"`
	VehicleNumber string `json:"vehicle_number"`
	Status        string `json:"status"`
}

type vehicleResponse struct {
	ID            string    `json:"id"`
	VehicleNumber string    `json:"vehicle_number"`
	DriverName    string    `json:"driver_name"`
	VehicleType   string    `json:"vehicle_type"`
	Phone         string    `json:"phone"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// createVehicle handles POST /vehicles.
func (s *Server) createVehicle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req createVehicleRequest
	if err := decodeJSON(r, &req); err != nil {
		s.fail(w, start, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	req.VehicleNumber = strings.TrimSpace(req.VehicleNumber)
	req.DriverName = strings.TrimSpace(req.DriverName)
	req.VehicleType = strings.TrimSpace(req.VehicleType)
	req.Phone = strings.TrimSpace(req.Phone)

	if req.VehicleNumber == "" {
		s.fail(w, start, http.StatusBadRequest, "vehicle_number is required")
		return
	}
	if req.DriverName == "" {
		s.fail(w, start, http.StatusBadRequest, "driver_name is required")
		return
	}
	if req.VehicleType == "" {
		s.fail(w, start, http.StatusBadRequest, "vehicle_type is required")
		return
	}
	if req.Phone == "" {
		s.fail(w, start, http.StatusBadRequest, "phone is required")
		return
	}

	v := &models.Vehicle{
		ID:            store.NewID("veh_"),
		VehicleNumber: req.VehicleNumber,
		DriverName:    req.DriverName,
		VehicleType:   req.VehicleType,
		Phone:         req.Phone,
		Status:        models.StatusActive,
		CreatedAt:     time.Now().UTC().Truncate(time.Second),
	}

	if err := s.store.CreateVehicle(r.Context(), v); err != nil {
		if errors.Is(err, store.ErrConflict) {
			s.fail(w, start, http.StatusConflict, "a vehicle with this vehicle_number already exists")
			return
		}
		s.logger.Printf("api: create vehicle: %v", err)
		s.fail(w, start, http.StatusInternalServerError, "failed to create vehicle")
		return
	}

	s.writeJSON(w, start, http.StatusOK, createVehicleResponse{
		ID:            v.ID,
		VehicleNumber: v.VehicleNumber,
		Status:        v.Status,
	})
}

// listVehicles handles GET /vehicles.
func (s *Server) listVehicles(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	vehicles, err := s.store.ListVehicles(r.Context())
	if err != nil {
		s.logger.Printf("api: list vehicles: %v", err)
		s.fail(w, start, http.StatusInternalServerError, "failed to list vehicles")
		return
	}

	out := make([]vehicleResponse, 0, len(vehicles))
	for _, v := range vehicles {
		out = append(out, vehicleResponse{
			ID:            v.ID,
			VehicleNumber: v.VehicleNumber,
			DriverName:    v.DriverName,
			VehicleType:   v.VehicleType,
			Phone:         v.Phone,
			Status:        v.Status,
			CreatedAt:     v.CreatedAt,
		})
	}

	page, total, paged, err := paginate(r.URL.Query(), out)
	if err != nil {
		s.fail(w, start, http.StatusBadRequest, err.Error())
		return
	}
	resp := map[string]any{"vehicles": page}
	if paged {
		resp["total_count"] = total
	}
	s.writeJSON(w, start, http.StatusOK, resp)
}
