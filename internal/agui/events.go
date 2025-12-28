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
