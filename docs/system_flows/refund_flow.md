# Refund Flow

**Scope:** Cross-contract financial reconciliation flow  
**Trigger:** Multiple sources (Stripe refund webhook, admin action, dispute resolution)  
**Authority:** Initiated externally, reconciled internally  
**Criticality:** SEV-1 (affects money + trust + compliance)

## Flow Sequence
```
Trigger Sources:
  1. Stripe webhook: charge.refunded (supporter-initiated via Stripe dashboard)
  2. Admin CRM: Manual refund action (support/ops decision)
  3. Dispute Resolution: Approved refund case (from dispute flow)
  ↓
[Refund Handler] determine refund source & validate
  ↓
[Identity] resolve customer/subscription → user_id, creator_id
  ↓
[Subscription] fetch subscription state & billing period
  ↓
[Economy] create ledger debit (negative entry for creator)
  ↓
[Subscription] determine subscription action:
  ├─ Full refund → cancel subscription immediately
  ├─ Partial refund → keep subscription active
  └─ Proration refund → adjust next billing amount
  ↓
[Content] access decision:
  ├─ Full refund → revoke access immediately
  ├─ Partial refund → keep access until period end
  └─ Proration → no access change
  ↓
[Membership] record refund event in relationship history
  ↓
[Dispute] (if refund from dispute) close dispute case as "refunded"
  ↓
[Notification] send emails:
  ├─ Supporter: "Refund processed: $X.XX"
  └─ Creator: "Refund issued to [supporter]: -$X.XX"
  ↓
[Reporting] update creator financials:
  ├─ MRR adjustment
  ├─ Refund rate metric
  └─ Net revenue recalculation
```

