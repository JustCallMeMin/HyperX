# HyperX

**Contract-Compliant Event-Sourced Creator Membership Platform**

HyperX is a creator membership platform inspired by Patreon, designed around event sourcing, strict contract compliance, and explicit bounded context separation. The system prioritizes data integrity, auditability, and long-term evolvability over short-term convenience.

---

## Overview

HyperX enables creators to monetize content through tiered memberships while ensuring financial transparency, deterministic state reconstruction, and clear ownership of business responsibilities.

The platform is built to support correctness at scale: every state transition is event-driven, every read model is rebuildable, and every business boundary is enforced through contracts.

### Core Capabilities

- Subscription management with multiple membership tiers
- Creator economy with transparent, auditable ledger accounting
- Content access control based on subscription entitlements
- Real-time analytics and reporting through projections
- Multi-channel notification delivery
- Dispute and chargeback resolution workflows

---

## Architecture

### Event-Sourced Core

- Append-only events table as the single source of truth
- Projections for read-optimized queries, fully rebuildable from events
- Invariant views for cross-contract data integrity validation
- Reconciliation procedures for state recovery and consistency enforcement

### Bounded Contexts (9 Contracts)

1. Identity and Access – Authentication, authorization, principals
2. Core Subscription – Subscription lifecycle and payment state
3. Creator Economy – Ledger, wallets, payouts, fees
4. Content Distribution – Content visibility and access policies
5. Membership Relationships – Supporter–creator relationships and loyalty
6. Notification – Multi-channel message delivery
7. Dispute Resolution – Chargebacks and evidence handling
8. Creator Reporting – Revenue reporting and analytics
9. Feed and Discovery – Content feeds and creator discovery

---

## Quick Start

### Prerequisites

- Go 1.23 or later
- PostgreSQL 16 or later
- Docker 24 or later (optional, for local development)

### Local Development (Docker)

```
git clone <repository-url>
cd HyperX

cp .env.example .env

docker-compose up -d

./scripts/migrate.sh

curl http://localhost:8080/health/schema
```

### Manual Setup (Without Docker)

```
createdb hyperx

psql -d hyperx -f migrations/001_initial_schema.sql

psql -d hyperx -c "SELECT * FROM schema_health_check WHERE NOT is_passing;"

export DATABASE_URL="postgres://user:pass@localhost:5432/hyperx"
export SERVER_PORT=8080

go run cmd/server/main.go
```

---

## Project Structure

```
HyperX/
├── cmd/
│   └── server/
├── internal/
│   ├── config/
│   ├── database/
│   └── health/
├── pkg/
├── migrations/
│   └── 001_initial_schema.sql
├── docs/
│   ├── contracts/
│   ├── flows/
│   ├── events/
│   └── guardrails/
├── scripts/
│   ├── migrate.sh
│   ├── deploy.sh
│   └── dev.sh
├── docker/
│   ├── Dockerfile
│   └── docker-compose.yml
├── .env.example
├── README.md
└── go.mod
```

---

## Database Schema

### Core Tables (40 Total)

- Event store: 1
- Webhook idempotency: 1
- Identity and access: 4
- Subscriptions: 3
- Payments: 3
- Content distribution: 6
- Creator economy: 7
- Membership relationships: 6
- Notifications: 4
- Dispute resolution: 4
- Reporting: 6
- Feed and discovery: 4

### Invariant Views (13 Total)

- Core invariants for sessions, payments, access, ledger, and wallets
- Grace period SLA violations exceeding four hours
- Negative creator balances
- Active subscriptions without payment methods
- Payments without corresponding ledger entries

### Rebuild and Reconciliation Procedures

- reconcile_creator_wallet(creator_id)
- reconcile_all_wallets()
- mark_inconsistent_subscriptions()
- rebuild_membership_projection(membership_id)
- rebuild_membership_state_history(membership_id)
- rebuild_revenue_reports()

---

## Health Checks

### Endpoints

- GET /health – Liveness
- GET /health/ready – Dependency readiness
- GET /health/schema – Schema and invariant validation

### Schema Validation

```
SELECT * FROM schema_health_check WHERE NOT is_passing;
```

An empty result indicates a healthy system.

---

## Key Workflows

### Payment Success

Webhook ingestion → Idempotency validation → Event emission → Subscription state update → Access entitlement grant → Ledger credit → Notification dispatch → Reporting update

### Subscription Lifecycle

Created → Active → Grace Period → Cancelled or Expired

Inconsistent states are detected and resolved through reconciliation procedures.

### Content Access Resolution

Request → Identity and principal resolution → Entitlement evaluation → Tier gate enforcement → Access granted or denied

---

## Guardrails

### Event Sourcing Principles

- Events are immutable, append-only, past-tense facts
- Events are the single source of truth
- Projections are rebuildable and disposable
- Side effects are isolated behind translators
- All timestamps are stored in UTC

### Contract Authority Hierarchy

1. Contracts
2. Event definitions
3. Invariants and policies
4. Human instructions
5. AI-generated output

Detailed constraints are defined in the docs/guardrails directory.

---

## Security

- Identity-first request handling
- Principal-based authorization with explicit capabilities
- Fail-closed security model
- Full auditability through events
- Idempotent webhook processing

---

## Observability

### Metrics

- Payment success rate
- Subscription churn
- Creator earnings (gross and net)
- Access entitlement latency
- Invariant violation count

### Alerts

- Grace period SLA violations
- Negative creator balances
- Payments without ledger entries
- Webhook processing failures

---

## Deployment

### Production

```
./scripts/deploy.sh production
curl https://api.hyperx.com/health/ready
```

### Database Migrations

```
./scripts/migrate.sh
./scripts/migrate.sh validate
```

Refer to DEPLOYMENT.md for detailed deployment instructions.

---

## Documentation Index

- Contracts: docs/contracts
- Flows: docs/flows
- Events: docs/events
- Guardrails: docs/guardrails
- Invariants: docs/invariants.md
- Boundaries: BOUNDARIES.md

---

## Testing

```
go test ./...

go test -tags=integration ./...

psql -d hyperx -c "SELECT * FROM schema_health_check WHERE NOT is_passing;"
```

---

## Contributing

### Development Workflow

1. Create a feature branch
2. Implement changes in compliance with guardrails
3. Run tests and schema validation
4. Submit a pull request with contract impact documented

### Contract Changes

Contract modifications require updating the relevant contract definition, schema if necessary, event definitions, invariant views, and documentation.

---

## License

[Specify License]

---

## Acknowledgments

This project is built on principles from event sourcing, domain-driven design, contract-first architecture, and bounded context isolation. It is influenced by the creator economy model popularized by Patreon and foundational work by Greg Young and Vaughn Vernon.

---

**Contract-Compliant**

**Event-Sourced**

**Rebuildable by Design**

**Production-Ready**
