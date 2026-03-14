package adkgo

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"school-with-ai/modules/chat/thread"
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

	// Replay thread messages as session events.
	for _, msg := range agg.Messages {
		evt := messageToSessionEvent(msg)
		if evt == nil {
			continue
		}
		if err := svc.AppendEvent(ctx, sess, evt); err != nil {
			return nil, err
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

// messageToSessionEvent converts a domain thread.Message to an adk-go session.Event.
func messageToSessionEvent(msg thread.Message) *session.Event {
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
		var result map[string]any
		if msg.Content != "" {
			_ = json.Unmarshal([]byte(msg.Content), &result)
		}
		if result == nil {
			result = map[string]any{"result": msg.Content}
		}
		content = &genai.Content{
			Role: "user", // adk-go convention: function responses come as user role
			Parts: []*genai.Part{{
				FunctionResponse: &genai.FunctionResponse{
					ID:       msg.ToolCallID,
					Name:     msg.ToolName,
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
