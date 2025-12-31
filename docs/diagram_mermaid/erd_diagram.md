erDiagram
    SUBSCRIPTION ||--o{ PAYMENT_EVENT : records
    SUBSCRIPTION ||--|| ACCESS_ENTITLEMENT : owns
    SUBSCRIPTION ||--o{ RECONCILIATION_RUN : reconciled_by
    SUBSCRIPTION ||--o{ INVARIANT_VIOLATION : detects

    SUBSCRIPTION {
        uuid subscription_id PK
        uuid user_id
        string state
        string current_tier
        int projection_version
        timestamp created_at_utc
        timestamp updated_at_utc
    }

    PAYMENT_EVENT {
        uuid event_id PK
        uuid subscription_id FK
        string event_type
        string provider_reference
        timestamp occurred_at_utc
        int schema_version
    }

    ACCESS_ENTITLEMENT {
        uuid entitlement_id PK
        uuid subscription_id FK
        timestamp start_at_utc
        timestamp expires_at_utc
    }

    RECONCILIATION_RUN {
        uuid run_id PK
        uuid subscription_id FK
        int applied_projection_version
        timestamp executed_at_utc
    }

    INVARIANT_VIOLATION {
        uuid violation_id PK
        uuid subscription_id FK
        string invariant_type
        timestamp detected_at_utc
        boolean resolved
    }
