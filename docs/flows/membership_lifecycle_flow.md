# Membership Lifecycle Flow

**Scope:** Cross-contract relationship management flow  
**Trigger:** Supporter actions (join, pause, resume, cancel) or system events  
**Authority:** Social semantics layer, does NOT control money or access directly  
**Criticality:** SEV-2 (affects loyalty tracking + creator insights, not money/access directly)

## Flow Sequence
```
Trigger Sources:
  1. Supporter clicks "Subscribe" (join)
  2. Supporter clicks "Pause membership" (pause)
  3. Supporter clicks "Resume membership" (resume)
  4. Supporter clicks "Cancel subscription" (cancel)
  5. System event: PaymentFailed → auto-pause consideration
  6. System event: SubscriptionCancelled → relationship end
  ↓
[Membership] validate action:
  ├─ Check: Does relationship exist? (for pause/resume/cancel)
  ├─ Check: Is action allowed in current state?
  ├─ State machine validation (see state diagram below)
  └─ If invalid → return 400 error with explanation
  ↓
[Membership] determine relationship action:
  ├─ JOIN: Create new relationship record
  ├─ PAUSE: Update state, set pause metadata
  ├─ RESUME: Update state, clear pause metadata
  └─ CANCEL: Update state, set end metadata
  ↓
[Membership] record state transition:
  ├─ Update membership.state
  ├─ Update membership.state_changed_at
  ├─ Append to membership.state_history (audit trail)
  └─ Set reason (user_initiated, payment_failed, etc.)
  ↓
[Membership] calculate loyalty metrics:
  ├─ Total months as member (lifetime)
  ├─ Consecutive months (streak, resets on cancel)
  ├─ Total lifetime value (sum of all payments)
  ├─ Loyalty tier (bronze, silver, gold based on tenure)
  └─ Churn risk score (if applicable)
  ↓
[Subscription] (if subscription action needed):
  ├─ JOIN → Already handled by subscription flow (not membership's job)
  ├─ PAUSE → Set subscription.paused_at, stop billing
  ├─ RESUME → Clear paused_at, resume billing
  └─ CANCEL → Set subscription.cancel_at_period_end = true
  ↓
[Content] (access implications):
  ├─ JOIN → No direct action (access granted by subscription flow)
  ├─ PAUSE → Access REMAINS active until period_end (supporter paid for period)
  ├─ RESUME → Access continues (was never revoked during pause)
  └─ CANCEL → Access REMAINS active until period_end (graceful exit)
  ↓
[Notification] send lifecycle emails:
  ├─ JOIN: "Welcome to [Creator]'s community!"
  ├─ PAUSE: "Your membership is paused, reactivates on [date]"
  ├─ RESUME: "Welcome back! Your membership is active again"
  └─ CANCEL: "We're sorry to see you go" + feedback survey
  ↓
[Notification] notify creator:
  ├─ JOIN: "New member: [Supporter]"
  ├─ PAUSE: "[Supporter] paused their membership"
  ├─ RESUME: "[Supporter] resumed their membership"
  └─ CANCEL: "[Supporter] cancelled their membership"
  ↓
[Reporting] update creator metrics:
  ├─ Active member count
  ├─ Paused member count
  ├─ Churn rate (cancellations / active members)
  ├─ Retention rate (members retained after 3/6/12 months)
  └─ Average lifetime value per member
  ↓
[Feed] (optional) update feed personalization:
  ├─ CANCEL → Reduce creator's content in supporter's feed
  ├─ RESUME → Restore creator's content prominence
  └─ JOIN → Add creator to supporter's followed creators
```

## Membership State Machine
```
States:
- active: Supporter is paying member, full access
- paused: Supporter temporarily paused, retains access until period_end
- cancelled: Supporter cancelled, retains access until period_end, then becomes past_member
- past_member: Former member, no longer paying, no access
- churned: Cancelled and period_end passed (subset of past_member)

Transitions:
┌─────────────────────────────────────────────────────────────┐
│  [none] ──join──> active                                    │
│                                                               │
│  active ──pause──> paused                                    │
│  paused ──resume──> active                                   │
│                                                               │
│  active ──cancel──> cancelled                                │
│  paused ──cancel──> cancelled                                │
│                                                               │
│  cancelled ──[period_end]──> churned (system transition)    │
│                                                               │
│  churned ──rejoin──> active (creates new relationship)      │
│  past_member ──rejoin──> active                             │
└─────────────────────────────────────────────────────────────┘

Invalid Transitions (blocked):
- active → churned (must go through cancelled first)
- paused → churned (must cancel first)
- cancelled → paused (cannot pause after cancel)
- cancelled → active (must rejoin, creates new relationship)
```

