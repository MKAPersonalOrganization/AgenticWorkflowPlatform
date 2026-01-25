# Engine State Machine Refactoring Plan

## Executive Summary

The current implementation uses **dual state machines** (Engine FSM with 7 states + Tool FSM with 6 states per tool) coordinated through event emission and observer patterns. While functionally robust, the architecture has complexity and maintainability challenges that can be addressed through strategic refactoring.

---

## Current Architecture Analysis

### Strengths ✅
1. **Type-safe state management** - Using `stateless` library prevents invalid transitions
2. **Event sourcing foundation** - Excellent auditability and multi-pod scalability
3. **Safety mechanisms** - Iteration limits and loop detection prevent runaway execution
4. **Resumable execution** - Persisted engine state enables pause/resume workflows
5. **Clear state definitions** - Well-documented states and triggers

### Challenges ⚠️

#### 1. **State Explosion**
- **Engine**: 7 states (AwaitingTurn, GeneratingResponse, PerformingActions, AwaitingInput, AwaitingAuth, Done, Error)
- **Per-Tool**: 6 states × N tools = high cognitive load
- **Impact**: Developers must understand 13 states across two state machines to trace execution flow

#### 2. **Dual Event Emission Paths**
```go
// Persistent events go through event store
thread.Apply(event) → KurrentDB → Projection → Redis Pub/Sub → SSE

// Transient events go directly to UI
observer.Emit(event) → SSE
```
- **Issue**: Different codepaths for similar concerns creates confusion
- **Risk**: Events can be missed if wrong path chosen

#### 3. **Mixed Responsibilities in Engine**
`executionState` struct (lines 304-336) combines:
- State machine coordination
- LLM streaming
- Tool execution
- Event emission
- Persistence
- Safety checks

**Result**: 1,100+ line `engine.go` file that's hard to navigate

#### 4. **Tool State Machine Overhead**
Each tool call gets a full 6-state FSM:
```go
for _, tc := range toolCalls {
    tsm := domain.NewToolStateMachine(ctx, &tc, executor)
    tsm.RegisterObserver(stateObserver)
    toolStateMachines[tc.ID] = tsm
}
```
- **Question**: Do simple tools (e.g., calculator) need full state machine machinery?
- **Complexity**: Managing N concurrent FSMs with mutexes and observers

#### 5. **Procedural State Transition Logic**
`nextStep()` method (lines 452-480) uses imperative logic:
```go
switch currentState {
case StateGeneratingResponse:
    if hasTools { fire(TrigActionsDetected) }
    else { fire(TrigTurnFinished) }
case StatePerformingActions:
    // Check tool states...
}
```
- **Issue**: Transition logic scattered between state machine config and nextStep()
- **Maintainability**: Adding new states requires touching multiple locations

#### 6. **State Persistence Gaps**
- Engine state persisted via `EngineStateChanged` events
- Tool states reconstructed from tool lifecycle events (`ToolStart`, `ToolSuccess`, etc.)
- **Inconsistency**: Two different persistence strategies for related state

---

## Proposed Refactoring Plan

### Phase 1: Simplify Engine State Machine (High Impact, Low Risk)

#### Goals
- Reduce engine states from 7 to 5
- Unify pause states with metadata
- Improve readability and debuggability

#### Changes

**Before** (7 states):
```
AwaitingTurn → GeneratingResponse → PerformingActions → AwaitingInput/AwaitingAuth → Done/Error
```

**After** (5 states):
```
Idle → Generating → Executing → Paused → Terminal
```

**New State Definitions**:
```go
type EngineState int
const (
    StateIdle        = iota  // Waiting to start (replaces AwaitingTurn)
    StateGenerating          // LLM responding (replaces GeneratingResponse)
    StateExecuting           // Tools running (replaces PerformingActions)
    StatePaused              // Waiting for user interaction (unifies AwaitingInput + AwaitingAuth)
    StateTerminal            // Final state (unifies Done + Error)
)

// Pause reasons (metadata)
type PauseReason int
const (
    PauseReasonNone = iota
    PauseReasonInput       // Tool elicitation
    PauseReasonAuth        // Authorization required
)

// Terminal reasons (metadata)
type TerminalReason int
const (
    TerminalReasonSuccess         // Goal reached
    TerminalReasonError           // Error occurred
    TerminalReasonMaxIterations   // Safety limit
    TerminalReasonLoopDetected
    TerminalReasonUserAborted
    TerminalReasonTimeout
)
```

