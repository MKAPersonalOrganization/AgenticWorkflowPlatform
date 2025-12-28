package agui

import (
	"fmt"
	"time"
)

// BaseEvent is the base event for all events in the Agent User Interaction Protocol.
type BaseEvent struct {
	Type      string      `json:"type"`
	Timestamp *int64      `json:"timestamp,omitempty"`
	RawEvent  interface{} `json:"rawEvent,omitempty"`
}

// Text Message Events
//
// These events represent the lifecycle of text messages in a conversation. Text
// message events follow a streaming pattern, where content is delivered
// incrementally. A message begins with a TextMessageStart event, followed by one
// or more TextMessageContent events that deliver chunks of text as they become
// available, and concludes with a TextMessageEnd event.
//
// This streaming approach enables real-time display of message content as it's
// generated, creating a more responsive user experience compared to waiting for
// the entire message to be complete before showing anything.
//
// Event Flow:
//   1. TextMessageStart - Initializes a new text message with a unique messageId
//   2. TextMessageContent (one or more) - Delivers chunks of text incrementally
//   3. TextMessageEnd - Marks the completion of the message
//
// The TextMessageContent events each contain a delta field with a chunk of
// text. Frontends should concatenate these deltas in the order received to
// construct the complete message. The messageId property links all related
// events, allowing the frontend to associate content chunks with the correct
// message.

// TextMessageStartEvent signals the start of a text message.
//
// The TextMessageStart event initializes a new text message in the conversation.
// It establishes a unique messageId that will be referenced by subsequent
// content chunks and the end event. This event allows frontends to prepare the UI
// for an incoming message, such as creating a new message bubble with a loading
// indicator. The role property identifies whether the message is coming from the
// assistant or potentially another participant in the conversation.
//
// Properties:
//   - messageId: Unique identifier for the message
//   - role: Role of the message sender ("developer", "system", "assistant", "user", "tool")
type TextMessageStartEvent struct {
	BaseEvent
	MessageID string `json:"messageId"`
	Role      string `json:"role,omitempty"`
}

// TextMessageContentEvent represents a chunk of content in a streaming text message.
//
// The TextMessageContent event delivers incremental parts of the message text as
// they become available. Each event contains a small chunk of text in the delta
// property that should be appended to previously received chunks. The streaming
// nature of these events enables real-time display of content, creating a more
// responsive and engaging user experience. Implementations should handle these
// events efficiently to ensure smooth text rendering without visible delays or
// flickering.
//
// Properties:
//   - messageId: Matches the ID from TextMessageStart
//   - delta: Text content chunk (non-empty)
type TextMessageContentEvent struct {
	BaseEvent
	MessageID string `json:"messageId"`
	Delta     string `json:"delta"`
}

// TextMessageEndEvent signals the end of a text message.
//
// The TextMessageEnd event marks the completion of a streaming text message.
// After receiving this event, the frontend knows that the message is complete and
// no further content will be added. This allows the UI to finalize rendering,
// remove any loading indicators, and potentially trigger actions that should occur
// after message completion, such as enabling reply controls or performing
// automatic scrolling to ensure the full message is visible.
//
// Properties:
//   - messageId: Matches the ID from TextMessageStart
type TextMessageEndEvent struct {
	BaseEvent
	MessageID string `json:"messageId"`
}

// TextMessageChunkEvent is a convenience event that expands to Start → Content → End automatically.
//
// The TextMessageChunk event lets you omit explicit TextMessageStart and
// TextMessageEnd events. The client stream transformer expands chunks into the
// standard triad:
//
//  1. First chunk for a message must include messageId and will emit
//     TextMessageStart (role defaults to assistant when not provided).
//  2. Each chunk with a delta emits a TextMessageContent for the current
//     messageId.
//  3. TextMessageEnd is emitted automatically when the stream switches to a new
//     message ID or when the stream completes.
//
// Properties:
//   - messageId: Optional unique identifier for the message; required on the first chunk of a message
//   - role: Optional role of the sender ("developer", "system", "assistant", "user")
//   - delta: Optional text content of the message
type TextMessageChunkEvent struct {
	BaseEvent
	MessageID *string `json:"messageId,omitempty"`
	Role      *string `json:"role,omitempty"`
	Delta     *string `json:"delta,omitempty"`
}

