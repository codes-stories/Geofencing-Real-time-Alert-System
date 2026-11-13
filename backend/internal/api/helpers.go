package api

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// parseTimestamp accepts RFC3339 (with or without nanoseconds) and common
// ISO 8601 variants without a zone designator (treated as UTC).
func parseTimestamp(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("timestamp is required")
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.000Z07:00",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC().Truncate(time.Second), nil
		}
	}
	return time.Time{}, fmt.Errorf("timestamp must be an ISO 8601 / RFC3339 value, e.g. 2025-01-15T10:35:00Z")
}

// parseLimit clamps the requested page size into [1, 500], defaulting to 50.
func parseLimit(raw string) (int, error) {
	if raw == "" {
		return 50, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("limit must be an integer")
	}
	if n < 1 {
		return 0, fmt.Errorf("limit must be between 1 and 500")
	}
	if n > 500 {
		return 500, nil
	}
	return n, nil
}

// parseOffset returns the page offset, defaulting to 0.
func parseOffset(raw string) (int, error) {
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("offset must be a non-negative integer")
	}
	return n, nil
}

// parseOptionalTime parses an optional RFC3339 query parameter.
func parseOptionalTime(name, raw string) (*time.Time, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	t, err := parseTimestamp(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be an ISO 8601 / RFC3339 value", name)
	}
	return &t, nil
}

// paginate applies the optional `limit`/`offset` query parameters to a list
// response. When neither parameter is present the full list is returned
// unchanged (total_count is then omitted by callers) so default responses
// keep the exact shape defined by the API contract.
func paginate[T any](q url.Values, items []T) (page []T, total int, requested bool, err error) {
	limitRaw := strings.TrimSpace(q.Get("limit"))
	offsetRaw := strings.TrimSpace(q.Get("offset"))
	total = len(items)
	if limitRaw == "" && offsetRaw == "" {
		return items, total, false, nil
	}

	limit, err := parseLimit(limitRaw)
	if err != nil {
		return nil, 0, true, err
	}
	offset, err := parseOffset(offsetRaw)
	if err != nil {
		return nil, 0, true, err
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return items[offset:end], total, true, nil
}
