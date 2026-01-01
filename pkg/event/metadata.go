package event

import (
	"time"

	"github.com/google/uuid"
)

// NewMetadata creates metadata for a new event from scratch.
// Use this when creating an event that doesn't have a direct cause
// (e.g., events from external webhooks).
func NewMetadata(
	eventType string,
	schemaVersion int,
	causationID *uuid.UUID,
	correlationID *uuid.UUID,
	idempotencyKey *string,
	actor *Actor,
	originEventType *string,
) Metadata {
	return Metadata{
		EventID:         uuid.New(),
		EventType:       eventType,
		OccurredAt:      time.Now().UTC(),
		SchemaVersion:   schemaVersion,
		CausationID:     causationID,
		CorrelationID:   correlationID,
		IdempotencyKey:  idempotencyKey,
		Actor:           actor,
		OriginEventType: originEventType,
	}
}

// NewMetadataFromEvent creates metadata for an event that was caused by another event.
// This automatically sets up the causation chain and preserves correlation/actor info.
// This is the most common way to create events in the system - most events are
// reactions to other events.
func NewMetadataFromEvent(causingEvent Event, eventType string, schemaVersion int) Metadata {
	// Preserve the correlation ID if it exists, otherwise use the causing event's ID
	// as the correlation root. This groups all events in a workflow together.
	var correlationID *uuid.UUID
	if causingEvent.Metadata().CorrelationID != nil {
		correlationID = causingEvent.Metadata().CorrelationID
	} else {
		corrID := causingEvent.EventID()
		correlationID = &corrID
	}

	return Metadata{
		EventID:         uuid.New(),
		EventType:       eventType,
		OccurredAt:      time.Now().UTC(),
		SchemaVersion:   schemaVersion,
		CausationID:     func() *uuid.UUID { id := causingEvent.EventID(); return &id }(),
		CorrelationID:   correlationID,
		IdempotencyKey:  causingEvent.Metadata().IdempotencyKey,
		Actor:           causingEvent.Metadata().Actor,
		OriginEventType: func() *string { t := causingEvent.EventType(); return &t }(),
	}
}
