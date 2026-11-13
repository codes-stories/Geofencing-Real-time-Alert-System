package store

import "time"

// truncateSecond normalises timestamps to whole seconds so API responses
// render cleanly as RFC3339 (e.g. 2025-01-15T10:35:00Z).
func truncateSecond(t time.Time) time.Time { return t.UTC().Truncate(time.Second) }