// Lifecycle Events
//
// These events represent the lifecycle of an agent run. A typical agent run
// follows a predictable pattern: it begins with a RunStarted event, may contain
// multiple optional StepStarted/StepFinished pairs, and concludes with either
// a RunFinished event (success) or a RunError event (failure).
//
// Lifecycle events provide crucial structure to agent runs, enabling frontends to
// track progress, manage UI states appropriately, and handle errors gracefully.
// They create a consistent framework for understanding when operations begin and
// end, making it possible to implement features like loading indicators, progress
// tracking, and error recovery mechanisms.
//
// Event Flow:
//   1. RunStarted (mandatory) - Signals the start of an agent run
//   2. StepStarted/StepFinished (optional, can occur multiple times) - Granular progress tracking
//   3. RunFinished OR RunError (mandatory) - Signals completion or failure
//
// The RunStarted and either RunFinished or RunError events are mandatory,
// forming the boundaries of an agent run. Step events are optional and may occur
// multiple times within a run, allowing for structured, observable progress tracking.

// RunStartedEvent indicates that a run has started.
type RunStartedEvent struct {
	BaseEvent
	ThreadID    string      `json:"threadId"`
	RunID       string      `json:"runId"`
	ParentRunID *string     `json:"parentRunId,omitempty"`
	Input       interface{} `json:"input,omitempty"`
}

// RunFinishedEvent signals the successful completion of an agent run.
//
// The RunFinished event indicates that an agent has successfully completed all its work
// for the current run. Upon receiving this event, frontends should finalize any UI states
// that were waiting on the agent's completion. This event marks a clean termination point
// and indicates that no further processing will occur in this run unless explicitly requested.
// The optional result field can contain any output data produced by the agent run.
//
// Properties:
//   - threadId: ID of the conversation thread
//   - runId: ID of the agent run
//   - result: Optional result data from run
type RunFinishedEvent struct {
	BaseEvent
	ThreadID string      `json:"threadId"`
	RunID    string      `json:"runId"`
	Result   interface{} `json:"result,omitempty"`
}

// RunErrorEvent signals an error during an agent run.
//
// The RunError event indicates that the agent encountered an error it could not recover from,
// causing the run to terminate prematurely. This event provides information about what went wrong,
// allowing frontends to display appropriate error messages and potentially offer recovery options.
// After a RunError event, no further processing will occur in this run.
//
// Properties:
//   - message: Error message
//   - code: Optional error code
type RunErrorEvent struct {
	BaseEvent
	Message string  `json:"message"`
	Code    *string `json:"code,omitempty"`
}

// StepStartedEvent signals the start of a step within an agent run.
//
// The StepStarted event indicates that the agent is beginning a specific subtask or phase
// of its processing. Steps provide granular visibility into the agent's progress, enabling
// more precise tracking and feedback in the UI. Steps are optional but highly recommended
// for complex operations that benefit from being broken down into observable stages.
// The stepName could be the name of a node or function that is currently executing.
//
// Properties:
//   - stepName: Name of the step
type StepStartedEvent struct {
	BaseEvent
	StepName string `json:"stepName"`
}

// StepFinishedEvent signals the completion of a step within an agent run.
//
// The StepFinished event indicates that the agent has completed a specific subtask or phase.
// When paired with a corresponding StepStarted event, it creates a bounded context for a
// discrete unit of work. Frontends can use these events to update progress indicators, show
// completion animations, or reveal results specific to that step. The stepName must match
// the corresponding StepStarted event to properly pair the beginning and end of the step.
//
// Properties:
//   - stepName: Name of the step
type StepFinishedEvent struct {
	BaseEvent
	StepName string `json:"stepName"`
}

