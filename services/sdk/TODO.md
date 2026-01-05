# Agent SDK Enhancement TODOs

This document tracks the implementation of features missing from the Go SDK compared to the .NET Agent Framework. Each TODO is designed to be logically separate and fully testable before moving to the next.

**Priority Legend:**
- 🔴 **Critical** - Core functionality gaps
- 🟡 **Important** - Quality of life improvements  
- 🟢 **Nice to have** - Advanced features

---

## 🔴 TODO 1: Implement RunOptions for Per-Run Configuration

**Status:** ✅ Complete  
**Priority:** Critical  
**Dependencies:** None

### Objective
Add flexible per-run configuration options instead of baking all parameters into AgentConfig. This allows different invocations of the same agent to use different parameters.

### Tasks
- [x] Create `RunOptions` struct in `run_options.go`
  - Temperature override
  - MaxTokens override
  - TopP override
  - StopSequences override
  - AdditionalProperties map
- [x] Update `Agent.Run()` signature to accept `*RunOptions`
- [x] Update `runIteration()` to merge RunOptions with AgentConfig
- [x] Update `ToChatOptions()` to accept both AgentConfig and RunOptions
- [x] Write tests for RunOptions merging logic
- [x] Update all agent implementations to pass nil RunOptions (backward compatible)

### Tests Required
- ✅ `TestRunOptionsClone` - Cloning works correctly with deep copy
- ✅ `TestRunOptionsCloneNil` - Nil clone returns nil
- ✅ `TestToChatOptionsWithNilRunOptions` - Verify nil RunOptions uses defaults from AgentConfig
- ✅ `TestToChatOptionsRunOptionsOverridesConfig` - Verify RunOptions overrides AgentConfig values
- ✅ `TestToChatOptionsPartialOverride` - Verify partial overrides work correctly
- ✅ `TestToChatOptionsAdditionalProperties` - Verify additional properties are preserved
- ✅ `TestToChatOptionsAllParameters` - All parameters can be overridden

### Acceptance Criteria
- ✅ All existing tests pass
- ✅ New RunOptions tests pass (7/7 passing)
- ✅ Can override temperature per run without changing AgentConfig
- ✅ Clean API design with value types (no pointers, no nil checks)
- ✅ Explicit `DefaultRunOptions()` for clarity
- ✅ Deep copy prevents mutation issues
- ✅ All agent implementations updated

### Design Notes
**Clean Design Principles Applied:**
- RunOptions is a **value type** (not pointer) for safety and clarity
- `DefaultRunOptions()` function provides explicit default construction
- No nil checks needed - always pass a RunOptions value
- API is self-documenting: `agent.Run(..., sdk.DefaultRunOptions())`
- Compiler enforces correct usage at call sites

---

## 🔴 TODO 2: Create AgentThread Abstraction

**Status:** Not Started  
**Priority:** Critical  
**Dependencies:** None (independent of TODO 1)

### Objective
Formalize thread lifecycle management with a proper AgentThread interface and implementations. This provides better separation between thread state, message storage, and conversation management.

### Tasks
- [ ] Create `thread.go` with `AgentThread` interface
  - GetID() string
  - GetMessages() []domain.Message
  - AddMessages(messages []domain.Message) error
  - Serialize() ([]byte, error)
  - GetType() ThreadType (local vs remote)
- [ ] Create `ThreadType` enum (Local, Remote)
- [ ] Implement `InMemoryThread` struct
- [ ] Add thread factory methods to Agent
  - `GetNewThread() AgentThread`
  - `DeserializeThread(data []byte) (AgentThread, error)`
- [ ] Update existing `ThreadState` interface to work with AgentThread
- [ ] Write comprehensive thread tests

### Tests Required
- `TestInMemoryThreadCreation` - Create and use in-memory thread
- `TestThreadMessageManagement` - Add/retrieve messages
- `TestThreadSerialization` - Serialize and deserialize thread
- `TestThreadIsolation` - Multiple threads don't interfere

### Acceptance Criteria
- ✅ AgentThread interface is well-defined
- ✅ InMemoryThread implementation works correctly
- ✅ Can create, use, and serialize threads
- ✅ Existing ThreadState integration maintained

---

## 🔴 TODO 3: Implement MessageStore Interface

**Status:** Not Started  
**Priority:** Critical  
**Dependencies:** TODO 2 (AgentThread)

### Objective
Create pluggable message persistence layer with support for different storage backends and message reduction strategies.

### Tasks
- [ ] Create `message_store.go` with `MessageStore` interface
  - AddMessage(message domain.Message) error
  - GetMessages() []domain.Message
  - GetMessagesByRole(role string) []domain.Message
  - Clear() error
  - Count() int
