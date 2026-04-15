# ADR 023: Standard Elicitation Patterns for Interactive Tools

## Status
Accepted

## Context
Many tools require human-in-the-loop (HITL) interaction to request binary approvals (Confirmations) or structured data (Inputs). To support a stateless, multi-pod environment while providing a premium, real-time user experience, these interactions must follow a standardized "Pause-and-Resume" pattern.

This ADR defines the standard for implementing these interactive tools using the `foundation/agent/io` abstraction.

## Decision
All interactive tools MUST use the `agentio` package (aliased from `aicompass/foundation/agent/io`) to satisfy their interaction requirements.

### 1. Implementation Patterns

#### A. Binary Confirmation (`Confirm`)
Use this for non-destructive authorization or simple "Yes/No" approvals.
- **Backend call**: `agentio.Confirm(ctx, prompt, data)`
- **Behavior**: Emits a `ToolNeedsConfirmation` event.
- **UI**: Renders specialized "Allow" and "Deny" buttons in the chat stream.

```go
func (t *MyTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
    approved, err := agentio.Confirm(ctx, "Do you want to proceed?", nil)
    if err != nil {
        return agent.Result{}, err // Engine handles the pause
    }
    if !approved {
        return agent.TextResult("Cancelled"), nil
    }
    return agent.TextResult("Done"), nil
}
```

#### B. Custom Input (`Read`)
Use this for requesting text, structured data, or missing parameters.
- **Backend call**: `agentio.Read(ctx, prompt, schema)`
- **Behavior**: Emits a `ToolNeedsInput` event.
- **UI**: Renders a text input field in the chat stream. If a `schema` is provided, the UI can eventually support rich form generation.

```go
func (t *MyTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
    answer, err := agentio.Read(ctx, "Please enter your name:", nil)
    if err != nil {
        return agent.Result{}, err // Engine handles the pause
    }
    return agent.TextResult("Hello " + answer), nil
}
```

### 2. The Pause-Resume Lifecycle
- **Statelessness**: When a tool calls `io.Read` or `io.Confirm`, it returns a sentinel error that causes the engine to pause and the pod to stop executing that specific run.
- **Persistence**: The user's response is saved in the database as a `UserAnsweredTool` event.
- **Re-injection**: When resumed, the orchestrator injects a `CachedReadHandler` into the Go `context.Context`.
- **Transparent Resumption**: The tool is re-executed from the beginning. When it hits the same `io.Read` call, the `CachedReadHandler` returns the saved answer from the context instead of pausing again.

### 3. Developer Requirements
1.  **Immediate Error Return**: Tools MUST return the error from `Confirm` or `Read` immediately. They should not wrap it in a way that hides the sentinel type.
2.  **Graceful Cancellation**: Tools SHOULD handle the case where `Confirm` returns `false` (Deny) by returning a meaningful result (e.g., "Operation cancelled") rather than an error.
3.  **Idempotency (Recommended)**: Since tools are re-executed from the start upon resumption, developers should be mindful of side effects that might occur before the first interaction point.

## Consequences
- **Stateless Scaling**: HITL tools work correctly across any pod in the cluster.
- **Unified UX**: All tools share a consistent interaction style, reducing user friction.
- **Traceability**: All user interactions are persisted in the event stream for later audit or re-play.
- **Simplified Tool Logic**: Developers focus on the "what" (asking the question) while the framework handles the "how" (broadcasting, waiting, and resuming).
