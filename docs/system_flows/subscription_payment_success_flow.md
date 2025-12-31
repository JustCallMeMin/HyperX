# Subscription Payment Success Flow

**Scope:** Cross-contract revenue flow  
**Trigger:** Stripe webhook (invoice.payment_succeeded)  
**Authority:** Event-driven financial reconciliation  
**Criticality:** SEV-1 (affects revenue + access + trust + creator payouts)

## Flow Sequence
```
Stripe webhook: invoice.payment_succeeded
  ↓
[Stripe] --payment_succeeded--> [Webhook Handler]
  ↓
[Identity] resolve customer → user_id, creator_id
  ↓
[Subscription] update subscription state:
  ├─ New subscription: pending → active
  ├─ Renewal: active → active (extend period)
  ├─ Recovery from grace: past_due → active
  └─ Reactivation: cancelled → active (rare edge case)
  ↓
[Subscription] calculate billing period:
  ├─ period_start = invoice.period_start
  ├─ period_end = invoice.period_end
  └─ next_billing_date = period_end + 1 day
  ↓
[Economy] create ledger credit entry:
  ├─ Credit supporter (if prepaid model)
  └─ Credit creator (platform fee already deducted by Stripe)
  ↓
[Content] grant/extend access:
  ├─ Create access_entitlement record
  ├─ valid_from = period_start
  ├─ valid_until = period_end
  └─ tier_id = subscription.tier_id
  ↓
[Membership] update relationship:
  ├─ Record successful payment event
  ├─ Increment total_payments_count
  ├─ Update last_payment_at
  └─ Adjust loyalty_status if applicable
  ↓
[Notification] send success emails:
  ├─ Supporter: "Payment successful, access active"
  └─ Creator: "New payment from [supporter]: $X.XX"
  ↓
[Reporting] update creator metrics:
  ├─ MRR (if new or reactivation)
  ├─ Revenue this month
  ├─ Active subscriber count
  └─ Payment success rate
```