## Authority Guarantees
- **membership_relationships_contract:** sole owner of relationship state and social semantics
- **membership_relationships_contract:** does NOT control payment or access (observes, not enforces)
- **subscription_contract:** owns billing state (active, paused, cancelled)
- **content_distribution_contract:** owns access enforcement (membership only tracks)
- **notification_contract:** sends lifecycle emails, does NOT decide what happens
- **creator_reporting_contract:** tracks churn metrics, does NOT affect membership state

## Membership Actions & Business Rules

### 1. JOIN (Create Membership)
- **Trigger:** Supporter subscribes to creator for first time
- **Precondition:** No active membership between supporter and creator
- **Membership action:** 
  - Create membership record with state="active"
  - Set joined_at, first_payment_at
  - Initialize loyalty metrics (lifetime_value=0, tenure_months=0)
- **Subscription action:** Already handled by subscription contract (payment success flow)
- **Access action:** Already handled by content contract (subscription flow grants access)
- **Notification:** Welcome email to supporter, new member alert to creator
- **Reporting:** +1 active member count

### 2. PAUSE (Temporarily Pause Membership)
- **Trigger:** Supporter clicks "Pause membership" (not all platforms offer this)
- **Precondition:** Membership state = "active"
- **Membership action:**
  - Update membership.state = "paused"
  - Set paused_at, pause_reason (user_initiated, payment_failed_auto_pause)
  - Calculate resume_at (e.g., 1 month, 3 months, custom)
- **Subscription action:**
  - Set subscription.paused_at
  - Stop Stripe billing (pause subscription via API)
  - Retains access until current period_end
- **Access action:**
  - Access REMAINS active until period_end (supporter paid for current period)
  - After period_end: access revoked, no billing until resume
- **Notification:** Pause confirmation email, creator notified
- **Reporting:** -1 active member, +1 paused member
- **Policy:** Pause max duration = 6 months (auto-cancel after 6 months paused)

### 3. RESUME (Resume Paused Membership)
- **Trigger:** Supporter clicks "Resume membership" or scheduled resume_at date reached
- **Precondition:** Membership state = "paused"
- **Membership action:**
  - Update membership.state = "active"
  - Clear paused_at, resume_at
  - Record resumed_at
- **Subscription action:**
  - Clear subscription.paused_at
  - Resume Stripe billing (immediate charge or at next period)
  - Restart subscription cycle
- **Access action:**
  - If still within period_end: access already active (no change)
  - If past period_end: access restored immediately upon successful payment
- **Notification:** Resume confirmation email, creator notified
- **Reporting:** +1 active member, -1 paused member
- **Policy:** If payment fails on resume → transition to payment failure flow (grace period)

### 4. CANCEL (End Membership)
- **Trigger:** Supporter clicks "Cancel subscription"
- **Precondition:** Membership state = "active" or "paused"
- **Membership action:**
  - Update membership.state = "cancelled"
  - Set cancelled_at, cancellation_reason (user_initiated, payment_failed, refund, etc.)
  - Calculate churned_at = subscription.period_end (when access actually ends)
  - Record cancellation_survey_response (if supporter provided feedback)
- **Subscription action:**
  - Set subscription.cancel_at_period_end = true
  - Stripe subscription cancelled at period_end (not immediately)
  - No refund (supporter retains access until period_end)
- **Access action:**
  - Access REMAINS active until period_end (graceful exit)
  - After period_end: access revoked, state → churned
- **Notification:** 
  - Cancellation confirmation email (with feedback survey)
  - Creator notified (with cancellation reason if provided)
- **Reporting:** -1 active member (at period_end), churn_rate updated
- **Policy:** Supporter can resubscribe anytime (creates new relationship)

