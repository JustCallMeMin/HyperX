package adapters

import (
	"context"

	"hyperx/internal/economy/ports"
	"hyperx/pkg/event"
	"hyperx/pkg/eventbus"
)

type EventPublisherAdapter struct {
	bus eventbus.Bus
}

func NewEventPublisherAdapter(bus eventbus.Bus) ports.EventPublisher {
	return &EventPublisherAdapter{bus: bus}
}

func (a *EventPublisherAdapter) Publish(ctx context.Context, evt event.Event) error {
	return a.bus.Publish(ctx, evt)
}
