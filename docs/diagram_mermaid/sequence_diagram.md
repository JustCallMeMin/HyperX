sequenceDiagram
    participant PP as Payment Provider
    participant WH as Webhook Handler
    participant ES as Payment Event Store
    participant RE as Reconciliation Engine
    participant DB as Projection DB
    participant EB as System Event Bus

    PP->>WH: PaymentSucceeded / Chargeback
    WH->>ES: append PaymentEvent (atomic, immutable)
    Note over WH,ES: event_id, causation_id, provider_ref stored

    ES->>RE: trigger reconciliation (async)
    RE->>ES: load full payment history (order-independent)
    RE->>RE: apply precedence matrix

    RE->>DB: update Subscription (optimistic lock)
    RE->>DB: write AccessEntitlement (absolute UTC window)

    RE-->>EB: emit SystemEvent (AccessGranted / InvariantViolated)
    Note over EB: replay-safe, deduplicated by event_id
