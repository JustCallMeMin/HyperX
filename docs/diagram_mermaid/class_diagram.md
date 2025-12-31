classDiagram
direction LR

class Subscription {
  +UUID subscriptionId
  +SubscriptionState state
  +Tier currentTier
  +int projectionVersion
  +Instant updatedAtUtc
  +void incrementVersion()
}

class PaymentEvent {
  +UUID eventId
  +UUID subscriptionId
  +PaymentEventType type
  +Instant occurredAtUtc
  +int schemaVersion
}

class AccessEntitlement {
  +UUID entitlementId
  +UUID subscriptionId
  +Tier tier
  +Instant startAtUtc
  +Instant expiresAtUtc
}

class InvariantViolation {
  +UUID violationId
  +InvariantType type
  +boolean isResolved
  +Instant detectedAtUtc
}

Subscription "1" --* "many" PaymentEvent : payment_history
Subscription "1" --o "1" AccessEntitlement : access
Subscription "1" --o "many" InvariantViolation : audit
