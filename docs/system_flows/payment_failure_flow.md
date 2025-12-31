# Payment Failure → Grace Period → Cancellation Flow

**Scope:** Cross-contract lifecycle flow  
**Trigger:** Stripe webhook (invoice.payment_failed)  
**Authority:** Event-driven with time-based state transitions  
**Criticality:** SEV-1 (affects retention + revenue + access)

## Flow Sequence
```
Stripe webhook: invoice.payment_failed
  ↓
[Stripe] --payment_failed--> [Webhook Handler]
  ↓
[Identity] resolve customer → user_id
  ↓
[Subscription] transition state: active → past_due
  ↓
[Subscription] start grace period (e.g., 7 days)
  ↓
[Notification] email supporter: "Payment failed, please update card"
  ↓
[Reporting] update creator dashboard: "1 at-risk subscriber"
  ↓
--- GRACE PERIOD (async, scheduled) ---
  ↓
[Background Job] retry payment (Day 1, 3, 5)
  ↓
  ├─ SUCCESS → [goto: Payment Success Flow]
  │
  └─ ALL RETRIES FAILED
      ↓
[Subscription] transition: past_due → cancelled
  ↓
[Content] revoke access
  ↓
[Membership] record cancellation reason: "payment_failure"
  ↓
[Economy] finalize ledger (no refund, just close period)
  ↓
[Notification] email supporter: "Subscription cancelled"
  ↓
[Notification] email creator: "Lost 1 subscriber"
  ↓
[Reporting] update churn metrics
```

## Authority Guarantees
- subscription_contract: sole owner of subscription state (active/past_due/cancelled)
- subscription_contract: defines grace period duration (not notification or content)
- content_distribution_contract: revokes access only when subscription_state = cancelled
- creator_economy_contract: does NOT issue refunds on payment failure
- notification_contract: executes messaging only, does not decide retry logic

## Idempotency Strategy

### Webhook Level
- webhook_id stored in processed_webhooks
- Duplicate invoice.payment_failed for same invoice_id → skip

### State Transition Level
- Subscription state transition idempotency_key = f"state_transition:{subscription_id}:{target_state}:{attempt_timestamp}"
- SQL check: if subscription already in target state → no-op
- Prevents race condition: webhook retry + scheduled job both try to cancel

### Retry Payment Level
- Payment retry idempotency_key = f"retry_payment:{subscription_id}:{retry_attempt_number}"
- Stored in payment_attempts table with UNIQUE constraint
- Prevents duplicate Stripe payment intent creation

### Access Revoke Level
- Access revocation idempotency_key = f"revoke_access:{subscription_id}:{cancelled_at_timestamp}"
- Content contract checks: if access already revoked → no-op

### Notification Level
- Payment failure notification: f"payment_failed:{invoice_id}:{recipient_type}"
- Cancellation notification: f"subscription_cancelled:{subscription_id}:{recipient_type}"
- Prevents duplicate "your payment failed" emails during webhook retries

## Error Handling

| Failure Point | Root Cause | Compensation Strategy | Criticality |
|--------------|------------|----------------------|-------------|
| Identity resolution fails | Stripe customer_id not mapped | Manual ops queue, alert support | SEV-2 |
| Subscription state transition fails | DB deadlock, constraint violation | Retry 3x with exponential backoff, then alert | SEV-1 |
| Grace period job scheduling fails | Job queue down | Dead letter queue, manual ops intervention | SEV-1 |
| Payment retry fails (technical) | Stripe API timeout | Retry job itself (up to 3 attempts), alert if all fail | SEV-2 |
| Payment retry fails (business) | Declined card, insufficient funds | Continue to next scheduled retry, do NOT alert | Normal |
| Access revoke fails | Content service down | Async retry queue (indefinite), alert ops | SEV-2 |
| Notification fails | Email provider down | Queue retry (24hr window), do NOT block cancellation | SEV-3 |
| Reporting update fails | Analytics DB lag | Eventually consistent, acceptable lag up to 1 hour | SEV-3 |

### Critical Rules
- **State transition MUST succeed** before any side effects
- **Access revoke failure MUST NOT block cancellation** (eventual consistency acceptable)
- **Notification failure MUST NOT prevent retry attempts** or cancellation
- **Payment retry exhaustion MUST trigger cancellation** (no infinite grace period)

## Observability Requirements

### Events Emitted (in sequence)
1. **PaymentFailed** (from webhook handler)
   - payload: {subscription_id, invoice_id, amount, failure_reason, attempt_count}
2. **GracePeriodStarted** (from subscription)
   - payload: {subscription_id, grace_end_date, retry_schedule}