// Tool Call Events
//
// These events represent the lifecycle of tool calls made by agents. Tool calls
// follow a streaming pattern similar to text messages. When an agent needs to use
// a tool, it emits a ToolCallStart event, followed by one or more ToolCallArgs
// events that stream the arguments being passed to the tool, and concludes with a
// ToolCallEnd event.
//
// This streaming approach allows frontends to show tool executions in real-time,
// making the agent's actions transparent and providing immediate feedback about
// what tools are being invoked and with what parameters.
//
// Event Flow:
//   1. ToolCallStart - Indicates agent is invoking a tool
//   2. ToolCallArgs (one or more) - Streams the tool arguments incrementally
//   3. ToolCallEnd - Marks completion of tool call specification
//   4. ToolCallResult - Delivers the execution result (sent after tool execution)
//
// The ToolCallArgs events each contain a delta field with a chunk of the
// arguments. Frontends should concatenate these deltas in the order received to
// construct the complete arguments object. The toolCallId property links all
// related events, allowing the frontend to associate argument chunks with the
// correct tool call.

// ToolCallStartEvent signals the start of a tool call.
//
// The ToolCallStart event indicates that the agent is invoking a tool to perform
// a specific function. This event provides the name of the tool being called and
// establishes a unique toolCallId that will be referenced by subsequent events
// in this tool call. Frontends can use this event to display tool usage to users,
// such as showing a notification that a specific operation is in progress. The
// optional parentMessageId allows linking the tool call to a specific message in
// the conversation, providing context for why the tool is being used.
//
// Properties:
//   - toolCallId: Unique identifier for the tool call
//   - toolCallName: Name of the tool being called
//   - parentMessageId: Optional ID of the parent message
type ToolCallStartEvent struct {
	BaseEvent
	ToolCallID      string  `json:"toolCallId"`
	ToolCallName    string  `json:"toolCallName"`
	ParentMessageID *string `json:"parentMessageId,omitempty"`
}

// ToolCallArgsEvent represents a chunk of argument data for a tool call.
//
// The ToolCallArgs event delivers incremental parts of the tool's arguments as
// they become available. Each event contains a segment of the argument data in the
// delta property. These deltas are often JSON fragments that, when combined,
// form the complete arguments object for the tool. Streaming the arguments is
// particularly valuable for complex tool calls where constructing the full
// arguments may take time. Frontends can progressively reveal these arguments to
// users, providing insight into exactly what parameters are being passed to tools.
//
// Properties:
//   - toolCallId: Matches the ID from ToolCallStart
//   - delta: Argument data chunk
type ToolCallArgsEvent struct {
	BaseEvent
	ToolCallID string `json:"toolCallId"`
	Delta      string `json:"delta"`
}

// ToolCallEndEvent signals the end of a tool call.
//
// The ToolCallEnd event marks the completion of a tool call. After receiving
// this event, the frontend knows that all arguments have been transmitted and the
// tool execution is underway or completed. This allows the UI to finalize the tool
// call display and prepare for potential results. In systems where tool execution
// results are returned separately, this event indicates that the agent has
// finished specifying the tool and its arguments, and is now waiting for or has
// received the results.
//
// Properties:
//   - toolCallId: Matches the ID from ToolCallStart
type ToolCallEndEvent struct {
	BaseEvent
	ToolCallID string `json:"toolCallId"`
}

// ToolCallResultEvent provides the result of a tool call execution.
//
// The ToolCallResult event delivers the output or result from a tool that was
// previously invoked by the agent. This event is sent after the tool has been
// executed by the system and contains the actual output generated by the tool.
// Unlike the streaming pattern of tool call specification (start, args, end), the
// result is delivered as a complete unit since tool execution typically produces a
// complete output. Frontends can use this event to display tool results to users,
// append them to the conversation history, or trigger follow-up actions based on
// the tool's output.
//
// Properties:
//   - messageId: ID of the conversation message this result belongs to
//   - toolCallId: Matches the ID from the corresponding ToolCallStart event
//   - content: The actual result/output content from the tool execution
//   - role: Optional role identifier, typically "tool" for tool results
type ToolCallResultEvent struct {
	BaseEvent
	MessageID  string  `json:"messageId"`
	ToolCallID string  `json:"toolCallId"`
	Content    string  `json:"content"`
	Role       *string `json:"role,omitempty"`
}

