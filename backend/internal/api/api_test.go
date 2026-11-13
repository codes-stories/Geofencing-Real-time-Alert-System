package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"geofencing/backend/internal/api"
	"geofencing/backend/internal/store"
	"geofencing/backend/internal/ws"
)

// testEnv bundles a running test server with its collaborators.
type testEnv struct {
	srv *httptest.Server
	st  store.Store
	hub *ws.Hub
}

func newTestEnv(t *testing.T, envVars map[string]string) *testEnv {
	t.Helper()
	for k, v := range envVars {
		t.Setenv(k, v)
	}
	st := store.NewMemoryStore()
	hub := ws.NewHub()
	go hub.Run()
	t.Cleanup(hub.Stop)

	logger := log.New(io.Discard, "", 0)
	srv := httptest.NewServer(api.NewServer(st, hub, logger).Handler())
	t.Cleanup(srv.Close)
	return &testEnv{srv: srv, st: st, hub: hub}
}

// call performs a JSON request and decodes the response envelope.
func (e *testEnv) call(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.srv.URL+path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("%s %s: non-JSON response %q: %v", method, path, raw, err)
	}
	return res.StatusCode, decoded
}

// mustCall fails the test on a non-2xx response.
func (e *testEnv) mustCall(t *testing.T, method, path string, body any) map[string]any {
	t.Helper()
	status, decoded := e.call(t, method, path, body)
	if status < 200 || status >= 300 {
		t.Fatalf("%s %s = %d, want 2xx: %v", method, path, status, decoded)
	}
	return decoded
}

func (e *testEnv) mustTimeNS(t *testing.T, decoded map[string]any) {
	t.Helper()
	v, ok := decoded["time_ns"]
	if !ok {
		t.Fatalf("response missing time_ns: %v", decoded)
	}
	s, ok := v.(string)
	if !ok || s == "" {
		t.Fatalf("time_ns must be a non-empty string, got %#v", v)
	}
	if _, err := strconv.ParseInt(s, 10, 64); err != nil {
		t.Fatalf("time_ns %q is not an integer: %v", s, err)
	}
}

var squareCoords = [][]float64{
	{37.7749, -122.4194},
	{37.7849, -122.4194},
	{37.7849, -122.4094},
	{37.7749, -122.4094},
	{37.7749, -122.4194},
}

func createGeofence(t *testing.T, e *testEnv, name, category string) string {
	t.Helper()
	resp := e.mustCall(t, "POST", "/geofences", map[string]any{
		"name":        name,
		"description": "test zone",
		"coordinates": squareCoords,
		"category":    category,
	})
	id, _ := resp["id"].(string)
	if id == "" {
		t.Fatalf("create geofence returned no id: %v", resp)
	}
	return id
}

func createVehicle(t *testing.T, e *testEnv, number string) string {
	t.Helper()
	resp := e.mustCall(t, "POST", "/vehicles", map[string]any{
		"vehicle_number": number,
		"driver_name":    "Test Driver",
		"vehicle_type":   "truck",
		"phone":          "+10000000000",
	})
	id, _ := resp["id"].(string)
	if id == "" {
		t.Fatalf("create vehicle returned no id: %v", resp)
	}
	return id
}

// --- tests ---

func TestHealth(t *testing.T) {
	e := newTestEnv(t, nil)
	status, body := e.call(t, "GET", "/health", nil)
	if status != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("health = %d %v, want 200 ok", status, body)
	}
	e.mustTimeNS(t, body)
}

