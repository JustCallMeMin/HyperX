package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"hyperx/internal/subscription/events"
	"hyperx/internal/subscription/ports"
	"hyperx/pkg/clock"
)

// HandlePaymentSucceeded processes a PaymentSucceeded event and grants access.
// This is part of the reference slice: PaymentSucceeded → AccessGranted.
// When a payment succeeds, we immediately grant access to the content.
type HandlePaymentSucceeded struct {
	eventPublisher ports.EventPublisher
	clock          clock.Clock
}

func NewHandlePaymentSucceeded(
	eventPublisher ports.EventPublisher,
	clock clock.Clock,
) *HandlePaymentSucceeded {
	return &HandlePaymentSucceeded{
		eventPublisher: eventPublisher,
		clock:          clock,
	}
}

// Execute handles the payment succeeded event.
// This is where the business logic lives: when payment succeeds, grant access.
// We use the clock abstraction so we can test time-dependent logic easily.
func (h *HandlePaymentSucceeded) Execute(ctx context.Context, evt *events.PaymentSucceeded) error {
	// Validate the event payload first - fail fast if data is bad
	if err := evt.Payload().Validate(); err != nil {
		return err
	}

	subscriptionID := evt.Aggregate().AggregateID()

	// Calculate when access should expire (typically 1 month from now)
	accessExpiresAt := h.calculateAccessExpiresAt()

	// Create the AccessGranted event, linking it to the payment event via causation
	// This creates an audit trail: we can see that access was granted because payment succeeded
	accessGranted := events.NewAccessGranted(
		evt,
		subscriptionID,
		uuid.Nil, // TODO: resolve userID from subscription
		uuid.Nil, // TODO: resolve creatorID from subscription
		accessExpiresAt,
	)

	// Publish the event - this will trigger downstream handlers
	return h.eventPublisher.Publish(ctx, accessGranted)
}

// calculateAccessExpiresAt determines when the access should expire.
// Currently hardcoded to 1 month, but this could be based on subscription tier,
// billing period, or other business rules.
func (h *HandlePaymentSucceeded) calculateAccessExpiresAt() time.Time {
	now := h.clock.Now()
	return now.AddDate(0, 1, 0) // 1 month from now
}
