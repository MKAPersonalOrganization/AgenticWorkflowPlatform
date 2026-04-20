package thread

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ThreadAggregate is an event-sourced aggregate representing a conversation thread.
// Following the pattern from EventSourcing.NetCore, business logic is implemented
// as methods on the aggregate that validate rules and enqueue events.
type ThreadAggregate struct {
	BaseAggregate
	UserID           string
	Title            string
	Messages         []Message
	ActiveToolStates map[string]ToolCallState
	State            map[string]any // Session state persisted from agent runs
	CreatedAt        time.Time
	UpdatedAt        time.Time
	IsDeleted        bool
}

// NewThreadAggregate creates a new thread aggregate.
func NewThreadAggregate(id string) *ThreadAggregate {
	return &ThreadAggregate{
		BaseAggregate: BaseAggregate{
			ID: id,
		},
		Messages:         make([]Message, 0),
		ActiveToolStates: make(map[string]ToolCallState),
		State:            make(map[string]any),
	}
}

// OpenThread creates a new thread. This is a static factory method like ShoppingCart.Open.
func OpenThread(threadID, userID, title string) (*ThreadAggregate, error) {
	if threadID == "" {
		return nil, errors.New("thread ID is required")
	}
	if userID == "" {
		return nil, errors.New("user ID is required")
	}

	t := NewThreadAggregate(threadID)
	event := &ThreadCreated{
		ThreadID:  threadID,
		UserID:    userID,
		Title:     title,
		Timestamp: time.Now(),
	}

	t.Enqueue(event, t.Apply)
	return t, nil
}

// AddMessage adds a message to the thread.
func (t *ThreadAggregate) AddMessage(messageID, role, content string, toolCalls []ToolCall, toolCallID, toolName, agentName string, attachments ...Attachment) error {
	if t.IsDeleted {
		return errors.New("cannot add message to deleted thread")
	}
	if messageID == "" {
		return errors.New("message ID is required")
	}
	if role == "" {
		return errors.New("message role is required")
	}

	// Emit specific event based on role
	var event Event
	switch role {
	case "user":
		event = &UserMessageAdded{
			ThreadID:    t.ID,
			MessageID:   messageID,
			Content:     content,
			Attachments: attachments,
			Timestamp:   time.Now(),
		}
	case "assistant":
		event = &AssistantMessageAdded{
			ThreadID:  t.ID,
			MessageID: messageID,
			Content:   content,
			ToolCalls: toolCalls,
			AgentName: agentName,
			Timestamp: time.Now(),
		}
	case "tool":
		event = &ToolResultAdded{
			ThreadID:   t.ID,
			MessageID:  messageID,
			ToolCallID: toolCallID,
			ToolName:   toolName,
			Result:     content,
			AgentName:  agentName,
			Timestamp:  time.Now(),
		}
	default:
		return errors.New("unsupported message role: " + role)
	}

	t.Enqueue(event, t.Apply)
	return nil
}

// ChangeTitle changes the thread title.
func (t *ThreadAggregate) ChangeTitle(newTitle string) error {
	if t.IsDeleted {
		return errors.New("cannot change title of deleted thread")
	}

	// Idempotent - no event if title unchanged
	if t.Title == newTitle {
		return nil
	}

	event := &ThreadTitleUpdated{
		ThreadID:  t.ID,
		Title:     newTitle,
		Timestamp: time.Now(),
	}

	t.Enqueue(event, t.Apply)
	return nil
}

// Delete marks the thread as deleted.
func (t *ThreadAggregate) Delete() error {
	if t.IsDeleted {
		return errors.New("thread already deleted")
	}

	event := &ThreadDeleted{
		ThreadID:  t.ID,
		Timestamp: time.Now(),
	}

	t.Enqueue(event, t.Apply)
	return nil
}

// RecordUsage records token usage for a message in the thread.
func (t *ThreadAggregate) RecordUsage(event *TokenUsageReported) error {
	if t.IsDeleted {
		return errors.New("cannot record usage for deleted thread")
	}

	t.Enqueue(event, t.Apply)
	return nil
}

// RecordToolEvent records a tool lifecycle event in the thread.
func (t *ThreadAggregate) RecordToolEvent(event Event) error {
	if t.IsDeleted {
		return errors.New("cannot record tool event for deleted thread")
	}

	t.Enqueue(event, t.Apply)
	return nil
}