- [ ] Implement `InMemoryMessageStore`
- [ ] Add optional message reduction support
  - Create `ReductionStrategy` interface
  - Implement `KeepLastNStrategy`
  - Implement `KeepWithinTokenLimitStrategy`
- [ ] Integrate MessageStore with AgentThread
- [ ] Update Agent to use MessageStore from thread
- [ ] Write message store tests

### Tests Required
- `TestInMemoryMessageStore` - Basic CRUD operations
- `TestMessageStoreThreadSafety` - Concurrent access
- `TestKeepLastNReduction` - Reduction strategy works
- `TestMessageStoreWithThread` - Integration with AgentThread

### Acceptance Criteria
- ✅ MessageStore interface is clean and extensible
- ✅ InMemoryMessageStore works correctly
- ✅ Message reduction strategies work
- ✅ Thread-safe for concurrent access
- ✅ Easy to implement custom stores (file, database, etc.)

---

## 🔴 TODO 4: Separate Streaming and Complete Execution Paths

**Status:** Not Started  
**Priority:** Critical  
**Dependencies:** TODO 1 (RunOptions)

### Objective
Provide distinct execution paths for streaming and non-streaming (complete) responses, matching .NET's RunAsync vs RunStreamingAsync pattern.

### Tasks
- [ ] Create `agent_response.go` with response types
  - `AgentRunResponse` - Complete response
  - `StreamUpdate` - Streaming update
- [ ] Rename current `Run()` to `RunStreaming()`
- [ ] Implement new `RunComplete()` method
  - Accumulates all streaming updates
  - Returns complete `AgentRunResponse`
  - Includes continuation token support
- [ ] Add `responseAccumulator` helper
- [ ] Update event emission to work with both paths
- [ ] Write tests for both execution paths

### Tests Required
- `TestRunComplete` - Returns complete response
- `TestRunStreaming` - Streams updates correctly
- `TestResponseAccumulation` - Streaming updates convert to complete response
- `TestBothPathsProduceSameResult` - Verify consistency

### Acceptance Criteria
- ✅ `RunComplete()` returns full response after completion
- ✅ `RunStreaming()` provides real-time updates
- ✅ Both paths produce equivalent final state
- ✅ Event emission works correctly for both
- ✅ All existing streaming tests pass

---

## 🟡 TODO 5: Implement ContextProvider Interface

**Status:** Not Started  
**Priority:** Important  
**Dependencies:** TODO 2 (AgentThread)

### Objective
Add mechanism to inject contextual information before agent runs, separate from message history. This allows dynamic context without polluting message storage.

### Tasks
- [ ] Create `context_provider.go` with `ContextProvider` interface
  - GetContext(ctx context.Context, thread AgentThread) ([]domain.Message, error)
  - OnSuccess(ctx context.Context, input, response []domain.Message) error
  - OnFailure(ctx context.Context, err error) error
- [ ] Add `ContextProvider` field to AgentThread
- [ ] Update `runIteration()` to call GetContext before LLM invocation
- [ ] Update success path to call OnSuccess
- [ ] Update error path to call OnFailure
- [ ] Write context provider tests

### Tests Required
- `TestContextProviderInjection` - Context is injected before run
- `TestContextProviderOnSuccess` - Success callback is invoked
- `TestContextProviderOnFailure` - Failure callback is invoked
- `TestNilContextProviderIsValid` - Nil provider doesn't break anything

### Acceptance Criteria
- ✅ Can inject context messages before each run
- ✅ Context messages don't persist in message store
- ✅ Success/failure callbacks work correctly
- ✅ Nil provider is safe (no-op)

---

## 🟡 TODO 6: Add AgentMetadata and Service Discovery

**Status:** Not Started  
**Priority:** Important  
**Dependencies:** None (independent)

### Objective
Add rich metadata about agents and implement service discovery pattern for extensibility.

### Tasks
- [ ] Create `agent_metadata.go` with `AgentMetadata` struct
  - ProviderName string
  - ModelName string
  - Capabilities []string
  - Version string
- [ ] Add `GetMetadata() *AgentMetadata` method to Agent
- [ ] Implement `GetService(serviceType string) (interface{}, bool)` pattern
- [ ] Register common services (ChatClient, ToolRegistry, etc.)
- [ ] Write metadata and service discovery tests

### Tests Required
- `TestAgentMetadata` - Metadata is populated correctly
- `TestServiceDiscovery` - Can query for known services
- `TestServiceDiscoveryUnknown` - Returns false for unknown services
- `TestMetadataFromChatClient` - Metadata derives from underlying client

