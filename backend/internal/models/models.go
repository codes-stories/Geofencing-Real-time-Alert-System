// Package models defines the domain types shared across the geofencing API.
package models

import "time"

// Geofence categories accepted by the API.
const (
	CategoryDeliveryZone   = "delivery_zone"
	CategoryRestrictedZone = "restricted_zone"
	CategoryTollZone       = "toll_zone"
	CategoryCustomerArea   = "customer_area"
)

// ValidCategories reports whether c is a supported geofence category.
func ValidCategories(c string) bool {
	switch c {
	case CategoryDeliveryZone, CategoryRestrictedZone, CategoryTollZone, CategoryCustomerArea:
		return true
	}
	return false
}

// Categories lists the supported geofence categories in display order.
func Categories() []string {
	return []string{
		CategoryDeliveryZone,
		CategoryRestrictedZone,
		CategoryTollZone,
		CategoryCustomerArea,
	}
}

// Alert event types accepted by the API.
const (
	EventTypeEntry = "entry"
	EventTypeExit  = "exit"
	EventTypeBoth  = "both"
)

// ValidEventTypes reports whether t is a supported alert event type.
func ValidEventTypes(t string) bool {
	switch t {
	case EventTypeEntry, EventTypeExit, EventTypeBoth:
		return true
	}
	return false
}

// Geofence statuses.
const (
	StatusActive   = "active"
	StatusInactive = "inactive"
)

// Geofence is a named polygonal boundary on the map.
type Geofence struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Coordinates [][]float64 `json:"coordinates"`
	Category    string      `json:"category"`
	Status      string      `json:"status,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
}

// Vehicle is a tracked asset with an assigned driver.
type Vehicle struct {
	ID            string    `json:"id"`
	VehicleNumber string    `json:"vehicle_number"`
	DriverName    string    `json:"driver_name"`
	VehicleType   string    `json:"vehicle_type"`
	Phone         string    `json:"phone"`
	Status        string    `json:"status,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// Location is a single recorded position of a vehicle.
type Location struct {
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Timestamp time.Time `json:"timestamp"`
}

// GeofenceStatus describes a geofence that currently contains a vehicle.
type GeofenceStatus struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Status       string `json:"status,omitempty"`
	Category     string `json:"category,omitempty"`
}

// Violation is a historical geofence entry/exit event.
type Violation struct {
	ID            string    `json:"id"`
	VehicleID     string    `json:"vehicle_id"`
	VehicleNumber string    `json:"vehicle_number"`
	GeofenceID    string    `json:"geofence_id"`
	GeofenceName  string    `json:"geofence_name"`
	EventType     string    `json:"event_type"`
	Latitude      float64   `json:"latitude"`
	Longitude     float64   `json:"longitude"`
	Timestamp     time.Time `json:"timestamp"`
}

// ViolationFilter captures the query options for violation history.
type ViolationFilter struct {
	VehicleID  string
	GeofenceID string
	StartDate  *time.Time
	EndDate    *time.Time
	Limit      int
	Offset     int
}

// AlertConfig is a persisted alert rule for geofence events.
// An empty VehicleID means the rule applies to all vehicles.
type AlertConfig struct {
	ID            string    `json:"alert_id"`
	GeofenceID    string    `json:"geofence_id"`
	GeofenceName  string    `json:"geofence_name,omitempty"`
	VehicleID     string    `json:"vehicle_id,omitempty"`
	VehicleNumber string    `json:"vehicle_number,omitempty"`
	EventType     string    `json:"event_type"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

// VehicleRef identifies the vehicle involved in an alert event.
type VehicleRef struct {
	VehicleID     string `json:"vehicle_id"`
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
}

// GeofenceRef identifies the geofence involved in an alert event.
type GeofenceRef struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Category     string `json:"category"`
}

// LocationRef is the position reported with an alert event.
type LocationRef struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// AlertEvent is both the WebSocket payload and the stored alert record.
type AlertEvent struct {
	EventID   string      `json:"event_id"`
	EventType string      `json:"event_type"`
	Timestamp time.Time   `json:"timestamp"`
	Vehicle   VehicleRef  `json:"vehicle"`
	Geofence  GeofenceRef `json:"geofence"`
	Location  LocationRef `json:"location"`
}