// eventApplicators is the ordered dispatch table from event type to
// per-category applicator. Each returns true when it handled the event
// so Apply can short-circuit. An event that matches nothing is silently
// ignored — matching the prior switch-statement's default arm.
var eventApplicators = []func(*ThreadAggregate, Event) bool{
	(*ThreadAggregate).applyMetadataEvent,
	(*ThreadAggregate).applyMessageEvent,
	(*ThreadAggregate).applyToolElicitationEvent,
	(*ThreadAggregate).applyToolStatusEvent,
	(*ThreadAggregate).applyRunLifecycleEvent,
	(*ThreadAggregate).applyStateEvent,
}

// Apply applies an event to update the thread's state.
// This is the only place where state mutations happen.
func (t *ThreadAggregate) Apply(event Event) {
	for _, apply := range eventApplicators {
		if apply(t, event) {
			return
		}
	}
}

// applyMetadataEvent handles thread-wide lifecycle events: creation,
// title changes, deletion.
func (t *ThreadAggregate) applyMetadataEvent(event Event) bool {
	switch e := event.(type) {
	case *ThreadCreated:
		t.applyThreadCreated(e)
	case *ThreadTitleUpdated:
		t.Title = e.Title
		t.UpdatedAt = e.Timestamp
	case *ThreadDeleted:
		t.IsDeleted = true
		t.UpdatedAt = e.Timestamp
	default:
		return false
	}
	return true
}

// applyMessageEvent handles message-producing events: user / assistant
// turns, tool results, code-execution results.
func (t *ThreadAggregate) applyMessageEvent(event Event) bool {
	switch e := event.(type) {
	case *UserMessageAdded:
		t.applyUserMessageAdded(e)
	case *AssistantMessageAdded:
		t.applyAssistantMessageAdded(e)
	case *ToolResultAdded:
		t.applyToolResultAdded(e)
	case *CodeExecutionRan:
		t.applyCodeExecutionRan(e)
	default:
		return false
	}
	return true
}

// applyToolElicitationEvent handles the HITL pause requests and their
// resolutions: tool needs input / confirmation / auth, and the user's
// input-provided reply.
func (t *ThreadAggregate) applyToolElicitationEvent(event Event) bool {
	switch e := event.(type) {
	case *ToolNeedsInput:
		t.applyToolNeedsInput(e)
	case *ToolNeedsConfirmation:
		t.applyToolNeedsConfirmation(e)
	case *ToolNeedsAuth:
		t.applyToolNeedsAuth(e)
	case *InputProvided:
		t.applyInputProvided(e)
	default:
		return false
	}
	return true
}

// applyToolStatusEvent handles tool-lifecycle flips that don't carry
// new content: auth grant/deny plus external cancellation.
func (t *ThreadAggregate) applyToolStatusEvent(event Event) bool {
	switch e := event.(type) {
	case *AuthGranted:
		t.updateToolStatus(e.ToolCallID, "AuthGranted")
	case *AuthDenied:
		t.updateToolStatus(e.ToolCallID, "AuthDenied")
	case *ToolCancelled:
		delete(t.ActiveToolStates, e.ToolCallID)
		t.UpdatedAt = e.Timestamp
	default:
		return false
	}
	return true
}

// applyRunLifecycleEvent absorbs the run-level events that only bump
// the thread's last-activity timestamp — run start/finish and token
// usage reports.
func (t *ThreadAggregate) applyRunLifecycleEvent(event Event) bool {
	switch event.(type) {
	case *RunStarted, *RunFinished, *TokenUsageReported:
		t.UpdatedAt = eventTimestamp(event)
		return true
	}
	return false
}

// applyStateEvent handles session-state mutations (ADK-Go state-delta
// propagation) — kept as its own category so future state-shaped events
// can join without touching the main dispatcher.
func (t *ThreadAggregate) applyStateEvent(event Event) bool {
	if e, ok := event.(*StateChangeReported); ok {
		t.applyStateChangeReported(e)
		return true
	}
	return false
}

// ─── Per-event application helpers ───────────────────────────────────────────
//
// Each method owns the state mutation for one event type. Grouping them by
// concern (thread metadata / message additions / tool lifecycle / state
// mutations) keeps Apply a flat dispatch table.

func (t *ThreadAggregate) applyThreadCreated(e *ThreadCreated) {
	t.ID = e.ThreadID
	t.UserID = e.UserID
	t.Title = e.Title
	t.CreatedAt = e.Timestamp
	t.UpdatedAt = e.Timestamp
	t.IsDeleted = false
}

