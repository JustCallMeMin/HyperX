# Context Boundaries - Compile-Time Enforcement

## Rule: No Cross-Context Imports

Contexts in `internal/` **MUST NOT** import from other contexts.

Allowed imports:
- `hyperx/pkg/*` (shared primitives)
- Own context packages
- Standard library
- External dependencies

**FORBIDDEN:**
```go
import "hyperx/internal/subscription"
import "hyperx/internal/economy"
```

## Communication Patterns

### 1. Events (Primary)
Contexts communicate via events published through `pkg/eventbus`.

```go
bus.Publish(ctx, paymentSucceededEvent)
```

### 2. Ports (Interfaces)
Contexts expose interfaces in their `ports/` directory.

```go
import "hyperx/internal/economy/ports"

var ledgerRepo economy.ports.LedgerRepository
```

### 3. Events Only (Preferred)
Prefer events over direct port calls for cross-context communication.

## Enforcement

This is enforced by:
1. Go's `internal/` package visibility rules
2. Build-time checks (future: linter)
3. Code review discipline

## Violations

If you find yourself importing another context:
- **STOP**
- Ask: "Can this be an event instead?"
- If not, define a port interface in the target context
- Never import domain/application/projection from another context
