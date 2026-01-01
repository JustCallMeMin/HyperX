# Event Lifecycle Matrix

## Purpose

This document defines **who is allowed to emit which events**,  
**under which contract**, and **under what rules**.

This is a **governance document**, not an implementation detail.

Violating these rules is considered a **system architecture violation**.

---

## Definitions

- **Emit**: create and persist an immutable event
- **Consume**: read event to build projections or trigger side effects
- **Projection**: derived state rebuilt from events
- **Causation**: why an event exists (what caused it)

---

## Payment Events

| Event Type | Emit By | Contract Owner | Trigger Source | Notes |
|-----------|--------|----------------|---------------|-------|
| PaymentSucceeded | System | core_subscription | Stripe webhook | Immutable |
| PaymentFailed | System | core_subscription | Stripe webhook | Retry-safe |
| RefundIssued | System | dispute_resolution | Stripe webhook / ops | No direct refund |
| ChargebackCreated | System | dispute_resolution | Stripe webhook | SEV-1 |
| ChargebackResolved | System | dispute_resolution | Stripe webhook | Final state |

### Rules

- Payment events **MUST originate from payment providers**
- Users and admins **MUST NOT emit** payment events
- `provider_event_id` is mandatory for all payment events
- Payment events **DO NOT mutate ledger or access directly**

---

## Ledger Entry Events

| Event Type | Emit By | Contract Owner | Trigger Source | Notes |
|-----------|--------|----------------|---------------|-------|
| LedgerCredit | System | creator_economy | PaymentSucceeded | Append-only |
| LedgerDebit | System | creator_economy | Refund / Chargeback | No reversal |
| PlatformFeeApplied | System | creator_economy | PaymentSucceeded | Fee snapshot |

### Rules

- Ledger events **MUST have causation_id = payment_event_id**
- Ledger events are **append-only**
- Ledger events **MUST NOT be modified or deleted**
- Dispute resolution **MUST NOT mutate ledger directly**

---

## Membership State Events

| Event Type | Emit By | Contract Owner | Trigger Source | Notes |
|-----------|--------|----------------|---------------|-------|
| MembershipStateChanged | System / User / Admin | membership_relationship | Event-driven | Controlled transitions |

### Actor Permissions

| Actor | Allowed Transitions |
|------|--------------------|
| System | active → paused, active → churned |
| Supporter | active → cancelled |
| Creator | ❌ Not allowed |
| Admin | any → any (reason required) |

### Rules

- Membership state **MUST be changed via event**
- Direct updates to `memberships` table are forbidden
- Admin actions **MUST include reason**
- Membership projections **MUST be rebuildable from events**

---

## Access Entitlement (Derived Only)

Access entitlement is **NOT event-driven**.

| Action | Derived From |
|------|-------------|
| Grant access | PaymentSucceeded |
| Revoke access | Refund / Chargeback |
| Expire access | Time-based job |

### Rules

- No AccessGranted or AccessRevoked events exist
- Access is a **time-bounded projection**
- Access can always be rebuilt from payment history

---

## Notification Events

| Event | Emit By | Notes |
|------|--------|-------|
| NotificationIntentCreated | System | Side-effect only |

### Rules

- Notification failures **MUST NOT block business flow**
- Notification logic **MUST NOT mutate core state**
- Notification delivery must be idempotent

---

## Global Event Rules

### Rule 1 — Single Authority

Each event type has **exactly one owning contract**.  
Other contracts may consume but **must not emit**.

---

### Rule 2 — Causation Chain

Every event **MUST explain why it exists**.

Events without causation are invalid.

---

### Rule 3 — Replay Safety

Replaying events **MUST NOT trigger external side effects**.

Side effects must be handled asynchronously with idempotency.

---

### Rule 4 — Idempotency

Emitting the same event multiple times **MUST NOT corrupt state**.

---

### Rule 5 — No Direct Mutation

Events **MUST NOT directly mutate tables** outside their projection.

---

### Rule 6 — Failure Containment

Event consumer failure **MUST NOT rollback event history**.

---

### Rule 7 — Human Override

Humans may only affect the system via **explicit admin events**.

Direct SQL mutation is forbidden.

---

## Projection Mapping (Reference)

| Event | Projection Updated |
|------|-------------------|
| PaymentEvent | subscriptions, billing_cycles |
| LedgerEntryEvent | ledger_entries, creator_wallets |
| MembershipStateChanged | memberships, membership_state_history |

---

## Final Note

This document is part of the **system invariants**.

Any change to this file requires:
- Architecture review
- Version bump
- Migration plan
