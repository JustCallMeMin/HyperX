package event

import "errors"

var (
	ErrMissingEventID       = errors.New("event_id is required")
	ErrMissingEventType     = errors.New("event_type is required")
	ErrMissingOccurredAt    = errors.New("occurred_at_utc is required")
	ErrInvalidSchemaVersion = errors.New("schema_version must be >= 1")
	ErrInvalidCausation     = errors.New("causation_id must reference a valid event")
	ErrEventChainTooDeep    = errors.New("event chain depth exceeds maximum allowed (3)")
	ErrEventImmutable       = errors.New("events are immutable and cannot be modified")
)