func TestCreateGeofenceValidation(t *testing.T) {
	e := newTestEnv(t, nil)

	valid := map[string]any{
		"name":        "Zone",
		"coordinates": squareCoords,
		"category":    "delivery_zone",
	}

	tests := []struct {
		name    string
		mutate  func(m map[string]any)
		rawBody string // when set, sent verbatim instead of a mutated map
		wantSub string
	}{
		{"missing name", func(m map[string]any) { m["name"] = "" }, "", "name is required"},
		{"missing category", func(m map[string]any) { m["category"] = "" }, "", "category is required"},
		{"bad category", func(m map[string]any) { m["category"] = "nope" }, "", "category must be one of"},
		{"missing coordinates", func(m map[string]any) { delete(m, "coordinates") }, "", "coordinates is required"},
		{"too few points", func(m map[string]any) {
			m["coordinates"] = [][]float64{{0, 0}, {1, 1}, {0, 0}}
		}, "", "at least 4 points"},
		{"unclosed ring", func(m map[string]any) {
			m["coordinates"] = [][]float64{{0, 0}, {0, 1}, {1, 1}, {1, 0}}
		}, "", "closed polygon"},
		{"latitude out of range", func(m map[string]any) {
			m["coordinates"] = [][]float64{{95, 0}, {0, 1}, {1, 1}, {95, 0}}
		}, "", "latitude"},
		{"invalid json", nil, `{"name":`, "invalid JSON body"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				status int
				body   map[string]any
			)
			if tt.rawBody != "" {
				res, err := http.Post(e.srv.URL+"/geofences", "application/json", strings.NewReader(tt.rawBody))
				if err != nil {
					t.Fatalf("post: %v", err)
				}
				defer res.Body.Close()
				status = res.StatusCode
				if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
					t.Fatalf("decode: %v", err)
				}
			} else {
				m := map[string]any{}
				for k, v := range valid {
					m[k] = v
				}
				tt.mutate(m)
				status, body = e.call(t, "POST", "/geofences", m)
			}
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (%v)", status, body)
			}
			msg, _ := body["error"].(string)
			if !strings.Contains(msg, tt.wantSub) {
				t.Fatalf("error %q does not contain %q", msg, tt.wantSub)
			}
			e.mustTimeNS(t, body)
		})
	}

	// The valid fixture must still succeed.
	status, body := e.call(t, "POST", "/geofences", valid)
	if status != http.StatusOK {
		t.Fatalf("valid geofence = %d %v", status, body)
	}
	e.mustTimeNS(t, body)
}

func TestListGeofencesFilterAndPagination(t *testing.T) {
	e := newTestEnv(t, nil)
	createGeofence(t, e, "Delivery A", "delivery_zone")
	createGeofence(t, e, "Restricted B", "restricted_zone")
	createGeofence(t, e, "Toll C", "toll_zone")

	status, body := e.call(t, "GET", "/geofences", nil)
	if status != 200 {
		t.Fatalf("list = %d", status)
	}
	all := body["geofences"].([]any)
	if len(all) != 3 {
		t.Fatalf("want 3 geofences, got %d", len(all))
	}
	if _, paged := body["total_count"]; paged {
		t.Fatalf("unpaged response must not include total_count: %v", body)
	}
	e.mustTimeNS(t, body)

	_, body = e.call(t, "GET", "/geofences?category=restricted_zone", nil)
	filtered := body["geofences"].([]any)
	if len(filtered) != 1 {
		t.Fatalf("want 1 restricted geofence, got %d", len(filtered))
	}

	_, body = e.call(t, "GET", "/geofences?limit=2", nil)
	if got := len(body["geofences"].([]any)); got != 2 {
		t.Fatalf("limit=2 returned %d", got)
	}
	if tc := body["total_count"].(float64); tc != 3 {
		t.Fatalf("total_count = %v, want 3", tc)
	}

	_, body = e.call(t, "GET", "/geofences?limit=2&offset=2", nil)
	if got := len(body["geofences"].([]any)); got != 1 {
		t.Fatalf("offset=2 returned %d, want 1", got)
	}

	status, body = e.call(t, "GET", "/geofences?limit=abc", nil)
	if status != 400 {
		t.Fatalf("bad limit = %d, want 400", status)
	}
}

func TestVehicleLifecycleAndDuplicate(t *testing.T) {
	e := newTestEnv(t, nil)
	id := createVehicle(t, e, "KA-01-AB-1234")

	status, body := e.call(t, "POST", "/vehicles", map[string]any{
		"vehicle_number": "KA-01-AB-1234",
		"driver_name":    "Someone",
		"vehicle_type":   "car",
		"phone":          "+1",
	})
	if status != http.StatusConflict {
		t.Fatalf("duplicate vehicle = %d, want 409 (%v)", status, body)
	}
	e.mustTimeNS(t, body)

	status, body = e.call(t, "POST", "/vehicles", map[string]any{"vehicle_number": "X"})
	if status != 400 {
		t.Fatalf("missing fields = %d, want 400", status)
	}

	body = e.mustCall(t, "GET", "/vehicles", nil)
	vehicles := body["vehicles"].([]any)
	if len(vehicles) != 1 {
		t.Fatalf("want 1 vehicle, got %d", len(vehicles))
	}
	first := vehicles[0].(map[string]any)
	if first["id"] != id || first["vehicle_number"] != "KA-01-AB-1234" {
		t.Fatalf("unexpected vehicle: %v", first)
	}
	if _, ok := first["created_at"].(string); !ok {
		t.Fatalf("created_at missing: %v", first)
	}
}

