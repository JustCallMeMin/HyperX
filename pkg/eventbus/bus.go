// Package eventbus provides the abstraction for event-driven communication.
// Contexts communicate by publishing and subscribing to events, not by
// calling each other directly. This keeps boundaries clean and enables
// easy testing and future distributed deployment.
package eventbus

import (
	"context"

	"hyperx/pkg/event"
)

// Bus is the interface for publishing and subscribing to events.
// This abstraction allows us to swap implementations (in-memory for tests,
// message queue for production) without changing business logic.
type Bus interface {
	Publish(ctx context.Context, evt event.Event) error
	Subscribe(ctx context.Context, eventType string, handler Handler) error
	Unsubscribe(ctx context.Context, eventType string, handler Handler) error
}

// Handler is a function that processes an event.
// Handlers should be idempotent - they may be called multiple times
// for the same event (e.g., during replay or retries).
type Handler func(ctx context.Context, evt event.Event) error
