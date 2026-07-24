package ingest

import "time"

// StaleAfter is the MVP staleness window (SCR-4): a listing not re-seen within
// this period is considered removed/expired at the source and marked stale.
const StaleAfter = 48 * time.Hour

// IsStale reports whether a listing last seen at lastSeen is stale as of now,
// given maxAge. It is the single, clock-injectable staleness rule (SCR-4).
func IsStale(lastSeen, now time.Time, maxAge time.Duration) bool {
	if lastSeen.IsZero() {
		return false
	}
	return now.Sub(lastSeen) >= maxAge
}
