package events

import (
	"time"

	"github.com/google/uuid"
	"hyperx/pkg/event"
)

type LedgerCredit struct {
	metadata  event.Metadata
	aggregate EconomyAggregate
	payload  LedgerCreditPayload
}

func NewLedgerCredit(
	causingEvent event.Event,
	creatorID uuid.UUID,
	amountCents int64,
	currency string,
) *LedgerCredit {
	metadata := event.NewMetadataFromEvent(causingEvent, "LedgerCredit", 1)

	return &LedgerCredit{
		metadata: metadata,
		aggregate: EconomyAggregate{
			CreatorID: creatorID,
		},
		payload: LedgerCreditPayload{
			AmountCents: amountCents,
			Currency:    currency,
			EntryType:   "payment_credit",
		},
	}
}

func (e *LedgerCredit) EventID() uuid.UUID {
	return e.metadata.EventID
}

func (e *LedgerCredit) EventType() string {
	return e.metadata.EventType
}

func (e *LedgerCredit) OccurredAt() time.Time {
	return e.metadata.OccurredAt
}

func (e *LedgerCredit) SchemaVersion() int {
	return e.metadata.SchemaVersion
}

func (e *LedgerCredit) Metadata() event.Metadata {
	return e.metadata
}

func (e *LedgerCredit) Aggregate() event.Aggregate {
	return e.aggregate
}

func (e *LedgerCredit) Payload() event.Payload {
	return e.payload
}

type EconomyAggregate struct {
	CreatorID uuid.UUID
}

func (a EconomyAggregate) AggregateID() uuid.UUID {
	return a.CreatorID
}

func (a EconomyAggregate) AggregateType() string {
	return "economy"
}

type LedgerCreditPayload struct {
	AmountCents int64
	Currency    string
	EntryType   string
}

func (p LedgerCreditPayload) Validate() error {
	if p.AmountCents <= 0 {
		return event.ErrInvalidSchemaVersion
	}
	if p.Currency == "" {
		return event.ErrMissingEventType
	}
	if p.EntryType == "" {
		return event.ErrMissingEventID
	}
	return nil
}
