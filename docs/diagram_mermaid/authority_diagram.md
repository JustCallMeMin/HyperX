flowchart LR
    PaymentProvider[(Payment Provider)]
    WebhookHandler[Webhook Handler]
    PaymentEventDB[(PAYMENT_EVENT)]
    ProjectionDB[(SUBSCRIPTION + ACCESS_ENTITLEMENT)]
    ReconciliationEngine

    PaymentProvider -->|authoritative facts| WebhookHandler
    WebhookHandler -->|append only| PaymentEventDB

    PaymentEventDB --> ReconciliationEngine
    ReconciliationEngine -->|derived projection| ProjectionDB

    ProjectionDB -.->|read only| PublicAPI
