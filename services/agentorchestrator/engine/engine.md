# Engine State Machine

The Engine follows a cyclical state machine designed for multi-agent collaboration and human-in-the-loop interaction.

## 1. Core Execution Cycle
Represents the standard autonomous loop and agent-to-agent transfers.

```mermaid
stateDiagram-v2
    [*] --> AwaitingTurn : New Message
    AwaitingTurn --> GeneratingResponse : StartExecution
    
    GeneratingResponse --> PerformingActions : ActionsDetected
    GeneratingResponse --> Done : TurnFinished
    
    PerformingActions --> GeneratingResponse : ActionsCompleted
    PerformingActions --> SwitchingAgent : HandoffInitiated
    PerformingActions --> Done : StopIteration (AwaitingInput)
    
    SwitchingAgent --> GeneratingResponse : AgentSwitched
    
    Done --> [*]
```

## 2. Human-in-the-Loop Lifecycle
When a tool requests user input (e.g. `ask_human`), the engine transitions to `Done` with `TerminationAwaitingInput`.

```mermaid
stateDiagram-v2
    direction LR
    
    PerformingActions --> Done : StopIteration
    note right of Done
        Reason: TerminationAwaitingInput
        (Engine yields control)
    end note
```

> [!NOTE]
> When a user provides input, the Engine is re-started with the new message history. `inferState` identifies that the previous elicitation has been answered, and the engine resumes (typically determining the next step based on the provided input).