### 5. REJOIN (Resubscribe After Churn)
- **Trigger:** Churned supporter resubscribes
- **Precondition:** Membership state = "churned" or "past_member"
- **Membership action:**
  - Create NEW membership record (fresh start)
  - Link to previous relationship via previous_membership_id (for analytics)
  - Reset streak (consecutive_months = 1)
  - Retain lifetime_value history (add to previous)
- **Subscription action:** Handled by subscription contract (new subscription created)
- **Access action:** Handled by content contract (subscription flow grants access)
- **Notification:** "Welcome back!" email (different from initial join)
- **Reporting:** +1 active member, reactivation_rate metric updated
- **Policy:** Previous relationship remains in DB for analytics (not deleted)

### 6. AUTO-CHURN (System Transition)
- **Trigger:** Background job detects cancelled subscription with period_end <= now
- **Precondition:** Membership state = "cancelled" AND subscription.period_end <= now
- **Membership action:**
  - Update membership.state = "churned"
  - Set churned_at = now
  - Calculate final_lifetime_value
- **Subscription action:** Subscription already cancelled (no change)
- **Access action:** Access already revoked by content contract (subscription expired)
- **Notification:** None (supporter already notified at cancellation)
- **Reporting:** Churn metrics updated (final churn confirmation)
- **Frequency:** Background job runs every hour

## Idempotency Strategy

### Membership State Transition Level
- **Key:** f"membership_transition:{membership_id}:{target_state}:{timestamp}"
- **Storage:** membership_state_changes table with UNIQUE constraint
- **Protection:** Prevents duplicate state transitions if user clicks "Cancel" twice
- **Check:** if membership.state already matches target_state AND changed_at within 5 minutes → skip transition

### Loyalty Metrics Calculation Level
- **Key:** f"loyalty_metrics:{membership_id}:{calculation_date}"
- **Storage:** In-memory calculation, stored in membership table
- **Protection:** Metrics recalculated on every state change (idempotent calculation)
- **Check:** tenure_months = COUNT(payments) / payments_per_month

### Subscription Action Level (coordinated)
- **Key:** Subscription contract owns idempotency (membership just requests)
- **API call:** POST /subscriptions/{id}/pause with idempotency key
- **Protection:** Subscription contract handles duplicate pause/resume/cancel requests
- **Retry:** If subscription API call fails, membership transition to error state, retry async

### Notification Level
- **Key:** f"lifecycle_notification:{membership_id}:{action}:{recipient}"
- **Storage:** sent_notifications table
- **Protection:** Prevents duplicate "membership cancelled" emails
- **Check:** if notification sent within last 24 hours for same action → skip

### Reporting Update Level
- **Key:** f"membership_metrics:{creator_id}:{action}:{date}"
- **Storage:** reporting_update_log table
- **Protection:** Prevents double-counting cancellations in churn rate
- **Check:** if update already logged → skip metric recalculation

## Error Handling

| Failure Point | Root Cause | Compensation Strategy | Criticality |
|--------------|------------|----------------------|-------------|
| State validation fails | Invalid transition (e.g., active→churned) | Return 400 error with state machine explanation | SEV-3 |
| Membership state update fails | DB timeout, constraint violation | Retry 3x, then dead letter queue | SEV-2 |
| Subscription API call fails (pause/cancel) | Subscription service down, Stripe API timeout | Retry async (up to 3 attempts), alert ops | SEV-2 |
| Loyalty metrics calculation fails | Missing payment data, division by zero | Log error, use stale metrics, recalculate on next transition | SEV-3 |
| Notification fails | Email provider down | Queue retry (24hr window), do NOT block state transition | SEV-3 |
| Reporting update fails | Analytics DB lag | Eventually consistent, acceptable lag up to 1 hour | SEV-3 |
| Cancellation survey save fails | DB timeout | Async retry, acceptable to lose survey data (non-critical) | SEV-4 |

### Critical Rules
- **Membership state transition MUST succeed** before subscription API calls (membership is source of truth for intent)
- **Subscription API failure is acceptable** with async retry (eventual consistency)
- **Access is NOT membership's concern** (content contract handles based on subscription state)
- **Notification failure MUST NOT block** lifecycle transitions
- **If membership state update fails** → return error to user, do NOT proceed with subscription changes

