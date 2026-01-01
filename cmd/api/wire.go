package main

import (
	"context"

	economy_adapters "hyperx/internal/economy/adapters"
	economy_app "hyperx/internal/economy/application"
	subscription_adapters "hyperx/internal/subscription/adapters"
	subscription_app "hyperx/internal/subscription/application"
	subscription_events "hyperx/internal/subscription/events"
	"hyperx/pkg/clock"
	"hyperx/pkg/event"
	"hyperx/pkg/eventbus"

	"github.com/google/uuid"
)

// wireReferenceSlice sets up the PaymentSucceeded → Ledger → AccessGranted flow.
// This is the reference implementation that demonstrates how contexts communicate
// via events. This is what all future flows should look like.
//
// The flow:
// 1. PaymentSucceeded event is published
// 2. Subscription handler grants access (AccessGranted event)
// 3. Economy handler creates ledger entry (LedgerCredit event)
//
// Note: In a real system, you'd use a dependency injection framework (like wire or fx),
// but this manual wiring makes the dependencies explicit and easy to understand.
func wireReferenceSlice() (*ReferenceSlice, error) {
	// Create the event bus - this is how contexts communicate
	bus := eventbus.NewMemoryBus()
	utcClock := clock.NewUTCClock()

	// Wire up the subscription context
	// When a payment succeeds, we need to grant access
	subscriptionEventPublisher := subscription_adapters.NewEventPublisherAdapter(bus)
	subscriptionHandler := subscription_app.NewHandlePaymentSucceeded(
		subscriptionEventPublisher,
		utcClock,
	)

	// Wire up the economy context
	// When a payment succeeds, we need to record it in the ledger
	economyEventPublisher := economy_adapters.NewEventPublisherAdapter(bus)
	economyHandler := economy_app.NewSettlePayment(economyEventPublisher)

	// Subscribe to PaymentSucceeded events and trigger both handlers
	// This is where the orchestration happens - both contexts react to the same event
	bus.Subscribe(context.Background(), "PaymentSucceeded", func(ctx context.Context, evt event.Event) error {
		// Double-check the event type (defensive programming)
		if evt.EventType() != "PaymentSucceeded" {
			return nil
		}

		// Type assert to get the concrete type for the subscription handler
		// (it needs the specific type, while economy handler uses the interface)
		paymentSucceeded, ok := evt.(*subscription_events.PaymentSucceeded)
		if !ok {
			return nil
		}

		// First, grant access (subscription context)
		if err := subscriptionHandler.Execute(ctx, paymentSucceeded); err != nil {
			return err
		}

		// Then, record in ledger (economy context)
		// TODO: Resolve creatorID from subscription instead of generating a new UUID
		creatorID := uuid.New()
		return economyHandler.Execute(ctx, evt, creatorID)
	})

	return &ReferenceSlice{
		bus:                 bus,
		subscriptionHandler: subscriptionHandler,
		economyHandler:      economyHandler,
	}, nil
}

// ReferenceSlice holds the wired components for the reference flow.
// This is useful for testing and demonstrates the complete setup.
type ReferenceSlice struct {
	bus                 eventbus.Bus
	subscriptionHandler *subscription_app.HandlePaymentSucceeded
	economyHandler      *economy_app.SettlePayment
}
