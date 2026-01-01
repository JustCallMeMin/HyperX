package application

import (
	"context"

	"github.com/google/uuid"
	economy_events "hyperx/internal/economy/events"
	"hyperx/internal/economy/ports"
	"hyperx/pkg/event"
)

// SettlePayment processes a payment event and creates a ledger entry.
// This is part of the reference slice: PaymentSucceeded → LedgerCredit.
// The economy context is responsible for all financial record-keeping.
// Note: This handler works with the generic event.Event interface, not
// the specific PaymentSucceeded type. This keeps boundaries clean - economy
// doesn't need to import subscription events, it just needs to know about payments.
type SettlePayment struct {
	eventPublisher ports.EventPublisher
}

func NewSettlePayment(eventPublisher ports.EventPublisher) *SettlePayment {
	return &SettlePayment{
		eventPublisher: eventPublisher,
	}
}

// Execute creates a ledger credit entry when a payment succeeds.
// This maintains the financial ledger - every payment must be recorded.
// We use the PaymentPayload interface so we don't need to import
// subscription events directly (respecting context boundaries).
func (s *SettlePayment) Execute(ctx context.Context, paymentEvent event.Event, creatorID uuid.UUID) error {
	// Only process PaymentSucceeded events - ignore others
	if paymentEvent.EventType() != "PaymentSucceeded" {
		return ErrInvalidPaymentPayload
	}

	// Extract payment data using the interface - this is how we cross context boundaries
	// without importing each other's event types
	payload, ok := paymentEvent.Payload().(event.PaymentPayload)
	if !ok {
		return ErrInvalidPaymentPayload
	}

	// Validate before we create the ledger entry
	if err := payload.Validate(); err != nil {
		return err
	}

	// Create the ledger credit event, linking it to the payment via causation
	// This creates an immutable record: "Payment X caused Ledger Credit Y"
	ledgerCredit := economy_events.NewLedgerCredit(
		paymentEvent,
		creatorID,
		payload.GetAmountCents(),
		payload.GetCurrency(),
	)

	// Publish the event - this will update the ledger projection
	return s.eventPublisher.Publish(ctx, ledgerCredit)
}