// ToolCallChunkEvent is a convenience event that expands to Start → Args → End automatically.
//
// The ToolCallChunk event lets you omit explicit ToolCallStart and
// ToolCallEnd events. The client stream transformer expands chunks into the
// standard tool-call triad:
//
//  1. First chunk for a tool call must include toolCallId and toolCallName and
//     will emit ToolCallStart (propagating any parentMessageId).
//  2. Each chunk with a delta emits a ToolCallArgs for the current
//     toolCallId.
//  3. ToolCallEnd is emitted automatically when the stream switches to a new
//     toolCallId or when the stream completes.
//
// Properties:
//   - toolCallId: Optional on later chunks; required on the first chunk of a tool call
//   - toolCallName: Optional on later chunks; required on the first chunk of a tool call
//   - parentMessageId: Optional ID of the parent message
//   - delta: Optional argument data chunk (often a JSON fragment)
type ToolCallChunkEvent struct {
	BaseEvent
	ToolCallID      *string `json:"toolCallId,omitempty"`
	ToolCallName    *string `json:"toolCallName,omitempty"`
	ParentMessageID *string `json:"parentMessageId,omitempty"`
	Delta           *string `json:"delta,omitempty"`
}

// State Management Events
//
// These events are used to manage and synchronize the agent's state with the
// frontend. State management in the protocol follows an efficient snapshot-delta
// pattern where complete state snapshots are sent initially or infrequently, while
// incremental updates (deltas) are used for ongoing changes.
//
// This approach optimizes for both completeness and efficiency: snapshots ensure
// the frontend has the full state context, while deltas minimize data transfer for
// frequent updates. Together, they enable frontends to maintain an accurate
// representation of agent state without unnecessary data transmission.
//
// Event Flow:
//   1. StateSnapshot - Delivers a comprehensive representation of the agent's current state
//   2. StateDelta (multiple) - Provides incremental updates using JSON Patch operations
//   3. StateSnapshot (occasional) - Occasional full refresh to resynchronize
//   4. MessagesSnapshot - Delivers complete message history
//
// The combination of snapshots and deltas allows frontends to efficiently track
// changes to agent state while ensuring consistency. Snapshots serve as
// synchronization points that reset the state to a known baseline, while deltas
// provide lightweight updates between snapshots.

// StateSnapshotEvent provides a complete snapshot of an agent's state.
//
// The StateSnapshot event delivers a comprehensive representation of the agent's
// current state. This event is typically sent at the beginning of an interaction
// or when synchronization is needed. It contains all state variables relevant to
// the frontend, allowing it to completely rebuild its internal representation.
// Frontends should replace their existing state model with the contents of this
// snapshot rather than trying to merge it with previous state.
//
// Properties:
//   - snapshot: Complete state snapshot
type StateSnapshotEvent struct {
	BaseEvent
	Snapshot interface{} `json:"snapshot"`
}

// StateDeltaEvent provides a partial update to an agent's state using JSON Patch.
//
// The StateDelta event contains incremental updates to the agent's state in the
// form of JSON Patch operations (as defined in RFC 6902). Each delta represents
// specific changes to apply to the current state model. This approach is
// bandwidth-efficient, sending only what has changed rather than the entire state.
// Frontends should apply these patches in sequence to maintain an accurate state
// representation. If a frontend detects inconsistencies after applying patches, it
// may request a fresh StateSnapshot.
//
// Properties:
//   - delta: Array of JSON Patch operations (RFC 6902)
type StateDeltaEvent struct {
	BaseEvent
	Delta []interface{} `json:"delta"`
}