## Observability Requirements

### Events Emitted (in sequence)

1. **MembershipActionInitiated** (from membership)
   - payload: {membership_id, supporter_id, creator_id, action: "join" | "pause" | "resume" | "cancel", initiated_by: "supporter" | "system", reason?}

2. **MembershipStateChanged** (from membership)
   - payload: {membership_id, prior_state, new_state, changed_at, reason, initiated_by}

3. **LoyaltyMetricsUpdated** (from membership)
   - payload: {membership_id, tenure_months, consecutive_months, lifetime_value, loyalty_tier, churn_risk_score?}

4. **SubscriptionActionRequested** (from membership → subscription)
   - payload: {subscription_id, action: "pause" | "resume" | "cancel", requested_by_membership_id}

5. **LifecycleNotificationSent** (from notification, 2 events)
   - payload: {membership_id, action, recipient: "supporter" | "creator", sent_at}

6. **MembershipMetricsUpdated** (from reporting)
   - payload: {creator_id, active_members_delta, paused_members_delta, churned_members_delta, churn_rate_new}

### Structured Logging
Each step logs:
- **correlation_id** (same across entire lifecycle action)
- **membership_id**
- **supporter_id** (user_id)
- **creator_id**
- **action** ("join" | "pause" | "resume" | "cancel" | "rejoin")
- **prior_state**
- **new_state**
- **reason** (user_initiated, payment_failed, etc.)
- **initiated_by** ("supporter" | "system" | "admin")
- **step_name**
- **step_status** (success | failed | compensated | skipped)
- **timestamp**

### Metrics (for alerting)

#### Real-time Metrics
- **membership_action_duration_seconds** (p50, p95, p99)
  - Alert if p99 > 5 seconds (supporter experience degraded)
  
- **membership_state_transition_failures** (total, by action)
  - Alert if > 10 failures in 1 hour
  
- **subscription_api_call_failure_rate** (failures / total calls)
  - Alert if > 1% (indicates subscription service issues)
  
- **invalid_state_transition_attempts** (blocked transitions / total attempts)
  - Baseline: expect < 0.1% (indicates UX bugs if higher)

#### Business Metrics
- **cancellation_rate** (cancellations / active members, 30-day rolling)
  - Baseline: expect 5-10% monthly churn for healthy subscriptions
  - Alert creator if > 15% (retention crisis)
  
- **pause_rate** (pauses / active members, 30-day rolling)
  - Track if pause feature is used (product insight)
  
- **resume_rate** (resumes / paused members, 30-day rolling)
  - Baseline: expect 30-50% of paused members to resume
  - Low resume rate → consider removing pause feature (users just cancel anyway)
  
- **rejoin_rate** (rejoins / churned members, 90-day window)
  - Baseline: expect 5-15% of churned members to rejoin within 3 months
  - Track winback campaign effectiveness
  
- **average_tenure_months** (sum(tenure) / member count, per creator)
  - Healthy: 12+ months average tenure
  - Alert creator if < 6 months (indicates retention issues)
  
- **lifetime_value_by_cohort** (LTV by join month, per creator)
  - Track if newer cohorts have lower LTV (quality decay signal)

#### Retention Metrics (key for creator health)
- **3-month retention** (members active at 3mo / members who joined 3mo ago)
  - Baseline: 60-70% for healthy communities
  
- **6-month retention** (same calculation, 6mo)
  - Baseline: 40-50%
  
- **12-month retention** (same calculation, 12mo)
  - Baseline: 20-30%

### Audit Trail

#### Memberships Table
- membership_id, supporter_id, creator_id, subscription_id (foreign key)
- state (active, paused, cancelled, churned, past_member)
- joined_at, first_payment_at, cancelled_at, churned_at
- paused_at, resume_at (if paused)
- cancellation_reason, cancellation_survey_response
- tenure_months, consecutive_months, lifetime_value, loyalty_tier
- previous_membership_id (if rejoin, links to prior relationship)

#### Membership State History (immutable log)
- state_change_id, membership_id
- prior_state, new_state, changed_at
- reason, initiated_by (supporter | system | admin)
- metadata (e.g., pause_duration, cancellation_feedback)

