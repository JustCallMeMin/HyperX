-- ============================================================
-- Event Store Schema
-- Single source of truth for all events
-- ============================================================
CREATE TABLE events (
  event_id UUID PRIMARY KEY,
  event_type VARCHAR NOT NULL,
  aggregate_type VARCHAR NOT NULL,
  aggregate_id UUID NOT NULL,
  schema_version INT NOT NULL,
  occurred_at_utc TIMESTAMP NOT NULL,
  causation_id UUID,
  correlation_id UUID,
  idempotency_key VARCHAR,
  actor_type VARCHAR,
  actor_identity_id UUID,
  origin_event_type VARCHAR,
  payload_json JSONB NOT NULL
);

COMMENT ON TABLE events IS
'Immutable event store - append only, never updated or deleted';

CREATE INDEX idx_events_aggregate 
ON events(aggregate_type, aggregate_id, occurred_at_utc);

CREATE INDEX idx_events_type_time 
ON events(event_type, occurred_at_utc);

CREATE INDEX idx_events_causation 
ON events(causation_id);

CREATE INDEX idx_events_correlation 
ON events(correlation_id);

CREATE UNIQUE INDEX uniq_events_idempotency 
ON events(idempotency_key) 
WHERE idempotency_key IS NOT NULL;

-- ============================================================
-- Identity Access Tables
-- ============================================================
CREATE TABLE identities (
  identity_id UUID PRIMARY KEY,
  identity_type VARCHAR NOT NULL,
  provider VARCHAR NOT NULL,
  provider_subject VARCHAR NOT NULL,
  status VARCHAR NOT NULL,
  created_at_utc TIMESTAMP NOT NULL
);

CREATE UNIQUE INDEX uniq_identity_provider
ON identities(provider, provider_subject);

CREATE TABLE principals (
  principal_id UUID PRIMARY KEY,
  identity_id UUID NOT NULL,
  principal_type VARCHAR NOT NULL,
  scope VARCHAR NOT NULL,
  created_at_utc TIMESTAMP NOT NULL
);

CREATE INDEX idx_principals_identity ON principals(identity_id);

CREATE TABLE role_assignments (
  principal_id UUID NOT NULL,
  role VARCHAR NOT NULL,
  scope VARCHAR NOT NULL,
  granted_at_utc TIMESTAMP NOT NULL
);

CREATE UNIQUE INDEX uniq_role_assignment
ON role_assignments(principal_id, role, scope);

CREATE TABLE session_contexts (
  session_id UUID PRIMARY KEY,
  identity_id UUID NOT NULL,
  principal_id UUID NOT NULL,
  issued_at_utc TIMESTAMP NOT NULL,
  expires_at_utc TIMESTAMP NOT NULL,
  revoked_at_utc TIMESTAMP
);

CREATE INDEX idx_sessions_identity ON session_contexts(identity_id);
CREATE INDEX idx_sessions_principal ON session_contexts(principal_id);

-- ============================================================
-- Core Subscription Tables
-- ============================================================
CREATE TABLE subscription_tiers (
  tier_id UUID PRIMARY KEY,
  creator_identity_id UUID NOT NULL,
  tier_name VARCHAR NOT NULL,
  billing_interval VARCHAR NOT NULL,
  price_amount_cents INT NOT NULL,
  currency VARCHAR NOT NULL,
  created_at_utc TIMESTAMP NOT NULL,
  archived_at_utc TIMESTAMP
);

COMMENT ON TABLE subscription_tiers IS
'Tier changes never rewrite history';

CREATE TABLE subscriptions (
  subscription_id UUID PRIMARY KEY,
  supporter_identity_id UUID NOT NULL,
  creator_identity_id UUID NOT NULL,
  current_tier_id UUID,
  state VARCHAR NOT NULL,
  started_at_utc TIMESTAMP NOT NULL,
  current_period_start_utc TIMESTAMP,
  current_period_end_utc TIMESTAMP,
  cancel_at_period_end BOOLEAN NOT NULL DEFAULT FALSE,
  cancelled_at_utc TIMESTAMP,
  projection_version INT NOT NULL,
  updated_at_utc TIMESTAMP NOT NULL
);

COMMENT ON TABLE subscriptions IS
'One row = current derived state (projection)';

