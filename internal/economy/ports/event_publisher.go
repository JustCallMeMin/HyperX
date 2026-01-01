package ports

import (
	"context"

	"hyperx/pkg/event"
)

type EventPublisher interface {
	Publish(ctx context.Context, evt event.Event) error
}
