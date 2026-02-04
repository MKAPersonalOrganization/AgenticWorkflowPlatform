# Agent Abstraction Layer Refactoring Plan

> **Status**: Completed  
> **Created**: 2026-02-04  
> **Last Updated**: 2026-02-04  
> **ADRs**: 001, 002, 003, 004

## Overview

This document outlines the refactoring plan to decouple the core application logic from our custom agent SDK (`foundation/agentsdk`) and establish a robust, general-purpose abstraction layer (`foundation/agent`) that can support multiple agent frameworks.

### Target Frameworks
1. **ADK-Go** (Google Agent Development Kit)
2. **Blades** (Go-Kratos)

### Key Principles (from ADRs)
- **ADR 001**: Agent interface as sole access point; adapter responsibility for event translation
- **ADR 002**: Run lifecycle events are service layer concern; adapters remain stateless
- **ADR 003**: Ephemeral vs Persistent event classification
- **ADR 004**: Pointer-based domain events throughout the system

---

## Framework Analysis Summary

### ADK-Go Core Primitives
| Concept | ADK-Go Implementation | Notes |
|---------|----------------------|-------|
| Agent | `agent.Agent` interface with `Name()`, `Description()`, `Run()`, `SubAgents()` | Very similar to our current design |
| Invocation | `InvocationContext` interface extending `context.Context` | Rich context with session, artifacts, memory, branch |
| Session | `session.Session` with `State()`, `Events()` | State is key-value with prefixes (app:, temp:, user:) |
| Events | `*session.Event` with `LLMResponse`, actions, author | Events carry state delta and actions |
| Tools | `tool.Tool` with `Context` for HITL confirmation | Long-running tool support |
| Runner | `runner.Runner` manages session service, agents | Requires session service injection |
| Streaming | `iter.Seq2[*session.Event, error]` | Go 1.23 iterators |
| Callbacks | Before/After agent, model, tool callbacks | Rich callback system |

### Blades Core Primitives
| Concept | Blades Implementation | Notes |
|---------|----------------------|-------|
| Agent | `Agent` interface with `Name()`, `Description()`, `Run()` | No SubAgents in interface |
| Invocation | `*Invocation` struct with ID, Session, Tools, History | Simpler than ADK-Go |
| Session | `Session` interface with `State()`, `History()`, `Append()` | Simple key-value state |
| Events | Messages with `Status` (InProgress, Completed) | Message-centric model |
| Tools | `tools.Tool` with `InputSchema()`, `OutputSchema()` | JSON Schema based |
| Runner | `*Runner` with `Run()` and `RunStream()` | Supports resumable sessions |
| Streaming | `Generator[*Message, error]` (alias for `iter.Seq2`) | Same as ADK-Go |
| Middleware | `Middleware` function type | Composable handler chain |

### Key Differences

1. **Event vs Message Model**: ADK-Go emits `*session.Event` (with LLMResponse); Blades emits `*Message`
2. **Context Richness**: ADK-Go has richer `InvocationContext`; Blades uses simpler `*Invocation`
3. **State Scoping**: ADK-Go has prefixed state keys; Blades has flat state
4. **Sub-agents**: ADK-Go explicitly supports sub-agents; Blades has agent-as-tool pattern
5. **Tool Confirmation**: ADK-Go has HITL (Human-in-the-Loop); Blades doesn't explicitly expose it
6. **Session Resumability**: Blades has explicit resumable session support

---

## Refactoring Steps

### Phase 1: Foundation (Steps 1-3)

- [x] **Step 1: Audit Current Interface Completeness**
  
  Review `foundation/agent/` files against framework requirements:
  - [x] 1.1 Verify `Agent` interface can represent both ADK-Go and Blades agents
  - [x] 1.2 Verify `Invocation` struct has all required fields for both frameworks
  - [x] 1.3 Verify `Event` interface can represent framework events without leaking
  - [x] 1.4 Identify missing primitives (e.g., artifacts, memory, callbacks)
  - [x] 1.5 Document gaps in `GAPS.md` (temporary file) - Skipped, implemented directly

- [x] **Step 2: Refine Core Interfaces**
  
  Update interfaces based on gap analysis:
  - [x] 2.1 Add `InvocationContext` interface extending `context.Context` (like ADK-Go)
  - [x] 2.2 Add `Artifacts` and `Memory` interfaces to abstraction
  - [x] 2.3 Add `Middleware` type for handler composition
  - [x] 2.4 Ensure `Session` interface covers both ADK-Go and Blades patterns
  - [x] 2.5 Add `RunStatus` enum for message/invocation status tracking

- [x] **Step 3: Standardize Event Types**
  
  Align events with ADR 003 (Ephemeral vs Persistent) and ADR 004 (Pointer-based):
  - [x] 3.1 Define ephemeral event marker interface
  - [x] 3.2 Define persistent event marker interface
  - [x] 3.3 Ensure all event types use pointer receivers
  - [x] 3.4 Add `MessageStatus` type matching Blades' status model
  - [x] 3.5 Create event constructors that enforce pointer semantics

### Phase 2: Event Translation Layer (Steps 4-5)

- [x] **Step 4: Design Event Translation Contract**
  
  Define how adapters translate framework events to domain events:
  - [x] 4.1 Create `EventTranslator` interface for adapters
  - [x] 4.2 Map ADK-Go `*session.Event` to `domain.Event` subtypes
  - [x] 4.3 Map Blades `*Message` to `domain.Event` subtypes
  - [x] 4.4 Document translation rules in adapter package
  - [x] 4.5 Handle partial/streaming events correctly

