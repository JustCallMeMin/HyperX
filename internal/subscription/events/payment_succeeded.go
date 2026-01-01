package events

import (
	"time"

	"github.com/google/uuid"
	"hyperx/pkg/event"
)

// PaymentSucceeded represents a successful payment from a payment provider (e.g., Stripe).
// This is a business event - it represents a fact that happened in the real world.
// It's typically created when we receive a webhook from the payment provider.
type PaymentSucceeded struct {
	metadata  event.Metadata
	aggregate PaymentAggregate
	payload  PaymentSucceededPayload
}

// NewPaymentSucceeded creates a new PaymentSucceeded event.
// This is usually called by the webhook handler when we receive confirmation
// from the payment provider that a payment was successful.
func NewPaymentSucceeded(
	subscriptionID uuid.UUID,
	provider string,
	providerEventID string,
	amountCents int64,
	currency string,
	causationID *uuid.UUID,
	idempotencyKey string,
) *PaymentSucceeded {
	// Payment events always come from the system (via webhook), never from users
	actor := &event.Actor{
		ActorType: "system",
	}

	metadata := event.NewMetadata(
		"PaymentSucceeded",
		1,
		causationID,
		nil,
		&idempotencyKey,
		actor,
		nil,
	)

	return &PaymentSucceeded{
		metadata: metadata,
		aggregate: PaymentAggregate{
			SubscriptionID: subscriptionID,
		},
		payload: PaymentSucceededPayload{
			Provider:        provider,
			ProviderEventID: providerEventID,
			AmountCents:     amountCents,
			Currency:        currency,
		},
	}
}

func (e *PaymentSucceeded) EventID() uuid.UUID {
	return e.metadata.EventID
}

func (e *PaymentSucceeded) EventType() string {
	return e.metadata.EventType
}

func (e *PaymentSucceeded) OccurredAt() time.Time {
	return e.metadata.OccurredAt
}

func (e *PaymentSucceeded) SchemaVersion() int {
	return e.metadata.SchemaVersion
}

func (e *PaymentSucceeded) Metadata() event.Metadata {
	return e.metadata
}

func (e *PaymentSucceeded) Aggregate() event.Aggregate {
	return e.aggregate
}

func (e *PaymentSucceeded) Payload() event.Payload {
	return e.payload
}

// PaymentAggregate identifies which subscription this payment is for.
type PaymentAggregate struct {
	SubscriptionID uuid.UUID
}

func (a PaymentAggregate) AggregateID() uuid.UUID {
	return a.SubscriptionID
}

func (a PaymentAggregate) AggregateType() string {
	return "subscription"
}

// PaymentSucceededPayload contains the business data about the payment.
// This implements event.PaymentPayload so other contexts (like economy)
// can extract payment information without importing subscription events.
type PaymentSucceededPayload struct {
	Provider        string // e.g., "stripe"
	ProviderEventID string // The event ID from the payment provider (for idempotency)
	AmountCents     int64  // Amount in cents (to avoid floating point issues)
	Currency        string // e.g., "usd"
}

func (p PaymentSucceededPayload) GetAmountCents() int64 {
	return p.AmountCents
}

func (p PaymentSucceededPayload) GetCurrency() string {
	return p.Currency
}

func (p PaymentSucceededPayload) GetProvider() string {
	return p.Provider
}

func (p PaymentSucceededPayload) GetProviderEventID() string {
	return p.ProviderEventID
}

// Validate ensures the payment data is valid before we process it.
// We check this early to fail fast if the webhook data is malformed.
func (p PaymentSucceededPayload) Validate() error {
	if p.Provider == "" {
		return event.ErrMissingEventType
	}
	if p.ProviderEventID == "" {
		return event.ErrMissingEventID
	}
	if p.AmountCents <= 0 {
		return event.ErrInvalidSchemaVersion
	}
	if p.Currency == "" {
		return event.ErrMissingEventType
	}
	return nil
}