func (t *ThreadAggregate) applyUserMessageAdded(e *UserMessageAdded) {
	msg := Message{
		ID:        e.MessageID,
		Role:      "user",
		Content:   e.Content,
		Timestamp: e.Timestamp,
	}
	if len(e.Attachments) > 0 {
		atts := make([]any, len(e.Attachments))
		for i, att := range e.Attachments {
			atts[i] = map[string]any{
				"url":      att.URL,
				"mimeType": att.MimeType,
				"fileName": att.FileName,
			}
		}
		msg.Metadata = map[string]any{"attachments": atts}
	}
	t.Messages = append(t.Messages, msg)
	t.UpdatedAt = e.Timestamp
}

func (t *ThreadAggregate) applyAssistantMessageAdded(e *AssistantMessageAdded) {
	t.Messages = append(t.Messages, Message{
		ID:        e.MessageID,
		Role:      "assistant",
		Content:   e.Content,
		ToolCalls: e.ToolCalls,
		AgentName: e.AgentName,
		Timestamp: e.Timestamp,
	})
	t.UpdatedAt = e.Timestamp
}

func (t *ThreadAggregate) applyToolResultAdded(e *ToolResultAdded) {
	t.Messages = append(t.Messages, Message{
		ID:         e.MessageID,
		Role:       "tool",
		Content:    e.Result,
		ToolCallID: e.ToolCallID,
		ToolName:   e.ToolName,
		AgentName:  e.AgentName,
		Timestamp:  e.Timestamp,
	})
	t.UpdatedAt = e.Timestamp
	// Remove the tool from active states when result is added.
	delete(t.ActiveToolStates, e.ToolCallID)
}

// applyCodeExecutionRan materialises one code-execution block as a chat
// message row so the card survives a refresh. Structured context
// (language, output, outcome) travels in Metadata; the projection reads
// these back into the AguiThreadState sidecar. The synthesised ID
// matches the projection's key so the aggregate view and the projection
// agree on message_id — important for ON CONFLICT dedup and frontend
// lookups.
func (t *ThreadAggregate) applyCodeExecutionRan(e *CodeExecutionRan) {
	t.Messages = append(t.Messages, Message{
		ID:        "codeexec_" + e.MessageID + "_" + e.BlockID,
		Role:      "code_execution",
		Content:   e.Code,
		AgentName: e.AgentName,
		Timestamp: e.Timestamp,
		Metadata: map[string]any{
			"codeExecution": map[string]any{
				"language": e.Language,
				"output":   e.Output,
				"outcome":  e.Outcome,
				"blockId":  e.BlockID,
			},
		},
	})
	t.UpdatedAt = e.Timestamp
}

func (t *ThreadAggregate) applyToolNeedsInput(e *ToolNeedsInput) {
	t.ActiveToolStates[e.ToolCallID] = ToolCallState{
		ToolCallID:         e.ToolCallID,
		OriginalToolCallID: e.OriginalToolCallID,
		ToolName:           e.ToolName,
		AgentName:          e.AgentName,
		Status:             "AwaitingInput",
		Elicitation: map[string]any{
			"question": e.Elicitation.Question,
			"schema":   e.Elicitation.Schema,
		},
	}
}

func (t *ThreadAggregate) applyToolNeedsConfirmation(e *ToolNeedsConfirmation) {
	t.ActiveToolStates[e.ToolCallID] = ToolCallState{
		ToolCallID:         e.ToolCallID,
		OriginalToolCallID: e.OriginalToolCallID,
		ToolName:           e.ToolName,
		AgentName:          e.AgentName,
		Status:             "AwaitingConfirmation",
		Elicitation: map[string]any{
			"question": e.Question,
		},
	}
}

func (t *ThreadAggregate) applyToolNeedsAuth(e *ToolNeedsAuth) {
	t.ActiveToolStates[e.ToolCallID] = ToolCallState{
		ToolCallID: e.ToolCallID,
		ToolName:   e.ToolName,
		AgentName:  e.AgentName,
		Status:     "AwaitingAuth",
		AuthSpec: map[string]any{
			"resource": e.AuthSpec.Resource,
			"action":   e.AuthSpec.Action,
			"reason":   e.AuthSpec.Reason,
			"scopes":   e.AuthSpec.Scopes,
		},
	}
}

