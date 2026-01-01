package ports

import (
	"context"

	"github.com/google/uuid"
)

type EntitlementReader interface {
	HasAccess(ctx context.Context, userID uuid.UUID, creatorID uuid.UUID) (bool, error)
	GetAccessExpiresAt(ctx context.Context, userID uuid.UUID, creatorID uuid.UUID) (int64, error)
}