// TestLocationEntryExitAlerts covers the core business flow: containment
// detection, entry/exit transitions, violation history, alert-rule matching
// and the payload delivered over WebSocket.
func TestLocationEntryExitAlerts(t *testing.T) {
	e := newTestEnv(t, nil)

	geoID := createGeofence(t, e, "Restricted Airport", "restricted_zone")
	vehID := createVehicle(t, e, "KA-01-AB-1234")

	// Connect a WebSocket client before triggering events.
	w := watchAlerts(t, e.srv.URL)

	// 1. Entry into the zone (restricted zones alert without an explicit rule).
	resp := e.mustCall(t, "POST", "/vehicles/location", map[string]any{
		"vehicle_id": vehID,
		"latitude":   37.7800,
		"longitude":  -122.4150,
		"timestamp":  "2025-01-15T10:35:00Z",
	})
	if resp["location_updated"] != true {
		t.Fatalf("location_updated: %v", resp)
	}
	inside := resp["current_geofences"].([]any)
	if len(inside) != 1 {
		t.Fatalf("want 1 geofence, got %d (%v)", len(inside), resp)
	}
	g := inside[0].(map[string]any)
	if g["geofence_id"] != geoID || g["status"] != "inside" || g["geofence_name"] != "Restricted Airport" {
		t.Fatalf("unexpected geofence status: %v", g)
	}
	e.mustTimeNS(t, resp)

	evt := w.next(t)
	if evt["event_type"] != "entry" {
		t.Fatalf("event_type = %v, want entry", evt["event_type"])
	}
	veh := evt["vehicle"].(map[string]any)
	if veh["vehicle_number"] != "KA-01-AB-1234" || veh["driver_name"] != "Test Driver" {
		t.Fatalf("vehicle ref: %v", veh)
	}
	gf := evt["geofence"].(map[string]any)
	if gf["geofence_id"] != geoID || gf["category"] != "restricted_zone" {
		t.Fatalf("geofence ref: %v", gf)
	}
	loc := evt["location"].(map[string]any)
	if loc["latitude"] != 37.78 || loc["longitude"] != -122.415 {
		t.Fatalf("location ref: %v", loc)
	}
	if id, _ := evt["event_id"].(string); !strings.HasPrefix(id, "evt_") {
		t.Fatalf("event_id: %v", evt["event_id"])
	}
	if _, ok := evt["timestamp"].(string); !ok {
		t.Fatalf("timestamp missing: %v", evt)
	}

	// 2. Staying inside must not produce a duplicate event.
	e.mustCall(t, "POST", "/vehicles/location", map[string]any{
		"vehicle_id": vehID,
		"latitude":   37.7801,
		"longitude":  -122.4151,
		"timestamp":  "2025-01-15T10:36:00Z",
	})
	w.expectNone(t)

	// 3. Exit triggers an exit alert and a second violation.
	resp = e.mustCall(t, "POST", "/vehicles/location", map[string]any{
		"vehicle_id": vehID,
		"latitude":   37.7000,
		"longitude":  -122.4300,
		"timestamp":  "2025-01-15T10:40:00Z",
	})
	if got := len(resp["current_geofences"].([]any)); got != 0 {
		t.Fatalf("want no geofences outside, got %d", got)
	}
	if evt := w.next(t); evt["event_type"] != "exit" {
		t.Fatalf("event_type = %v, want exit", evt["event_type"])
	}

	// 4. History: two records, newest first, with joined names.
	body := e.mustCall(t, "GET", "/violations/history?vehicle_id="+vehID, nil)
	violations := body["violations"].([]any)
	if len(violations) != 2 {
		t.Fatalf("want 2 violations, got %d", len(violations))
	}
	first := violations[0].(map[string]any)
	if first["event_type"] != "exit" || first["vehicle_number"] != "KA-01-AB-1234" ||
		first["geofence_name"] != "Restricted Airport" {
		t.Fatalf("violation record: %v", first)
	}
	if tc := body["total_count"].(float64); tc != 2 {
		t.Fatalf("total_count = %v, want 2", tc)
	}
	e.mustTimeNS(t, body)

	// 5. Current location reflects the latest update.
	body = e.mustCall(t, "GET", "/vehicles/location/"+vehID, nil)
	cur := body["current_location"].(map[string]any)
	if cur["latitude"] != 37.7 || cur["timestamp"] != "2025-01-15T10:40:00Z" {
		t.Fatalf("current_location: %v", cur)
	}
	if body["vehicle_number"] != "KA-01-AB-1234" {
		t.Fatalf("vehicle_number: %v", body)
	}
	if got := len(body["current_geofences"].([]any)); got != 0 {
		t.Fatalf("expected no current geofences, got %d: %v", got, body)
	}
}