#### Loyalty Metrics Snapshots (for trend analysis)
- snapshot_id, membership_id, snapshot_date
- tenure_months, consecutive_months, lifetime_value, loyalty_tier
- churn_risk_score (ML-predicted, if available)

#### Cancellation Surveys (for product insights)
- survey_id, membership_id, submitted_at
- cancellation_reason (dropdown: too_expensive, content_quality, etc.)
- feedback_text (free form, optional)
- would_recommend (yes/no/maybe)

## Human Override Points

### Ops Can Intervene At:

#### 1. **Force membership state change**
- **Use case:** Membership stuck in incorrect state due to bug
- **Action:** Manually update membership.state via admin CRM
- **Authority:** Admin with "membership_override" permission
- **Validation:** Must provide reason (min 20 chars)
- **Side effects:** 
  - State history logged with admin_user_id
  - Subscription state NOT automatically changed (admin must coordinate)
  - Notification sent to supporter explaining change
- **Audit:** admin_user_id, override_reason, old_state, new_state

#### 2. **Extend grace period for cancellation**
- **Use case:** Supporter cancelled by mistake, wants to undo within 24 hours
- **Action:** Revert membership from "cancelled" to "active"
- **Authority:** Admin with "cancellation_reversal" permission OR supporter themselves (within 24h)
- **Validation:** Only allowed if period_end not yet passed
- **Side effects:**
  - Subscription.cancel_at_period_end set to false
  - Access remains active (was never revoked)
  - "Cancellation reversed" notification sent
- **Audit:** reverted_by (supporter_id | admin_user_id), reversal_reason

#### 3. **Manually adjust loyalty metrics**
- **Use case:** Loyalty metrics incorrect due to data migration bug
- **Action:** Directly update tenure_months, lifetime_value fields
- **Authority:** Admin with "loyalty_correction" permission
- **Validation:** Requires supporting documentation (e.g., payment receipts)
- **Side effects:** Loyalty tier recalculated, creator dashboard updated
- **Audit:** admin_user_id, correction_reason, old_values, new_values, supporting_docs

#### 4. **Grant "lifetime member" status**
- **Use case:** Creator wants to honor long-time supporter with permanent access
- **Action:** Set membership.lifetime_member = true, disable billing
- **Authority:** Creator themselves OR admin with "lifetime_grant" permission
- **Validation:** Requires minimum tenure (e.g., 24 months) OR creator approval
- **Side effects:**
  - Subscription cancelled, no future billing
  - Access remains active permanently (special access rule)
  - Badge displayed on supporter profile
- **Audit:** granted_by (creator_id | admin_user_id), grant_reason

#### 5. **Bulk cancel memberships (emergency)**
- **Use case:** Creator account suspended for TOS violation, cancel all memberships
- **Action:** Bulk update all active memberships to "cancelled"
- **Authority:** Admin with "bulk_operations" permission (requires dual approval)
- **Validation:** Requires executive approval for bulk operations
- **Side effects:**
  - All subscriptions cancelled immediately
  - All access revoked immediately (emergency override)
  - Refunds initiated for current period (prorated)
  - Notifications sent to all affected supporters
- **Audit:** admin_user_id, approver_id, bulk_reason, affected_count

### Audit Requirements
- All manual actions logged in memberships.admin_actions table
- Required fields: admin_user_id, action_type, reason (min 20 chars), timestamp
- For bulk operations: approver_id (second admin) required
- Immutable: cannot delete membership records (soft delete only, state="past_member")
- Retention: Indefinite (relationship history is valuable for analytics)

## Replay Safety

### Membership Action Replay
- **Scenario:** Supporter clicks "Cancel" button twice rapidly
- **Safety:** Check membership.state + state_changed_at before transition
- **Result:** 
  - If already cancelled within last 5 minutes → return success (idempotent)
  - If already cancelled > 5 minutes ago → return error "Already cancelled"

### State Transition Replay
- **Scenario:** State transition succeeds but response times out, retry triggered
- **Safety:** UNIQUE constraint on membership_state_changes table
- **Result:** Second state change fails with SQL error, error handler recognizes as duplicate