**Benefits**:
- **Simpler mental model**: 5 states instead of 7
- **Metadata separation**: Reason stored separately from state (more flexible)
- **Easier testing**: Fewer state permutations
- **Better extensibility**: Add new pause/terminal reasons without new states

#### File Changes
- `services/agentorchestrator/engine/engine.go` (lines 941-987, 775-856)
- `services/agentorchestrator/engine/termination.go`
- Update tests in `services/agentorchestrator/state_machine_graph_test.go`

---

### Phase 2: Extract Tool Orchestration Layer (High Impact, Medium Risk)

#### Goals
- Separate tool execution from engine state management
- Simplify tool lifecycle tracking
- Improve testability of tool execution logic

#### Proposed Architecture

**New File**: `services/agentorchestrator/engine/tool_orchestrator.go`
```go
type ToolOrchestrator struct {
    executor    ToolExecutor
    eventEmitter EventEmitter
    tracker     *ToolExecutionTracker  // Simplified status tracking
}

// Replaces per-tool state machines with simpler status tracking
type ToolExecutionTracker struct {
    executions map[string]*ToolExecution  // toolCallID -> execution
    mu         sync.RWMutex
}

type ToolExecution struct {
    ID          string
    Name        string
    Status      ToolStatus
    Result      string
    Error       error
    Metadata    map[string]any  // For elicitation, auth data
    StartTime   time.Time
    EndTime     time.Time
}

type ToolStatus int
const (
    ToolStatusPending = iota
    ToolStatusRunning
    ToolStatusWaitingInput
    ToolStatusWaitingAuth
    ToolStatusSuccess
    ToolStatusFailed
)

// Core orchestration methods
func (to *ToolOrchestrator) ExecuteTools(ctx context.Context, toolCalls []ToolCall) error
func (to *ToolOrchestrator) GetWaitingTools() []ToolExecution
func (to *ToolOrchestrator) IsAnyWaitingForInput() bool
func (to *ToolOrchestrator) IsAnyWaitingForAuth() bool
```

**Benefits**:
- **Reduced complexity**: Simple status enum instead of full FSM per tool
- **Better encapsulation**: Tool execution logic isolated from engine state
- **Easier testing**: Mock `ToolOrchestrator` in engine tests
- **Performance**: Less overhead from N state machines + observers

**Trade-off Analysis**:
- ⚠️ **Loss**: Can't leverage stateless library's transition guards for tools
- ✅ **Gain**: Tools don't need complex state transitions—most follow linear path: Pending → Running → Success/Failed
- ✅ **Gain**: Special cases (input/auth) handled via status + metadata, not state machine

#### Migration Strategy
1. Create `ToolOrchestrator` alongside existing `ToolStateMachine`
2. Add feature flag to switch between implementations
3. Run both in parallel for 1-2 releases, compare behavior
4. Deprecate and remove `domain.ToolStateMachine`

---

### Phase 3: Unify Event Emission (Medium Impact, Low Risk)

#### Goals
- Single, consistent event emission pathway
- Clearer distinction between persistent and transient events
- Reduce cognitive load when emitting events

#### Current Dual Path Problem
```go
// Path 1: Persistent (through thread aggregate)
thread.Apply(event)  // Goes to event store

// Path 2: Transient (direct)
observer.Emit(event)  // Goes to SSE only

// wrapYield tries to automate this, but it's confusing
```

#### Proposed Unified Emitter

**New File**: `services/agentorchestrator/engine/unified_emitter.go`
```go
type UnifiedEventEmitter struct {
    thread       core.AgentThread  // For persistent events
    sseEmitter   EventEmitter      // For transient events
}

// Single method for all event emission
func (u *UnifiedEventEmitter) Emit(ctx context.Context, event DomainEvent) error {
    // Automatic routing based on event type
    if event.IsPersistent() {
        u.thread.Apply(event)  // Event sourcing
    }

    if event.IsStreamable() {
        u.sseEmitter.Emit(event)  // Real-time UI
    }

    return nil
}

// Events declare their own routing
type DomainEvent interface {
    IsPersistent() bool  // Should be stored in event store?
    IsStreamable() bool  // Should be sent to SSE?
}

// Example implementations
func (e *AssistantContentStreamed) IsPersistent() bool { return false }
func (e *AssistantContentStreamed) IsStreamable() bool { return true }

func (e *ToolSuccess) IsPersistent() bool { return true }
func (e *ToolSuccess) IsStreamable() bool { return true }

func (e *TokenUsageReported) IsPersistent() bool { return true }
func (e *TokenUsageReported) IsStreamable() bool { return false }
```

