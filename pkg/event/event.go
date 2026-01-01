// Package event defines the canonical event model for the entire system.
// All events in the system must implement the Event interface, ensuring
// consistency, replayability, and proper causation tracking.
package event

import (
	"time"

	"github.com/google/uuid"
)

// Event represents an immutable fact that has occurred in the system.
// Events are the single source of truth - all state is derived from events.
// This interface ensures every event has the required metadata for:
// - Replay safety (event_id, occurred_at, schema_version)
// - Causation tracking (causation_id, correlation_id)
// - Idempotency (idempotency_key)
type Event interface {
	EventID() uuid.UUID
	EventType() string
	OccurredAt() time.Time
	SchemaVersion() int
	Metadata() Metadata
	Aggregate() Aggregate
	Payload() Payload
}

// Aggregate represents the entity that this event relates to.
// This helps with event sourcing - we can rebuild state by replaying
// all events for a specific aggregate.
type Aggregate interface {
	AggregateID() uuid.UUID
	AggregateType() string
}

// Payload contains the business-specific data for an event.
// Each event type has its own payload structure, but they all must
// be able to validate themselves.
type Payload interface {
	Validate() error
}

// Metadata contains all the system-level information about an event.
// This is separate from the business payload to keep concerns separated.
type Metadata struct {
	EventID         uuid.UUID  // Globally unique identifier for this event
	EventType       string     // The type of event (e.g., "PaymentSucceeded")
	OccurredAt      time.Time  // When the event actually happened (UTC)
	SchemaVersion   int        // Version of the payload schema (for evolution)
	CausationID     *uuid.UUID // The event that caused this one (for tracing)
	CorrelationID   *uuid.UUID // Groups related events together (for workflows)
	IdempotencyKey  *string    // Prevents duplicate processing (from external systems)
	Actor           *Actor     // Who/what triggered this event
	OriginEventType *string    // The original event type that started this chain
}

// Actor represents who or what initiated the event.
// This is important for audit trails and authorization checks.
type Actor struct {
	ActorType       string     // "system", "user", "admin", etc.
	ActorIdentityID *uuid.UUID // The specific identity (if applicable)
}

// Validate ensures all required metadata fields are present and valid.
// This is called before persisting any event to catch errors early.
func (m Metadata) Validate() error {
	if m.EventID == uuid.Nil {
		return ErrMissingEventID
	}
	if m.EventType == "" {
		return ErrMissingEventType
	}
	if m.OccurredAt.IsZero() {
		return ErrMissingOccurredAt
	}
	if m.SchemaVersion < 1 {
		return ErrInvalidSchemaVersion
	}
	return nil
}
