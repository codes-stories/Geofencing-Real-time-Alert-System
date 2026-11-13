// Package store provides persistence for geofences, vehicles, locations,
// violations and alert rules. Two implementations are available: a
// PostgreSQL/PostGIS store and an in-memory store used when no DATABASE_URL
// is configured (handy for local development and unit tests).
package store

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"geofencing/backend/internal/models"
)

// Sentinel errors returned by Store implementations.
var (
	ErrNotFound = errors.New("record not found")
	ErrConflict = errors.New("duplicate record")
)

// Store is the persistence boundary of the application.
type Store interface {
	// Geofences
	CreateGeofence(ctx context.Context, g *models.Geofence) error
	ListGeofences(ctx context.Context, category string) ([]models.Geofence, error)
	GetGeofence(ctx context.Context, id string) (*models.Geofence, error)

	// Vehicles
	CreateVehicle(ctx context.Context, v *models.Vehicle) error
	ListVehicles(ctx context.Context) ([]models.Vehicle, error)
	GetVehicle(ctx context.Context, id string) (*models.Vehicle, error)

	// Locations and geofence membership state
	SaveLocation(ctx context.Context, vehicleID string, loc models.Location) error
	GetLatestLocation(ctx context.Context, vehicleID string) (*models.Location, error)
	InsideGeofences(ctx context.Context, lat, lng float64) ([]models.Geofence, error)
	GetGeofenceMembership(ctx context.Context, vehicleID string) ([]string, error)
	SetGeofenceMembership(ctx context.Context, vehicleID string, geofenceIDs []string) error

	// Violation history
	CreateViolation(ctx context.Context, v *models.Violation) error
	ListViolations(ctx context.Context, f models.ViolationFilter) ([]models.Violation, int, error)

	// Alert rules
	CreateAlertConfig(ctx context.Context, a *models.AlertConfig) error
	ListAlertConfigs(ctx context.Context, geofenceID, vehicleID string) ([]models.AlertConfig, error)

	// Triggered alert events (historical record of real-time alerts)
	CreateAlertEvent(ctx context.Context, e models.AlertEvent) error

	Ping(ctx context.Context) error
	Close()
}

// NewID returns a unique identifier with the given prefix, e.g. "geo_1a2b3c4d".
//
// The value embeds the current unix millisecond and a per-process sequence
// number, so lexicographic order matches creation order. That gives list
// endpoints a deterministic tie-breaker when created_at timestamps are equal
// (they are truncated to whole seconds for the API).
func NewID(prefix string) string {
	millis := uint64(time.Now().UnixMilli())
	seq := idSeq.Add(1)
	return prefix + fmt.Sprintf("%016x", millis<<16|seq&0xffff)
}

// idSeq backs NewID's per-millisecond sequence number.
var idSeq atomic.Uint64
