# ADR 004: Standardization of Pointer-based Domain Events

## Status
Accepted

## Context
Domain events in the event-sourcing architecture are used for both state reconstruction (aggregates) and inter-service coordination. During the implementation of tool results and long-running agent loops, a technical mismatch was discovered:
1. Deserialization logic in infrastructure layers (KurrentDB) often yields pointers to structs.
2. Type-switch handlers in aggregate `Apply` methods and orchestration logic often matched on value types (`case UserMessageAdded:` instead of `case *UserMessageAdded:`).
3. This mismatch led to silent failures where crucial state changes (like tool results) were ignored by aggregates, resulting in inconsistent state and data loss during history reconstruction.

## Decision
All domain events MUST be handled as **pointers** throughout the system lifecycle, from creation to persistence and state application.

### Implementation Constraints

1. **Aggregates**: All `Apply(event Event)` methods MUST use pointer types in their type switches (e.g., `case *UserMessageAdded:`).
2. **Persistence**: Deserialization logic (e.g., in `KurrentDBEventStore`) MUST return pointers to event structs cast to the `domain.Event` interface.
3. **Creation**: When enqueuing events in an aggregate (e.g., `thread.Enqueue(&event, ...)`), a pointer to the event MUST be passed to ensure consistent type identification.
4. **Protocols**: All event-related interfaces (`domain.Event`, `core.Event`) are implemented by pointers to structs.
5.  **Interface Enforcement**: All `domain.Event` interface methods (`EventType`, `EventTimestamp`, `AggregateID`, `sealed`) will be implemented using **pointer receivers**. This ensures that only pointers to event structs implement the `Event` interface, providing compile-time enforcement of this decision.

## Consequences

### Positive
- **Type Safety**: Eliminates the "missing case" bug where an event exists but is ignored due to pointer/value mismatch in type switches.
- **Consistency**: All layers (Domain, Infrastructure, Adapters) share the same semantic understanding of event types.
- **Performance**: Passing pointers avoids copying large event payloads (e.g., message content or tool results).
- **Extensibility**: Pointer receivers on event methods are now strictly supported without ambiguity.

### Negative
- **Null Safety**: All handlers must be aware that an event interface might contain a nil pointer (though standard orchestration prevents this).
- **Test Updates**: Existing unit tests that used value-based assertions must be updated to use pointer-based assertions.

## Related ADRs
- ADR 001: Agent Framework Architectural Boundary
- ADR 003: Event Classification - Ephemeral vs. Persistent
