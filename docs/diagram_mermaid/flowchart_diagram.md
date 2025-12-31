flowchart TD
    A[Incoming Payment Event] --> B[Persist PaymentEvent]
    B --> C[Evaluate Invariants]

    C -->|Violated| D[Mark Subscription = inconsistent]
    D --> E[Trigger Reconciliation]

    C -->|Valid| F[No state change]

    E --> G[Load full event history]
    G --> H[Apply precedence matrix]
    H --> I[Compute new AccessEntitlement]
    I --> J[Apply with projection_version check]

    J -->|Version conflict| K[Retry reconciliation]
    J -->|Success| L[Emit system events]