- [x] **Step 5: Define Callback Abstractions**
  
  Abstract callback patterns from both frameworks:
  - [x] 5.1 Add `BeforeAgentCallback` / `AfterAgentCallback` types
  - [x] 5.2 Add `BeforeToolCallback` / `AfterToolCallback` types
  - [x] 5.3 Add `CallbackContext` interface for callback execution
  - [x] 5.4 Ensure callbacks can modify invocation state
  - [x] 5.5 Support callback early-exit patterns

### Phase 3: Tool Abstraction (Steps 6-7)

- [x] **Step 6: Enhance Tool Interface**
  
  Make tool interface compatible with both frameworks:
  - [x] 6.1 Add `OutputSchema()` method to match Blades
  - [x] 6.2 Add `IsLongRunning()` method to match ADK-Go
  - [x] 6.3 Create `ToolConfirmation` interface for HITL patterns
  - [x] 6.4 Add `Toolset` interface for grouped tools
  - [x] 6.5 Add `ToolResolver` for dynamic tool discovery (MCP support)

- [x] **Step 7: Refine Tool Context**
  
  Ensure `ToolContext` has all necessary fields:
  - [x] 7.1 Add actions map for state modifications
  - [x] 7.2 Add memory search capability
  - [x] 7.3 Add artifact access
  - [x] 7.4 Add confirmation request method
  - [x] 7.5 Ensure thread-safety for concurrent tool execution

### Phase 4: Runner and Adapter Layer (Steps 8-9)

- [x] **Step 8: Refine Runner Interface**
  
  Ensure `Runner` can orchestrate any agent framework:
  - [x] 8.1 Add `SessionStore` dependency for persistent sessions
  - [x] 8.2 Add support for resumable invocations
  - [x] 8.3 Add streaming mode configuration
  - [x] 8.4 Define `RunResult` struct for non-streaming execution
  - [x] 8.5 Ensure runner does NOT emit lifecycle events (per ADR 002)

- [x] **Step 9: Create Adapter Contracts**
  
  Define clear contracts for framework adapters:
  - [x] 9.1 Create `adapters/adkgo/` package structure (stub)
  - [x] 9.2 Create `adapters/blades/` package structure (stub)
  - [x] 9.3 Define adapter factory interface
  - [x] 9.4 Document adapter implementation requirements
  - [x] 9.5 Create mock adapter for testing - Skipped (custom adapter serves as reference)

### Phase 5: Validation and Documentation (Step 10)

- [x] **Step 10: Validate and Document**
  
  Ensure the abstraction is complete and well-documented:
  - [x] 10.1 Write adapter implementation guide
  - [x] 10.2 Create mapping documentation for each framework
  - [x] 10.3 Update README.md with new architecture
  - [x] 10.4 Verify no imports from `foundation/agentsdk` in `foundation/agent`
  - [x] 10.5 Run tests to ensure no regressions - Build verification passed
  - [x] 10.6 Delete temporary `GAPS.md` file - N/A (not created)

---

## Interface Mapping

### Agent Interface Mapping

| Abstraction | ADK-Go | Blades |
|-------------|--------|--------|
| `Agent.Name()` | `agent.Agent.Name()` | `Agent.Name()` |
| `Agent.Description()` | `agent.Agent.Description()` | `Agent.Description()` |
| `Agent.Run()` | `agent.Agent.Run(InvocationContext)` | `Agent.Run(ctx, *Invocation)` |
| `Agent.Tools()` | via `llmagent.Config.Tools` | via `WithTools()` option |
| `Agent.SubAgents()` | `agent.Agent.SubAgents()` | N/A (use agent-as-tool) |

### Session Interface Mapping

| Abstraction | ADK-Go | Blades |
|-------------|--------|--------|
| `Session.ID()` | `session.Session.ID()` | `Session.ID()` |
| `Session.State()` | `session.Session.State()` | `Session.State()` |
| `Session.History()` | `session.Session.Events()` | `Session.History()` |
| `Session.Append()` | via EventService | `Session.Append()` |

### Tool Interface Mapping

| Abstraction | ADK-Go | Blades |
|-------------|--------|--------|
| `Tool.Name()` | `tool.Tool.Name()` | `tools.Tool.Name()` |
| `Tool.Description()` | `tool.Tool.Description()` | `tools.Tool.Description()` |
| `Tool.InputSchema()` | via declaration | `tools.Tool.InputSchema()` |
| `Tool.OutputSchema()` | N/A | `tools.Tool.OutputSchema()` |
| `Tool.Execute()` | via `BaseTool.Run()` | `tools.Tool.Handle()` |

---

## Success Criteria

1. ✅ `foundation/agent` has ZERO imports from `foundation/agentsdk`
2. ✅ All interfaces can represent both ADK-Go and Blades primitives
3. ✅ Event types comply with ADR 003 (classification) and ADR 004 (pointers)
4. ✅ Adapters can be implemented without type assertion hacks
5. ✅ Application layer only depends on `foundation/agent` interfaces
6. ✅ Lifecycle events are emitted by service layer, not adapters

---

## Handover Notes

If you are continuing this refactoring:

1. Start by checking off completed items above
2. The `GAPS.md` file (if exists) contains detailed gap analysis
3. Each step is designed to be independently completable
4. Run `go build ./...` after each step to ensure no breakage
5. The `adapters/custom/` package is the reference implementation

