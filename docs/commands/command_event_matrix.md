# Command → Event Matrix

## Purpose

This document defines the final boundary between intent and fact.

- Commands express intent
- Events record facts
- Only events are persisted
- All state is derived from events

This document is authoritative.  
Violating it is considered a system architecture violation.

---

## Core Principles

1. Commands do NOT mutate database state
2. Commands emit at most one event
3. Events are immutable and replayable
4. Projections are rebuildable
5. Humans never mutate state directly

---

## Definitions

- Command: an explicit request to change the system
- Event: an immutable fact that already happened
- Emit: persist an event after validation
- Reject: command fails, no event emitted
- Actor: who initiated the command

---

## 1. Payment Commands

### 1.1 ReceivePaymentWebhook

| Attribute | Value |
|---------|------|
| Issued By | System |
| Source | Stripe Webhook |
| Emits | PaymentSucceeded, PaymentFailed |
| Contract Owner | core_subscription |

Validation rules:

- provider_event_id is required
- (provider, provider_event_id) must be idempotent
- User or Admin must not issue this command
- Payload must match PaymentEvent schema

---

### 1.2 ReceiveRefundWebhook

| Attribute | Value |
|---------|------|
| Issued By | System |
| Source | Stripe Webhook |
| Emits | RefundIssued |
| Contract Owner | dispute_resolution |

Validation rules:

- Refund must reference an existing payment event
- Refund does not mutate ledger directly

---

### 1.3 ReceiveChargebackWebhook

| Attribute | Value |
|---------|------|
| Issued By | System |
| Source | Stripe Webhook |
| Emits | ChargebackCreated, ChargebackResolved |
| Contract Owner | dispute_resolution |

Validation rules:

- Chargeback must reference original payment
- ChargebackCreated is SEV-1
- ChargebackResolved is terminal

---

## 2. Ledger Commands

### 2.1 ApplyLedgerEntry

| Attribute | Value |
|---------|------|
| Issued By | System |
| Trigger | PaymentEvent |
| Emits | LedgerCredit, LedgerDebit, PlatformFeeApplied |
| Contract Owner | creator_economy |

Validation rules:

- causation_id must equal payment_event_id
- Idempotency key is required
- Append-only; no update or delete

Forbidden:

- Manual admin invocation
- Direct SQL writes to ledger
- Ledger mutation from dispute handlers

---

## 3. Membership Commands

### 3.1 CancelMembership

| Attribute | Value |
|---------|------|
| Issued By | Supporter |
| Emits | MembershipStateChanged |
| Contract Owner | membership_relationship |

Validation rules:

- Current state must be active
- Actor must own membership
- Transition: active to cancelled

---

### 3.2 PauseMembership

| Attribute | Value |
|---------|------|
| Issued By | System |
| Trigger | PaymentFailed or policy |
| Emits | MembershipStateChanged |

Validation rules:

- Current state must be active
- Transition: active to paused

---

### 3.3 ResumeMembership

| Attribute | Value |
|---------|------|
| Issued By | System |
| Trigger | PaymentSucceeded |
| Emits | MembershipStateChanged |

Validation rules:

- Current state must be paused
- Transition: paused to active

---

### 3.4 AdminOverrideMembershipState

| Attribute | Value |
|---------|------|
| Issued By | Admin |
| Emits | MembershipStateChanged |

Validation rules:

- Admin identity is required
- Reason is required
- Any state transition is allowed
- Action must be auditable

---

## 4. Notification Commands

### 4.1 CreateNotificationIntent

| Attribute | Value |
|---------|------|
| Issued By | System |
| Emits | NotificationIntentCreated |
| Contract Owner | notification |

Rules:

- Must be idempotent
- Must not block business flow
- Must not mutate core state

---

## 5. Global Command Rules

Authorization failure rejects the command with no event.

Validation always happens before event emission.

A command emits exactly one event or zero events.

Commands never return state and never write projections.

Every retryable command must be idempotent.

Direct database mutation is forbidden.

---

## 6. API Surface

### Webhooks

POST /webhooks/stripe  
ReceivePaymentWebhook

### Membership APIs

POST /memberships/{id}/cancel  
CancelMembership

POST /memberships/{id}/pause  
PauseMembership

POST /memberships/{id}/resume  
ResumeMembership

### Admin APIs

POST /admin/memberships/{id}/override  
AdminOverrideMembershipState

---

## Final Law

APIs issue commands.  
Commands emit events.  
Events build state.

Any violation of this flow is a design error.