// MessagesSnapshotEvent provides a snapshot of all messages in a conversation.
//
// The MessagesSnapshot event delivers a complete history of messages in the
// current conversation. Unlike the general state snapshot, this focuses
// specifically on the conversation transcript. This event is useful for
// initializing the chat history, synchronizing after connection interruptions, or
// providing a comprehensive view when a user joins an ongoing conversation.
// Frontends should use this to establish or refresh the conversational context
// displayed to users.
//
// Messages is a discriminated union based on the "role" field and can contain:
//   - DeveloperMessage (role: "developer")
//   - SystemMessage (role: "system")
//   - AssistantMessage (role: "assistant")
//   - UserMessage (role: "user")
//   - ToolMessage (role: "tool")
//   - ActivityMessage (role: "activity")
//
// Properties:
//   - messages: Array of Message objects (discriminated by "role" field)
type MessagesSnapshotEvent struct {
	BaseEvent
	Messages []Message `json:"messages"`
}

// Activity Events
//
// Activity Events expose structured, in-progress activity updates that occur
// between chat messages. They follow the same snapshot/delta pattern as the state
// system so that UIs can render a complete activity view immediately and then
// incrementally update it as new information arrives.

// ActivitySnapshotEvent delivers a complete snapshot of an activity message.
//
// Frontends should either create a new ActivityMessage or replace the existing
// one with the payload supplied by the snapshot.
//
// Properties:
//   - messageId: Identifier for the ActivityMessage this event updates
//   - activityType: Activity discriminator (for example "PLAN", "SEARCH")
//   - content: Structured JSON payload representing the full activity state
//   - replace: Per spec, defaults to true. In Go, must be set explicitly as the zero value is false
type ActivitySnapshotEvent struct {
	BaseEvent
	MessageID    string      `json:"messageId"`
	ActivityType string      `json:"activityType"`
	Content      interface{} `json:"content"`
	Replace      bool        `json:"replace"`
}

// ActivityDeltaEvent applies incremental updates to an existing activity using JSON Patch operations.
//
// Activity deltas should be applied in order to the previously synchronized
// activity content. If an application detects divergence, it can request or emit a
// fresh ActivitySnapshot to resynchronize.
//
// Properties:
//   - messageId: Identifier for the target activity message
//   - activityType: Activity discriminator (mirrors the value from the most recent snapshot)
//   - patch: Array of RFC 6902 JSON Patch operations to apply to the activity data
type ActivityDeltaEvent struct {
	BaseEvent
	MessageID    string        `json:"messageId"`
	ActivityType string        `json:"activityType"`
	Patch        []interface{} `json:"patch"`
}

// Special Events
//
// Special events provide flexibility in the protocol by allowing for
// system-specific functionality and integration with external systems. These
// events don't follow the standard lifecycle or streaming patterns of other event
// types but instead serve specialized purposes.

// RawEvent is used to pass through events from external systems.
//
// The Raw event acts as a container for events originating from external systems
// or sources that don't natively follow the Agent UI Protocol. This event type
// enables interoperability with other event-based systems by wrapping their events
// in a standardized format. The enclosed event data is preserved in its original
// form inside the event property, while the optional source property identifies
// the system it came from. Frontends can use this information to handle external
// events appropriately, either by processing them directly or by delegating them
// to system-specific handlers.
//
// Properties:
//   - event: Original event data
//   - source: Optional source identifier
type RawEvent struct {
	BaseEvent
	Event  interface{} `json:"event"`
	Source *string     `json:"source,omitempty"`
}

// CustomEvent is used for application-specific custom events.
//
// The Custom event provides an extension mechanism for implementing features not
// covered by the standard event types. Unlike Raw events which act as
// passthrough containers, Custom events are explicitly part of the protocol but
// with application-defined semantics. The name property identifies the specific
// custom event type, while the value property contains the associated data. This
// mechanism allows for protocol extensions without requiring formal specification
// changes. Teams should document their custom events to ensure consistent
// implementation across frontends and agents.
//
// Properties:
//   - name: Name of the custom event
//   - value: Value associated with the event
type CustomEvent struct {
	BaseEvent
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

// GenerateMessageID generates a unique message ID
func GenerateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// GenerateRunID generates a unique run ID
func GenerateRunID() string {
	return fmt.Sprintf("run_%d", time.Now().UnixNano())
}

// GenerateToolCallID generates a unique tool call ID
func GenerateToolCallID() string {
	return fmt.Sprintf("tool_%d", time.Now().UnixNano())
}
