# Identity Revocation Policy & Procedures

## 1. Purpose

This document defines how identities are revoked within the system in a secure,
auditable, and contract-compliant manner.

Identity revocation is a **security control**, not a business or product decision.
Its sole purpose is to prevent authentication and request execution by a specific
identity or group of identities.

---

## 2. Scope

This policy applies to:
- User identities
- Service identities
- Support and administrative identities

This policy does NOT govern:
- Subscription lifecycle changes
- Membership state transitions
- Content access entitlements
- Financial or ledger operations

---

## 3. Definitions

### Identity
A canonical, authenticated entity represented in the `identities` table and
resolved from an external authentication provider.

### Revoke
A permanent action that prevents an identity from authenticating or creating
new sessions.

### Suspend
A temporary action that prevents authentication but allows future restoration.

---

## 4. Identity States

| Status     | Meaning                              | Authentication |
|------------|--------------------------------------|----------------|
| active     | Identity is valid                    | Allowed        |
| suspended  | Temporarily disabled                 | Denied         |
| revoked    | Permanently disabled                 | Denied         |

Identities MUST NOT be deleted from the system.

---

## 5. Revocation Levels

### 5.1 Single Identity Revocation

**Use Cases**
- Account compromise
- Terms of service violation
- Employee or support agent offboarding

**Procedure**
1. Update identity status to `revoked`
2. Revoke all active sessions for the identity
3. Record audit metadata (actor, reason, timestamp)

**Effects**
- Immediate logout
- No new authentication allowed
- No business state is modified

---

### 5.2 Batch Identity Revocation

**Use Cases**
- Authentication provider compromise
- Automated abuse detection
- Internal credential leakage

**Procedure**
1. Determine the affected identity set
2. Revoke all selected identities
3. Revoke all associated sessions
4. Emit security alerts and audit logs

Decision logic MUST NOT live in the identity service.
The identity service executes revocation only.

---

### 5.3 System-wide Revocation (Emergency)

**Use Cases**
- Token signing key compromise
- Critical authentication infrastructure breach

**Procedure**
1. Revoke all identities
2. Revoke all active sessions
3. Disable authentication entry points
4. Rotate credentials and secrets
5. Restore identities selectively after remediation

---

## 6. What Revocation Does NOT Do

Revocation does NOT:
- Cancel subscriptions
- Modify memberships
- Grant or revoke content access
- Trigger refunds or chargebacks
- Emit business notifications automatically

Business consequences must be handled by their respective contracts.

---

## 7. Audit & Observability

Each revocation MUST be logged with:
- identity_id
- revoked_by (principal_id or system)
- reason (required)
- timestamp
- revocation scope (single | batch | global)

Recommended metrics:
- revoked_identity_count
- revocation_rate
- repeated_revocation_attempts

---

## 8. Safety Rules

- Revocation is fail-closed
- Revocation is idempotent
- Revocation must not cascade into business logic
- Session revocation must be immediate

---

## 9. Restoration Policy

Revoked identities MUST NOT be automatically restored.

Restoration requires:
- New identity issuance, or
- Explicit administrative override with justification and audit trail

---

## 10. Related Contracts

- identity_access_contract
- notification_contract (optional alerts)
- dispute_resolution_contract (for contested revocations)
