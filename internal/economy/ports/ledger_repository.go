package ports

import (
	"context"

	"github.com/google/uuid"
)

type LedgerRepository interface {
	AppendEntry(ctx context.Context, entry *LedgerEntry) error
	FindByCreatorID(ctx context.Context, creatorID uuid.UUID) ([]*LedgerEntry, error)
	FindByEventID(ctx context.Context, eventID uuid.UUID) (*LedgerEntry, error)
}

type LedgerEntry struct {
	EntryID    uuid.UUID
	CreatorID  uuid.UUID
	EventID    uuid.UUID
	AmountCents int64
	Currency   string
	EntryType  string
	CreatedAt  int64
}
