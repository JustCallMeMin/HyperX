# Stripe Chargeback Resolution Flow

**Scope:** Cross-contract operational flow  
**Trigger:** Stripe webhook (charge.dispute.created)  
**Authority:** Event-driven, non-mutating orchestration  
**Criticality:** SEV-1 (affects money + access + trust)

## Flow Sequence
```
Stripe webhook: charge.dispute.created
  ↓
[Stripe] --dispute.created--> [Webhook Handler]
  ↓
[Identity] resolve customer → user_id
  ↓
[Subscription] fetch subscription state
  ↓
[Economy] create ledger debit (negative) ← MUST succeed
  ↓
[Dispute] create case (auto-review rules)
  ↓ (if auto_action=revoke)
[Content] revoke access immediately
  ↓
[Notification] email supporter + creator
  ↓
[Reporting] update creator metrics
```

## Authority Guarantees
- identity_access_contract: sole owner of external ↔ internal mapping
- dispute_resolution_contract: does not execute refunds
- creator_economy_contract: only ledger mutator
- notification_contract: executes side effects only

## Idempotency Strategy
- Webhook level: webhook_id deduplication (90 days)
- Ledger level: idempotency_key = f"chargeback:{dispute_id}"
- Dispute level: external_dispute_id = stripe_dispute_id
- Notification level: per-recipient deduplication

## Error Handling
| Failure Point | Compensation | Criticality |
|--------------|--------------|-------------|
| Identity resolution | Manual ops queue | SEV-2 |
| Ledger write | BLOCK flow, alert | SEV-1 |
| Access revoke | Async retry | SEV-2 |
| Notification | Queue retry | SEV-3 |

## Observability
- **Events:** 6 events emitted (ChargebackReceived → CreatorMetricsUpdated)
- **Logging:** correlation_id across all steps
- **Metrics:** duration, failures, amount
- **Audit:** Full event chain in dispute case

## Human Override Points
1. Pause auto-revoke (before content step)
2. Compensating ledger entry (after dispute resolved)
3. Notification suppression (bulk fraud scenarios)

All overrides logged with admin_user_id + reason.

## Replay Safety
- Webhook replay: deduplicated at handler level
- Ledger replay: SQL constraint prevents duplicates
- Notification replay: disabled during ops replay