3. **PaymentFailureNotificationSent** (from notification)
   - payload: {subscription_id, recipient: supporter, sent_at}
4. **PaymentRetryAttempted** (from subscription, emitted per retry)
   - payload: {subscription_id, retry_number, stripe_payment_intent_id, result}
5. **SubscriptionCancelled** (from subscription, if all retries fail)
   - payload: {subscription_id, cancellation_reason: "payment_failure", cancelled_at}
6. **AccessRevoked** (from content)
   - payload: {subscription_id, revoked_at}
7. **CancellationNotificationSent** (from notification, 2 emails)
   - payload: {subscription_id, recipient: supporter/creator, sent_at}
8. **ChurnMetricsUpdated** (from reporting)
   - payload: {creator_id, lost_mrr, churn_reason}

### Structured Logging
Each step logs:
- **correlation_id** (same across entire flow, including scheduled retries)
- **subscription_id**
- **invoice_id**
- **user_id**
- **creator_id**
- **grace_period_end** (timestamp)
- **retry_attempt** (1, 2, 3, or "exhausted")
- **step_name** (e.g., "start_grace_period", "retry_payment_attempt_2", "revoke_access")
- **step_status** (success / failed / compensated / skipped)
- **timestamp**

### Metrics (for alerting)

#### Real-time Metrics
- **payment_failure_rate** (failures / total payments, 1-hour window)
  - Alert if > 5% (indicates Stripe issue or card network problem)
- **grace_period_conversion_rate** (recovered / entered_grace, 7-day window)
  - Track health of retry strategy
- **retry_success_by_attempt** (success rate for attempt 1, 2, 3)
  - Optimize retry timing
- **access_revoke_lag_seconds** (time from cancellation to actual revoke)
  - Alert if p99 > 60 seconds

#### Business Metrics
- **subscribers_in_grace_period_count** (point-in-time)
- **monthly_churn_from_payment_failure** (cancellations / active subscribers)
- **recovered_revenue** (successful retries * subscription amount)

### Audit Trail
- **Subscription record** stores:
  - state_history: [{state, timestamp, reason}]
  - payment_failure_count (lifetime)
  - last_grace_period_start
  - grace_period_recovery (true/false)
  
- **Payment attempts table** stores:
  - subscription_id, retry_attempt, stripe_payment_intent_id, result, attempted_at
  
- **Access revocation log** stores:
  - subscription_id, revoked_at, revoke_reason, revoked_by (system/manual)

## Human Override Points

### Ops Can Intervene At:

#### 1. **Extend grace period**
- **Use case:** Long-time supporter traveling, card temporarily blocked
- **Action:** Extend grace_period_end by N days (max 30 days total)
- **Authority:** subscription_contract (via admin API)
- **Side effects:** Reschedule cancellation job, send notification to supporter
- **Audit:** Logged with admin_user_id, reason, extension_days

#### 2. **Manual payment retry**
- **Use case:** Supporter updated card but scheduled retry not due yet
- **Action:** Trigger immediate payment retry (bypasses schedule)
- **Authority:** subscription_contract
- **Side effects:** If successful → transition to active, send success notification
- **Audit:** Logged with admin_user_id, reason

#### 3. **Cancel immediately (skip grace)**
- **Use case:** Fraud detected, supporter requested immediate cancellation
- **Action:** Force transition to cancelled, revoke access immediately
- **Authority:** subscription_contract
- **Side effects:** Cancel all scheduled retry jobs, send cancellation notification
- **Audit:** Logged with admin_user_id, reason (fraud/user_request)

#### 4. **Restore after cancellation**
- **Use case:** Payment succeeded after grace period expired (rare timing issue)
- **Action:** Reactivate subscription, restore access, credit ledger
- **Authority:** subscription_contract + economy_contract (coordinated)
- **Side effects:** Create compensating ledger entry, send reactivation notification
- **Audit:** Logged with admin_user_id, reason, requires dual approval

#### 5. **Suppress retry attempts**
- **Use case:** Card is known to be permanently invalid (closed account)
- **Action:** Mark subscription for immediate cancellation, skip remaining retries
- **Authority:** subscription_contract
- **Side effects:** Cancel retry jobs, proceed directly to cancellation
- **Audit:** Logged with admin_user_id, reason

### Audit Requirements
- All manual actions logged in subscription.admin_actions table
- Required fields: admin_user_id, action_type, reason (min 10 chars), timestamp
- Immutable: cannot delete or edit manual action records
- Displayed in admin CRM with full context

## Replay Safety

