package custom

import (
	"fmt"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core"
)

// ToDomainEvent translates a core.Event to a domain.Event.
func ToDomainEvent(e core.Event) domain.Event {
	if e == nil {
		return nil
	}

	// Convert core events to proper domain events
	switch evt := e.(type) {
	case *core.UserMessageAdded:
		return &domain.UserMessageAdded{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			Content:   evt.Content,
			UserID:    evt.UserID,
			Timestamp: evt.Timestamp,
		}
	case *core.AssistantMessageAdded:
		return &domain.AssistantMessageAdded{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			Content:   evt.Content,
			ToolCalls: MapCoreToolCallsToDomain(evt.ToolCalls),
			AgentName: evt.AgentName,
			Timestamp: evt.Timestamp,
		}
	case *core.ToolResultAdded:
		return &domain.ToolResultAdded{
			ThreadID:   evt.ThreadID,
			MessageID:  evt.MessageID,
			ToolCallID: evt.ToolCallID,
			ToolName:   evt.ToolName,
			Result:     evt.Result,
			IsError:    evt.IsError,
			AgentName:  evt.AgentName,
			Timestamp:  evt.Timestamp,
		}
	case *core.TokenUsageReported:
		return (*domain.TokenUsageReported)(evt)
	case *core.ExecutionStateReported:
		return (*domain.ExecutionStateReported)(evt)
	case *core.AssistantMessageStarted:
		return (*domain.AssistantMessageStarted)(evt)
	case *core.AssistantContentStreamed:
		return (*domain.AssistantContentStreamed)(evt)
	case *core.RunStarted:
		return (*domain.RunStarted)(evt)
	case *core.RunFinished:
		return (*domain.RunFinished)(evt)
	case *core.TurnFinished:
		return (*domain.TurnFinished)(evt)
	case *core.AssistantMessageCompleted:
		// Manual conversion for AssistantMessageCompleted
		return &domain.AssistantMessageCompleted{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			Timestamp: evt.Timestamp,
		}
	default:
		// Handle value types that need special conversion
		if evt, ok := e.(core.EngineStateChanged); ok {
			return &domain.EngineStateChanged{
				ThreadID:  evt.ThreadID,
				NewState:  evt.State, // core.State -> domain.NewState
				Timestamp: evt.Timestamp,
			}
		}

		// If it's already a domain event, return as-is
		if de, ok := e.(domain.Event); ok {
			return de
		}
		// Unknown event type, skip it (this might include transient events like streaming)
		return nil
	}
}

// MapCoreToolCallsToDomain maps core.ToolCall to domain.ToolCall
func MapCoreToolCallsToDomain(calls []core.ToolCall) []domain.ToolCall {
	if calls == nil {
		return nil
	}
	res := make([]domain.ToolCall, len(calls))
	for i, c := range calls {
		res[i] = domain.ToolCall{
			ID:        c.ID,
			Type:      c.Type,
			Name:      c.Name,
			Arguments: c.Arguments,
		}
	}
	return res
}

// ConvertCoreEventToAgent converts domain.Event to agent.Event.
func ConvertCoreEventToAgent(coreEvent domain.Event) agent.Event {
	if coreEvent == nil {
		return nil
	}

	eventType := MapCoreEventType(coreEvent.EventType())
	data := ConvertCoreEventData(coreEvent)

	return &agent.BaseEvent{
		EventType: eventType,
		EventTime: coreEvent.EventTimestamp(),
		EventData: data,
	}
}

// MapCoreEventType maps core event types to agent event types.
func MapCoreEventType(coreType string) agent.EventType {
	switch coreType {
	case "UserMessageAdded", "AssistantMessageAdded", "user_message_added", "assistant_message_added":
		return agent.EventTypeMessage
	case "ToolStart", "tool_call_started":
		return agent.EventTypeToolCall
	case "ToolSuccess", "ToolError", "ToolResultAdded", "tool_result_added":
		return agent.EventTypeToolResult
	case "TokenUsageReported", "token_usage_reported":
		return agent.EventTypeUsage
	case "AgentHandoff", "agent_handoff":
		return agent.EventTypeHandoff
	case "TitleGenerated", "title_generated":
		return agent.EventTypeTitleGenerated
	case "ToolNeedsInput", "ToolNeedsAuth", "tool_interaction":
		return agent.EventTypeToolInteraction
	case "Error", "error":
		return agent.EventTypeError
	default:
		return agent.EventType(coreType)
	}
}

