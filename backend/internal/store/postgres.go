package store

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"geofencing/backend/internal/geo"
	"geofencing/backend/internal/models"
)

// schemaSQL is the embedded, idempotent DDL applied at startup.
//
//go:embed schema.sql
var schemaSQL string

// PostgresStore persists data in PostgreSQL with PostGIS for geospatial
// containment queries (GiST-indexed ST_Contains).
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore connects to PostgreSQL using DATABASE_URL, applies the
// schema and returns a ready-to-use store.
func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	s := &PostgresStore{pool: pool}
	if err := s.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return s, nil
}

// NewPostgresStoreWithRetry waits for the database to become available,
// which is useful when the server starts alongside docker-compose services.
func NewPostgresStoreWithRetry(ctx context.Context, databaseURL string, attempts int, delay time.Duration) (*PostgresStore, error) {
	var lastErr error
	for i := 0; i < attempts; i++ {
		s, err := NewPostgresStore(ctx, databaseURL)
		if err == nil {
			return s, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, lastErr
}

func (s *PostgresStore) migrate(ctx context.Context) error {
	for _, stmt := range strings.Split(schemaSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			// Skip empty chunks; comments before a statement travel with it.
			if stmt == "" {
				continue
			}
		}
		// Strip leading comment lines so the statement itself is executable.
		lines := strings.Split(stmt, "\n")
		kept := lines[:0]
		for _, l := range lines {
			if strings.HasPrefix(strings.TrimSpace(l), "--") {
				continue
			}
			kept = append(kept, l)
		}
		stmt = strings.TrimSpace(strings.Join(kept, "\n"))
		if stmt == "" {
			continue
		}
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("exec statement %q: %w", firstWords(stmt), err)
		}
	}
	return nil
}

func firstWords(s string) string {
	words := strings.Fields(s)
	if len(words) > 6 {
		return strings.Join(words[:6], " ") + "..."
	}
	return strings.Join(words, " ")
}

// --- Geofences ---

func (s *PostgresStore) CreateGeofence(ctx context.Context, g *models.Geofence) error {
	coords, err := json.Marshal(g.Coordinates)
	if err != nil {
		return err
	}
	geoJSON := geo.PolygonGeoJSON(g.Coordinates)
	_, err = s.pool.Exec(ctx, `
		INSERT INTO geofences (id, name, description, coordinates, category, status, geom, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, ST_SetSRID(ST_GeomFromGeoJSON($7), 4326), $8)`,
		g.ID, g.Name, g.Description, coords, g.Category, g.Status, geoJSON, g.CreatedAt,
	)
	return wrapErr(err)
}