### Acceptance Criteria
- ✅ Metadata is accessible and accurate
- ✅ Service discovery works for common services
- ✅ Unknown services return (nil, false)
- ✅ Extensible for custom services

---

## 🟡 TODO 7: Implement Transactional Error Handling

**Status:** Not Started  
**Priority:** Important  
**Dependencies:** TODO 4 (Separate execution paths)

### Objective
Ensure agent state remains consistent on errors by implementing transactional event emission and rollback on failure.

### Tasks
- [ ] Create `transaction.go` with event transaction support
  - `EventTransaction` type
  - `Begin()`, `Commit()`, `Rollback()` methods
- [ ] Update `runIteration()` to use transactions
  - Collect all events in transaction
  - Only commit on success
  - Rollback on any error
- [ ] Add error context preservation
- [ ] Ensure partial streaming content is handled correctly
- [ ] Write transaction tests

### Tests Required
- `TestTransactionCommitOnSuccess` - Events persist on success
- `TestTransactionRollbackOnError` - Events don't persist on error
- `TestPartialStreamingError` - Partial content handled correctly
- `TestThreadStateConsistency` - Thread state never corrupted

### Acceptance Criteria
- ✅ No partial state on errors
- ✅ All events commit atomically
- ✅ Thread state remains consistent
- ✅ Error context is preserved

---

## 🟡 TODO 8: Add Tool Execution Control

**Status:** Not Started  
**Priority:** Important  
**Dependencies:** TODO 1 (RunOptions)

### Objective
Provide fine-grained control over tool execution, including tool mode, required tools, and disabled tools.

### Tasks
- [ ] Create `ToolMode` type (Auto, Required, None)
- [ ] Add tool control fields to `RunOptions`
  - ToolMode
  - RequiredTool *string
  - DisabledTools []string
- [ ] Update `ToChatOptions()` to apply tool mode
- [ ] Implement tool filtering logic
- [ ] Update tool execution to respect mode
- [ ] Write tool control tests

### Tests Required
- `TestToolModeNone` - No tools are called
- `TestToolModeRequired` - At least one tool is called
- `TestRequiredTool` - Specific tool is called
- `TestDisabledTools` - Specified tools are not available

### Acceptance Criteria
- ✅ Can disable all tools (ToolMode.None)
- ✅ Can require tool usage (ToolMode.Required)
- ✅ Can specify required tool by name
- ✅ Can disable specific tools
- ✅ Default behavior unchanged (ToolMode.Auto)

---

## 🟢 TODO 9: Implement Conversation ID and Remote Storage Support

**Status:** Not Started  
**Priority:** Nice to have  
**Dependencies:** TODO 2 (AgentThread)

### Objective
Support AI services that manage conversations server-side (like OpenAI Assistants API).

### Tasks
- [ ] Add `ConversationID` field to AgentThread
- [ ] Implement `RemoteThread` type for server-side storage
- [ ] Add auto-detection of thread type from responses
- [ ] Update Agent to handle both local and remote threads
- [ ] Add thread type migration logic
- [ ] Write remote thread tests (with mocks)

### Tests Required
- `TestRemoteThreadCreation` - Create thread with conversation ID
- `TestThreadTypeDetection` - Auto-detect thread type from response
- `TestLocalToRemoteMigration` - Handle service requiring remote storage
- `TestRemoteThreadOperations` - Basic operations work

### Acceptance Criteria
- ✅ Can create threads with conversation IDs
- ✅ Auto-detects when service requires remote storage
- ✅ Graceful migration from local to remote
- ✅ Clear error messages when incompatible

---

## 🟢 TODO 10: Add Agent Middleware Pattern

**Status:** Not Started  
**Priority:** Nice to have  
**Dependencies:** TODO 4 (Separate execution paths)

### Objective
Implement middleware/decorator pattern for composable agent behaviors (logging, telemetry, retry logic, etc.).

### Tasks
- [ ] Create `middleware.go` with middleware interfaces
  - `AgentMiddleware` interface
  - `AgentHandler` function type
- [ ] Implement middleware chaining
- [ ] Create built-in middleware
  - `LoggingMiddleware`
  - `RetryMiddleware`
  - `TimeoutMiddleware`
- [ ] Update Agent to support middleware
- [ ] Write middleware tests

### Tests Required
- `TestMiddlewareChaining` - Multiple middleware compose correctly
- `TestLoggingMiddleware` - Logs are generated
- `TestRetryMiddleware` - Retries on transient errors
- `TestMiddlewareOrder` - Order is preserved

