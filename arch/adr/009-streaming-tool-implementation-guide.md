# ADR 009: Streaming Tool Implementation Guide

## Status
Accepted

## Context
Tools in the agent framework traditionally execute synchronously: the engine invokes `Execute()`, waits for the complete result, and then emits a single `TOOL_SUCCESS` event to the frontend. For tools that perform slow operations (e.g., calling an external LLM, querying a knowledge base), this means the user stares at a loading spinner until the entire response is ready.

We introduced a streaming tool architecture that delivers content to the frontend **token-by-token** as it is generated. There are two distinct streaming phases, each addressing a different use case:

1. **Argument streaming** (LLM phase): The LLM generates tool call arguments as streamed tokens. The `StreamProcessor` automatically emits `ToolContentStreamed` events with `field="__arguments__"` for every tool call — no tool-side code changes needed. This is how the **think tool** streams: its content *is* the arguments.

2. **Execution streaming** (tool phase): Tools that call external APIs during execution (e.g., OpenAI, a RAG pipeline) can implement `StreamingExecutableTool` to stream their output via a callback. This is how the **knowledge base tool** streams its synthesized answer.

This ADR documents the decision and serves as a practical guide for adding new streaming tools.

## Decision

### Two Streaming Mechanisms

| Mechanism | When it fires | Who implements it | Field value | Example tool |
|-----------|--------------|-------------------|-------------|-------------|
| **Argument streaming** | During LLM response, as tool call arguments are generated token-by-token | `StreamProcessor` (automatic) | `__arguments__` | `think` |
| **Execution streaming** | During `ExecuteStreaming()`, when the tool calls an external API | The tool itself via `StreamCallback` | Tool-defined (e.g., `answer`) | `query_knowledge_base` |

### Adding a New Streaming Tool

#### Case A: Content lives in the arguments (like `think`)

If the tool's display content is part of its JSON arguments (e.g., `{"thoughts": "..."}`, `{"code": "..."}`, `{"explanation": "..."}`), **you get streaming for free**. The `StreamProcessor` already emits `ToolContentStreamed` events with `field="__arguments__"` for every argument delta.

**Backend checklist:**
1. Define your tool as a normal `ExecutableTool` — no streaming interface needed.
2. Register via `PluginProvider` as usual (see ADR 008).

**Frontend checklist:**
1. In [chat-streaming.js](assets/js/chat/chat-streaming.js), the `__extractStreamingArgs()` method already parses partial JSON and extracts string fields. If your tool's schema uses simple top-level string properties, it works automatically.
2. In your plugin's `ui.templ`, use `message.parsedArgs?.yourField` for display and add a streaming cursor indicator:
   ```html
   <div x-text="message.parsedArgs?.yourField"></div>
   <template x-if="message.isStreaming">
       <span class="inline-block w-2 h-4 bg-primary/50 animate-pulse ml-0.5"></span>
   </template>
   ```

#### Case B: Content is generated during execution (like `query_knowledge_base`)

If the tool calls an external API and wants to stream the response in real-time, implement the `StreamingExecutableTool` interface.

**Backend checklist:**

1. **Implement both interfaces** — your tool must satisfy `ExecutableTool` (non-streaming fallback) and `StreamingExecutableTool`:
   ```go
   // In foundation/agent/tool.go:
   type StreamCallback func(delta string, field string) bool
   
   type StreamingExecutableTool interface {
       Tool
       StreamingExecutor
   }
   ```

2. **Add `ExecuteStreaming` method** — call `stream(delta, "yourField")` for each token:
   ```go
   func (t *MyTool) ExecuteStreaming(
       ctx context.Context,
       args agent.Arguments,
       stream agent.StreamCallback,
   ) (agent.Result, error) {
       // ... set up external API call with streaming enabled ...
       
       for token := range externalStream {
           if !stream(token, "answer") {
               break // consumer disconnected
           }
       }
       
       // Return the complete result for persistence
       return agent.JSONResult(fullResult), nil
   }
   ```

3. **Add compile-time interface verification**:
   ```go
   var (
       _ agent.ExecutableTool          = (*MyTool)(nil)
       _ agent.StreamingExecutableTool = (*MyTool)(nil)
   )
   ```

4. **Keep `Execute()` as a non-streaming fallback** — it must still work independently for contexts where streaming is not available.

**Frontend checklist:**

1. **Handle your field in `__handleToolContentStreamed`** — add a branch in [chat-streaming.js](assets/js/chat/chat-streaming.js):
   ```javascript
   } else if (fieldKey === 'yourField') {
       if (!msg.parsedResult) msg.parsedResult = {};
       msg.parsedResult.yourField = msg.streamingContent[fieldKey];
   }
   ```

2. **Add streaming UI state in your `ui.templ`** — show streaming content with a cursor while `message.isStreaming` is true, and show the final result when it's false:
   ```html
   <!-- Streaming state -->
   <template x-if="message.isStreaming && message.parsedResult?.yourField">
       <div x-html="renderMarkdown(message.parsedResult.yourField)"></div>
       <span class="animate-pulse">▍</span>
   </template>
   
   <!-- Final state -->
   <template x-if="message.result && !message.isStreaming">
       <div x-html="renderMarkdown(message.parsedResult?.yourField)"></div>
   </template>
   ```

### Event Pipeline

A streaming delta traverses the following path from origin to the user's browser:

```
StreamProcessor / Tool.ExecuteStreaming()
  → core.ToolContentStreamed event
    → EventPipeline converts to domain.ToolContentStreamed
      → Orchestrator dispatches (transient, NOT persisted)
        → RedisBroadcaster.Broadcast() via pub/sub
          → SSE handler: ToAguiEvents() → WriteSSEEvent() → flusher.Flush()
            → Browser: EventSource.onmessage
              → __handleToolContentStreamed() → Alpine.js reactivity → UI update
```

Key properties:
- `ToolContentStreamed` is a **transient** event — it is broadcast but never persisted to the event store (see ADR 003).
- Each SSE event is flushed immediately — no batching.
- The `StreamCallback` returns `false` if the consumer disconnects, which tools must respect to avoid wasted work.

### Concurrency Model

The engine executes multiple tool calls in parallel via `errgroup`. Each tool call gets its own streaming callback, keyed by tool call ID in a `sync.Map`. This prevents race conditions when multiple streaming tools run concurrently.

## Consequences

**Positive:**
- **Argument streaming is free**: Any tool whose content is in the arguments automatically streams to the frontend with zero backend changes.
- **Execution streaming is opt-in**: Only tools that call external APIs need to implement `StreamingExecutableTool`.
- **Consistent UX**: All streaming tools use the same `ToolContentStreamed` event and `message.isStreaming` flag, giving a uniform loading/streaming/complete lifecycle.

**Negative:**
- **Partial JSON parsing**: Argument streaming requires the frontend to parse incomplete JSON, which is inherently fragile for complex nested schemas. The current `__extractStreamingArgs` handles flat string fields reliably but may need extension for deeply nested structures.
- **Dual interface**: Streaming tools must implement both `Execute` and `ExecuteStreaming`, adding some boilerplate.

## Constraints

1. Streaming tools MUST still return a complete `Result` from `ExecuteStreaming()` — the final result is persisted via the `ToolResultAdded` durable event and is the source of truth on page reload.
2. Tools MUST check the `StreamCallback` return value and stop streaming if it returns `false`.
3. The `field` parameter in `StreamCallback` must be a stable, documented string — the frontend uses it to route content to the correct UI element.
4. Do NOT use `field="__arguments__"` in `ExecuteStreaming` — that field is reserved for the automatic argument streaming from `StreamProcessor`.
