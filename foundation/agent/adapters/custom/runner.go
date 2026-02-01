package custom

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/registry"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/agentorchestrator"
)

// RunnerAdapter adapts agentorchestrator.Service to the agent.Runner interface.
type RunnerAdapter struct {
	log     *logger.Logger
	service *agentorchestrator.Service
}

// NewRunnerAdapter creates a runner adapter from an orchestrator service.
func NewRunnerAdapter(log *logger.Logger, service *agentorchestrator.Service) agent.Runner {
	return &RunnerAdapter{
		log:     log,
		service: service,
	}
}

// GetService returns the underlying orchestrator service.
func (r *RunnerAdapter) GetService() *agentorchestrator.Service {
	return r.service
}

// Run executes an agent with the given request.
func (r *RunnerAdapter) Run(ctx context.Context, req *agent.RunRequest) iter.Seq2[agent.Event, error] {
	return func(yield func(agent.Event, error) bool) {
		// Load thread and history
		threadMgmt := r.service.GetThreadManager()
		agg, err := threadMgmt.Load(ctx, req.SessionID)
		if err != nil && !errors.Is(err, domain.ErrThreadNotFound) {
			yield(nil, fmt.Errorf("failed to load thread: %w", err))
			return
		}

		var thread core.AgentThread
		var messages []core.Message

		if agg != nil {
			thread = threadMgmt.ToAgentThread(agg)
			var hErr error
			messages, hErr = thread.RetrieveFullConversationHistory(ctx)
			if hErr != nil {
				yield(nil, fmt.Errorf("failed to retrieve history: %w", hErr))
				return
			}
		} else {
			thread = core.NewInMemoryThreadWithID(req.SessionID)
			messages = []core.Message{}
		}

		// Create RunContext
		runCtx := core.RunContext{
			ThreadID:       req.SessionID,
			UserID:         req.UserID,
			UserContent:    extractUserPrompt(req.Message),
			Thread:         thread,
			InitialHistory: messages,
		}

		if req.Message != nil && req.Message.ID != "" {
			runCtx.MessageID = req.Message.ID
		} else {
			runCtx.MessageID = core.GenerateID()
		}

		// Apply configuration if provided
		if req.Config != nil {
			runCtx.Options = core.RunOptions{
				Temperature:   &req.Config.Temperature,
				MaxTokens:     &req.Config.MaxTokens,
				ModelID:       req.Config.ModelID,
				StopSequences: req.Config.StopSequences,
			}
		}

		// Run the agent and convert events
		for event, err := range r.service.Run(ctx, req.AgentName, runCtx) {
			if err != nil {
				yield(nil, err)
				continue
			}

			// Convert core.Event to agent.Event
			agentEvent := convertCoreEventToAgent(event)
			if agentEvent != nil {
				if !yield(agentEvent, nil) {
					return
				}
			}
		}
	}
}

// GetAgent returns an agent by name.
func (r *RunnerAdapter) GetAgent(name string) (agent.Agent, error) {
	factory, ok := registry.GlobalAgents.Get(name)
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", name)
	}

	// Create the agent config using the factory
	config := factory(context.Background(), nil)

	return NewAgentAdapter(config), nil
}

// ListAgents returns all available agent names.
func (r *RunnerAdapter) ListAgents() []string {
	return registry.GlobalAgents.List()
}

// extractUserPrompt extracts the text from a message.
func extractUserPrompt(msg *agent.Message) string {
	if msg == nil {
		return ""
	}

	for _, c := range msg.Content {
		if c.Type == agent.ContentTypeText {
			return c.Text
		}
	}

	return ""
}

// convertCoreEventToAgent converts core.Event to agent.Event.
func convertCoreEventToAgent(coreEvent core.Event) agent.Event {
	if coreEvent == nil {
		return nil
	}

	eventType := mapCoreEventType(coreEvent.EventType())
	data := convertCoreEventData(coreEvent)

	return &agent.BaseEvent{
		EventType: eventType,
		EventTime: coreEvent.EventTimestamp(),
		EventData: data,
	}
}

// mapCoreEventType maps core event types to agent event types.
func mapCoreEventType(coreType string) agent.EventType {
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
	case "Error", "error":
		return agent.EventTypeError
	default:
		return agent.EventType(coreType)
	}
}

// convertCoreEventData converts core event data to agent event data.
func convertCoreEventData(coreEvent core.Event) any {
	eventType := coreEvent.EventType()

	switch eventType {
	case "UserMessageAdded", "AssistantMessageAdded", "AssistantMessageCompleted", "user_message_added", "assistant_message_added":
		// These events often carry the message directly
		if msgEvent, ok := coreEvent.(interface{ GetMessage() core.Message }); ok {
			msg := msgEvent.GetMessage()
			return &agent.MessageEvent{
				Message: convertCoreMessageToAgent(msg),
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
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
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
	}

	// Return raw event if we can't convert it
	return coreEvent
}

// convertCoreMessageToAgent converts core.Message to agent.Message.
func convertCoreMessageToAgent(msg core.Message) *agent.Message {
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
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
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