CREATE INDEX idx_subscriptions_supporter ON subscriptions(supporter_identity_id);
CREATE INDEX idx_subscriptions_creator ON subscriptions(creator_identity_id);

CREATE TABLE billing_cycles (
  billing_cycle_id UUID PRIMARY KEY,
  subscription_id UUID NOT NULL,
  cycle_start_utc TIMESTAMP NOT NULL,
  cycle_end_utc TIMESTAMP NOT NULL,
  created_at_utc TIMESTAMP NOT NULL
);

CREATE INDEX idx_billing_cycles_subscription ON billing_cycles(subscription_id);

-- ============================================================
-- Payment Tables
-- ============================================================
CREATE TABLE payment_events (
  payment_event_id UUID PRIMARY KEY,
  event_id UUID NOT NULL,
  subscription_id UUID NOT NULL,
  provider VARCHAR NOT NULL,
  provider_event_id VARCHAR NOT NULL,
  event_type VARCHAR NOT NULL,
  amount_cents INT NOT NULL,
  currency VARCHAR NOT NULL,
  occurred_at_utc TIMESTAMP NOT NULL,
  CONSTRAINT fk_payment_events_event FOREIGN KEY (event_id) REFERENCES events(event_id)
);

COMMENT ON TABLE payment_events IS
'Projection from events table - denormalized for query performance';

CREATE UNIQUE INDEX uniq_payment_provider_event
ON payment_events(provider, provider_event_id);

CREATE UNIQUE INDEX uniq_payment_events_event_id
ON payment_events(event_id);

CREATE INDEX idx_payment_events_subscription
ON payment_events(subscription_id);

CREATE TABLE payment_attempts (
  payment_attempt_id UUID PRIMARY KEY,
  subscription_id UUID NOT NULL,
  provider VARCHAR NOT NULL,
  provider_payment_intent_id VARCHAR,
  attempt_number INT NOT NULL,
  amount_cents INT NOT NULL,
  currency VARCHAR NOT NULL,
  attempted_at_utc TIMESTAMP NOT NULL,
  status VARCHAR NOT NULL,
  failure_reason VARCHAR
);

CREATE INDEX idx_payment_attempts_subscription
ON payment_attempts(subscription_id);

-- ============================================================
-- Access Entitlements
-- ============================================================
CREATE TABLE access_entitlements (
  entitlement_id UUID PRIMARY KEY,
  subscription_id UUID NOT NULL,
  tier_id UUID NOT NULL,
  access_start_utc TIMESTAMP NOT NULL,
  access_end_utc TIMESTAMP,
  granted_by_event_id UUID NOT NULL,
  revoked_by_event_id UUID,
  created_at_utc TIMESTAMP NOT NULL,
  CONSTRAINT fk_access_granted_event FOREIGN KEY (granted_by_event_id) REFERENCES events(event_id),
  CONSTRAINT fk_access_revoked_event FOREIGN KEY (revoked_by_event_id) REFERENCES events(event_id)
);

COMMENT ON TABLE access_entitlements IS
'Access is time-bounded and replayable';

CREATE INDEX idx_access_subscription
ON access_entitlements(subscription_id);

-- ============================================================
-- Content Distribution
-- ============================================================
CREATE TABLE creator_content (
  content_id UUID PRIMARY KEY,
  creator_identity_id UUID NOT NULL,
  content_type VARCHAR NOT NULL,
  status VARCHAR NOT NULL,
  created_at_utc TIMESTAMP NOT NULL,
  published_at_utc TIMESTAMP,
  archived_at_utc TIMESTAMP
);

COMMENT ON TABLE creator_content IS
'Logical content unit; owns publication lifecycle only';

CREATE TABLE content_assets (
  asset_id UUID PRIMARY KEY,
  content_id UUID NOT NULL,
  asset_type VARCHAR NOT NULL,
  storage_reference VARCHAR NOT NULL,
  created_at_utc TIMESTAMP NOT NULL
);

COMMENT ON TABLE content_assets IS
'Assets inherit visibility from parent content';

CREATE INDEX idx_content_assets_content
ON content_assets(content_id);