func (s *PostgresStore) ListGeofences(ctx context.Context, category string) ([]models.Geofence, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, description, coordinates, category, status, created_at
		FROM geofences
		WHERE ($1 = '' OR category = $1)
		ORDER BY created_at ASC, id ASC`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Geofence{}
	for rows.Next() {
		g, err := scanGeofence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *g)
	}
	return out, rows.Err()
}

func (s *PostgresStore) GetGeofence(ctx context.Context, id string) (*models.Geofence, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, description, coordinates, category, status, created_at
		FROM geofences WHERE id = $1`, id)
	g, err := scanGeofence(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return g, err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanGeofence(row rowScanner) (*models.Geofence, error) {
	var (
		g      models.Geofence
		coords []byte
	)
	err := row.Scan(&g.ID, &g.Name, &g.Description, &coords, &g.Category, &g.Status, &g.CreatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(coords, &g.Coordinates); err != nil {
		return nil, fmt.Errorf("decode coordinates: %w", err)
	}
	g.CreatedAt = truncateSecond(g.CreatedAt)
	return &g, nil
}

// --- Vehicles ---

func (s *PostgresStore) CreateVehicle(ctx context.Context, v *models.Vehicle) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO vehicles (id, vehicle_number, driver_name, vehicle_type, phone, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		v.ID, v.VehicleNumber, v.DriverName, v.VehicleType, v.Phone, v.Status, v.CreatedAt,
	)
	return wrapErr(err)
}

func (s *PostgresStore) ListVehicles(ctx context.Context) ([]models.Vehicle, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, vehicle_number, driver_name, vehicle_type, phone, status, created_at
		FROM vehicles ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Vehicle{}
	for rows.Next() {
		var v models.Vehicle
		if err := rows.Scan(&v.ID, &v.VehicleNumber, &v.DriverName, &v.VehicleType, &v.Phone, &v.Status, &v.CreatedAt); err != nil {
			return nil, err
		}
		v.CreatedAt = truncateSecond(v.CreatedAt)
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *PostgresStore) GetVehicle(ctx context.Context, id string) (*models.Vehicle, error) {
	var v models.Vehicle
	err := s.pool.QueryRow(ctx, `
		SELECT id, vehicle_number, driver_name, vehicle_type, phone, status, created_at
		FROM vehicles WHERE id = $1`, id).
		Scan(&v.ID, &v.VehicleNumber, &v.DriverName, &v.VehicleType, &v.Phone, &v.Status, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	v.CreatedAt = truncateSecond(v.CreatedAt)
	return &v, nil
}

// --- Locations and membership ---

func (s *PostgresStore) SaveLocation(ctx context.Context, vehicleID string, loc models.Location) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO vehicle_locations (vehicle_id, latitude, longitude, recorded_at)
		VALUES ($1, $2, $3, $4)`,
		vehicleID, loc.Latitude, loc.Longitude, loc.Timestamp,
	)
	return wrapErr(err)
}

func (s *PostgresStore) GetLatestLocation(ctx context.Context, vehicleID string) (*models.Location, error) {
	var loc models.Location
	err := s.pool.QueryRow(ctx, `
		SELECT latitude, longitude, recorded_at
		FROM vehicle_locations
		WHERE vehicle_id = $1
		ORDER BY recorded_at DESC, id DESC
		LIMIT 1`, vehicleID).
		Scan(&loc.Latitude, &loc.Longitude, &loc.Timestamp)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	loc.Timestamp = truncateSecond(loc.Timestamp)
	return &loc, nil
}

// InsideGeofences returns every active geofence whose polygon contains the
// point. PostGIS performs the test with a GiST-indexed ST_Contains query;
// coordinates are supplied as (longitude, latitude) per ST_Point's signature.
func (s *PostgresStore) InsideGeofences(ctx context.Context, lat, lng float64) ([]models.Geofence, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, description, coordinates, category, status, created_at
		FROM geofences
		WHERE status = 'active'
		  AND geom IS NOT NULL
		  AND ST_Contains(geom, ST_SetSRID(ST_Point($1, $2), 4326))
		ORDER BY created_at ASC, id ASC`, lng, lat)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.Geofence{}
	for rows.Next() {
		g, err := scanGeofence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *g)
	}
	return out, rows.Err()
}

func (s *PostgresStore) GetGeofenceMembership(ctx context.Context, vehicleID string) ([]string, error) {
	var ids []string
	err := s.pool.QueryRow(ctx, `
		SELECT geofence_ids FROM vehicle_geofence_state WHERE vehicle_id = $1`, vehicleID).
		Scan(&ids)
	if errors.Is(err, pgx.ErrNoRows) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, nil
}