## Authority Guarantees
- **subscription_contract:** sole owner of subscription state transitions (pending→active, past_due→active)
- **creator_economy_contract:** sole owner of ledger mutations (credit entries)
- **content_distribution_contract:** sole owner of access grants (creates entitlements)
- **membership_relationships_contract:** records payment history, does NOT affect access
- **identity_access_contract:** validates payment belongs to correct user (prevents payment misattribution)
- **Stripe:** external authority for payment success (we trust Stripe's payment_succeeded event)

## Payment Types & State Transitions

### 1. Initial Subscription Payment
- **Trigger:** First payment after signup
- **Subscription state:** pending → active
- **Access grant:** Create new access_entitlement
- **Ledger:** First credit entry for this subscription
- **Membership:** Create relationship record (first_payment_at = now)
- **Reporting:** +1 active subscriber, +MRR

### 2. Renewal Payment
- **Trigger:** Recurring payment at end of billing period
- **Subscription state:** active → active (state unchanged, but period extends)
- **Access grant:** Extend access_entitlement.valid_until to new period_end
- **Ledger:** New credit entry for current period
- **Membership:** Update last_payment_at, increment total_payments_count
- **Reporting:** Revenue +X, payment_count +1 (MRR unchanged)

### 3. Recovery Payment (from grace period)
- **Trigger:** Payment retry succeeded during grace period
- **Subscription state:** past_due → active
- **Access grant:** 
  - If access still active (during grace): extend valid_until
  - If access revoked (grace expired): restore access, create new entitlement
- **Ledger:** New credit entry (may include late fees if policy)
- **Membership:** Mark as "recovered", reset failure_count to 0
- **Reporting:** -1 at-risk subscriber, +1 active subscriber, revenue +X

### 4. Reactivation Payment (after cancellation)
- **Trigger:** Supporter re-subscribes after previous cancellation
- **Subscription state:** cancelled → active (unusual, usually creates new subscription)
- **Access grant:** Create new access_entitlement (prior entitlement expired)
- **Ledger:** New credit entry
- **Membership:** Create new relationship record (or reactivate old one with gap tracking)
- **Reporting:** +1 active subscriber, +MRR, reactivation_rate +1

### 5. Upgrade/Downgrade Payment (mid-cycle tier change)
- **Trigger:** Payment for new tier after mid-cycle change
- **Subscription state:** active → active (tier_id changes)
- **Access grant:** Update entitlement tier_id immediately
- **Ledger:** Prorated credit entries (refund old tier, charge new tier)
- **Membership:** Record tier change event
- **Reporting:** MRR adjustment (delta between tiers)

## Idempotency Strategy

### Webhook Level
- **Key:** f"payment_success_webhook:{stripe_invoice_id}"
- **Storage:** processed_webhooks table
- **TTL:** 90 days (Stripe webhook retention)
- **Protection:** Prevents duplicate processing if Stripe resends webhook

### Subscription State Transition Level
- **Key:** f"subscription_activate:{subscription_id}:{invoice_id}"
- **Storage:** subscription_state_changes table with UNIQUE constraint
- **Protection:** Prevents duplicate activation on webhook retry
- **Check:** if subscription.state already matches target state AND last_invoice_id = current_invoice_id → skip transition

### Ledger Level
- **Key:** f"payment_ledger:{stripe_invoice_id}" OR f"payment_ledger:{stripe_charge_id}"
- **Storage:** Economy contract enforces UNIQUE constraint on idempotency_key
- **Protection:** Prevents duplicate credit entries (critical for financial accuracy)
- **Critical:** Ledger write MUST be idempotent (double credit = incorrect creator balance)

### Access Grant Level
- **Key:** f"access_grant:{subscription_id}:{period_start}:{period_end}"
- **Storage:** access_entitlements table with UNIQUE constraint on (subscription_id, period_start)
- **Protection:** Prevents duplicate entitlement records
- **Check:** if entitlement exists with same period_start → update valid_until instead of insert

### Membership Event Level
- **Key:** f"membership_payment:{subscription_id}:{invoice_id}"
- **Storage:** membership_payment_events table
- **Protection:** Prevents duplicate payment event records
- **Check:** if event exists with same invoice_id → skip insert

### Notification Level
- **Supporter key:** f"payment_success_notification:supporter:{invoice_id}"
- **Creator key:** f"payment_success_notification:creator:{invoice_id}"
- **Storage:** Notification contract sent_notifications table
- **Protection:** Prevents duplicate "payment successful" emails

### Reporting Update Level
- **Key:** f"payment_metrics_update:{creator_id}:{invoice_id}"
- **Storage:** reporting_update_log table
- **Protection:** Prevents double-counting revenue in metrics
- **Check:** if update already logged → skip metric recalculation

## Error Handling

| Failure Point | Root Cause | Compensation Strategy | Criticality |
|--------------|------------|----------------------|-------------|
| Identity resolution fails | customer_id not mapped, invalid subscription_id | Manual ops queue, alert support, BLOCK flow | SEV-1 |
| Invoice validation fails | Invoice amount mismatch, invalid period dates | Log error, alert finance, BLOCK flow | SEV-1 |
| Subscription state transition fails | DB deadlock, invalid state transition | Retry 3x with exponential backoff, then dead letter queue | SEV-1 |
| Ledger write fails | DB timeout, constraint violation | Retry 3x, then dead letter queue, BLOCK flow | SEV-1 |
| Access grant fails | Content service down, tier not found | Async retry queue (indefinite), alert ops | SEV-2 |
| Membership update fails | DB timeout, relationship not found | Async retry, eventual consistency acceptable | SEV-3 |
| Notification fails | Email provider down, invalid email | Queue retry (24hr window), do NOT block flow | SEV-3 |
| Reporting update fails | Analytics DB lag, calculation error | Eventually consistent, acceptable lag up to 1 hour | SEV-3 |

### Critical Rules
- **Subscription state transition MUST succeed** before any downstream effects
- **Ledger write MUST succeed** before flow completes (financial accuracy critical)
- **Access grant failure is acceptable** with async retry (eventual consistency via background job)
- **Notification failure MUST NOT block** payment processing (supporter has paid, notification is courtesy)
- **If both subscription AND ledger fail** → entire flow to dead letter queue, manual ops intervention required

### Amount Validation
- **Check:** invoice.amount_paid matches subscription.tier.price (accounting for discounts/coupons)
- **Tolerance:** Allow ±1 cent difference (rounding issues)
- **If mismatch > 1 cent:** 
  - Log SEV-2 error with details
  - Process payment anyway (Stripe is authority)
  - Create reconciliation task for finance team
  - Alert if mismatch > 10% of expected amount

## Observability Requirements

### Events Emitted (in sequence)

1. **PaymentSucceeded** (from webhook handler)
   - payload: {invoice_id, subscription_id, supporter_id, creator_id, amount_paid, currency, period_start, period_end, payment_type: "initial" | "renewal" | "recovery" | "reactivation"}

2. **SubscriptionActivated** (from subscription)
   - payload: {subscription_id, prior_state, new_state: "active", invoice_id, billing_period_start, billing_period_end, next_billing_date}

3. **LedgerCreditCreated** (from economy)
   - payload: {ledger_entry_id, creator_id, amount, platform_fee, creator_net, creator_balance_after, invoice_id}

4. **AccessGranted** (from content)
   - payload: {entitlement_id, subscription_id, supporter_id, tier_id, valid_from, valid_until, access_type: "new" | "extended" | "restored"}

5. **MembershipPaymentRecorded** (from membership)
   - payload: {relationship_id, supporter_id, creator_id, payment_number, total_lifetime_value, loyalty_tier}

6. **PaymentSuccessNotificationSent** (from notification, 2 events)
   - payload: {invoice_id, recipient: "supporter" | "creator", sent_at, amount}

7. **CreatorMetricsUpdated** (from reporting)
   - payload: {creator_id, mrr_delta, revenue_this_month, active_subscribers_delta, payment_success_rate}

### Structured Logging
Each step logs:
- **correlation_id** (same across entire flow)
- **invoice_id** (Stripe invoice ID)
- **charge_id** (Stripe charge ID, for direct payment tracing)
- **subscription_id**
- **supporter_id** (user_id)
- **creator_id**
- **amount_paid** (in cents)
- **currency** (USD, EUR, etc.)
- **payment_type** ("initial" | "renewal" | "recovery" | "reactivation")
- **billing_period_start** (UTC timestamp)
- **billing_period_end** (UTC timestamp)
- **step_name**
- **step_status** (success | failed | compensated | skipped)
- **duration_ms** (step execution time)
- **timestamp**

### Metrics (for alerting)

#### Real-time Metrics
- **payment_success_flow_duration_seconds** (p50, p95, p99)
  - Alert if p99 > 5 seconds (indicates bottleneck)
  
- **payment_success_flow_failures_total** (by failure_point)
  - Alert if > 10 failures in 1 hour (indicates systemic issue)
  
- **ledger_write_failure_rate** (ledger failures / total payments)
  - Alert if > 0.1% (critical financial accuracy issue)
  
- **access_grant_failure_rate** (access failures / total payments)
  - Alert if > 1% (supporter paid but can't access content)
  
- **access_grant_lag_seconds** (time from payment to access active)
  - Alert if p99 > 60 seconds (poor supporter experience)

#### Business Metrics
- **payment_success_rate** (succeeded / (succeeded + failed), 24hr rolling)
  - Baseline: expect ~95%+
  - Alert if < 90% (card network issues or Stripe problems)
  
- **mrr_growth_rate** (daily MRR delta / prior MRR)
  - Track health of subscription business
  
- **new_subscriber_count** (initial payments, 24hr rolling)
  - Track acquisition funnel health
  
- **renewal_success_rate** (renewal payments / expected renewals)
  - Baseline: expect ~90%+ for healthy subscriptions
  - Alert if < 80% (indicates payment method issues or churn)
  
- **recovery_rate** (recovery payments / grace period subscriptions)
  - Track effectiveness of retry strategy
  
- **average_revenue_per_payment** (total revenue / payment count)
  - Track if tier distribution or pricing changes

#### Financial Reconciliation Metrics
- **stripe_revenue_vs_ledger_mismatch** (daily)
  - Compare: sum(Stripe invoice.amount_paid) vs sum(internal ledger credits)
  - Alert if mismatch > $100 or > 0.5% of daily revenue
  
- **platform_fee_calculation_accuracy** (sampled)
  - Verify: platform_fee = amount_paid * fee_rate for random sample
  - Alert if any mismatch found

### Audit Trail

#### Payments Table (denormalized for fast queries)
- payment_id, invoice_id, charge_id, subscription_id, supporter_id, creator_id
- amount_paid, currency, platform_fee, creator_net
- payment_type, billing_period_start, billing_period_end
- processed_at, status (succeeded | pending | failed)

#### Ledger Entries (immutable, append-only)
- ledger_entry_id, creator_id, entry_type: "payment_credit"
- amount (positive), balance_before, balance_after
- invoice_id, idempotency_key
- created_at, never updated or deleted

#### Subscription State History
- subscription_id, prior_state, new_state, reason: "payment_succeeded"
- invoice_id (foreign key), changed_at, changed_by: "system"

#### Access Entitlements (current + historical)
- entitlement_id, subscription_id, supporter_id, tier_id
- valid_from, valid_until, created_at, updated_at
- Soft delete: revoked_at (for audit trail)

#### Membership Payment Events (relationship history)
- event_id, relationship_id, supporter_id, creator_id
- event_type: "payment_succeeded", invoice_id, amount
- payment_number (1st, 2nd, 3rd...), created_at

## Human Override Points

### Ops Can Intervene At:

#### 1. **Manual payment reconciliation**
- **Use case:** Webhook missed, payment succeeded in Stripe but not in our system
- **Action:** Manually trigger payment success flow with invoice_id
- **Authority:** Admin with "payment_reconciliation" permission
- **Validation:** Check Stripe API to confirm invoice.status = "paid" before processing
- **Side effects:** Full payment flow executes (same as webhook path)
- **Audit:** admin_user_id, reconciliation_reason, stripe_invoice_url

#### 2. **Force access grant despite payment issues**
- **Use case:** Payment succeeded but access grant failed, supporter complaining
- **Action:** Manually create access_entitlement via admin CRM
- **Authority:** Admin with "access_override" permission
- **Validation:** Verify payment exists in payments table before granting access
- **Side effects:** Access granted, notification sent to supporter
- **Audit:** admin_user_id, override_reason, payment_id, entitlement_id

#### 3. **Correct ledger entry**
- **Use case:** Ledger entry created with wrong amount (bug in fee calculation)
- **Action:** Create compensating ledger entry (debit or credit) with explanation
- **Authority:** Finance team only (not general admin)
- **Validation:** Requires supporting ticket or documentation
- **Side effects:** Creator balance adjusted, finance team notified
- **Audit:** finance_user_id, correction_reason, original_entry_id, correction_amount, ticket_id

#### 4. **Extend billing period manually**
- **Use case:** Payment processed late (Stripe delay), need to adjust period_end
- **Action:** Update subscription.period_end and access_entitlement.valid_until
- **Authority:** Admin with "billing_adjustment" permission
- **Validation:** Can only extend, cannot shorten (prevents access loss)
- **Side effects:** Next billing date recalculated, notification sent to supporter
- **Audit:** admin_user_id, extension_reason, old_period_end, new_period_end

#### 5. **Override payment type classification**
- **Use case:** Payment incorrectly classified (renewal vs reactivation), affects metrics
- **Action:** Update payment.payment_type in payments table
- **Authority:** Admin with "payment_classification" permission
- **Side effects:** Reporting metrics recalculated (async batch job)
- **Audit:** admin_user_id, reclassification_reason, old_type, new_type

### Audit Requirements
- All manual actions logged in payments.admin_actions table
- Required fields: admin_user_id, action_type, reason (min 20 chars), timestamp
- For finance corrections: ticket_id or documentation_url required
- Immutable: cannot delete or edit payment records or admin action logs
- Retention: 7 years (financial compliance)

## Replay Safety

### Stripe Webhook Replay
- **Scenario:** Stripe resends invoice.payment_succeeded webhook
- **Safety:** invoice_id deduplication in processed_webhooks table
- **Result:** Entire flow skipped if invoice already processed
- **Log:** Replay attempt logged for monitoring (helps detect Stripe issues)

### Subscription State Transition Replay
- **Scenario:** Webhook retry after partial flow completion
- **Safety:** Check subscription.state + subscription.last_invoice_id before transition
- **Result:** If already active with same invoice_id → skip transition, proceed to next step

### Ledger Write Replay
- **Scenario:** Ledger write succeeds but response times out, retry triggered
- **Safety:** idempotency_key UNIQUE constraint on ledger_entries table
- **Result:** Second write fails with SQL error, error handler recognizes as duplicate and continues flow

### Access Grant Replay
- **Scenario:** Access grant job runs twice due to retry
- **Safety:** UNIQUE constraint on (subscription_id, period_start) in entitlements table
- **Result:** Second grant attempt:
  - If exact duplicate period → SQL error, safely ignored
  - If period_end extension → UPDATE instead of INSERT (idempotent)

### Notification Replay
- **Scenario:** Notification job retries after timeout
- **Safety:** Idempotency key in sent_notifications table
- **Result:** No duplicate "payment successful" emails sent

### Reporting Update Replay
- **Scenario:** Metrics update job runs twice due to job queue restart
- **Safety:** Update log tracks processed invoice_ids
- **Result:** Metrics not double-counted, second update is no-op

## Edge Cases & Business Rules

### Payment Amount Validation
- **Expected amount:** subscription.tier.price * billing_frequency
- **Discounts:** Apply coupon/discount logic, verify invoice.discount matches
- **Tolerance:** ±1 cent acceptable (floating point rounding)
- **Action if mismatch > 1 cent:**
  - Process payment (Stripe is authority)
  - Log reconciliation issue
  - Alert finance team if mismatch > 10%

### Billing Period Calculation
- **Standard:** period_end = period_start + 1 month (for monthly) or + 1 year (for annual)
- **Edge case:** Month-end handling (Jan 31 → Feb 28, not Mar 3)
- **Stripe alignment:** Always use invoice.period_start and invoice.period_end from Stripe (authoritative)
- **Next billing date:** period_end + 1 day (Stripe charges on this date)

### Proration Handling (mid-cycle tier changes)
- **Stripe behavior:** Sends separate invoice.payment_succeeded for prorated amount
- **Our handling:** Process as separate payment, update access_entitlement tier_id immediately
- **Ledger:** Create two entries (refund credit for old tier, charge debit for new tier difference)
- **Reporting:** MRR adjusted by net difference

### Trial Period Expiration
- **Scenario:** Trial ends, first real payment occurs
- **Payment type:** Classified as "initial" (not "renewal")
- **Subscription state:** trial → active
- **Access grant:** Extend entitlement from trial tier to paid tier
- **Reporting:** +1 active subscriber, +MRR (trial doesn't count in MRR)

### Payment During Cancellation Period
- **Scenario:** Supporter cancels subscription (cancel_at_period_end = true), then payment succeeds for current period
- **Policy:** Process payment normally, access granted until period_end
- **Subscription state:** active_until_cancel → active_until_cancel (unchanged)
- **Next billing:** No next billing (cancellation scheduled)
- **Ledger:** Standard credit entry (supporter paid for current period)

### Multiple Subscriptions Same Supporter-Creator Pair
- **Policy:** Allowed (supporter can have multiple tier subscriptions to same creator, if creator allows)
- **Access grant:** Create separate entitlements for each subscription
- **Access logic:** Grant highest tier access across all active subscriptions
- **Ledger:** Separate entries per subscription

### Payment Success After Refund Initiated
- **Scenario:** Admin initiates refund, but new payment succeeds before refund processes
- **Detection:** Check for pending refunds on this subscription
- **Policy:** 
  - Process payment normally
  - Cancel refund if possible (Stripe API)
  - If refund already processed, create compensating charge
- **Alert:** Manual ops review required (unusual scenario)

### Payment Method Changed Mid-Flow
- **Scenario:** Payment succeeded with old card, but supporter updated card during webhook processing
- **Policy:** Process payment with old card (Stripe already charged it)
- **Next payment:** Will use new card (Stripe handles this)
- **No action needed:** Flow continues normally

### Currency Conversion (if multi-currency)
- **Policy:** Creator sets prices in creator_currency, supporter pays in supporter_currency
- **Conversion:** Stripe handles conversion, we store both amounts
- **Ledger:** Credit in creator_currency (what creator receives)
- **Reporting:** Revenue in USD (normalized for platform-wide metrics)

## Integration Points

### Stripe API
- **Invoice retrieval:** GET /v1/invoices/:id (if webhook payload incomplete)
- **Customer retrieval:** GET /v1/customers/:id (for metadata verification)
- **Subscription retrieval:** GET /v1/subscriptions/:id (for current state verification)
- **Rate limit:** 100 req/sec (shared with all Stripe API calls)
- **Fallback:** If Stripe API fails during webhook processing, queue for retry (don't block webhook response)

### Payment Provider Reconciliation
- **Daily job:** Fetch all Stripe invoices with status="paid" from past 24 hours
- **Compare:** Stripe invoice list vs internal payments table
- **Alert:** If invoice in Stripe but not in our system (missed webhook)
- **Action:** Trigger manual reconciliation flow for missing invoices
- **Metrics:** Track missed_webhook_count (should be near zero)

### Creator Payout System (downstream)
- **Trigger:** Ledger credit entry created
- **Action:** Creator balance updated, available for payout
- **Payout schedule:** Weekly or monthly (creator preference)
- **Minimum payout:** $50 (configurable)
- **Integration:** Creator economy contract exposes get_available_balance() API

### Tax Calculation (if applicable)
- **Provider:** TaxJar, Avalara, or similar (if required)
- **Timing:** Tax calculated before Stripe charge (not during webhook)
- **Webhook:** Includes tax amount in invoice.tax
- **Ledger:** Store gross amount (includes tax), net amount (creator receives)
- **Reporting:** Tax tracked separately for compliance

## Performance & Scale Considerations

### Database Load
- **Payments table:** Indexed on (subscription_id, processed_at), (supporter_id, processed_at), (creator_id, processed_at)
- **Ledger entries:** Partitioned by month, indexed on (creator_id, created_at)
- **Access entitlements:** Indexed on (supporter_id, valid_until), (subscription_id)
- **Expected load:** 10,000 payments/day at scale → 300,000/month

### Webhook Processing
- **Concurrency:** Process webhooks concurrently (up to 50 workers)
- **Idempotency:** Critical (webhooks can arrive out of order or duplicate)
- **Timeout:** Respond to Stripe within 30 seconds (Stripe requirement)
- **Async work:** Queue long-running tasks (access grant, notification) after webhook response

### Critical Path Optimization
- **Must complete synchronously:** Subscription state + ledger write (< 2 seconds)
- **Can be async:** Access grant, notification, reporting (eventual consistency acceptable)
- **Database optimization:** 
  - Connection pooling (min 20, max 100 connections)
  - Prepared statements for common queries
  - Read replicas for reporting queries (not for payment processing)

### Stripe API Rate Limits
- **Webhook processing:** 0 API calls per webhook (all data in payload)
- **Reconciliation:** 1 API call per invoice check
- **Expected usage:** ~100 API calls/hour for reconciliation → well under limits

### Access Grant Performance
- **Target latency:** p99 < 5 seconds from payment to content accessible
- **Optimization:** 
  - Async queue with high priority
  - Content service caching of tier permissions
  - CDN edge caching of access checks (with short TTL)

---

## Summary: Flow Characteristics

| Characteristic | Value |
|----------------|-------|
| **Duration** | 1-3 seconds (happy path, most steps synchronous) |
| **Cross-contract depth** | 7 contracts (identity, subscription, economy, content, membership, notification, reporting) |
| **Critical path** | Subscription activation + ledger write (< 2s required) |
| **Async components** | Access grant (priority queue), notification, reporting |
| **Human intervention frequency** | ~1% of cases (mostly reconciliation for missed webhooks) |
| **Replay risk** | Medium-high (Stripe retries webhooks on timeout) |
| **Data consistency requirement** | Strong for subscription+ledger, eventual for access/reporting (within 60s) |
| **Financial impact** | Direct (every payment = revenue + creator payout obligation) |
| **Volume** | Highest of all flows (90%+ of payment events are successes) |
| **SLA** | p99 < 5s from payment to access active |

---