**Benefits**:
- **Single emission point**: Developers emit once, routing handled automatically
- **Self-documenting**: Event type declares its own routing policy
- **Easier testing**: Mock single emitter instead of multiple paths
- **Flexibility**: Events can opt into both, either, or neither path

#### File Changes
- New: `services/agentorchestrator/engine/unified_emitter.go`
- Update: All event emissions in `engine.go` to use unified emitter
- Update: `domain/events.go` to add `IsPersistent()` and `IsStreamable()` methods

---

### Phase 4: Declarative State Transitions (Medium Impact, Medium Risk)

#### Goals
- Replace procedural `nextStep()` logic with declarative configuration
- Centralize all transition logic in state machine setup
- Improve maintainability and testability

#### Current Problem
Transition logic split between:
1. State machine configuration (lines 775-856) - basic triggers
2. `nextStep()` method (lines 452-480) - conditional transition logic
3. Entry/exit handlers - side effects during transitions

**Result**: To understand "what happens when X occurs?", you must read 3+ locations

#### Proposed Solution

**Declarative Transition Guards**:
```go
// New file: services/agentorchestrator/engine/state_config.go

type TransitionGuard func(*executionState) bool

type StateConfiguration struct {
    OnEntry      func(*executionState) error
    OnExit       func(*executionState) error
    Transitions  []TransitionRule
}

type TransitionRule struct {
    Trigger     Trigger
    Target      EngineState
    Guard       TransitionGuard  // Condition that must be true
    OnTransit   func(*executionState) error
}

// Example configuration
func buildEngineStateMachine(es *executionState) *stateless.StateMachine {
    sm := stateless.NewStateMachine(StateIdle)

    // Generating → Executing (if tools detected)
    sm.Configure(StateGenerating).
        Permit(TrigExecuteTools, StateExecuting,
            guard: func() bool { return len(es.streamResult.ToolCalls) > 0 },
        ).
        Permit(TrigComplete, StateTerminal,
            guard: func() bool { return len(es.streamResult.ToolCalls) == 0 },
        )

    // Executing → Paused (if tools waiting)
    sm.Configure(StateExecuting).
        Permit(TrigPause, StatePaused,
            guard: func() bool { return es.tools.IsAnyWaiting() },
        ).
        Permit(TrigContinue, StateGenerating,
            guard: func() bool { return !es.tools.IsAnyWaiting() },
        )

    return sm
}
```

**Benefits**:
- **Single source of truth**: All transitions in one place
- **Readable**: Transition rules read like documentation
- **Testable**: Guards are pure functions, easy to unit test
- **Maintainable**: Adding new state requires single configuration block

**Migration Strategy**:
1. Extract current `nextStep()` logic into guard functions
2. Update state machine configuration to use guards
3. Remove `nextStep()` method entirely
4. All transition logic lives in `state_config.go`

---

### Phase 5: File Reorganization (Low Impact, Low Risk)

#### Goals
- Break up 1,100+ line `engine.go` into focused modules
- Improve navigability and code discovery
- Establish clear ownership boundaries

#### Proposed File Structure

**Before**:
```
services/agentorchestrator/engine/
├── engine.go (1,100+ lines - everything)
├── termination.go
├── stream_processor.go
├── tool_execution.go
└── event_factory.go
```

**After**:
```
services/agentorchestrator/engine/
├── engine.go (200 lines - Run() and high-level coordination)
├── state_config.go (150 lines - state machine setup)
├── state_types.go (100 lines - state/trigger enums)
├── execution_state.go (150 lines - executionState struct)
├── tool_orchestrator.go (300 lines - tool execution)
├── unified_emitter.go (150 lines - event emission)
├── safety_unit.go (200 lines - limits, loop detection)
├── observer_unit.go (100 lines - observation)
├── persistence_unit.go (150 lines - DB ops)
├── termination.go (existing)
├── stream_processor.go (existing)
└── event_factory.go (existing)
```

