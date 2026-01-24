# State Machine Combinations & Visualization

This document describes the state machine models used in the orchestration engine and tool lifecycle.

## Combined State Machine Model

The system uses a layered state machine approach:
1.  **Engine (Outer Layer)**: Manages the iterative LLM loop and high-level execution flow.
2.  **Tool (Inner Layer)**: Manages the lifecycle of individual tool executions, including complex scenarios like user elicitation and authorization.

### State Legend
- **Engine States**: Prefixed with `E_`
- **Tool States**: Prefixed with `T_`

```mermaid
stateDiagram-v2
    [*] --> E_AwaitingTurn

    E_AwaitingTurn --> E_GeneratingResponse : StartExecution
    
    state E_GeneratingResponse {
        [*] --> Streaming
        Streaming --> AssistantMessage : FinishStreaming
    }

    E_GeneratingResponse --> E_PerformingActions : ActionsDetected (Tool Calls Found)
    E_GeneratingResponse --> E_Done : TurnFinished (No Tools)

    state E_PerformingActions {
        [*] --> MapTools
        MapTools --> ExecuteTools : NewToolStateMachine()
        
        state ExecuteTools {
            [*] --> T_Idle
            T_Idle --> T_Executing : ToolStart
            
            T_Executing --> T_Completed : ToolSuccess
            T_Executing --> T_Error : ToolError
            
            T_Executing --> T_AwaitingInput : ToolNeedsInput (Elicitation)
            T_Executing --> T_AwaitingAuth : ToolNeedsAuth (Authorization)
            
            T_AwaitingInput --> T_Executing : InputProvided
            T_AwaitingAuth --> T_Executing : AuthGranted
            T_AwaitingAuth --> T_Error : AuthDenied
            
            T_AwaitingInput --> T_Error : ToolCancelled
        }
    }

    E_PerformingActions --> E_GeneratingResponse : ActionsCompleted (All tools terminal)
    E_PerformingActions --> E_Done : StopIteration (Tool needs interaction)

    E_Done --> [*]
    
    note right of E_PerformingActions
        If any tool enters T_AwaitingInput or T_AwaitingAuth,
        the Engine triggers StopIteration to yield back to the user.
    end note
```

---

## Tool State Machine Diagram

Standalone lifecycle model for an individual tool call:

```mermaid
stateDiagram-v2
    [*] --> Idle : TrigToolStart
    
    Idle --> Executing : ToolStart
    
    state Executing {
        [*] --> Processing
        Processing --> Success : TrigToolSuccess
        Processing --> Error : TrigToolError
        Processing --> NeedsInput : TrigToolNeedsInput
        Processing --> NeedsAuth : TrigToolNeedsAuth
    }
    
    Executing --> Completed : ToolSuccess
    Executing --> ErrorState : ToolError
    Executing --> AwaitingInput : ToolNeedsInput
    Executing --> AwaitingAuth : ToolNeedsAuth
    
    AwaitingInput --> Executing : InputProvided
    AwaitingInput --> ErrorState : ToolCancelled
    
    AwaitingAuth --> Executing : AuthGranted
    AwaitingAuth --> ErrorState : AuthDenied
    AwaitingAuth --> ErrorState : ToolCancelled
    
    Completed --> [*]
    ErrorState --> [*]

    note right of Completed: Result Persisted
    note right of ErrorState: Error Reported
```

## Implementation Highlights

- **Stateless Library usage**: Both layers use `github.com/qmuntal/stateless`, allowing for declarative configuration of transitions and actions.
- **Hierarchical Modeling**: While functionally separate types, the `Engine` coordinates the `ToolStateMachines` within its `StatePerformingActions` lifecycle.
- **External State Storage**: The machines allow state resumption by recreating the machine and "firing" it based on persisted conversation history (reconciliation logic in `Engine.inferState`).
- **Concurrency Safety**: The `ToolStateMachine` is thread-safe and can be safely accessed by multiple goroutines:
  - The `stateless.StateMachine` library is internally thread-safe for state transitions
  - Mutable fields (`result`, `elicitation`, `observers`) are protected by a `sync.Mutex`
  - Observer callbacks are invoked without holding locks to prevent deadlocks
  - Read-only fields (`toolCall`, `executor`, `ctx`) are safe to access concurrently without locks

## Architecture & Event Flow

The `Engine` is the authoritative source of truth for both durable state and real-time streaming:

1.  **Centralized Emission**: Consolidated all event handling into a single `Engine.emit` helper.
    - **Durable Events**: State machine events (tool lifecycle, usage) are persisted to the EventStore.
    - **Streaming Events**: Transient events (tokens, delimiters) are dispatched directly to the `EventRouter` for performance.
2.  **Streamlined API**: The `ProcessUserMessage` call chain returns a simple `error`. Emission is handled internally by the engine via the `EventEmitter`.
3.  **Duplication Control**: A hybrid persist-or-dispatch strategy ensures events are delivered exactly once to the UI.