### Subscription API Call Replay
- **Scenario:** Pause request sent to subscription API, timeout, retry triggered
- **Safety:** Subscription API has own idempotency (membership passes idempotency key)
- **Result:** Subscription API returns success for duplicate request (already paused)

### Loyalty Metrics Recalculation Replay
- **Scenario:** Metrics calculation job runs twice
- **Safety:** Calculation is deterministic (same inputs → same outputs)
- **Result:** Metrics updated twice with same values (no harm)

### Notification Replay
- **Scenario:** Lifecycle notification job retries
- **Safety:** Idempotency key in sent_notifications table
- **Result:** No duplicate "membership cancelled" emails

## Edge Cases & Business Rules

### Pause Duration Limits
- **Minimum pause:** 1 month (no point pausing for less)
- **Maximum pause:** 6 months (auto-cancel after 6 months)
- **Enforcement:** When supporter selects pause duration, validate against limits
- **Auto-resume:** If pause_at specified, background job resumes membership automatically
- **Auto-cancel:** If paused for 6 months, background job transitions to cancelled

### Cancellation During Pause
- **Scenario:** Supporter pauses membership, then cancels during pause period
- **Policy:** Allowed, immediately transitions to cancelled
- **Access impact:** 
  - If still within original period_end: access retained until period_end
  - If past period_end: no access (was already paused without access)
- **Refund:** No refund (supporter already paid for period before pausing)

### Rejoin vs New Subscription
- **Scenario:** Supporter rejoins after churning
- **Policy:** Create NEW membership record, link to previous via previous_membership_id
- **Reason:** Clean slate for analytics, but retain history for LTV calculation
- **Loyalty metrics:**
  - lifetime_value: sum of old + new (cumulative)
  - consecutive_months: reset to 1 (streak broken)
  - tenure_months: starts at 1 (new relationship)

### Multiple Memberships to Same Creator
- **Scenario:** Supporter subscribes to multiple tiers from same creator
- **Policy:** Create separate membership records (one per subscription)
- **Analytics:** Track as separate relationships (supporter can have multiple motivations)
- **Access:** Content contract handles access (grants highest tier access)

### Membership After Refund
- **Scenario:** Supporter receives full refund, membership status?
- **Policy:** Membership state → cancelled (via refund flow coordination)
- **Access:** Revoked immediately (refund = immediate exit)
- **Lifetime value:** Adjusted (refund deducted from LTV)
- **Analytics:** Tracked separately (refund_churn vs voluntary_churn)

### Loyalty Tier Thresholds
- **Bronze:** 1-6 months tenure
- **Silver:** 7-12 months tenure
- **Gold:** 13-24 months tenure
- **Platinum:** 25+ months tenure
- **Benefits:** Creator can set perks per tier (e.g., Platinum members get shoutouts)
- **Calculation:** tenure_months = COUNT(successful payments) (not calendar months)

### Churn Risk Scoring (ML-based, optional)
- **Inputs:** Payment failure history, engagement (content views), tenure, tier
- **Output:** churn_risk_score (0.0-1.0, higher = more likely to churn)
- **Thresholds:**
  - < 0.3: low risk
  - 0.3-0.6: medium risk
  - > 0.6: high risk (creator should engage)
- **Update frequency:** Recalculated daily via batch job
- **Action:** Creator notified of high-risk members (retention opportunity)

### Membership During Dispute
- **Scenario:** Supporter files chargeback, membership status?
- **Policy:** 
  - Membership state → cancelled (via chargeback flow)
  - Access revoked immediately (chargeback = hostile exit)
  - Lifetime value adjusted (chargeback deducted)
- **Analytics:** Tracked as dispute_churn (different from voluntary)

### Scheduled Cancellation Notification
- **Scenario:** Supporter cancels (cancel_at_period_end = true), period_end is 20 days away
- **Policy:** Send reminder notification 7 days before access ends
- **Message:** "Your membership with [Creator] ends in 7 days. Resubscribe to keep access."
- **Frequency:** One reminder per cancellation (not daily spam)

