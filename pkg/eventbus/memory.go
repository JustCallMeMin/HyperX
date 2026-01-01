package eventbus

import (
	"context"
	"sync"

	"hyperx/pkg/event"
)

// MemoryBus is an in-memory implementation of the event bus.
// This is perfect for testing and development. For production, you'd
// replace this with a message queue implementation (e.g., RabbitMQ, Kafka).
type MemoryBus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler // event type -> list of handlers
}

// NewMemoryBus creates a new in-memory event bus.
// All events are processed synchronously in the same process.
func NewMemoryBus() *MemoryBus {
	return &MemoryBus{
		handlers: make(map[string][]Handler),
	}
}

// Publish sends an event to all subscribed handlers.
// Handlers are called synchronously - if one fails, we stop processing.
// In production, you'd want async processing with retries and dead letter queues.
func (b *MemoryBus) Publish(ctx context.Context, evt event.Event) error {
	// Get handlers for this event type (read lock for concurrent reads)
	b.mu.RLock()
	handlers := b.handlers[evt.EventType()]
	b.mu.RUnlock()

	// Call each handler in sequence
	// Note: In a real system, you'd want to handle errors better
	// (e.g., continue processing other handlers even if one fails)
	for _, handler := range handlers {
		if err := handler(ctx, evt); err != nil {
			return err
		}
	}

	return nil
}

// Subscribe registers a handler for a specific event type.
// Multiple handlers can subscribe to the same event type - they'll all be called.
func (b *MemoryBus) Subscribe(ctx context.Context, eventType string, handler Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[eventType] = append(b.handlers[eventType], handler)
	return nil
}

// Unsubscribe removes a handler from the subscription list.
// This is rarely used in practice, but useful for cleanup in tests.
func (b *MemoryBus) Unsubscribe(ctx context.Context, eventType string, handler Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	handlers := b.handlers[eventType]
	for i, h := range handlers {
		// Compare function pointers to find the handler to remove
		if &h == &handler {
			b.handlers[eventType] = append(handlers[:i], handlers[i+1:]...)
			return nil
		}
	}

	return nil
}
