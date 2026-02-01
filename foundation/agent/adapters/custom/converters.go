package custom

import (
	"fmt"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core"
)

// ConvertCoreEventToAgent converts core.Event to agent.Event.
func ConvertCoreEventToAgent(coreEvent core.Event) agent.Event {
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

// ConvertCoreEventData converts core event data to agent event data.
func ConvertCoreEventData(coreEvent core.Event) any {
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
