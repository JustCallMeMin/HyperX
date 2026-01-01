package adapters

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"hyperx/internal/subscription/ports"
)

type InMemorySubscriptionRepository struct {
	mu           sync.RWMutex
	subscriptions map[uuid.UUID]*ports.SubscriptionProjection
}

func NewInMemorySubscriptionRepository() ports.SubscriptionRepository {
	return &InMemorySubscriptionRepository{
		subscriptions: make(map[uuid.UUID]*ports.SubscriptionProjection),
	}
}

func (r *InMemorySubscriptionRepository) Save(ctx context.Context, subscription *ports.SubscriptionProjection) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.subscriptions[subscription.SubscriptionID] = subscription
	return nil
}

func (r *InMemorySubscriptionRepository) FindByID(ctx context.Context, subscriptionID uuid.UUID) (*ports.SubscriptionProjection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sub, ok := r.subscriptions[subscriptionID]
	if !ok {
		return nil, nil
	}

	return sub, nil
}

func (r *InMemorySubscriptionRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*ports.SubscriptionProjection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*ports.SubscriptionProjection
	for _, sub := range r.subscriptions {
		if sub.UserID == userID {
			results = append(results, sub)
		}
	}

	return results, nil
}
