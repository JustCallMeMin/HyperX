package ports

import (
	"context"

	"github.com/google/uuid"
)

type IdentityProvider interface {
	ResolveUserID(ctx context.Context, externalID string, provider string) (uuid.UUID, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (*User, error)
}

type User struct {
	UserID uuid.UUID
	Email  string
}
