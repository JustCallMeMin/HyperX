package adapters

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"hyperx/internal/economy/ports"
)

type InMemoryLedgerRepository struct {
	mu     sync.RWMutex
	entries map[uuid.UUID]*ports.LedgerEntry
	byCreator map[uuid.UUID][]*ports.LedgerEntry
}

func NewInMemoryLedgerRepository() ports.LedgerRepository {
	return &InMemoryLedgerRepository{
		entries:    make(map[uuid.UUID]*ports.LedgerEntry),
		byCreator:  make(map[uuid.UUID][]*ports.LedgerEntry),
	}
}

func (r *InMemoryLedgerRepository) AppendEntry(ctx context.Context, entry *ports.LedgerEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries[entry.EntryID] = entry
	r.byCreator[entry.CreatorID] = append(r.byCreator[entry.CreatorID], entry)
	return nil
}

func (r *InMemoryLedgerRepository) FindByCreatorID(ctx context.Context, creatorID uuid.UUID) ([]*ports.LedgerEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := r.byCreator[creatorID]
	if entries == nil {
		return []*ports.LedgerEntry{}, nil
	}

	return entries, nil
}

func (r *InMemoryLedgerRepository) FindByEventID(ctx context.Context, eventID uuid.UUID) (*ports.LedgerEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, entry := range r.entries {
		if entry.EventID == eventID {
			return entry, nil
		}
	}

	return nil, nil
}
