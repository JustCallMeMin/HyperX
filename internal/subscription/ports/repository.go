package ports

import (
	"context"

	"github.com/google/uuid"
)

type SubscriptionRepository interface {
	Save(ctx context.Context, subscription *SubscriptionProjection) error
	FindByID(ctx context.Context, subscriptionID uuid.UUID) (*SubscriptionProjection, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]*SubscriptionProjection, error)
}

type SubscriptionProjection struct {
	SubscriptionID uuid.UUID
	UserID         uuid.UUID
	CreatorID      uuid.UUID
	State          string
	Version        int64
}
