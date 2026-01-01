package events

import (
	"time"

	"github.com/google/uuid"
	"hyperx/pkg/event"
)

type AccessGranted struct {
	metadata  event.Metadata
	aggregate SubscriptionAggregate
	payload  AccessGrantedPayload
}

func NewAccessGranted(
	causingEvent event.Event,
	subscriptionID uuid.UUID,
	userID uuid.UUID,
	creatorID uuid.UUID,
	expiresAt time.Time,
) *AccessGranted {
	metadata := event.NewMetadataFromEvent(causingEvent, "AccessGranted", 1)

	return &AccessGranted{
		metadata: metadata,
		aggregate: SubscriptionAggregate{
			SubscriptionID: subscriptionID,
		},
		payload: AccessGrantedPayload{
			UserID:    userID,
			CreatorID: creatorID,
			ExpiresAt: expiresAt,
		},
	}
}

func (e *AccessGranted) EventID() uuid.UUID {
	return e.metadata.EventID
}

func (e *AccessGranted) EventType() string {
	return e.metadata.EventType
}

func (e *AccessGranted) OccurredAt() time.Time {
	return e.metadata.OccurredAt
}

func (e *AccessGranted) SchemaVersion() int {
	return e.metadata.SchemaVersion
}

func (e *AccessGranted) Metadata() event.Metadata {
	return e.metadata
}

func (e *AccessGranted) Aggregate() event.Aggregate {
	return e.aggregate
}

func (e *AccessGranted) Payload() event.Payload {
	return e.payload
}

type SubscriptionAggregate struct {
	SubscriptionID uuid.UUID
}

func (a SubscriptionAggregate) AggregateID() uuid.UUID {
	return a.SubscriptionID
}

func (a SubscriptionAggregate) AggregateType() string {
	return "subscription"
}

type AccessGrantedPayload struct {
	UserID    uuid.UUID
	CreatorID uuid.UUID
	ExpiresAt time.Time
}

func (p AccessGrantedPayload) Validate() error {
	if p.UserID == uuid.Nil {
		return event.ErrMissingEventID
	}
	if p.CreatorID == uuid.Nil {
		return event.ErrMissingEventID
	}
	if p.ExpiresAt.IsZero() {
		return event.ErrMissingOccurredAt
	}
	return nil
}