CREATE TABLE visibility_policies (
  policy_id UUID PRIMARY KEY,
  content_id UUID NOT NULL,
  visibility_type VARCHAR NOT NULL,
  effective_from_utc TIMESTAMP NOT NULL
);

COMMENT ON TABLE visibility_policies IS
'Policies are declarative and time-aware';

CREATE INDEX idx_visibility_content
ON visibility_policies(content_id);

CREATE TABLE tier_gate_rules (
  tier_gate_rule_id UUID PRIMARY KEY,
  policy_id UUID NOT NULL,
  tier_id UUID NOT NULL,
  minimum_required BOOLEAN NOT NULL,
  effective_from_utc TIMESTAMP NOT NULL
);

COMMENT ON TABLE tier_gate_rules IS
'Consumes tier_id from subscription contract';

CREATE INDEX idx_tier_gate_policy
ON tier_gate_rules(policy_id);

CREATE TABLE access_resolution_logs (
  resolution_id UUID PRIMARY KEY,
  content_id UUID NOT NULL,
  principal_id UUID NOT NULL,
  resolution_result VARCHAR NOT NULL,
  reason VARCHAR NOT NULL,
  resolved_at_utc TIMESTAMP NOT NULL
);

COMMENT ON TABLE access_resolution_logs IS
'Observability only; never used for decisions';

CREATE TABLE content_visibility_projections (
  projection_id UUID PRIMARY KEY,
  content_id UUID NOT NULL,
  principal_id UUID NOT NULL,
  is_visible BOOLEAN NOT NULL,
  computed_at_utc TIMESTAMP NOT NULL
);

COMMENT ON TABLE content_visibility_projections IS
'Derived cache; safe to rebuild';

-- ============================================================
-- Creator Economy
-- ============================================================
CREATE TABLE ledger_entries (
  ledger_entry_id UUID PRIMARY KEY,
  creator_identity_id UUID NOT NULL,
  source_payment_event_id UUID,
  entry_type VARCHAR NOT NULL,
  amount_cents INT NOT NULL,
  currency VARCHAR NOT NULL,
  occurred_at_utc TIMESTAMP NOT NULL
);

COMMENT ON TABLE ledger_entries IS
'Ledger entries are immutable and authoritative';

CREATE INDEX idx_ledger_creator_time
ON ledger_entries(creator_identity_id, occurred_at_utc);

CREATE TABLE creator_wallets (
  creator_identity_id UUID PRIMARY KEY,
  available_balance_cents INT NOT NULL,
  pending_balance_cents INT NOT NULL,
  currency VARCHAR NOT NULL,
  last_computed_at_utc TIMESTAMP NOT NULL
);

COMMENT ON TABLE creator_wallets IS
'Derived from ledger_entries; rebuildable';

CREATE TABLE settlement_records (
  settlement_record_id UUID PRIMARY KEY,
  payment_event_id UUID NOT NULL,
  creator_identity_id UUID NOT NULL,
  settlement_status VARCHAR NOT NULL,
  settled_at_utc TIMESTAMP,
  note VARCHAR
);

COMMENT ON TABLE settlement_records IS
'Operational visibility only; derived from ledger';

CREATE TABLE platform_fee_policies (
  policy_id UUID PRIMARY KEY,
  fee_percentage DECIMAL NOT NULL,
  flat_fee_cents INT NOT NULL,
  effective_from_utc TIMESTAMP NOT NULL,
  effective_to_utc TIMESTAMP
);

COMMENT ON TABLE platform_fee_policies IS
'Policy changes never rewrite history';

CREATE TABLE applied_fee_snapshots (
  snapshot_id UUID PRIMARY KEY,
  payment_event_id UUID NOT NULL,
  policy_id_used UUID NOT NULL,
  fee_percentage_used DECIMAL NOT NULL,
  flat_fee_used_cents INT NOT NULL,
  applied_at_utc TIMESTAMP NOT NULL
);

COMMENT ON TABLE applied_fee_snapshots IS
'Exact fee parameters used for this payment';

CREATE TABLE payout_batches (
  payout_batch_id UUID PRIMARY KEY,
  currency VARCHAR NOT NULL,
  scheduled_at_utc TIMESTAMP NOT NULL,
  executed_at_utc TIMESTAMP,
  status VARCHAR NOT NULL
);

