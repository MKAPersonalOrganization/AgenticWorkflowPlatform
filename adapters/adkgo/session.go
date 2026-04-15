package adkgo

import (
	"aicompass/modules/chat/thread"
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"time"

	"google.golang.org/adk/session"
	"google.golang.org/adk/tool/toolconfirmation"
	"google.golang.org/genai"

	fagent "aicompass/foundation/agent"
)

// ──────────────────────────────────────────────────────────────────────────────
// BuildSessionFromThread creates an adk-go session pre-populated with the
// conversation history from a ThreadAggregate. This lets the LLM agent see the
// existing chat context when a new user message arrives.
//
// We use adk-go's InMemoryService so there is no external session store — the
// thread event store remains the single source of truth.
// ──────────────────────────────────────────────────────────────────────────────

const appName = "ai-chat-platform"

// BuildSessionFromThread creates a session.Session from thread history.
func BuildSessionFromThread(
	ctx context.Context,
	svc session.Service,
	agg *thread.ThreadAggregate,
	userID string,
) (session.Session, error) {
	// Create a fresh session (or re-use the threadID as sessionID).
	resp, err := svc.Create(ctx, &session.CreateRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: agg.ID,
	})
	if err != nil {
		return nil, err
	}
	sess := resp.Session

	// Pass 1: collect adk_request_confirmation FunctionCall IDs from
	// assistant messages. Tool result messages whose ToolCallID matches
	// one of these IDs are confirmation responses and must be converted
	// to a FunctionResponse with Name = "adk_request_confirmation" so
	// ADK's RequestConfirmationRequestProcessor can match them.
	confirmationMappings := collectConfirmationMappings(agg.Messages)

	// Pass 2: replay thread messages as session events.
	for _, msg := range agg.Messages {
		evt := messageToSessionEvent(msg, confirmationMappings)
		if evt == nil {
			continue
		}
		if err := svc.AppendEvent(ctx, sess, evt); err != nil {
			return nil, err
		}
	}

	// Inject persisted state from the aggregate into the session.
	// This restores state written by agents (e.g., via OutputKey) so that
	// subsequent agent runs can read values set by prior runs.
	if len(agg.State) > 0 {
		for key, value := range agg.State {
			if err := sess.State().Set(key, value); err != nil {
				return nil, fmt.Errorf("setting persisted state key %q: %w", key, err)
			}
		}
	}

	// Re-fetch to get the session with all events loaded.
	getResp, err := svc.Get(ctx, &session.GetRequest{
		AppName:   appName,
		UserID:    userID,
		SessionID: agg.ID,
	})
	if err != nil {
		return nil, err
	}
	return getResp.Session, nil
}

// collectConfirmationCallIDs scans assistant messages for tool calls named
// "adk_request_confirmation" and returns a set of their IDs.
func collectConfirmationCallIDs(msgs []thread.Message) map[string]bool {
	ids := make(map[string]bool)
	for _, msg := range msgs {
		if msg.Role != "assistant" {
			continue
		}
		for _, tc := range msg.ToolCalls {
			if tc.Name == toolconfirmation.FunctionCallName {
				ids[tc.ID] = true
			}
		}
	}
	return ids
}

// confirmationMappings holds the mapping between confirmation-related IDs.
// originalToWrapper maps original tool call ID -> confirmation wrapper ID.
type confirmationMappings struct {
	wrapperIDs     map[string]bool   // set of confirmation wrapper IDs
	originalToWrap map[string]string // original tool call ID -> wrapper ID
}

// collectConfirmationMappings scans assistant messages for "adk_request_confirmation"
// tool calls and returns both the wrapper ID set and a reverse mapping from
// original tool call ID to wrapper ID.
func collectConfirmationMappings(msgs []thread.Message) confirmationMappings {
	mappings := confirmationMappings{
		wrapperIDs:     make(map[string]bool),
		originalToWrap: make(map[string]string),
	}
	for _, msg := range msgs {
		if msg.Role != "assistant" {
			continue
		}
		for _, tc := range msg.ToolCalls {
			if tc.Name == toolconfirmation.FunctionCallName {
				mappings.wrapperIDs[tc.ID] = true
				// Extract original ID from wrapper args
				if originalID := extractOriginalCallID(tc.Arguments); originalID != "" {
					mappings.originalToWrap[originalID] = tc.ID
				}
			}
		}
	}
	return mappings
}