// ConvertCoreEventData converts domain event data to agent event data.
func ConvertCoreEventData(coreEvent domain.Event) any {
	eventType := coreEvent.EventType()

	switch eventType {
	case "UserMessageAdded", "AssistantMessageAdded", "AssistantMessageCompleted", "user_message_added", "assistant_message_added":
		// These events often carry the message directly
		if msgEvent, ok := coreEvent.(interface{ GetMessage() core.Message }); ok {
			msg := msgEvent.GetMessage()
			return &agent.MessageEvent{
				Message: ConvertCoreMessageToAgent(msg),
			}
		}

	case "ToolStart", "tool_call_started":
		// Handle both domain.ToolStart and legacy tool_call_started
		if ts, ok := coreEvent.(*domain.ToolStart); ok {
			return &agent.ToolCallEvent{
				ToolCall: &agent.ToolCall{
					ID:        ts.ToolCallID,
					Name:      ts.ToolName,
					Arguments: ts.ToolArguments,
				},
			}
		}
		if tcEvent, ok := coreEvent.(interface{ GetToolCall() *core.ToolCall }); ok {
			tc := tcEvent.GetToolCall()
			return &agent.ToolCallEvent{
				ToolCall: &agent.ToolCall{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			}
		}

	case "ToolSuccess", "ToolError", "ToolResultAdded", "tool_result_added":
		if tr, ok := coreEvent.(*domain.ToolResultAdded); ok {
			return &agent.ToolResultEvent{
				Result: &agent.ToolResult{
					ToolCallID: tr.ToolCallID,
					Output:     tr.Result,
					IsError:    false,
				},
			}
		}
		if ts, ok := coreEvent.(*domain.ToolSuccess); ok {
			return &agent.ToolResultEvent{
				Result: &agent.ToolResult{
					ToolCallID: ts.ToolCallID,
					Output:     ts.Result,
					IsError:    false,
				},
			}
		}
		if te, ok := coreEvent.(*domain.ToolError); ok {
			return &agent.ToolResultEvent{
				Result: &agent.ToolResult{
					ToolCallID: te.ToolCallID,
					Output:     te.Error,
					IsError:    true,
					Error:      te.Error,
				},
			}
		}
		if trEvent, ok := coreEvent.(interface{ GetToolResult() *core.ToolResult }); ok {
			tr := trEvent.GetToolResult()
			return &agent.ToolResultEvent{
				Result: &agent.ToolResult{
					ToolCallID: "", // Not directly available in core.ToolResult
					Output:     fmt.Sprintf("%v", tr.Result),
					IsError:    false, // core.ToolResult doesn't have IsError
				},
			}
		}

	case "token_usage_reported":
		if usageEvent, ok := coreEvent.(interface {
			GetPromptTokens() int
			GetCompletionTokens() int
			GetTotalTokens() int
		}); ok {
			return &agent.UsageEvent{
				Usage: &agent.Usage{
					PromptTokens:     usageEvent.GetPromptTokens(),
					CompletionTokens: usageEvent.GetCompletionTokens(),
					TotalTokens:      usageEvent.GetTotalTokens(),
				},
			}
		}

	case "agent_handoff":
		if hoEvent, ok := coreEvent.(interface {
			GetFromAgent() string
			GetToAgent() string
			GetReason() string
		}); ok {
			return &agent.HandoffEvent{
				FromAgent: hoEvent.GetFromAgent(),
				ToAgent:   hoEvent.GetToAgent(),
				Reason:    hoEvent.GetReason(),
			}
		}

	case "title_generated":
		if titleEvent, ok := coreEvent.(interface{ GetTitle() string }); ok {
			return &agent.TitleEvent{
				Title: titleEvent.GetTitle(),
			}
		}

	case "ToolNeedsInput":
		if ti, ok := coreEvent.(*domain.ToolNeedsInput); ok {
			return &agent.ToolInteractionEvent{
				ToolCallID: ti.ToolCallID,
				ToolName:   ti.ToolName,
				Type:       "input",
				Prompt:     ti.Elicitation.Question,
				Schema:     ti.Elicitation.Schema,
			}
		}

	case "ToolNeedsAuth":
		if ta, ok := coreEvent.(*domain.ToolNeedsAuth); ok {
			return &agent.ToolInteractionEvent{
				ToolCallID: ta.ToolCallID,
				ToolName:   ta.ToolName,
				Type:       "auth",
				AuthSpec:   ta.AuthSpec,
			}
		}
	}

	// Return raw event if we can't convert it
	return coreEvent
}

// ConvertCoreMessageToAgent converts core.Message to agent.Message.
func ConvertCoreMessageToAgent(msg core.Message) *agent.Message {
	agentMsg := &agent.Message{
		ID:        msg.ID,
		Role:      agent.MessageRole(msg.Role),
		Content:   make([]agent.Content, 0),
		Timestamp: msg.Timestamp,
	}

	if agentMsg.Timestamp.IsZero() {
		agentMsg.Timestamp = time.Now()
	}

	// Convert text content
	if msg.Content != "" {
		agentMsg.Content = append(agentMsg.Content, agent.Content{
			Type: agent.ContentTypeText,
			Text: msg.Content,
		})
	}

	// Convert tool calls
	for _, tc := range msg.ToolCalls {
		agentMsg.Content = append(agentMsg.Content, agent.Content{
			Type: agent.ContentTypeToolCall,
			ToolCall: &agent.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			},
		})
	}

	// Convert tool result
	if msg.ToolCallID != "" {
		agentMsg.Content = append(agentMsg.Content, agent.Content{
			Type: agent.ContentTypeToolResult,
			ToolResult: &agent.ToolResult{
				ToolCallID: msg.ToolCallID,
				Output:     msg.Content, // For tool result messages, content is the result
				IsError:    false,       // Need more context to determine error
			},
		})
	}

	return agentMsg
}