COMMENT ON TABLE payout_batches IS
'Execution container; must be idempotent';

CREATE TABLE payout_items (
  payout_item_id UUID PRIMARY KEY,
  payout_batch_id UUID NOT NULL,
  creator_identity_id UUID NOT NULL,
  amount_cents INT NOT NULL,
  currency VARCHAR NOT NULL,
  status VARCHAR NOT NULL,
  executed_at_utc TIMESTAMP
);

-- ============================================================
-- Membership Relationships
-- ============================================================
CREATE TABLE memberships (
  membership_id UUID PRIMARY KEY,
  supporter_identity_id UUID NOT NULL,
  creator_identity_id UUID NOT NULL,
  subscription_id UUID NOT NULL,
  current_state VARCHAR NOT NULL,
  joined_at_utc TIMESTAMP NOT NULL,
  first_payment_at_utc TIMESTAMP,
  paused_at_utc TIMESTAMP,
  resume_at_utc TIMESTAMP,
  cancelled_at_utc TIMESTAMP,
  churned_at_utc TIMESTAMP,
  previous_membership_id UUID
);

COMMENT ON TABLE memberships IS
'One row = current relationship state (projection)';

CREATE TABLE membership_state_history (
  state_change_id UUID PRIMARY KEY,
  membership_id UUID NOT NULL,
  prior_state VARCHAR,
  new_state VARCHAR NOT NULL,
  reason VARCHAR NOT NULL,
  initiated_by VARCHAR NOT NULL,
  changed_at_utc TIMESTAMP NOT NULL
);

COMMENT ON TABLE membership_state_history IS
'Append-only, replayable';

CREATE TABLE membership_loyalty_metrics (
  membership_id UUID PRIMARY KEY,
  tenure_months INT NOT NULL,
  consecutive_months INT NOT NULL,
  lifetime_value_cents INT NOT NULL,
  loyalty_tier VARCHAR,
  churn_risk_score DECIMAL,
  last_computed_at_utc TIMESTAMP NOT NULL
);

CREATE TABLE engagement_signals (
  signal_id UUID PRIMARY KEY,
  membership_id UUID NOT NULL,
  signal_type VARCHAR NOT NULL,
  occurred_at_utc TIMESTAMP NOT NULL
);

CREATE TABLE cancellation_surveys (
  survey_id UUID PRIMARY KEY,
  membership_id UUID NOT NULL,
  cancellation_reason VARCHAR,
  feedback_text VARCHAR,
  would_recommend VARCHAR,
  submitted_at_utc TIMESTAMP NOT NULL
);

CREATE TABLE membership_admin_actions (
  admin_action_id UUID PRIMARY KEY,
  membership_id UUID NOT NULL,
  admin_identity_id UUID NOT NULL,
  action_type VARCHAR NOT NULL,
  reason VARCHAR NOT NULL,
  performed_at_utc TIMESTAMP NOT NULL
);

-- ============================================================
-- Notification
-- ============================================================
CREATE TABLE notification_intents (
  intent_id UUID PRIMARY KEY,
  source_event_id UUID NOT NULL,
  recipient_identity_id UUID NOT NULL,
  notification_type VARCHAR NOT NULL,
  created_at_utc TIMESTAMP NOT NULL,
  CONSTRAINT fk_notification_source_event FOREIGN KEY (source_event_id) REFERENCES events(event_id)
);

CREATE UNIQUE INDEX uniq_notification_intent
ON notification_intents(source_event_id, recipient_identity_id, notification_type);

CREATE TABLE notification_templates (
  template_id UUID PRIMARY KEY,
  notification_type VARCHAR NOT NULL,
  channel VARCHAR NOT NULL,
  locale VARCHAR NOT NULL,
  version INT NOT NULL,
  created_at_utc TIMESTAMP NOT NULL
);

CREATE UNIQUE INDEX uniq_notification_template
ON notification_templates(notification_type, channel, locale, version);

CREATE TABLE notification_outbox (
  outbox_id UUID PRIMARY KEY,
  intent_id UUID NOT NULL,
  channel VARCHAR NOT NULL,
  provider VARCHAR NOT NULL,
  status VARCHAR NOT NULL,
  created_at_utc TIMESTAMP NOT NULL,
  last_attempted_at_utc TIMESTAMP
);

