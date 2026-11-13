-- Geofencing schema for PostgreSQL + PostGIS
-- Applied automatically at startup. Every statement is idempotent.

CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE IF NOT EXISTS geofences (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    coordinates  JSONB NOT NULL,
    category     TEXT NOT NULL CHECK (category IN ('delivery_zone','restricted_zone','toll_zone','customer_area')),
    status       TEXT NOT NULL DEFAULT 'active',
    geom         geometry(Polygon, 4326),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_geofences_category ON geofences (category);
CREATE INDEX IF NOT EXISTS idx_geofences_geom ON geofences USING GIST (geom);

CREATE TABLE IF NOT EXISTS vehicles (
    id             TEXT PRIMARY KEY,
    vehicle_number TEXT NOT NULL UNIQUE,
    driver_name    TEXT NOT NULL,
    vehicle_type   TEXT NOT NULL,
    phone          TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'active',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS vehicle_locations (
    id          BIGSERIAL PRIMARY KEY,
    vehicle_id  TEXT NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    latitude    DOUBLE PRECISION NOT NULL,
    longitude   DOUBLE PRECISION NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_vehicle_locations_vehicle_time
    ON vehicle_locations (vehicle_id, recorded_at DESC);

-- Remembers which geofences each vehicle is currently inside so that
-- entry/exit transitions can be detected on the next location update.
CREATE TABLE IF NOT EXISTS vehicle_geofence_state (
    vehicle_id     TEXT PRIMARY KEY REFERENCES vehicles (id) ON DELETE CASCADE,
    geofence_ids   TEXT[] NOT NULL DEFAULT '{}',
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS violations (
    id           TEXT PRIMARY KEY,
    vehicle_id   TEXT NOT NULL,
    geofence_id  TEXT NOT NULL,
    event_type   TEXT NOT NULL CHECK (event_type IN ('entry','exit')),
    latitude     DOUBLE PRECISION NOT NULL,
    longitude    DOUBLE PRECISION NOT NULL,
    timestamp    TIMESTAMPTZ NOT NULL,
    vehicle_number TEXT NOT NULL DEFAULT '',
    geofence_name  TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_violations_vehicle ON violations (vehicle_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_violations_geofence ON violations (geofence_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_violations_timestamp ON violations (timestamp DESC);

CREATE TABLE IF NOT EXISTS alert_configs (
    id           TEXT PRIMARY KEY,
    geofence_id  TEXT NOT NULL REFERENCES geofences (id) ON DELETE CASCADE,
    vehicle_id   TEXT REFERENCES vehicles (id) ON DELETE CASCADE,
    event_type   TEXT NOT NULL CHECK (event_type IN ('entry','exit','both')),
    status       TEXT NOT NULL DEFAULT 'active',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_alert_configs_geofence ON alert_configs (geofence_id);

-- Full JSONB payload of every real-time alert pushed over WebSocket.
CREATE TABLE IF NOT EXISTS alert_events (
    id           TEXT PRIMARY KEY,
    event_type   TEXT NOT NULL,
    vehicle_id   TEXT NOT NULL,
    geofence_id  TEXT NOT NULL,
    latitude     DOUBLE PRECISION NOT NULL,
    longitude    DOUBLE PRECISION NOT NULL,
    timestamp    TIMESTAMPTZ NOT NULL,
    payload      JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_alert_events_timestamp ON alert_events (timestamp DESC);