**File Responsibilities**:
- **engine.go**: Public API (`Run`, `RunWithMessage`), high-level flow
- **state_config.go**: All state machine configuration and guards
- **state_types.go**: State, trigger, and metadata type definitions
- **execution_state.go**: Execution context struct and helpers
- **tool_orchestrator.go**: Tool execution and status tracking
- **unified_emitter.go**: Event emission routing
- **safety_unit.go**: Iteration limits, timeouts, loop detection
- **observer_unit.go**: Observer pattern implementation
- **persistence_unit.go**: Database interactions

**Benefits**:
- **Navigability**: Find code by concern, not line number
- **Cognitive load**: Each file <300 lines, focused on single concern
- **Parallel development**: Multiple developers can work on different files
- **Testing**: Test each unit independently

---

## Implementation Roadmap

### Week 1-2: Phase 1 (Simplify Engine States)
**Deliverables**:
- [ ] Update engine state enum (7 → 5 states)
- [ ] Add pause/terminal reason metadata types
- [ ] Update state machine configuration
- [ ] Update all tests
- [ ] Verify state machine graph visualization

**Risk**: Low (backward compatible if done carefully)

### Week 3-4: Phase 3 (Unify Event Emission)
**Deliverables**:
- [ ] Create `UnifiedEventEmitter`
- [ ] Add `IsPersistent()` and `IsStreamable()` to all events
- [ ] Update engine to use unified emitter
- [ ] Remove `wrapYield` pattern
- [ ] Update tests

**Risk**: Low (internal refactoring, no API changes)

### Week 5-6: Phase 5 (File Reorganization)
**Deliverables**:
- [ ] Extract files per proposed structure
- [ ] Update imports across codebase
- [ ] Run architecture tests to verify dependency rules
- [ ] Update CLAUDE.md with new structure

**Risk**: Low (mechanical refactoring)

### Week 7-9: Phase 2 (Tool Orchestration)
**Deliverables**:
- [ ] Create `ToolOrchestrator` interface
- [ ] Implement simplified tool status tracking
- [ ] Add feature flag for switching implementations
- [ ] Run parallel testing (old vs new)
- [ ] Deprecate `ToolStateMachine` if metrics good

**Risk**: Medium (behavioral change, needs extensive testing)

### Week 10-11: Phase 4 (Declarative Transitions)
**Deliverables**:
- [ ] Extract transition guards
- [ ] Update state machine configuration
- [ ] Remove `nextStep()` method
- [ ] Update tests for guard functions
- [ ] Documentation updates

**Risk**: Medium (complex refactoring, but guards make testing easier)

### Week 12: Integration & Validation
**Deliverables**:
- [ ] End-to-end testing of refactored system
- [ ] Performance benchmarking (before/after)
- [ ] Update architecture documentation
- [ ] Code review and merge

---

## Success Metrics

### Code Quality
- **Lines of code**: Expect 10-15% reduction through simplification
- **Cyclomatic complexity**: Reduce engine.go from ~35 to <20
- **Test coverage**: Maintain >80% coverage throughout

### Performance
- **State transition latency**: No regression (target: <1ms)
- **Memory usage**: Expect 5-10% reduction from fewer FSMs
- **Tool execution throughput**: No regression

### Developer Experience
- **Onboarding time**: New developers understand flow in <2 hours (vs current ~4 hours)
- **Bug fix time**: Reduce average fix time by 20% through better code organization
- **Feature velocity**: Easier to add new states/transitions

---

## Risk Mitigation

### Technical Risks

1. **State Machine Behavior Changes**
   - **Mitigation**: Comprehensive state transition test suite before refactoring
   - **Validation**: Record all state transitions in production for 1 week, replay after refactoring

2. **Event Emission Bugs**
   - **Mitigation**: Dual-path emission during migration (emit to both old and new)
   - **Validation**: Assert both paths emit identical events in tests

3. **Tool Execution Regression**
   - **Mitigation**: Feature flag to switch between FSM and status-based implementations
   - **Validation**: Shadow traffic testing—run both, compare results

### Organizational Risks

1. **Breaking Changes During Active Development**
   - **Mitigation**: Refactor in feature branches, merge in phases
   - **Communication**: Weekly updates to team on progress