### Membership for Free Tiers
- **Scenario:** Creator offers "free tier" (newsletter only, no payment)
- **Policy:** Create membership record with state="active", no subscription_id
- **Access:** Content contract grants access to "free tier" content
- **Lifecycle:** Join, cancel (no pause, no payment concepts)
- **Analytics:** Tracked separately (free vs paid memberships)

## Integration Points

### Subscription Contract
- **API:** POST /subscriptions/{id}/pause, /subscriptions/{id}/resume, /subscriptions/{id}/cancel
- **Coordination:** Membership initiates action, subscription executes billing change
- **Failure handling:** If subscription API fails, membership state rolled back (eventual consistency via retry)
- **Idempotency:** Membership passes idempotency_key to subscription API

### Content Contract
- **Dependency:** Content contract queries membership state for access decisions
- **API:** Content contract calls GET /memberships?supporter_id=X&creator_id=Y to check state
- **Policy:** Access granted if membership.state IN ('active', 'paused', 'cancelled') AND subscription.period_end >= now
- **Cache:** Content contract caches membership state (5min TTL, invalidated on state change)

### Notification Contract
- **API:** POST /notifications/lifecycle with membership_id, action, recipient
- **Templates:** Predefined templates for join, pause, resume, cancel
- **Personalization:** Include tenure_months, loyalty_tier in email for supporter recognition
- **Batching:** Creator notifications batched (daily digest, not real-time)

### Reporting Contract
- **API:** POST /reporting/membership-metrics with creator_id, action, delta
- **Aggregation:** Reporting contract aggregates metrics (active count, churn rate, etc.)
- **Frequency:** Real-time updates for active member count, daily batch for retention cohorts
- **Dashboard:** Creator dashboard displays metrics from reporting contract

### Cancellation Survey Service (optional)
- **API:** POST /surveys/cancellation with membership_id, survey_response
- **Storage:** Separate surveys table (not in membership contract)
- **Privacy:** Survey responses anonymized for aggregate analysis (not shown to creator individually)
- **Usage:** Product team analyzes cancellation reasons to improve platform

## Performance & Scale Considerations

### Database Load
- **Memberships table:** Indexed on (supporter_id, state), (creator_id, state), (state, churned_at)
- **State history:** Partitioned by month, indexed on (membership_id, changed_at)
- **Expected load:** 1,000 lifecycle actions/day at scale (much lower than payment/content flows)

### Subscription API Call Latency
- **Target:** p99 < 2 seconds for pause/resume/cancel API calls
- **Timeout:** 5 seconds (if subscription API doesn't respond, retry async)
- **Retry strategy:** Exponential backoff (1s, 2s, 4s), max 3 attempts

### Loyalty Metrics Calculation
- **Complexity:** O(n) where n = payment count (typically 1-100 per member)
- **Optimization:** Cache calculation results, recalculate only on state change
- **Batch recalculation:** Daily job recalculates all memberships (drift correction)

### Churn Risk Scoring (if ML-based)
- **Latency:** Acceptable to have 24-hour lag (daily batch job)
- **Computation:** ML model inference on batch of members (not real-time)
- **Cost:** Minimal (simple logistic regression or decision tree model)

### Reporting Aggregation
- **Real-time:** Active member count updated on every state change (fast)
- **Batch:** Retention cohorts calculated daily at midnight
- **Query optimization:** Materialized views for common creator dashboard queries

---

## Summary: Flow Characteristics

| Characteristic | Value |
|----------------|-------|
| **Duration** | 2-5 seconds (state transition + subscription API call) |
| **Cross-contract depth** | 5 contracts (membership, subscription, content, notification, reporting) |
| **Critical path** | Membership state update + subscription API call |
| **Async components** | Loyalty metrics, notification, reporting (eventual consistency) |
| **Human intervention frequency** | ~2% of cases (cancellation reversals, loyalty corrections) |
| **Replay risk** | Low (idempotency via state checks and DB constraints) |
| **Data consistency requirement** | Strong for membership state, eventual for subscription/metrics |
| **Volume** | 1,000 actions/day at scale (much lower than payment/content) |
| **SLA** | p99 < 5s for supporter-initiated actions |
| **Business impact** | Indirect (affects loyalty tracking + creator insights, not money directly) |
| **Analytics value** | Very high (churn analysis, retention cohorts, LTV tracking) |

---