CREATE UNIQUE INDEX uniq_outbox_intent_channel
ON notification_outbox(intent_id, channel);

CREATE TABLE delivery_attempts (
  attempt_id UUID PRIMARY KEY,
  outbox_id UUID NOT NULL,
  provider_response VARCHAR,
  success BOOLEAN NOT NULL,
  attempted_at_utc TIMESTAMP NOT NULL
);

-- ============================================================
-- Dispute Resolution
-- ============================================================
CREATE TABLE dispute_cases (
  dispute_id UUID PRIMARY KEY,
  dispute_type VARCHAR NOT NULL,
  initiator_identity_id UUID NOT NULL,
  subject_reference VARCHAR NOT NULL,
  current_state VARCHAR NOT NULL,
  created_at_utc TIMESTAMP NOT NULL,
  closed_at_utc TIMESTAMP
);

CREATE TABLE dispute_events (
  dispute_event_id UUID PRIMARY KEY,
  event_id UUID NOT NULL,
  dispute_id UUID NOT NULL,
  event_type VARCHAR NOT NULL,
  actor_type VARCHAR NOT NULL,
  occurred_at_utc TIMESTAMP NOT NULL,
  CONSTRAINT fk_dispute_events_event FOREIGN KEY (event_id) REFERENCES events(event_id)
);

COMMENT ON TABLE dispute_events IS
'Projection from events table - denormalized for query performance';

CREATE UNIQUE INDEX uniq_dispute_events_event_id
ON dispute_events(event_id);

CREATE TABLE evidence_artifacts (
  artifact_id UUID PRIMARY KEY,
  dispute_id UUID NOT NULL,
  artifact_type VARCHAR NOT NULL,
  reference VARCHAR NOT NULL,
  submitted_at_utc TIMESTAMP NOT NULL
);

CREATE TABLE resolution_decisions (
  decision_id UUID PRIMARY KEY,
  dispute_id UUID NOT NULL,
  decision_type VARCHAR NOT NULL,
  decided_by VARCHAR NOT NULL,
  decided_at_utc TIMESTAMP NOT NULL
);

-- ============================================================
-- Reporting
-- ============================================================
CREATE TABLE revenue_reports (
  report_id UUID PRIMARY KEY,
  creator_identity_id UUID NOT NULL,
  time_period VARCHAR NOT NULL,
  currency VARCHAR NOT NULL,
  gross_earnings_cents INT NOT NULL,
  platform_fees_cents INT NOT NULL,
  net_earnings_cents INT NOT NULL,
  refund_amount_cents INT NOT NULL,
  chargeback_amount_cents INT NOT NULL,
  computed_at_utc TIMESTAMP NOT NULL
);

CREATE TABLE payout_status_reports (
  report_id UUID PRIMARY KEY,
  creator_identity_id UUID NOT NULL,
  currency VARCHAR NOT NULL,
  pending_payout_amount_cents INT NOT NULL,
  last_payout_at_utc TIMESTAMP,
  failed_payout_count INT NOT NULL,
  computed_at_utc TIMESTAMP NOT NULL
);

CREATE TABLE content_performance_reports (
  report_id UUID PRIMARY KEY,
  content_id UUID NOT NULL,
  creator_identity_id UUID NOT NULL,
  time_period VARCHAR NOT NULL,
  views INT NOT NULL,
  unique_viewers INT NOT NULL,
  engagement_events INT NOT NULL,
  computed_at_utc TIMESTAMP NOT NULL
);

CREATE TABLE supporter_activity_reports (
  report_id UUID PRIMARY KEY,
  creator_identity_id UUID NOT NULL,
  time_period VARCHAR NOT NULL,
  active_supporters INT NOT NULL,
  new_supporters INT NOT NULL,
  paused_supporters INT NOT NULL,
  churned_supporters INT NOT NULL,
  retention_rate DECIMAL NOT NULL,
  average_tenure_months DECIMAL NOT NULL,
  computed_at_utc TIMESTAMP NOT NULL
);

CREATE TABLE report_explanations (
  explanation_id UUID PRIMARY KEY,
  report_id UUID NOT NULL,
  source_type VARCHAR NOT NULL,
  source_reference_id UUID NOT NULL,
  note VARCHAR
);