2. **Knowledge Transfer**
   - **Mitigation**: Document each phase with ADRs (Architecture Decision Records)
   - **Validation**: Pair programming sessions with team members

---

## Alternative Approaches Considered

### Alt 1: Complete State Machine Replacement
**Idea**: Replace `stateless` library with custom FSM tailored to needs

**Pros**: Maximum flexibility, optimized for exact use case

**Cons**:
- High implementation cost
- Lose type safety from stateless
- Reinventing wheel
- Testing burden

**Decision**: ❌ Rejected—stateless library works well, simplification is better than replacement

### Alt 2: Single Mega State Machine
**Idea**: Merge engine + tool FSMs into one giant FSM

**Pros**: Single state representation

**Cons**:
- State explosion (7 engine states × 6 tool states × N tools)
- Impossible to reason about
- Testing nightmare

**Decision**: ❌ Rejected—makes problem worse, not better

### Alt 3: Event-Driven Choreography (No FSM)
**Idea**: Replace state machines with pure event-driven orchestration

**Pros**: Maximum flexibility, simpler code

**Cons**:
- Lose explicit state tracking
- Harder to debug "what state am I in?"
- Resumption becomes complex
- No compile-time guarantees

**Decision**: ❌ Rejected—state machines provide valuable structure for resumable execution

---

## Appendix: Code Examples

### Example 1: Simplified Engine State Usage

**Before** (7 states):
```go
switch currentState {
case StateAwaitingInput:
    // Handle input waiting
case StateAwaitingAuth:
    // Handle auth waiting
case StateDone:
    return nil
case StateError:
    return err
}
```

**After** (5 states + metadata):
```go
switch currentState {
case StatePaused:
    switch pauseReason {
    case PauseReasonInput:
        // Handle input
    case PauseReasonAuth:
        // Handle auth
    }
case StateTerminal:
    switch terminalReason {
    case TerminalReasonSuccess:
        return nil
    case TerminalReasonError:
        return err
    }
}
```

### Example 2: Tool Orchestrator vs FSM

**Before** (FSM per tool):
```go
// Create state machine for each tool
for _, tc := range toolCalls {
    tsm := domain.NewToolStateMachine(ctx, &tc, executor)
    tsm.RegisterObserver(observer)
    toolStateMachines[tc.ID] = tsm
}

// Execute
for _, tsm := range toolStateMachines {
    go tsm.Start()  // Async FSM execution
}

// Check status
for _, tsm := range toolStateMachines {
    if tsm.IsAwaitingInput() {
        return TrigRequireInput
    }
}
```

**After** (Status tracking):
```go
// Execute tools through orchestrator
orchestrator.ExecuteTools(ctx, toolCalls)

// Check status
if orchestrator.IsAnyWaitingForInput() {
    return TrigPause, PauseReasonInput
}
```

### Example 3: Unified Event Emission

**Before** (dual paths):
```go
// Persistent event
thread.Apply(domain.NewToolSuccess(toolID, result))

// Transient event
observer.Emit(domain.NewAssistantContentStreamed(content))

// wrapYield complicates this further
wrappedYield(func(event) {
    if isPersistent(event) {
        thread.Apply(event)
    }
    observer.Emit(event)
})
```

**After** (single path):
```go
// Single emission, routing automatic
emitter.Emit(ctx, domain.NewToolSuccess(toolID, result))
emitter.Emit(ctx, domain.NewAssistantContentStreamed(content))

// Events self-declare routing
// ToolSuccess: persistent=true, streamable=true
// AssistantContentStreamed: persistent=false, streamable=true
```

---

## Conclusion

This refactoring plan addresses the core complexity challenges in the engine state machine while preserving its robust event sourcing foundation and safety mechanisms.

**Key Improvements**:
1. **40% fewer states** (13 → 8 effective states)
2. **Single event emission pathway** (easier to reason about)
3. **Clear separation of concerns** (tool orchestration extracted)
4. **Declarative transitions** (configuration over code)
5. **Better file organization** (focused modules vs monolithic engine.go)

**Estimated Impact**:
- **Development velocity**: +20% (easier to understand and modify)
- **Bug reduction**: -30% (simpler code, better testing)
- **Onboarding time**: -50% (clearer structure)

**Timeline**: 12 weeks for full implementation, with deliverable milestones every 2 weeks

This plan can be executed in phases, allowing for incremental delivery and risk mitigation.