// extractOriginalCallID parses the arguments of an adk_request_confirmation
// FunctionCall and returns the original function call ID.
func extractOriginalCallID(argsJSON string) string {
	var args struct {
		OriginalFunctionCall struct {
			ID string `json:"id"`
		} `json:"originalFunctionCall"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return ""
	}
	return args.OriginalFunctionCall.ID
}

// GetConfirmationWrapperID looks up the confirmation wrapper ID for a given
// original tool call ID by scanning the thread messages. Returns "" if not found.
func GetConfirmationWrapperID(msgs []thread.Message, originalToolCallID string) string {
	mappings := collectConfirmationMappings(msgs)
	return mappings.originalToWrap[originalToolCallID]
}

// messageToSessionEvent converts a domain thread.Message to an adk-go session.Event.
//
// confirmationMappings contains:
// - wrapperIDs: set of adk_request_confirmation FunctionCall IDs
// - originalToWrap: map from original tool call ID -> wrapper ID
//
// For confirmation responses, we need to:
// 1. Detect if this is a confirmation response (OriginalToolCallID is set)
// 2. Look up the wrapper ID to use as the FunctionResponse ID
// 3. Set Name = "adk_request_confirmation" and Response = {"confirmed": true}
func messageToSessionEvent(msg thread.Message, mappings confirmationMappings) *session.Event {
	var content *genai.Content

	switch msg.Role {
	case "user":
		content = &genai.Content{
			Role:  "user",
			Parts: []*genai.Part{{Text: msg.Content}},
		}

	case "assistant":
		parts := buildAssistantParts(msg)
		if len(parts) == 0 {
			return nil
		}
		content = &genai.Content{
			Role:  "model",
			Parts: parts,
		}

	case "tool":
		// Tool result message — wrap as FunctionResponse.
		//
		// For confirmation responses (OriginalToolCallID is set), we need to:
		// - Look up the wrapper ID from original ID
		// - Use Name = "adk_request_confirmation"
		// - Use wrapper ID as FunctionResponse ID
		name := msg.ToolName
		var result map[string]any
		responseID := msg.ToolCallID
		isConfirmation := msg.OriginalToolCallID != ""

		if isConfirmation {
			name = toolconfirmation.FunctionCallName
			result = map[string]any{"confirmed": true}
			// Look up wrapper ID for this confirmation
			if wrapperID, ok := mappings.originalToWrap[msg.OriginalToolCallID]; ok {
				responseID = wrapperID
			}
		} else {
			if msg.Content != "" {
				_ = json.Unmarshal([]byte(msg.Content), &result)
			}
			if result == nil {
				result = map[string]any{"result": msg.Content}
			}
		}

		content = &genai.Content{
			Role: "user", // adk-go convention: function responses come as user role
			Parts: []*genai.Part{{
				FunctionResponse: &genai.FunctionResponse{
					ID:       responseID,
					Name:     name,
					Response: result,
				},
			}},
		}

	default:
		return nil
	}

	ts := msg.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}

	evt := &session.Event{
		ID:        msg.ID,
		Timestamp: ts,
		Actions:   session.EventActions{StateDelta: make(map[string]any), ArtifactDelta: make(map[string]int64)},
	}
	evt.Content = content
	evt.Author = authorFromRole(msg.Role, msg.AgentName)

	return evt
}

// buildAssistantParts constructs genai.Part slices for an assistant message,
// including text and any function calls (tool invocations).
func buildAssistantParts(msg thread.Message) []*genai.Part {
	var parts []*genai.Part

	if msg.Content != "" {
		parts = append(parts, &genai.Part{Text: msg.Content})
	}

	for _, tc := range msg.ToolCalls {
		var args map[string]any
		if tc.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Arguments), &args)
		}
		parts = append(parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   tc.ID,
				Name: tc.Name,
				Args: args,
			},
		})
	}

	return parts
}

// ──────────────────────────────────────────────────────────────────────────────
// adkSessionState — ReadonlyState adapter
//
// Wraps session.State so the adapter can populate Invocation.State, giving
// InstructionFunc and Requirements read access to values accumulated in the
// session (e.g. output written by a prior agent via Config.OutputKey).
//
// ADK's State.Get returns (any, error); our ReadonlyState.Get returns (any, bool).
// ──────────────────────────────────────────────────────────────────────────────

type adkSessionState struct {
	state session.State
}

func (s *adkSessionState) Get(key string) (any, bool) {
	v, err := s.state.Get(key)
	return v, err == nil
}

func (s *adkSessionState) All() iter.Seq2[string, any] {
	return s.state.All()
}

// Verify interface compliance at compile time.
var _ fagent.ReadonlyState = (*adkSessionState)(nil)

// authorFromRole maps a thread message role to an adk-go event author.
func authorFromRole(role string, agentName string) string {
	switch role {
	case "user":
		return "user"
	case "assistant":
		if agentName != "" {
			return agentName
		}
		return "model"
	case "tool":
		return "user" // adk-go convention
	default:
		return role
	}
}
