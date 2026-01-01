# Event Store Design

## Overview

The `events` table is the single source of truth for all events in the system. This is the event store that enables event sourcing.

## Schema

```sql
CREATE TABLE events (
  event_id UUID PRIMARY KEY,
  event_type VARCHAR NOT NULL,
  aggregate_type VARCHAR NOT NULL,
  aggregate_id UUID NOT NULL,
  schema_version INT NOT NULL,
  occurred_at_utc TIMESTAMP NOT NULL,
  causation_id UUID,
  correlation_id UUID,
  idempotency_key VARCHAR,
  actor_type VARCHAR,
  actor_identity_id UUID,
  origin_event_type VARCHAR,
  payload_json JSONB NOT NULL
);

CREATE INDEX idx_events_aggregate 
ON events(aggregate_type, aggregate_id, occurred_at_utc);

CREATE INDEX idx_events_type_time 
ON events(event_type, occurred_at_utc);

CREATE INDEX idx_events_causation 
ON events(causation_id);

CREATE INDEX idx_events_correlation 
ON events(correlation_id);

CREATE UNIQUE INDEX uniq_events_idempotency 
ON events(idempotency_key) 
WHERE idempotency_key IS NOT NULL;
```

## Why This Design

1. **Single Source of Truth**: All events live in one place, making replay and audit trails simple
2. **Schema Evolution**: `schema_version` allows payloads to evolve over time
3. **Causation Tracking**: `causation_id` and `correlation_id` enable event chain tracing
4. **Idempotency**: `idempotency_key` prevents duplicate processing
5. **Flexible Payloads**: JSONB allows different event types to have different payload structures

## Domain-Specific Event Tables

Tables like `payment_events` and `dispute_events` are **projections** from the events table. They exist for:
- Query performance (denormalized, indexed views)
- Domain-specific queries (e.g., "all payments for a subscription")
- Backward compatibility with existing code

These tables should be populated by projection builders that read from the `events` table.

## Event Replay

To rebuild any projection:

```sql
SELECT * 
FROM events 
WHERE aggregate_type = 'subscription' 
  AND aggregate_id = :subscription_id
ORDER BY occurred_at_utc ASC;
```

This gives you the complete event history for that aggregate, which you can replay to rebuild state.

## Migration Strategy

1. Start writing all new events to the `events` table
2. Create projection builders that populate domain-specific tables from `events`
3. Gradually migrate existing event writes to go through the `events` table
4. Eventually, domain-specific tables become read-only projections