CREATE TABLE report_rebuild_runs (
  rebuild_run_id UUID PRIMARY KEY,
  report_type VARCHAR NOT NULL,
  triggered_by VARCHAR NOT NULL,
  started_at_utc TIMESTAMP NOT NULL,
  completed_at_utc TIMESTAMP,
  status VARCHAR NOT NULL
);

-- ============================================================
-- Feed & Discovery
-- ============================================================
CREATE TABLE feed_items (
  feed_item_id UUID PRIMARY KEY,
  feed_type VARCHAR NOT NULL,
  content_id UUID NOT NULL,
  creator_identity_id UUID NOT NULL,
  rank_score DECIMAL NOT NULL,
  computed_at_utc TIMESTAMP NOT NULL
);

CREATE TABLE feed_computation_runs (
  computation_run_id UUID PRIMARY KEY,
  feed_type VARCHAR NOT NULL,
  triggered_by VARCHAR NOT NULL,
  started_at_utc TIMESTAMP NOT NULL,
  completed_at_utc TIMESTAMP,
  status VARCHAR NOT NULL
);

CREATE TABLE discovery_signals (
  signal_id UUID PRIMARY KEY,
  content_id UUID NOT NULL,
  signal_type VARCHAR NOT NULL,
  occurred_at_utc TIMESTAMP NOT NULL
);

CREATE TABLE creator_follows (
  follower_identity_id UUID NOT NULL,
  creator_identity_id UUID NOT NULL,
  followed_at_utc TIMESTAMP NOT NULL
);

CREATE UNIQUE INDEX uniq_creator_follow
ON creator_follows(follower_identity_id, creator_identity_id);

-- ============================================================
-- Invariant Views
-- ============================================================
-- I1. Session trỏ tới identity không tồn tại
CREATE OR REPLACE VIEW invariant_invalid_sessions AS
SELECT s.session_id, s.identity_id
FROM session_contexts s
LEFT JOIN identities i ON s.identity_id = i.identity_id
WHERE i.identity_id IS NULL;

-- I2. Duplicate payment event từ provider (should be empty)
CREATE OR REPLACE VIEW invariant_duplicate_payment_events AS
SELECT provider, provider_event_id, COUNT(*) AS occurrences
FROM payment_events
GROUP BY provider, provider_event_id
HAVING COUNT(*) > 1;

-- I3. Subscription active/grace nhưng KHÔNG có access hợp lệ
CREATE OR REPLACE VIEW invariant_active_subscription_without_access AS
SELECT s.subscription_id
FROM subscriptions s
LEFT JOIN access_entitlements a
  ON a.subscription_id = s.subscription_id
 AND now() BETWEEN a.access_start_utc
               AND COALESCE(a.access_end_utc, now())
WHERE s.state IN ('active', 'grace_period')
  AND a.entitlement_id IS NULL;

-- I4. Access không truy được về event (updated to check events table)
CREATE OR REPLACE VIEW invariant_access_without_event AS
SELECT a.entitlement_id, a.granted_by_event_id
FROM access_entitlements a
LEFT JOIN events e
  ON a.granted_by_event_id = e.event_id
WHERE e.event_id IS NULL;

-- I5. Payment event không có trong events table
CREATE OR REPLACE VIEW invariant_payment_event_missing_in_events AS
SELECT pe.payment_event_id, pe.event_id
FROM payment_events pe
LEFT JOIN events e ON pe.event_id = e.event_id
WHERE e.event_id IS NULL;

-- I6. Dispute event không có trong events table
CREATE OR REPLACE VIEW invariant_dispute_event_missing_in_events AS
SELECT de.dispute_event_id, de.event_id
FROM dispute_events de
LEFT JOIN events e ON de.event_id = e.event_id
WHERE e.event_id IS NULL;

-- I7. Ledger entry yêu cầu source nhưng thiếu
CREATE OR REPLACE VIEW invariant_ledger_without_source AS
SELECT ledger_entry_id
FROM ledger_entries
WHERE entry_type IN ('payment_credit', 'refund_debit', 'chargeback_debit')
  AND source_payment_event_id IS NULL;