func (s *PostgresStore) SetGeofenceMembership(ctx context.Context, vehicleID string, geofenceIDs []string) error {
	if geofenceIDs == nil {
		geofenceIDs = []string{}
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO vehicle_geofence_state (vehicle_id, geofence_ids, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (vehicle_id)
		DO UPDATE SET geofence_ids = EXCLUDED.geofence_ids, updated_at = now()`,
		vehicleID, geofenceIDs,
	)
	return wrapErr(err)
}

// --- Violations ---

func (s *PostgresStore) CreateViolation(ctx context.Context, v *models.Violation) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO violations (id, vehicle_id, geofence_id, event_type, latitude, longitude, timestamp, vehicle_number, geofence_name)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		v.ID, v.VehicleID, v.GeofenceID, v.EventType, v.Latitude, v.Longitude,
		v.Timestamp, v.VehicleNumber, v.GeofenceName,
	)
	return wrapErr(err)
}

func (s *PostgresStore) ListViolations(ctx context.Context, f models.ViolationFilter) ([]models.Violation, int, error) {
	where := []string{"TRUE"}
	args := []any{}
	add := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if f.VehicleID != "" {
		where = append(where, "vehicle_id = "+add(f.VehicleID))
	}
	if f.GeofenceID != "" {
		where = append(where, "geofence_id = "+add(f.GeofenceID))
	}
	if f.StartDate != nil {
		where = append(where, "timestamp >= "+add(*f.StartDate))
	}
	if f.EndDate != nil {
		where = append(where, "timestamp <= "+add(*f.EndDate))
	}
	whereSQL := strings.Join(where, " AND ")

	var total int
	countSQL := "SELECT COUNT(*) FROM violations WHERE " + whereSQL
	if err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	queryArgs := append(append([]any{}, args...), f.Limit, f.Offset)
	rows, err := s.pool.Query(ctx, `
		SELECT id, vehicle_id, vehicle_number, geofence_id, geofence_name,
		       event_type, latitude, longitude, timestamp
		FROM violations
		WHERE `+whereSQL+`
		ORDER BY timestamp DESC, id DESC
		LIMIT $`+fmt.Sprint(len(args)+1)+` OFFSET $`+fmt.Sprint(len(args)+2),
		queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []models.Violation{}
	for rows.Next() {
		var v models.Violation
		if err := rows.Scan(&v.ID, &v.VehicleID, &v.VehicleNumber, &v.GeofenceID,
			&v.GeofenceName, &v.EventType, &v.Latitude, &v.Longitude, &v.Timestamp); err != nil {
			return nil, 0, err
		}
		v.Timestamp = truncateSecond(v.Timestamp)
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// --- Alert rules ---

func (s *PostgresStore) CreateAlertConfig(ctx context.Context, a *models.AlertConfig) error {
	var vehicleID any
	if a.VehicleID != "" {
		vehicleID = a.VehicleID
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO alert_configs (id, geofence_id, vehicle_id, event_type, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		a.ID, a.GeofenceID, vehicleID, a.EventType, a.Status, a.CreatedAt,
	)
	return wrapErr(err)
}

func (s *PostgresStore) ListAlertConfigs(ctx context.Context, geofenceID, vehicleID string) ([]models.AlertConfig, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.geofence_id, COALESCE(g.name, ''), COALESCE(a.vehicle_id::text, ''),
		       COALESCE(v.vehicle_number, ''), a.event_type, a.status, a.created_at
		FROM alert_configs a
		LEFT JOIN geofences g ON g.id = a.geofence_id
		LEFT JOIN vehicles v ON v.id = a.vehicle_id
		WHERE ($1 = '' OR a.geofence_id = $1)
		  AND ($2 = '' OR a.vehicle_id::text = $2)
		ORDER BY a.created_at ASC, a.id ASC`, geofenceID, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.AlertConfig{}
	for rows.Next() {
		var a models.AlertConfig
		if err := rows.Scan(&a.ID, &a.GeofenceID, &a.GeofenceName, &a.VehicleID,
			&a.VehicleNumber, &a.EventType, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		a.CreatedAt = truncateSecond(a.CreatedAt)
		out = append(out, a)
	}
	return out, rows.Err()
}

// --- Alert events ---

func (s *PostgresStore) CreateAlertEvent(ctx context.Context, e models.AlertEvent) error {
	payload, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO alert_events (id, event_type, vehicle_id, geofence_id, latitude, longitude, timestamp, payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		e.EventID, e.EventType, e.Vehicle.VehicleID, e.Geofence.GeofenceID,
		e.Location.Latitude, e.Location.Longitude, e.Timestamp, payload,
	)
	return wrapErr(err)
}

func (s *PostgresStore) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *PostgresStore) Close() { s.pool.Close() }

// wrapErr maps PostgreSQL unique-violation errors onto ErrConflict so the
// API layer can answer 409 without leaking driver-specific types.
func wrapErr(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}

// LoadDatabaseURL reads the connection string from the environment.
func LoadDatabaseURL() string { return strings.TrimSpace(os.Getenv("DATABASE_URL")) }
