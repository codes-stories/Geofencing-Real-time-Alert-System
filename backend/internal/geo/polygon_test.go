package geo

import (
	"strings"
	"testing"
)

// unitSquare is a closed 1x1 degree box around (0.5, 0.5) in (lat, lng).
var unitSquare = [][]float64{
	{0, 0},
	{0, 1},
	{1, 1},
	{1, 0},
	{0, 0},
}

func TestContains(t *testing.T) {
	tests := []struct {
		name string
		ring [][]float64
		lat  float64
		lng  float64
		want bool
	}{
		{"center of square", unitSquare, 0.5, 0.5, true},
		{"outside left", unitSquare, 0.5, -0.5, false},
		{"outside above", unitSquare, 1.5, 0.5, false},
		{"outside corner", unitSquare, -0.1, 1.1, false},
		{"exact vertex", unitSquare, 0, 0, true},
		{"on edge", unitSquare, 0, 0.5, true},
		{"open ring still works", unitSquare[:4], 0.5, 0.5, true},
		{"open ring outside", unitSquare[:4], 2, 2, false},
		{
			"concave L-shape inside arm",
			[][]float64{{0, 0}, {0, 3}, {1, 3}, {1, 1}, {2, 1}, {2, 0}, {0, 0}},
			0.5, 0.5, true,
		},
		{
			"concave L-shape in notch (outside)",
			[][]float64{{0, 0}, {0, 3}, {1, 3}, {1, 1}, {2, 1}, {2, 0}, {0, 0}},
			1.5, 2.5, false,
		},
		{
			"real world SF delivery zone",
			[][]float64{
				{37.7749, -122.4194},
				{37.7849, -122.4194},
				{37.7849, -122.4094},
				{37.7749, -122.4094},
				{37.7749, -122.4194},
			},
			37.7800, -122.4150, true,
		},
		{
			"real world SF outside zone",
			[][]float64{
				{37.7749, -122.4194},
				{37.7849, -122.4194},
				{37.7849, -122.4094},
				{37.7749, -122.4094},
				{37.7749, -122.4194},
			},
			37.7700, -122.4150, false,
		},
		{"degenerate ring", [][]float64{{0, 0}, {1, 1}}, 0.5, 0.5, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Contains(tt.ring, tt.lat, tt.lng); got != tt.want {
				t.Errorf("Contains(%v, %v, %v) = %v, want %v", tt.ring, tt.lat, tt.lng, got, tt.want)
			}
		})
	}
}

func TestValidatePolygon(t *testing.T) {
	valid := [][]float64{
		{37.7749, -122.4194},
		{37.7849, -122.4194},
		{37.7849, -122.4094},
		{37.7749, -122.4094},
		{37.7749, -122.4194},
	}
	if err := ValidatePolygon(valid); err != nil {
		t.Fatalf("expected valid polygon, got error: %v", err)
	}

	tests := []struct {
		name    string
		coords  [][]float64
		wantSub string
	}{
		{
			"too few points",
			[][]float64{{0, 0}, {1, 1}, {0, 0}},
			"at least 4 points",
		},
		{
			"not closed",
			[][]float64{{0, 0}, {0, 1}, {1, 1}, {1, 0}},
			"closed polygon",
		},
		{
			"latitude out of range",
			[][]float64{{95, 0}, {0, 1}, {1, 1}, {95, 0}},
			"latitude",
		},
		{
			"longitude out of range",
			[][]float64{{0, -190}, {0, 1}, {1, 1}, {0, -190}},
			"longitude",
		},
		{
			"vertex with wrong arity",
			[][]float64{{0, 0, 9}, {0, 1}, {1, 1}, {0, 0, 9}},
			"[latitude, longitude] pair",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePolygon(tt.coords)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantSub)
			}
		})
	}
}

func TestBounds(t *testing.T) {
	minLat, minLng, maxLat, maxLng := Bounds(unitSquare)
	if minLat != 0 || maxLat != 1 || minLng != 0 || maxLng != 1 {
		t.Errorf("Bounds = (%v,%v,%v,%v), want (0,0,1,1)", minLat, minLng, maxLat, maxLng)
	}
}

func TestPolygonGeoJSON(t *testing.T) {
	// Open ring: the closing vertex must be appended automatically, and
	// vertices must be converted from [lat, lng] to GeoJSON [lng, lat].
	got := PolygonGeoJSON(unitSquare[:4])
	want := `{"type":"Polygon","coordinates":[[[0.0000000000,0.0000000000],[1.0000000000,0.0000000000],[1.0000000000,1.0000000000],[0.0000000000,1.0000000000],[0.0000000000,0.0000000000]]]}`
	if got != want {
		t.Errorf("PolygonGeoJSON = %s\nwant %s", got, want)
	}

	// Closed ring is emitted verbatim (no duplicate closing vertex).
	if got, want := PolygonGeoJSON(unitSquare), `{"type":"Polygon","coordinates":[[[0.0000000000,0.0000000000],[1.0000000000,0.0000000000],[1.0000000000,1.0000000000],[0.0000000000,1.0000000000],[0.0000000000,0.0000000000]]]}`; got != want {
		t.Errorf("PolygonGeoJSON(closed) = %s\nwant %s", got, want)
	}
}