### Webhook Replay
- **Scenario:** Stripe resends invoice.payment_failed webhook
- **Safety:** webhook_id deduplication at handler level
- **Result:** Entire flow skipped if webhook already processed

### Scheduled Job Replay
- **Scenario:** Job queue restarts, re-enqueues retry job
- **Safety:** Payment attempt idempotency_key prevents duplicate Stripe API calls
- **Result:** Job runs but Stripe API call is no-op

### State Transition Replay
- **Scenario:** Two concurrent processes try to cancel subscription
- **Safety:** SQL check: if state already cancelled → no-op
- **Result:** Second transition attempt is skipped, no duplicate side effects

### Access Revoke Replay
- **Scenario:** Access revoke job runs twice due to retry
- **Safety:** Content contract checks existing access state
- **Result:** Second revoke is no-op, no error thrown

### Notification Replay
- **Scenario:** Notification job retries after timeout
- **Safety:** Idempotency key prevents duplicate email send
- **Result:** No duplicate "your subscription was cancelled" emails

## Edge Cases & Business Rules

### Grace Period Duration
- **Default:** 7 days from first payment failure
- **Configurable:** Per creator tier (premium creators may want 14 days)
- **Maximum:** 30 days (regulatory compliance, prevents indefinite access)

### Retry Schedule
- **Default:** Day 1, Day 3, Day 5 (after initial failure)
- **Smart retry:** Skip retry if supporter already updated card (detected via Stripe webhook)
- **Backoff:** No exponential backoff (fixed schedule for predictability)

### Access During Grace Period
- **Policy:** Access REMAINS active during grace period
- **Rationale:** Supporter retention, good faith, reduce churn
- **Exception:** If dispute_resolution flags account as fraud → immediate revoke overrides grace

### Cancellation Reason Tracking
- **Primary reason:** "payment_failure"
- **Sub-reasons:** "card_declined", "insufficient_funds", "card_expired", "fraud_suspected"
- **Used for:** Creator reporting, churn analysis, retry optimization

### Interaction with Other Flows
- **If supporter manually cancels during grace period:**
  - Cancel scheduled retry jobs
  - Transition to cancelled immediately
  - Reason logged as "user_cancelled" (not "payment_failure")
  
- **If refund requested during grace period:**
  - Process refund via Dispute Resolution Flow
  - Cancel subscription and retry jobs
  - Revoke access immediately (refund = explicit exit)

### Multi-subscription Handling
- **Scenario:** Supporter has subscriptions to multiple creators, one payment fails
- **Policy:** Each subscription has independent grace period
- **No cross-subscription logic:** Failure on creator A does NOT affect creator B

## Time Zone Considerations

### Grace Period Calculation
- **All timestamps in UTC**
- **grace_period_end calculated as:** payment_failed_at + 7 days (168 hours)
- **Retry schedule in UTC:** Day 1 = 24h after failure, not "next day in supporter timezone"

### Notification Timing
- **Supporter emails:** Delivered immediately (no timezone optimization)
- **Creator emails:** Batched daily digest at 9 AM creator's local timezone
- **Rationale:** Supporters need immediate action, creators need daily summary

## Performance & Scale Considerations

### Database Load
- **Grace period queries:** Indexed on (state, grace_period_end) for scheduled job lookups
- **Payment attempt queries:** Partitioned by month for historical analysis
- **Expected load:** 1,000 payment failures/day at scale → 3,000 retry jobs/week

### Job Queue
- **Retry jobs:** Scheduled in distributed job queue (e.g., Sidekiq, Bull)
- **Concurrency:** Limit to 10 concurrent retry jobs (Stripe API rate limit)
- **Timeout:** 30 seconds per retry attempt (fail fast, retry job itself later)

### Stripe API Usage
- **Payment retry:** 1 API call per retry attempt
- **Expected rate:** ~500 retries/day → well under Stripe rate limits
- **Error handling:** Stripe 429 → exponential backoff, then dead letter queue

---

## Summary: Flow Characteristics

| Characteristic | Value |
|----------------|-------|
| **Duration** | 0-7 days (grace period) + up to 24h (final cancellation side effects) |
| **Cross-contract depth** | 6 contracts (identity, subscription, content, membership, economy, notification, reporting) |
| **Critical path** | State transition + retry scheduling |
| **Async components** | Retry jobs, access revoke, notifications, reporting |
| **Human intervention frequency** | ~5% of cases (estimated) |
| **Replay risk** | Medium (scheduled jobs + webhooks) |
| **Data consistency requirement** | Strong for subscription state, eventual for access/reporting |

---