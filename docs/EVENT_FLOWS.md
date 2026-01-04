# Event Flow Documentation

This document describes how events flow through the system architecture.

## 1. Tool Call Agent Persistence

**Purpose**: Track which agent executed each tool call and persist this information across page reloads.

### Real-Time Flow

```mermaid
sequenceDiagram
    participant Agent as Agent<br/>(agent.go)
    participant State as ThreadState<br/>(in_memory_thread_state.go)
    participant Notifier as UINotifier<br/>(ui_notifier.go)
    participant SSE as SSE Emitter
    participant UI as Frontend<br/>(chat.js)

    Agent->>Agent: Detects new tool call
    Agent->>State: ToolCallStartedEvent<br/>{toolCallId, agentName}
    State->>State: Mark as ephemeral
    Agent->>Notifier: ToolCallStartedEvent
    Notifier->>SSE: TOOL_CALL_AGENT<br/>{toolCallId, agentName}
    Notifier->>SSE: TOOL_CALL_START<br/>{toolCallId, toolCallName}
    SSE->>UI: Stream both events
    UI->>UI: Update tool call with agent name
    
    Note over Agent,State: Later: AssistantMessageCompletedEvent<br/>persists {agentName, toolCalls[]}
```

### Snapshot/Reload Flow

```mermaid
sequenceDiagram
    participant State as ThreadState
    participant Handler as Message Handler<br/>(worker_pool.go)
    participant SSE as SSE Emitter
    participant UI as Frontend<br/>(chat.js)

    UI->>Handler: Load thread
    Handler->>State: Get messages
    State-->>Handler: Messages with<br/>{agentName, toolCalls[]}
    Handler->>SSE: MESSAGES_SNAPSHOT
    
    loop For each assistant message with tool calls
        Handler->>SSE: TOOL_CALL_AGENT<br/>{toolCallId, agentName}
    end
    
    SSE->>UI: Stream all events
    UI->>UI: Apply TOOL_CALL_AGENT events
```

### Key Points

- **Consistent Event Flow**: Both streaming and snapshots emit `TOOL_CALL_AGENT` events
- **Real-Time**: 1 domain event (`ToolCallStartedEvent`) → 2 AGUI events (`TOOL_CALL_AGENT` + `TOOL_CALL_START`)
- **Snapshots**: Iterate assistant messages and emit `TOOL_CALL_AGENT` for each tool call (same events as streaming)
- **Persistence**: Agent name stored in `AssistantMessageCompletedEvent.agentName`
- **UI Processing**: Same handler processes `TOOL_CALL_AGENT` events regardless of source
