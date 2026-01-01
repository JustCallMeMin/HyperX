# Projection Builders Specification

## Purpose

This document defines **how events are translated into database projections**.

Rules:
- Projections are **derived state**
- Events are **source of truth**
- Projections may be dropped and rebuilt at any time
- Events are NEVER modified

---

## 1. PaymentEvent → Subscription & Billing Projections

### Source Event
- `PaymentSucceeded`
- `PaymentFailed`
- `RefundIssued`
- `ChargebackCreated`
- `ChargebackResolved`

### Target Tables
- `subscriptions`
- `billing_cycles`
- `access_entitlements`

---

### 1.1 PaymentSucceeded

#### Effects

| Table | Action |
|-----|--------|
| subscriptions | state → `active` |
| billing_cycles | insert new cycle |
| access_entitlements | grant time-bounded access |

#### Pseudo-code

```pseudo
on PaymentSucceeded(event):
  sub = load subscription(event.subscription_id)

  update subscriptions
    set state = 'active',
        current_period_start = event.occurred_at,
        current_period_end = occurred_at + billing_interval,
        updated_at = now()

  insert billing_cycles(
    subscription_id,
    cycle_start,
    cycle_end
  )

  insert access_entitlements(
    subscription_id,
    tier_id,
    access_start,
    access_end,
    granted_by_event_id
  )
