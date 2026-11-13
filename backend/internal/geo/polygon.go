// Package geo implements geospatial helpers for the geofencing API,
// notably polygon validation and point-in-polygon tests.
package geo

import (
	"fmt"
	"math"
	"strings"
)

// coordinateEpsilon is the tolerance used when checking that the first and
// last vertices of a ring are identical.
const coordinateEpsilon = 1e-9

// ValidatePolygon checks that coords form a valid geofence boundary:
//   - every vertex is a [latitude, longitude] pair
//   - latitude is within [-90, 90] and longitude within [-180, 180]
//   - the ring is closed (first vertex == last vertex)
//   - there are at least 4 vertices (3 unique + 1 closing point)
func ValidatePolygon(coords [][]float64) error {
	if len(coords) < 4 {
		return fmt.Errorf("coordinates must contain at least 4 points (3 unique points plus a closing point), got %d", len(coords))
	}
	for i, p := range coords {
		if len(p) != 2 {
			return fmt.Errorf("coordinates[%d] must be a [latitude, longitude] pair", i)
		}
		lat, lng := p[0], p[1]
		if math.IsNaN(lat) || math.IsInf(lat, 0) || math.IsNaN(lng) || math.IsInf(lng, 0) {
			return fmt.Errorf("coordinates[%d] contains a non-numeric value", i)
		}
		if lat < -90 || lat > 90 {
			return fmt.Errorf("coordinates[%d] latitude %.6f must be between -90 and 90", i, lat)
		}
		if lng < -180 || lng > 180 {
			return fmt.Errorf("coordinates[%d] longitude %.6f must be between -180 and 180", i, lng)
		}
	}
	first, last := coords[0], coords[len(coords)-1]
	if math.Abs(first[0]-last[0]) > coordinateEpsilon || math.Abs(first[1]-last[1]) > coordinateEpsilon {
		return fmt.Errorf("coordinates must form a closed polygon: first point [%v] and last point [%v] must be identical", first, last)
	}
	return nil
}

// Contains reports whether the point (lat, lng) lies inside the polygon ring
// using the ray casting algorithm. The ring may be open or closed (the last
// vertex optionally repeating the first). Points exactly on the boundary are
// treated as inside.
func Contains(ring [][]float64, lat, lng float64) bool {
	n := len(ring)
	if n < 3 {
		return false
	}
	// Drop a duplicated closing vertex so each edge is visited exactly once.
	if n > 3 && samePoint(ring[0], ring[n-1]) {
		n--
	}

	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		yi, xi := ring[i][0], ring[i][1]
		yj, xj := ring[j][0], ring[j][1]

		// Edge crosses the horizontal ray to the right of (lng, lat)?
		if (yi > lat) != (yj > lat) {
			xIntersect := (xj-xi)*(lat-yi)/(yj-yi) + xi
			if lng < xIntersect {
				inside = !inside
			}
		}
		// Vertex-on-ray / horizontal-edge cases: count when the vertex is the
		// upper endpoint of the edge, which avoids double counting.
		if yi == lat && xi == lng {
			return true
		}
		j = i
	}
	return inside
}

// Bounds returns the [minLat, minLng, maxLat, maxLng] box enclosing the ring.
func Bounds(ring [][]float64) (minLat, minLng, maxLat, maxLng float64) {
	if len(ring) == 0 {
		return 0, 0, 0, 0
	}
	minLat, maxLat = ring[0][0], ring[0][0]
	minLng, maxLng = ring[0][1], ring[0][1]
	for _, p := range ring[1:] {
		if p[0] < minLat {
			minLat = p[0]
		}
		if p[0] > maxLat {
			maxLat = p[0]
		}
		if p[1] < minLng {
			minLng = p[1]
		}
		if p[1] > maxLng {
			maxLng = p[1]
		}
	}
	return minLat, minLng, maxLat, maxLng
}

// PolygonGeoJSON renders the ring as a GeoJSON Polygon geometry string using
// [longitude, latitude] ordering (RFC 7946), suitable for PostGIS. The ring
// is closed automatically when the last vertex does not repeat the first.
func PolygonGeoJSON(ring [][]float64) string {
	if len(ring) == 0 {
		return `{"type":"Polygon","coordinates":[]}`
	}
	closed := ring
	if !samePoint(ring[0], ring[len(ring)-1]) {
		closed = make([][]float64, 0, len(ring)+1)
		closed = append(closed, ring...)
		closed = append(closed, ring[0])
	}
	var b strings.Builder
	b.WriteString(`{"type":"Polygon","coordinates":[[`)
	for i, p := range closed {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "[%.10f,%.10f]", p[1], p[0])
	}
	b.WriteString(`]]}`)
	return b.String()
}

func samePoint(a, b []float64) bool {
	return math.Abs(a[0]-b[0]) <= coordinateEpsilon && math.Abs(a[1]-b[1]) <= coordinateEpsilon
}