### Acceptance Criteria
- ✅ Can chain multiple middleware
- ✅ Middleware order is respected
- ✅ Built-in middleware work correctly
- ✅ Easy to implement custom middleware
- ✅ Middleware can modify requests/responses

---

## 🟢 TODO 11: Implement Background Operations Support

**Status:** Not Started  
**Priority:** Nice to have  
**Dependencies:** TODO 4 (Separate execution paths), TODO 1 (RunOptions)

### Objective
Support long-running background operations with polling and status tracking.

### Tasks
- [ ] Create `background_operation.go`
  - `BackgroundOperation` struct
  - `OperationStatus` enum
- [ ] Add `AllowBackgroundOperations` to RunOptions
- [ ] Implement `StartBackgroundRun()` method
- [ ] Implement `GetOperationStatus()` method
- [ ] Implement `WaitForOperation()` method
- [ ] Add operation storage/tracking
- [ ] Write background operation tests

### Tests Required
- `TestStartBackgroundOperation` - Operation starts correctly
- `TestPollOperationStatus` - Can check status
- `TestWaitForCompletion` - Wait blocks until complete
- `TestCancelBackgroundOperation` - Cancellation works

### Acceptance Criteria
- ✅ Can start operations in background
- ✅ Can poll for status
- ✅ Can wait for completion
- ✅ Proper cleanup of completed operations
- ✅ Cancellation works correctly

---

## 🟢 TODO 12: Add Structured Logging and Telemetry

**Status:** Not Started  
**Priority:** Nice to have  
**Dependencies:** TODO 10 (Middleware - optional but recommended)

### Objective
Integrate structured logging and OpenTelemetry support for production observability.

### Tasks
- [ ] Add `Logger` interface to support multiple logging libraries
- [ ] Integrate OpenTelemetry tracing
  - Add tracer to Agent
  - Create spans for Run operations
  - Create spans for tool executions
- [ ] Add metrics collection
  - Run duration
  - Token usage
  - Tool call counts
- [ ] Create `TelemetryMiddleware`
- [ ] Write telemetry tests

### Tests Required
- `TestStructuredLogging` - Logs are structured correctly
- `TestTracingSpans` - Spans are created and closed
- `TestMetricsCollection` - Metrics are recorded
- `TestTelemetryMiddleware` - Middleware emits telemetry

### Acceptance Criteria
- ✅ Structured logs include context (agentID, threadID, etc.)
- ✅ OpenTelemetry spans track operations
- ✅ Metrics are collected and exportable
- ✅ Zero impact when telemetry disabled
- ✅ Compatible with standard observability tools

---

## Implementation Guidelines

### Testing Strategy
- Each TODO must have 100% of its tests passing before moving to the next
- Run `go test ./services/sdk/...` after each change
- Maintain backward compatibility where possible
- Add integration tests for cross-TODO features

### Code Review Checklist
- [ ] All new code has tests
- [ ] All tests pass
- [ ] Documentation is updated
- [ ] Examples are provided for new features
- [ ] Backward compatibility maintained (or migration path documented)
- [ ] Performance impact assessed

### Branch Strategy
- Create feature branch per TODO (e.g., `feature/sdk-todo-1`)
- Merge only when all tests pass
- Keep main branch stable

---

## Progress Tracking

| TODO | Status | Tests Pass | Notes |
|------|--------|------------|-------|
| 1 - RunOptions | ✅ Complete | ✅ 7/7 | All tests passing, backward compatible |
| 2 - AgentThread | ⬜ Not Started | N/A | |
| 3 - MessageStore | ⬜ Not Started | N/A | |
| 4 - Streaming/Complete | ⬜ Not Started | N/A | |
| 5 - ContextProvider | ⬜ Not Started | N/A | |
| 6 - Metadata | ⬜ Not Started | N/A | |
| 7 - Transactions | ⬜ Not Started | N/A | |
| 8 - Tool Control | ⬜ Not Started | N/A | |
| 9 - Remote Storage | ⬜ Not Started | N/A | |
| 10 - Middleware | ⬜ Not Started | N/A | |
| 11 - Background Ops | ⬜ Not Started | N/A | |
| 12 - Telemetry | ⬜ Not Started | N/A | |

**Status Legend:**
- ⬜ Not Started
- 🔄 In Progress
- ✅ Complete (all tests pass)
- ⚠️ Blocked

---

## Notes

- Each TODO is designed to be independent and testable
- Dependencies are noted but many can be implemented in parallel
- Focus on critical items (🔴) first for maximum impact
- Nice-to-have items (🟢) can be deferred for later releases
- All changes should be backward compatible or provide clear migration paths
