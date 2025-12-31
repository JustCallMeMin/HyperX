# Stripe Chargeback Resolution Flow

Scope: Cross-contract operational flow  
Trigger: Stripe webhook (charge.dispute.created)  
Authority: Event-driven, non-mutating orchestration

Stripe webhook: charge.dispute.created
  ↓
[Stripe] --dispute.created--> [Webhook Handler]
  ↓
[Identity] resolve customer → user_id
  ↓
[Subscription] fetch subscription state
  ↓
[Economy] create ledger debit (negative)
  ↓
[Dispute] create case (auto-review rules)
  ↓ (if auto_action=revoke)
[Content] revoke access immediately
  ↓
[Notification] email supporter + creator
  ↓
[Reporting] update creator metrics

Authority Guarantees:
- identity_access_contract is the sole owner of external ↔ internal identity mapping
- dispute_resolution_contract does not execute refunds
- creator_economy_contract is the only ledger mutator
- notification_contract executes side effects only

Replay Safety:
- Webhook replay must not duplicate ledger entries
- Notification delivery disabled during replay
- Dispute cases deduplicated by external_dispute_id
