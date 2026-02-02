# ADR 003: Event Classification - Ephemeral vs. Persistent

## Status
Accepted

## Context
Events serve different purposes. Some represent business state requiring audit and replay. Others are transient signals for real-time coordination. Without clear classification, systems either bloat storage with everything or lose critical audit trails by storing nothing.

## Decision

Events are classified by **purpose**, not technical characteristics:

### Ephemeral Events (Transient)

**Purpose**: Real-time signaling, execution coordination
**Lifecycle**: Emitted once, consumed immediately, never replayed
**Storage**: Broadcast via pub/sub, never persisted
**Examples**: RunStarted, RunFinished, RunError, streaming indicators

**Constraint**: MUST NOT be persisted. System MUST recover without them.

### Persistent Events (Durable)

**Purpose**: Domain state changes, business facts, audit trail
**Lifecycle**: Persisted permanently, required for state reconstruction
**Storage**: Event store first, then broadcast
**Examples**: MessageAdded, ThreadCreated, tool results, user actions

**Constraint**: MUST be persisted before broadcast. Event store is source of truth.

## Classification Criteria

Apply in order:

1. Needed to reconstruct domain state? → **Persistent**
2. Needed for audit/compliance? → **Persistent**
3. Only relevant during active execution? → **Ephemeral**
4. Business-critical information? → **Persistent**
5. Otherwise → **Ephemeral**

## Patterns

**Ephemeral:**
- Execution lifecycle (start/stop/error)
- Progress indicators (streaming, percentages)
- Transient status updates

**Persistent:**
- Entity lifecycle (create/update/delete)
- Business transactions
- State transitions

## Consequences

**Positive:**
- Clear classification eliminates ambiguity
- Reduced storage costs and complexity
- System state reconstructable from persistent events alone

**Negative:**
- Historical ephemeral events unavailable for debugging
- Every new event requires classification decision

**Trade-off:** Storage efficiency over complete observability history

## Constraints

1. Events are EITHER ephemeral OR persistent, never both
2. Classification explicit in code
3. System recovers correctly using only persistent events
4. Consumers handle missed ephemeral events gracefully
5. Classification SHOULD NOT change after initial assignment

## Related ADRs
- ADR 001: Agent Framework Architectural Boundary
- ADR 002: Run Lifecycle Events and Service Layer Orchestration
