package store

import (
	"context"
	"sort"
	"sync"

	"geofencing/backend/internal/geo"
	"geofencing/backend/internal/models"
)

// MemoryStore is a concurrency-safe in-memory Store implementation. It is
// used when the server is started without a DATABASE_URL, which keeps local
// development and tests free of external dependencies. Geofence containment
// is computed with the same point-in-polygon algorithm covered by unit tests.
type MemoryStore struct {
	mu sync.RWMutex

	geofences    map[string]*models.Geofence
	vehicles     map[string]*models.Vehicle
	locations    map[string][]models.Location // vehicleID -> history (last is newest)
	membership   map[string][]string          // vehicleID -> geofence ids currently inside
	violations   []*models.Violation
	alertConfigs []*models.AlertConfig
	vehicleByNum map[string]string // vehicle_number -> id (unique index)
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		geofences:    map[string]*models.Geofence{},
		vehicles:     map[string]*models.Vehicle{},
		locations:    map[string][]models.Location{},
		membership:   map[string][]string{},
		vehicleByNum: map[string]string{},
	}
}

// --- Geofences ---

func (s *MemoryStore) CreateGeofence(_ context.Context, g *models.Geofence) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *g
	cp.Coordinates = cloneCoords(g.Coordinates)
	s.geofences[g.ID] = &cp
	return nil
}

func (s *MemoryStore) ListGeofences(_ context.Context, category string) ([]models.Geofence, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Geofence, 0, len(s.geofences))
	for _, g := range s.geofences {
		if category != "" && g.Category != category {
			continue
		}
		cp := *g
		cp.Coordinates = cloneCoords(g.Coordinates)
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *MemoryStore) GetGeofence(_ context.Context, id string) (*models.Geofence, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.geofences[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *g
	cp.Coordinates = cloneCoords(g.Coordinates)
	return &cp, nil
}

// --- Vehicles ---

func (s *MemoryStore) CreateVehicle(_ context.Context, v *models.Vehicle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.vehicleByNum[v.VehicleNumber]; exists {
		return ErrConflict
	}
	cp := *v
	s.vehicles[v.ID] = &cp
	s.vehicleByNum[v.VehicleNumber] = v.ID
	return nil
}

func (s *MemoryStore) ListVehicles(_ context.Context) ([]models.Vehicle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Vehicle, 0, len(s.vehicles))
	for _, v := range s.vehicles {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *MemoryStore) GetVehicle(_ context.Context, id string) (*models.Vehicle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.vehicles[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *v
	return &cp, nil
}

// --- Locations and membership ---

func (s *MemoryStore) SaveLocation(_ context.Context, vehicleID string, loc models.Location) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	loc.Timestamp = truncateSecond(loc.Timestamp)
	s.locations[vehicleID] = append(s.locations[vehicleID], loc)
	return nil
}

func (s *MemoryStore) GetLatestLocation(_ context.Context, vehicleID string) (*models.Location, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	hist := s.locations[vehicleID]
	if len(hist) == 0 {
		return nil, ErrNotFound
	}
	loc := hist[len(hist)-1]
	return &loc, nil
}

func (s *MemoryStore) InsideGeofences(_ context.Context, lat, lng float64) ([]models.Geofence, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Geofence, 0)
	for _, g := range s.geofences {
		if g.Status != models.StatusActive {
			continue
		}
		if geo.Contains(g.Coordinates, lat, lng) {
			cp := *g
			cp.Coordinates = cloneCoords(g.Coordinates)
			out = append(out, cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *MemoryStore) GetGeofenceMembership(_ context.Context, vehicleID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.membership[vehicleID]...), nil
}

func (s *MemoryStore) SetGeofenceMembership(_ context.Context, vehicleID string, geofenceIDs []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.membership[vehicleID] = append([]string(nil), geofenceIDs...)
	return nil
}

// --- Violations ---

func (s *MemoryStore) CreateViolation(_ context.Context, v *models.Violation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *v
	cp.Timestamp = truncateSecond(cp.Timestamp)
	s.violations = append(s.violations, &cp)
	return nil
}

func (s *MemoryStore) ListViolations(_ context.Context, f models.ViolationFilter) ([]models.Violation, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	matched := make([]*models.Violation, 0, len(s.violations))
	for _, v := range s.violations {
		if f.VehicleID != "" && v.VehicleID != f.VehicleID {
			continue
		}
		if f.GeofenceID != "" && v.GeofenceID != f.GeofenceID {
			continue
		}
		if f.StartDate != nil && v.Timestamp.Before(*f.StartDate) {
			continue
		}
		if f.EndDate != nil && v.Timestamp.After(*f.EndDate) {
			continue
		}
		matched = append(matched, v)
	}
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].Timestamp.Equal(matched[j].Timestamp) {
			return matched[i].ID > matched[j].ID
		}
		return matched[i].Timestamp.After(matched[j].Timestamp)
	})

	total := len(matched)
	start := f.Offset
	if start > total {
		start = total
	}
	end := start + f.Limit
	if end > total {
		end = total
	}
	out := make([]models.Violation, 0, end-start)
	for _, v := range matched[start:end] {
		out = append(out, *v)
	}
	return out, total, nil
}

// --- Alert rules ---

func (s *MemoryStore) CreateAlertConfig(_ context.Context, a *models.AlertConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := *a
	s.alertConfigs = append(s.alertConfigs, &cp)
	return nil
}

func (s *MemoryStore) ListAlertConfigs(_ context.Context, geofenceID, vehicleID string) ([]models.AlertConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.AlertConfig, 0, len(s.alertConfigs))
	for _, a := range s.alertConfigs {
		if geofenceID != "" && a.GeofenceID != geofenceID {
			continue
		}
		if vehicleID != "" && a.VehicleID != vehicleID {
			continue
		}
		cp := *a
		if g, ok := s.geofences[a.GeofenceID]; ok {
			cp.GeofenceName = g.Name
		}
		if a.VehicleID != "" {
			if v, ok := s.vehicles[a.VehicleID]; ok {
				cp.VehicleNumber = v.VehicleNumber
			}
		}
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// --- Alert events ---

func (s *MemoryStore) CreateAlertEvent(_ context.Context, _ models.AlertEvent) error {
	// Events are already persisted as violations; this hook exists so both
	// stores expose the same contract for future querying of triggered alerts.
	return nil
}

func (s *MemoryStore) Ping(context.Context) error { return nil }

func (s *MemoryStore) Close() {}

func cloneCoords(in [][]float64) [][]float64 {
	out := make([][]float64, len(in))
	for i, p := range in {
		out[i] = append([]float64(nil), p...)
	}
	return out
}