// applyInputProvided records the user's answer as the real tool result
// message. No placeholder was stored during the pause — this event is
// the single source of truth for the tool's output so the LLM sees a
// valid tool_call → tool_result pair. The lookup is two-step: try the
// direct tool-call ID, then fall back to OriginalToolCallID scan to
// handle confirmation responses that carry the original id.
func (t *ThreadAggregate) applyInputProvided(e *InputProvided) {
	state, keyToDelete, ok := t.findActiveToolState(e.ToolCallID)
	if !ok {
		return
	}
	t.Messages = append(t.Messages, Message{
		ID:                 e.MessageID,
		Role:               "tool",
		Content:            e.Input,
		ToolCallID:         e.ToolCallID,
		ToolName:           state.ToolName,
		AgentName:          state.AgentName,
		Timestamp:          e.Timestamp,
		OriginalToolCallID: state.OriginalToolCallID,
	})
	delete(t.ActiveToolStates, keyToDelete)
	t.UpdatedAt = e.Timestamp
}

// findActiveToolState returns the ToolCallState keyed by id, either
// directly or via a scan on OriginalToolCallID (confirmation-response
// path). The returned key is the actual map key — which the caller
// deletes to retire the tool from the active set.
func (t *ThreadAggregate) findActiveToolState(id string) (ToolCallState, string, bool) {
	if s, ok := t.ActiveToolStates[id]; ok {
		return s, id, true
	}
	for k, s := range t.ActiveToolStates {
		if s.OriginalToolCallID == id {
			return s, k, true
		}
	}
	return ToolCallState{}, "", false
}

// updateToolStatus flips the Status of an active tool call (no-op when
// the id is unknown). Shared by AuthGranted/AuthDenied.
func (t *ThreadAggregate) updateToolStatus(id, status string) {
	if state, ok := t.ActiveToolStates[id]; ok {
		state.Status = status
		t.ActiveToolStates[id] = state
	}
}

// applyStateChangeReported persists session state from agent runs. Keys
// prefixed with "__" (internal markers) or "temp:" (ADK ephemeral scope)
// are suppressed — they are never visible outside the run.
func (t *ThreadAggregate) applyStateChangeReported(e *StateChangeReported) {
	if !strings.HasPrefix(e.Key, "__") && !strings.HasPrefix(e.Key, "temp:") {
		if t.State == nil {
			t.State = make(map[string]any)
		}
		t.State[e.Key] = e.Value
	}
	t.UpdatedAt = e.Timestamp
}

// eventTimestamp pulls the Timestamp field from events that only update
// the thread's last-activity time — the three activity-meter events
// share this shape but not a common interface, so a small type switch
// does the lookup.
func eventTimestamp(event Event) time.Time {
	switch e := event.(type) {
	case *RunStarted:
		return e.Timestamp
	case *RunFinished:
		return e.Timestamp
	case *TokenUsageReported:
		return e.Timestamp
	}
	return time.Time{}
}

// StreamName returns the event stream name for this aggregate.
func (t *ThreadAggregate) StreamName() string {
	return StreamName("Thread", t.ID)
}

// ─── Snapshot Support ────────────────────────────────────────────────────────

// threadSnapshot is the serialisation format for ThreadAggregate snapshots.
type threadSnapshot struct {
	UserID           string                   `json:"user_id"`
	Title            string                   `json:"title"`
	Messages         []Message                `json:"messages"`
	ActiveToolStates map[string]ToolCallState `json:"active_tool_states,omitempty"`
	State            map[string]any           `json:"state,omitempty"`
	CreatedAt        time.Time                `json:"created_at"`
	UpdatedAt        time.Time                `json:"updated_at"`
	IsDeleted        bool                     `json:"is_deleted"`
}

// MarshalSnapshot serialises the aggregate's current state for snapshotting.
func (t *ThreadAggregate) MarshalSnapshot() (json.RawMessage, error) {
	snap := threadSnapshot{
		UserID:           t.UserID,
		Title:            t.Title,
		Messages:         t.Messages,
		ActiveToolStates: t.ActiveToolStates,
		State:            t.State,
		CreatedAt:        t.CreatedAt,
		UpdatedAt:        t.UpdatedAt,
		IsDeleted:        t.IsDeleted,
	}
	return json.Marshal(snap)
}

// UnmarshalSnapshot restores the aggregate's state from a snapshot.
func (t *ThreadAggregate) UnmarshalSnapshot(data json.RawMessage) error {
	var snap threadSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	t.UserID = snap.UserID
	t.Title = snap.Title
	t.Messages = snap.Messages
	t.ActiveToolStates = snap.ActiveToolStates
	if t.ActiveToolStates == nil {
		t.ActiveToolStates = make(map[string]ToolCallState)
	}
	t.State = snap.State
	if t.State == nil {
		t.State = make(map[string]any)
	}
	t.CreatedAt = snap.CreatedAt
	t.UpdatedAt = snap.UpdatedAt
	t.IsDeleted = snap.IsDeleted
	return nil
}