// TestAlertRuleMatching verifies that non-restricted zones only alert when a
// matching rule exists, and that rules scoped to another vehicle do not fire.
func TestAlertRuleMatching(t *testing.T) {
	e := newTestEnv(t, nil)

	geoID := createGeofence(t, e, "Delivery Zone", "delivery_zone")
	otherGeoID := createGeofence(t, e, "Toll Gate", "toll_zone")
	vehA := createVehicle(t, e, "AAA-111")
	vehB := createVehicle(t, e, "BBB-222")

	// Rule for vehicle A only, on the delivery zone.
	e.mustCall(t, "POST", "/alerts/configure", map[string]any{
		"geofence_id": geoID,
		"vehicle_id":  vehA,
		"event_type":  "entry",
	})

	w := watchAlerts(t, e.srv.URL)

	// Vehicle B entering the delivery zone: no matching rule → no alert.
	e.mustCall(t, "POST", "/vehicles/location", map[string]any{
		"vehicle_id": vehB, "latitude": 37.78, "longitude": -122.415,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
	w.expectNone(t)

	// Vehicle B entering the toll zone (no rules at all) → no alert.
	e.mustCall(t, "POST", "/vehicles/location", map[string]any{
		"vehicle_id": vehB, "latitude": 37.70, "longitude": -122.50,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
	w.expectNone(t)

	// Vehicle A entering the delivery zone → matching rule fires.
	e.mustCall(t, "POST", "/vehicles/location", map[string]any{
		"vehicle_id": vehA, "latitude": 37.78, "longitude": -122.415,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
	evt := w.next(t)
	if evt["event_type"] != "entry" {
		t.Fatalf("event_type = %v", evt["event_type"])
	}
	if g := evt["geofence"].(map[string]any); g["geofence_id"] != geoID {
		t.Fatalf("geofence = %v", g)
	}
	if v := evt["vehicle"].(map[string]any); v["vehicle_id"] != vehA {
		t.Fatalf("vehicle = %v", v)
	}
	_ = otherGeoID
}

func TestAlertConfigValidation(t *testing.T) {
	e := newTestEnv(t, nil)
	geoID := createGeofence(t, e, "Zone", "customer_area")

	status, body := e.call(t, "POST", "/alerts/configure", map[string]any{
		"geofence_id": "geo_missing", "event_type": "entry",
	})
	if status != 404 {
		t.Fatalf("unknown geofence = %d, want 404 (%v)", status, body)
	}

	vehID := createVehicle(t, e, "CCC-333")
	status, body = e.call(t, "POST", "/alerts/configure", map[string]any{
		"geofence_id": geoID, "vehicle_id": "veh_missing", "event_type": "entry",
	})
	if status != 404 {
		t.Fatalf("unknown vehicle = %d, want 404", status)
	}

	status, body = e.call(t, "POST", "/alerts/configure", map[string]any{
		"geofence_id": geoID, "event_type": "sideways",
	})
	if status != 400 {
		t.Fatalf("bad event_type = %d, want 400", status)
	}

	resp := e.mustCall(t, "POST", "/alerts/configure", map[string]any{
		"geofence_id": geoID, "vehicle_id": vehID, "event_type": "both",
	})
	if resp["status"] != "active" || !strings.HasPrefix(resp["alert_id"].(string), "alert_") {
		t.Fatalf("configure response: %v", resp)
	}
	if resp["vehicle_id"] != vehID {
		t.Fatalf("vehicle_id echo: %v", resp)
	}
	e.mustTimeNS(t, resp)

	// All-vehicle rule echoes an empty vehicle_id.
	resp = e.mustCall(t, "POST", "/alerts/configure", map[string]any{
		"geofence_id": geoID, "event_type": "exit",
	})
	if resp["vehicle_id"] != "" {
		t.Fatalf("all-vehicle rule vehicle_id = %#v, want empty", resp["vehicle_id"])
	}

	// List with filter returns joined names. Rules are ordered by id when
	// created within the same second, so locate the vehicle-scoped rule
	// instead of assuming positional order.
	body = e.mustCall(t, "GET", "/alerts?geofence_id="+geoID, nil)
	alerts := body["alerts"].([]any)
	if len(alerts) != 2 {
		t.Fatalf("want 2 rules, got %d", len(alerts))
	}
	var scoped bool
	for _, a := range alerts {
		rule := a.(map[string]any)
		if rule["geofence_name"] != "Zone" {
			t.Fatalf("joined geofence_name: %v", rule)
		}
		if rule["vehicle_id"] == vehID {
			scoped = true
			if rule["vehicle_number"] != "CCC-333" {
				t.Fatalf("joined vehicle_number: %v", rule)
			}
			if rule["event_type"] != "both" {
				t.Fatalf("event_type: %v", rule)
			}
		}
	}
	if !scoped {
		t.Fatalf("vehicle-scoped rule not found in %v", alerts)
	}
}

func TestViolationsHistoryFilters(t *testing.T) {
	e := newTestEnv(t, nil)
	geoID := createGeofence(t, e, "Zone", "toll_zone")
	vehID := createVehicle(t, e, "DDD-444")

	// Two entries then two exits across two dates.
	for _, ts := range []string{"2025-03-01T10:00:00Z", "2025-03-02T10:00:00Z"} {
		e.mustCall(t, "POST", "/vehicles/location", map[string]any{
			"vehicle_id": vehID, "latitude": 37.78, "longitude": -122.415, "timestamp": ts,
		})
		e.mustCall(t, "POST", "/vehicles/location", map[string]any{
			"vehicle_id": vehID, "latitude": 37.70, "longitude": -122.50,
			"timestamp": ts,
		})
	}

	// Date range covering only the first day.
	body := e.mustCall(t, "GET",
		"/violations/history?start_date=2025-03-01T00:00:00Z&end_date=2025-03-01T23:59:59Z", nil)
	if tc := body["total_count"].(float64); tc != 2 {
		t.Fatalf("date filter total_count = %v, want 2 (%v)", tc, body)
	}

	// Geofence filter.
	body = e.mustCall(t, "GET", "/violations/history?geofence_id="+geoID, nil)
	if tc := body["total_count"].(float64); tc != 4 {
		t.Fatalf("geofence filter total_count = %v, want 4", tc)
	}

	// Pagination.
	body = e.mustCall(t, "GET", "/violations/history?limit=3", nil)
	if got := len(body["violations"].([]any)); got != 3 {
		t.Fatalf("limit=3 returned %d", got)
	}
	body = e.mustCall(t, "GET", "/violations/history?limit=3&offset=3", nil)
	if got := len(body["violations"].([]any)); got != 1 {
		t.Fatalf("offset=3 returned %d, want 1", got)
	}

	// Invalid inputs.
	for _, path := range []string{
		"/violations/history?limit=abc",
		"/violations/history?limit=0",
		"/violations/history?offset=-1",
		"/violations/history?start_date=yesterday",
		"/violations/history?start_date=2025-06-01T00:00:00Z&end_date=2025-01-01T00:00:00Z",
	} {
		status, _ := e.call(t, "GET", path, nil)
		if status != 400 {
			t.Fatalf("%s = %d, want 400", path, status)
		}
	}
}

func TestUnknownVehicleLocation(t *testing.T) {
	e := newTestEnv(t, nil)
	status, body := e.call(t, "GET", "/vehicles/location/veh_nope", nil)
	if status != 404 {
		t.Fatalf("status = %d, want 404", status)
	}
	e.mustTimeNS(t, body)

	status, _ = e.call(t, "POST", "/vehicles/location", map[string]any{
		"vehicle_id": "veh_nope", "latitude": 0, "longitude": 0,
		"timestamp": "2025-01-01T00:00:00Z",
	})
	if status != 404 {
		t.Fatalf("update unknown vehicle = %d, want 404", status)
	}
}

func TestLocationValidation(t *testing.T) {
	e := newTestEnv(t, nil)
	vehID := createVehicle(t, e, "EEE-555")

	cases := []map[string]any{
		{"latitude": 91.0, "longitude": 0.0, "timestamp": "2025-01-01T00:00:00Z"},
		{"latitude": 0.0, "longitude": 181.0, "timestamp": "2025-01-01T00:00:00Z"},
		{"latitude": 0.0, "longitude": 0.0, "timestamp": "not-a-date"},
		{"longitude": 0.0, "timestamp": "2025-01-01T00:00:00Z"},
		{"latitude": 0.0, "timestamp": "2025-01-01T00:00:00Z"},
		{"latitude": 0.0, "longitude": 0.0},
	}
	for i, c := range cases {
		c["vehicle_id"] = vehID
		status, _ := e.call(t, "POST", "/vehicles/location", c)
		if status != 400 {
			t.Fatalf("case %d = %d, want 400 (%v)", i, status, c)
		}
	}
}

func TestRateLimitOnLocationUpdates(t *testing.T) {
	e := newTestEnv(t, map[string]string{"RATE_LIMIT_PER_MIN": "5"})
	vehID := createVehicle(t, e, "FFF-666")

	body := map[string]any{
		"vehicle_id": vehID, "latitude": 1.0, "longitude": 1.0,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	sawTooMany := false
	for i := 0; i < 8; i++ {
		status, resp := e.call(t, "POST", "/vehicles/location", body)
		if status == http.StatusTooManyRequests {
			sawTooMany = true
			e.mustTimeNS(t, resp)
			break
		}
		if status != 200 {
			t.Fatalf("request %d = %d, want 200 or 429", i, status)
		}
	}
	if !sawTooMany {
		t.Fatal("expected a 429 after exceeding RATE_LIMIT_PER_MIN")
	}
}

func TestCORSAndOptions(t *testing.T) {
	e := newTestEnv(t, nil)
	req, _ := http.NewRequest(http.MethodOptions, e.srv.URL+"/geofences", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("OPTIONS = %d, want 204", res.StatusCode)
	}
	if got := res.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("CORS header = %q", got)
	}
}

// --- WebSocket helpers ---

// alertWatcher reads WebSocket frames on a dedicated goroutine so tests can
// wait for messages (or their absence) without applying read deadlines to
// the connection itself — gorilla/websocket treats a read timeout as a
// permanent connection error.
type alertWatcher struct {
	msgs chan map[string]any
	errs chan error
}

func watchAlerts(t *testing.T, srvURL string) *alertWatcher {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(srvURL, "http") + "/ws/alerts"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	w := &alertWatcher{
		msgs: make(chan map[string]any, 16),
		errs: make(chan error, 1),
	}
	go func() {
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				w.errs <- err
				return
			}
			var evt map[string]any
			if err := json.Unmarshal(raw, &evt); err == nil {
				w.msgs <- evt
			}
		}
	}()
	t.Cleanup(func() { _ = conn.Close() })
	return w
}

// next waits up to 5s for an alert message.
func (w *alertWatcher) next(t *testing.T) map[string]any {
	t.Helper()
	select {
	case evt := <-w.msgs:
		return evt
	case err := <-w.errs:
		t.Fatalf("ws read: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for an alert")
	}
	return nil
}

// expectNone asserts that no alert arrives within 300ms.
func (w *alertWatcher) expectNone(t *testing.T) {
	t.Helper()
	select {
	case evt := <-w.msgs:
		t.Fatalf("unexpected alert received: %v", evt)
	case err := <-w.errs:
		t.Fatalf("ws read: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
}