-- I8. Wallet không khớp ledger
CREATE OR REPLACE VIEW invariant_wallet_mismatch AS
SELECT w.creator_identity_id,
       w.available_balance_cents + w.pending_balance_cents AS wallet_total,
       l.ledger_total
FROM creator_wallets w
JOIN (
  SELECT creator_identity_id,
         SUM(amount_cents) AS ledger_total
  FROM ledger_entries
  GROUP BY creator_identity_id
) l ON w.creator_identity_id = l.creator_identity_id
WHERE w.available_balance_cents + w.pending_balance_cents <> l.ledger_total;

-- I9. Membership hiện tại nhưng không có history
CREATE OR REPLACE VIEW invariant_membership_without_history AS
SELECT m.membership_id
FROM memberships m
LEFT JOIN membership_state_history h
  ON m.membership_id = h.membership_id
WHERE h.membership_id IS NULL;

-- ============================================================
-- Reconciliation Procedures
-- ============================================================
-- R1 — Rebuild Wallet từ Ledger
CREATE OR REPLACE PROCEDURE reconcile_creator_wallet(p_creator_id UUID)
LANGUAGE plpgsql
AS $$
BEGIN
  UPDATE creator_wallets w
  SET available_balance_cents = l.total,
      pending_balance_cents = 0,
      last_computed_at_utc = now()
  FROM (
    SELECT creator_identity_id,
           SUM(amount_cents) AS total
    FROM ledger_entries
    WHERE creator_identity_id = p_creator_id
    GROUP BY creator_identity_id
  ) l
  WHERE w.creator_identity_id = l.creator_identity_id;
END;
$$;

-- R2 — Rebuild ALL Wallets
CREATE OR REPLACE PROCEDURE reconcile_all_wallets()
LANGUAGE plpgsql
AS $$
BEGIN
  UPDATE creator_wallets w
  SET available_balance_cents = l.total,
      pending_balance_cents = 0,
      last_computed_at_utc = now()
  FROM (
    SELECT creator_identity_id,
           SUM(amount_cents) AS total
    FROM ledger_entries
    GROUP BY creator_identity_id
  ) l
  WHERE w.creator_identity_id = l.creator_identity_id;
END;
$$;

-- R3 — Mark Inconsistent Subscriptions
CREATE OR REPLACE PROCEDURE mark_inconsistent_subscriptions()
LANGUAGE plpgsql
AS $$
BEGIN
  UPDATE subscriptions
  SET state = 'inconsistent',
      updated_at_utc = now()
  WHERE subscription_id IN (
    SELECT subscription_id
    FROM invariant_active_subscription_without_access
  );
END;
$$;

-- R4 — Rebuild Membership Projection
CREATE OR REPLACE PROCEDURE rebuild_membership_projection(p_membership_id UUID)
LANGUAGE plpgsql
AS $$
BEGIN
  -- Placeholder:
  -- 1. Load membership_state_history ordered by changed_at_utc
  -- 2. Apply state machine in application code
  -- 3. Update memberships table
  RAISE NOTICE 'Membership % must be rebuilt by application-level reconciler', p_membership_id;
END;
$$;

-- R5 — Reporting Rebuild
CREATE OR REPLACE PROCEDURE rebuild_revenue_reports()
LANGUAGE plpgsql
AS $$
BEGIN
  TRUNCATE revenue_reports;

  INSERT INTO revenue_reports (
    report_id,
    creator_identity_id,
    time_period,
    currency,
    gross_earnings_cents,
    platform_fees_cents,
    net_earnings_cents,
    refund_amount_cents,
    chargeback_amount_cents,
    computed_at_utc
  )
  SELECT
    gen_random_uuid(),
    creator_identity_id,
    'all_time',
    currency,
    SUM(CASE WHEN amount_cents > 0 THEN amount_cents ELSE 0 END),
    SUM(CASE WHEN entry_type = 'platform_fee' THEN -amount_cents ELSE 0 END),
    SUM(amount_cents),
    SUM(CASE WHEN entry_type = 'refund_debit' THEN -amount_cents ELSE 0 END),
    SUM(CASE WHEN entry_type = 'chargeback_debit' THEN -amount_cents ELSE 0 END),
    now()
  FROM ledger_entries
  GROUP BY creator_identity_id, currency;
END;
$$;