## Authority Guarantees
- **creator_economy_contract:** sole owner of ledger mutations (refund entries)
- **subscription_contract:** decides subscription fate based on refund type
- **content_distribution_contract:** enforces access policy based on subscription decision
- **dispute_resolution_contract:** does NOT execute refunds, only requests them
- **identity_access_contract:** validates refund eligibility (prevents refund to wrong user)
- **Stripe:** external authority for payment reversal (we reconcile, don't initiate payment reversal)

## Refund Types & Business Rules

### 1. Full Refund
- **Definition:** 100% of last payment returned
- **Subscription action:** Cancel immediately
- **Access action:** Revoke immediately
- **Ledger:** Single debit entry = full payment amount
- **Common causes:** Service failure, supporter dissatisfaction, goodwill gesture

### 2. Partial Refund
- **Definition:** <100% of payment returned (e.g., refund $5 of $10 payment)
- **Subscription action:** Keep active until current period ends
- **Access action:** Keep access until period end (supporter paid something)
- **Ledger:** Single debit entry = partial amount
- **Common causes:** Proration for mid-month cancellation, service credit

### 3. Prorated Refund
- **Definition:** Refund based on unused days in billing period
- **Formula:** refund = (payment_amount / days_in_period) * unused_days
- **Subscription action:** Cancel subscription, set end_date = refund_date
- **Access action:** Revoke immediately (no unused time access)
- **Ledger:** Debit entry = calculated proration
- **Common causes:** Mid-cycle cancellation with proration policy

### 4. Goodwill Credit (Not a refund)
- **Definition:** Future credit applied, no money returned
- **Subscription action:** No change
- **Access action:** No change
- **Ledger:** Credit entry in wallet, no debit from creator
- **Common causes:** Service disruption, retention offer
- **Note:** This is NOT a refund flow, handled separately

## Idempotency Strategy

### Webhook Level
- **Key:** f"refund_webhook:{stripe_refund_id}"
- **Storage:** processed_webhooks table
- **TTL:** 90 days
- **Protection:** Prevents duplicate processing of same Stripe refund event

### Ledger Level
- **Key:** f"refund_ledger:{stripe_refund_id}" OR f"refund_ledger:{admin_refund_id}"
- **Storage:** Economy contract enforces UNIQUE constraint on idempotency_key
- **Protection:** Prevents duplicate negative ledger entries
- **Critical:** Ledger write MUST be idempotent (double refund = financial disaster)

### Subscription State Change Level
- **Key:** f"refund_subscription:{subscription_id}:{refund_id}"
- **Storage:** subscription_state_changes table
- **Protection:** Prevents double-cancellation or state corruption
- **Check:** if subscription already cancelled with reason="refund" → skip

### Access Revoke Level
- **Key:** f"refund_revoke_access:{subscription_id}:{refund_id}"
- **Storage:** access_revocations table
- **Protection:** Prevents duplicate revoke calls to content service
- **Check:** if access already revoked → no-op

### Notification Level
- **Supporter key:** f"refund_notification:supporter:{refund_id}"
- **Creator key:** f"refund_notification:creator:{refund_id}"
- **Storage:** Notification contract sent_notifications table
- **Protection:** Prevents duplicate "you've been refunded" emails

### Dispute Case Closure Level
- **Key:** f"refund_close_dispute:{dispute_id}"
- **Storage:** dispute_resolutions table
- **Protection:** Prevents marking dispute as "refunded" multiple times
- **Check:** if dispute already in terminal state → skip

## Error Handling

| Failure Point | Root Cause | Compensation Strategy | Criticality |
|--------------|------------|----------------------|-------------|
| Identity resolution fails | customer_id not mapped, subscription_id invalid | Manual ops queue with refund details, alert support | SEV-1 |
| Refund amount validation fails | Amount > original payment, negative amount | Block refund, log error, alert admin who initiated | SEV-1 |
| Ledger write fails | DB timeout, constraint violation, insufficient creator balance | Retry 3x, then dead letter queue, BLOCK entire flow | SEV-1 |
| Subscription cancellation fails | DB deadlock, state machine violation | Retry 3x, if still fails → manual ops queue | SEV-2 |
| Access revoke fails | Content service down, timeout | Async retry queue (indefinite), alert ops | SEV-2 |
| Dispute case closure fails | Dispute not found, already closed | Log warning, continue flow (non-blocking) | SEV-3 |
| Notification fails | Email provider down, invalid email | Queue retry (24hr window), do NOT block refund | SEV-3 |
| Reporting update fails | Analytics DB lag, calculation error | Eventually consistent, acceptable lag up to 1 hour | SEV-3 |

### Critical Rules
- **Ledger write MUST succeed** before any subscription or access changes
- **If ledger write fails, ENTIRE flow MUST halt** (no partial refund state)
- **Access revoke failure is acceptable** (eventual consistency via async retry)
- **Notification failure MUST NOT block refund** (refund is priority)
- **Stripe refund webhook and admin refund MUST deduplicate** (same stripe_refund_id)

### Ledger Insufficient Balance Handling
- **Scenario:** Creator ledger balance insufficient to cover refund debit
- **Policy:** 
  - Allow negative balance (debt to platform)
  - Flag creator account with "pending_debt"
  - Next payout withheld until balance positive
- **Alert:** Finance team notified for accounts with balance < -$1000
- **No blocking:** Refund MUST proceed even if creator balance goes negative

## Observability Requirements

### Events Emitted (in sequence)

1. **RefundInitiated** (from refund handler)
   - payload: {refund_id, source: "stripe_webhook" | "admin" | "dispute", stripe_refund_id?, admin_user_id?, dispute_id?, subscription_id, amount, refund_type}

2. **RefundLedgerEntryCreated** (from economy)
   - payload: {refund_id, ledger_entry_id, creator_id, amount, creator_balance_after}

3. **SubscriptionCancelledByRefund** (from subscription, if full refund)
   - payload: {subscription_id, refund_id, cancelled_at, prior_state}

4. **AccessRevokedByRefund** (from content, if full refund)
   - payload: {subscription_id, refund_id, revoked_at, content_ids}

5. **MembershipRefundRecorded** (from membership)
   - payload: {supporter_id, creator_id, refund_id, relationship_impact: "ended" | "none"}

6. **DisputeCaseClosedByRefund** (from dispute, if applicable)
   - payload: {dispute_id, refund_id, closed_at, resolution: "refunded"}

7. **RefundNotificationSent** (from notification, 2 events)
   - payload: {refund_id, recipient: "supporter" | "creator", sent_at, amount}

8. **CreatorFinancialsUpdated** (from reporting)
   - payload: {creator_id, refund_id, mrr_delta, refund_rate_new, net_revenue_new}

### Structured Logging
Each step logs:
- **correlation_id** (same across entire flow)
- **refund_id** (internal UUID)
- **stripe_refund_id** (if from Stripe)
- **admin_user_id** (if manual refund)
- **dispute_id** (if from dispute)
- **subscription_id**
- **supporter_id** (user_id)
- **creator_id**
- **amount** (refund amount in cents)
- **refund_type** ("full" | "partial" | "prorated")
- **step_name**
- **step_status** (success | failed | compensated | skipped)
- **timestamp**

### Metrics (for alerting)

#### Real-time Metrics
- **refund_flow_duration_seconds** (p50, p99)
  - Alert if p99 > 10 seconds (ledger write bottleneck)
  
- **refund_flow_failures_total** (by failure_point)
  - Alert if > 5 failures in 1 hour
  
- **ledger_write_failure_rate** (ledger failures / total refunds)
  - Alert if > 0.1% (indicates DB issues)
  
- **negative_creator_balance_count** (creators with balance < 0)
  - Alert if > 50 accounts (indicates systemic issue)

#### Business Metrics
- **refund_amount_total** (sum of all refunds, 24hr rolling)
  - Alert if daily refunds > 10% of daily revenue
  
- **refund_rate_by_creator** (refunds / total payments per creator)
  - Flag creators with refund_rate > 20% (potential abuse or quality issue)
  
- **refund_source_distribution** (webhook vs admin vs dispute, percentage)
  - Track operational load (high admin refunds = support burden)
  
- **average_time_to_refund** (from payment to refund, by source)
  - SLA: Admin refunds processed within 24 hours

#### Financial Reconciliation Metrics
- **stripe_refund_vs_ledger_mismatch_count**
  - Compare Stripe refund total vs internal ledger total (daily)
  - Alert if mismatch > $100 (reconciliation issue)

### Audit Trail

#### Refunds Table
- refund_id, stripe_refund_id, subscription_id, supporter_id, creator_id
- amount, refund_type, source, initiated_by (system | admin_user_id | dispute_id)
- reason (text, required for admin refunds)
- created_at, processed_at, status (pending | completed | failed)

#### Ledger Entries
- ledger_entry_id, creator_id, entry_type: "refund_debit"
- amount (negative), balance_after
- refund_id (foreign key), idempotency_key
- created_at

#### Subscription State History
- subscription_id, prior_state, new_state, reason: "refund"
- refund_id (foreign key), changed_at, changed_by

#### Access Revocation Log
- subscription_id, revoked_at, revoke_reason: "refund"
- refund_id (foreign key), content_ids (array)

#### Admin Actions (if manual refund)
- admin_user_id, action: "manual_refund", refund_id
- reason (min 20 chars, required), approved_by (if dual approval required)
- created_at, immutable

## Human Override Points

### Ops Can Intervene At:

#### 1. **Issue manual refund (primary intervention)**
- **Use case:** Supporter requests refund, Stripe dashboard unavailable, goodwill refund
- **Action:** Create refund via admin CRM, specify amount + type + reason
- **Authority:** Admin with "refund_permission" role
- **Dual approval:** Required if amount > $500 or refund_rate for creator > 15%
- **Side effects:** Triggers full refund flow (same as webhook path)
- **Audit:** admin_user_id, reason (required, min 20 chars), approved_by (if dual approval)

#### 2. **Override refund type decision**
- **Use case:** Full refund webhook received, but ops wants to keep subscription active
- **Action:** Mark refund as "partial_with_override", specify new subscription end_date
- **Authority:** Admin with "refund_override" permission
- **Side effects:** Subscription cancelled at specified date instead of immediately, access kept until then
- **Audit:** admin_user_id, override_reason, original_refund_type, new_refund_type

#### 3. **Reverse accidental refund**
- **Use case:** Admin issued refund by mistake, Stripe refund not yet settled
- **Action:** Cancel Stripe refund (if possible), create compensating ledger credit
- **Authority:** Admin with "refund_reversal" permission + Finance team approval
- **Side effects:** 
  - If Stripe refund cancelable → cancel via API
  - If not → create compensating charge or manual ledger credit
  - Restore subscription and access if cancelled
- **Audit:** admin_user_id, finance_approver_id, reversal_reason (required), stripe_reversal_id

#### 4. **Adjust creator balance manually**
- **Use case:** Ledger debit incorrect (e.g., refund processed twice due to bug)
- **Action:** Create compensating ledger credit with reason
- **Authority:** Finance team only (not general admin)
- **Side effects:** Creator balance adjusted, notification sent to creator
- **Audit:** finance_user_id, adjustment_reason, adjustment_amount, supporting_ticket_id (required)

#### 5. **Force access revoke despite partial refund**
- **Use case:** Abuse detected, supporter refunded but still accessing content
- **Action:** Override access policy, revoke immediately despite refund type
- **Authority:** Admin with "force_revoke" permission
- **Side effects:** Access revoked, flag added to supporter account
- **Audit:** admin_user_id, force_revoke_reason, abuse_category

### Audit Requirements
- All manual actions logged in refunds.admin_actions table
- Required fields: admin_user_id, action_type, reason (min 20 chars), timestamp
- Dual approval fields: approved_by (admin_user_id), approved_at (timestamp) - required if amount > $500
- Immutable: cannot delete or edit refund records or admin action logs
- Retention: 7 years (financial compliance)

## Replay Safety

### Stripe Webhook Replay
- **Scenario:** Stripe resends charge.refunded webhook
- **Safety:** stripe_refund_id deduplication in processed_webhooks
- **Result:** Entire flow skipped if refund already processed

### Admin Refund Duplicate Request
- **Scenario:** Admin clicks "Issue Refund" button twice
- **Safety:** Frontend disables button after click + backend checks for existing refund with same subscription_id in last 60 seconds
- **Result:** Second request blocked with error "Refund already in progress"

### Ledger Write Replay
- **Scenario:** Ledger write succeeds but response times out, retry triggered
- **Safety:** idempotency_key constraint on ledger_entries table
- **Result:** Second write fails with SQL unique constraint error, safely ignored

### Notification Replay
- **Scenario:** Notification job retries after timeout
- **Safety:** Idempotency key prevents duplicate email send
- **Result:** No duplicate "refund processed" emails

### Dispute Closure Replay
- **Scenario:** Dispute flow and refund flow both try to close same dispute
- **Safety:** Dispute state machine prevents transition from "refunded" to "refunded"
- **Result:** Second closure attempt is no-op

## Edge Cases & Business Rules

### Refund Eligibility Window
- **Policy:** Refunds allowed within 60 days of payment
- **Exception:** Admin can override for goodwill refunds (no time limit)
- **Enforcement:** Webhook handler validates refund_date - payment_date <= 60 days
- **If exceeded:** Log warning, allow refund to proceed (Stripe already processed it), flag for finance review

### Multiple Refunds for Same Subscription
- **Policy:** Multiple partial refunds allowed, but sum cannot exceed original payment
- **Validation:** Check sum(refunds) + new_refund_amount <= original_payment_amount
- **Enforcement:** Refund handler validates before ledger write
- **If exceeded:** Block refund, alert admin, require manual review

### Refund After Subscription Cancelled
- **Scenario:** Supporter cancels subscription, then requests refund
- **Policy:** Allowed if within eligibility window (60 days)
- **Subscription action:** No change (already cancelled)
- **Access action:** If access still active (end of billing period not reached), revoke immediately
- **Ledger:** Standard debit entry

### Refund During Grace Period
- **Scenario:** Subscription in grace period (payment failed), supporter requests refund
- **Policy:** Allowed, but refund amount = 0 if payment never succeeded
- **Validation:** Check if latest payment succeeded before allowing refund
- **Subscription action:** Cancel immediately, cancel scheduled retry jobs
- **Access action:** Revoke immediately

### Refund and Chargeback Interaction
- **Scenario:** Refund issued, then supporter also files chargeback
- **Policy:** 
  - If refund already processed → Stripe auto-resolves chargeback (we win)
  - If chargeback processed first → refund becomes no-op (already returned)
- **Detection:** Check for existing chargeback before processing refund
- **Action:** If chargeback exists, mark refund as "superseded_by_chargeback", skip ledger write

### Proration Calculation Details
- **Formula:** refund = (payment_amount / days_in_billing_period) * unused_days
- **Rounding:** Round down to nearest cent (favor platform)
- **Minimum refund:** $0.50 (below this, offer credit instead)
- **Edge case:** If unused_days = 0 (refund on last day of period) → refund = $0, offer credit

### Creator with Insufficient Balance
- **Balance check:** NO - creator can go negative
- **Refund precedence:** Supporter refund > creator balance (supporter satisfaction priority)
- **Creator impact:** Balance becomes negative, next payout withheld until positive
- **Platform risk mitigation:** Monitor creators with balance < -$1000, pause new subscriptions if < -$5000

### Tax Implications (if applicable)
- **Policy:** Refund includes tax if original payment included tax
- **Ledger:** Single debit entry includes tax amount
- **Reporting:** Tax refund amount tracked separately for accounting
- **Note:** Tax handling varies by jurisdiction, may require external tax service integration

## Integration Points

### Stripe API
- **Refund creation:** POST /v1/refunds (if admin-initiated)
- **Refund retrieval:** GET /v1/refunds/:id (to fetch details from webhook)
- **Refund cancellation:** POST /v1/refunds/:id/cancel (if reversal needed)
- **Rate limit:** 100 req/sec (shared with all Stripe API calls)
- **Idempotency:** Stripe-Idempotency-Key header = refund_id

### Payment Provider Reconciliation
- **Daily job:** Fetch all Stripe refunds from past 24 hours
- **Compare:** Stripe refund list vs internal refunds table
- **Alert:** If mismatch detected (refund in Stripe but not in our system)
- **Resolution:** Manual ops queue for investigation

### Accounting System (if integrated)
- **Export:** Daily batch export of refund ledger entries
- **Format:** CSV with columns: refund_id, creator_id, amount, date, reason
- **Delivery:** SFTP to accounting system or API webhook

## Performance & Scale Considerations

### Database Load
- **Refunds table:** Indexed on (subscription_id, created_at), (creator_id, created_at)
- **Ledger entries:** Partitioned by month, indexed on (creator_id, created_at)
- **Expected load:** 200 refunds/day at scale → 6,000 refunds/month

### Stripe API Usage
- **Admin refunds:** 1 API call per refund (create refund)
- **Webhook refunds:** 0 API calls (just reconcile)
- **Expected rate:** 50 admin refunds/day → well under Stripe rate limits

### Ledger Write Performance
- **Critical path:** Ledger write is synchronous, blocks entire flow
- **Target latency:** p99 < 500ms
- **Optimization:** Connection pooling, prepared statements, index on idempotency_key
- **Scaling:** If p99 > 1s → consider read replica for balance checks, write to primary only

### Notification Queue
- **Async delivery:** Notifications queued in background job system
- **Priority:** Low (refund already processed, notification is FYI)
- **Retry:** 3 attempts over 24 hours
- **Acceptable loss:** If all retries fail, acceptable (refund still valid)

---

## Summary: Flow Characteristics

| Characteristic | Value |
|----------------|-------|
| **Duration** | 1-5 seconds (happy path, synchronous) |
| **Cross-contract depth** | 7 contracts (identity, subscription, economy, content, membership, dispute, notification, reporting) |
| **Critical path** | Ledger write (MUST succeed) |
| **Async components** | Access revoke, notifications, reporting (eventual consistency acceptable) |
| **Human intervention frequency** | ~30% of cases (many refunds are admin-initiated) |
| **Replay risk** | High (multiple trigger sources: webhook + admin + dispute) |
| **Data consistency requirement** | Strong for ledger, eventual for access/reporting |
| **Financial impact** | Direct (every refund = revenue loss + creator debt risk) |
| **Compliance requirement** | High (7-year audit trail, tax implications) |

---