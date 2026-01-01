// Package clock provides time abstraction for testability.
// Instead of calling time.Now() directly, we use this interface so we can
// control time in tests (e.g., freeze time, fast-forward, etc.).
package clock

import "time"

// Clock abstracts time operations.
// This makes it easy to test time-dependent logic without waiting for real time.
type Clock interface {
	Now() time.Time
}

// UTCClock is the production implementation that returns real UTC time.
type UTCClock struct{}

func (c UTCClock) Now() time.Time {
	return time.Now().UTC()
}

// NewUTCClock creates a new UTC clock.
// This is what you'd use in production code.
func NewUTCClock() Clock {
	return UTCClock{}
}
