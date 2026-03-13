// Package adkgo provides an adapter for Google's ADK-Go agent framework.
//
// This adapter bridges ADK-Go's agent types to the foundation agent interfaces,
// enabling ADK-Go agents to be used through the framework-agnostic agent layer.
//
// Key responsibilities:
//   - Wrap ADK-Go agents as foundation agent.Agent implementations
//   - Wrap ADK-Go tools as foundation agent.ExecutableTool implementations
//   - Map between ADK-Go session.Event and foundation agent.Event types
//   - Bridge ADK-Go's callback model to foundation callbacks
package adkgo

import (
	"context"
	"encoding/json"
	"iter"
	"time"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/session"

	"school-with-ai/foundation/agent"
)

// ──────────────────────────────────────────────────────────────────────────────
// Agent Adapter: ADK-Go agent.Agent → foundation agent.Agent
// ──────────────────────────────────────────────────────────────────────────────

// AgentAdapter wraps an ADK-Go agent to implement the foundation agent.Agent interface.
type AgentAdapter struct {
	adkAgent adkagent.Agent
	config   agent.Config
}

// WrapAgent wraps an ADK-Go agent to implement agent.Agent.
func WrapAgent(adkAgent adkagent.Agent) agent.Agent {
	// Build sub-agents recursively.
	var subAgents []agent.Agent
	for _, sub := range adkAgent.SubAgents() {
		subAgents = append(subAgents, WrapAgent(sub))
	}

	return &AgentAdapter{
		adkAgent: adkAgent,
		config: agent.Config{
			Name:        adkAgent.Name(),
			Description: adkAgent.Description(),
			SubAgents:   subAgents,
		},
	}
}

// ID returns the agent's unique identifier.
func (a *AgentAdapter) ID() agent.ID {
	return a.adkAgent.Name()
}

// Name returns the agent's identifier.
func (a *AgentAdapter) Name() string {
	return a.adkAgent.Name()
}

// Description returns the agent's description.
func (a *AgentAdapter) Description() string {
	return a.adkAgent.Description()
}

// SubAgents returns child agents available for delegation.
func (a *AgentAdapter) SubAgents() []agent.Agent {
	return a.config.SubAgents
}

// Run executes the ADK-Go agent and translates events to foundation events.
// This bridges ADK-Go's iter.Seq2[*session.Event, error] to
// foundation's iter.Seq2[agent.Event, error].
func (a *AgentAdapter) Run(ctx context.Context, inv *agent.Invocation) iter.Seq2[agent.Event, error] {
	return func(yield func(agent.Event, error) bool) {
		// Note: Full ADK-Go execution requires a Runner with session management.
		// Direct agent.Run() requires an InvocationContext which is internal to
		// ADK-Go's runner. For production use, the adapter in adapters/adkgo/
		// handles the full lifecycle via runner.New().
		//
		// This adapter is for wrapping ADK-Go agents as foundation types for
		// discovery, metadata, and delegation — not for direct execution.
		yield(&agent.ErrorEvent{
			Error:     "direct execution requires adapters/adkgo RunnerAdapter; use that for full lifecycle",
			EventTime: time.Now(),
		}, nil)
	}
}

// Config returns the agent's configuration.
func (a *AgentAdapter) Config() agent.Config {
	return a.config
}

// ──────────────────────────────────────────────────────────────────────────────
// Tool Adapter: ADK-Go tool.Tool → foundation agent.ExecutableTool
// ──────────────────────────────────────────────────────────────────────────────

// ToolAdapter wraps an ADK-Go tool to implement the foundation agent.ExecutableTool interface.
type ToolAdapter struct {
	name        string
	description string
}

// WrapTool wraps an ADK-Go tool to implement agent.ExecutableTool.
// Note: ADK-Go's public tool.Tool interface only exposes Name, Description,
// and IsLongRunning. The actual execution interface (FunctionTool) is internal.
// For production, tools are wrapped at the adapter level (adapters/adkgo/tools.go).
func WrapTool(name, description string) agent.ExecutableTool {
	return &ToolAdapter{
		name:        name,
		description: description,
	}
}

func (t *ToolAdapter) Name() string          { return t.name }
func (t *ToolAdapter) Description() string   { return t.description }
func (t *ToolAdapter) Schema() *agent.Schema { return nil }

// Execute implements the Executor interface.
// Production execution happens in the adapters/adkgo layer, not here.
func (t *ToolAdapter) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	return agent.ErrorResult(&NotImplementedError{"use adapters/adkgo for tool execution"}), nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Event Adapter: ADK-Go session.Event → foundation agent.Event
// ──────────────────────────────────────────────────────────────────────────────

// TranslateSessionEvent converts an ADK-Go session.Event to foundation agent.Event(s).
func TranslateSessionEvent(evt *session.Event) []agent.Event {
	if evt == nil || evt.Content == nil {
		return nil
	}

	var events []agent.Event

	// Extract text content.
	var text string
	for _, p := range evt.Content.Parts {
		if p.Text != "" {
			text += p.Text
		}
	}

	if evt.Partial && text != "" {
		// Streaming delta.
		events = append(events, &agent.MessageDeltaEvent{
			MessageID: evt.ID,
			Delta:     text,
			EventTime: coalesceAgentTime(evt.Timestamp),
		})
		return events
	}

	// Complete message.
	if text != "" {
		role := agent.RoleAssistant
		if evt.Author == "user" {
			role = agent.RoleUser
		}
		events = append(events, &agent.MessageEvent{
			Message: &agent.Message{
				ID:     evt.ID,
				Role:   role,
				Author: evt.Author,
				Content: []agent.Content{{
					Type: agent.ContentTypeText,
					Text: text,
				}},
			},
			EventTime: coalesceAgentTime(evt.Timestamp),
		})
	}

	// Tool calls.
	for _, p := range evt.Content.Parts {
		if p.FunctionCall != nil {
			argsJSON, _ := json.Marshal(p.FunctionCall.Args)
			events = append(events, &agent.ToolCallEvent{
				ToolCall: &agent.ToolCall{
					ID:        p.FunctionCall.ID,
					Name:      p.FunctionCall.Name,
					Arguments: string(argsJSON),
				},
				AgentName: evt.Author,
				EventTime: coalesceAgentTime(evt.Timestamp),
			})
		}
	}

	// Tool results.
	for _, p := range evt.Content.Parts {
		if p.FunctionResponse != nil {
			resultJSON, _ := json.Marshal(p.FunctionResponse.Response)
			events = append(events, &agent.ToolResultEvent{
				Result: &agent.ToolResult{
					ToolCallID: p.FunctionResponse.ID,
					ToolName:   p.FunctionResponse.Name,
					Output:     string(resultJSON),
				},
				AgentName: evt.Author,
				EventTime: coalesceAgentTime(evt.Timestamp),
			})
		}
	}

	// Usage metadata.
	if evt.UsageMetadata != nil && evt.UsageMetadata.TotalTokenCount > 0 {
		events = append(events, &agent.UsageEvent{
			Usage: &agent.Usage{
				PromptTokens:     int(evt.UsageMetadata.PromptTokenCount),
				CompletionTokens: int(evt.UsageMetadata.CandidatesTokenCount),
				TotalTokens:      int(evt.UsageMetadata.TotalTokenCount),
			},
			EventTime: coalesceAgentTime(evt.Timestamp),
		})
	}

	return events
}

func coalesceAgentTime(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now()
	}
	return t
}

// NotImplementedError indicates a feature is not yet implemented.
type NotImplementedError struct {
	Feature string
}

func (e *NotImplementedError) Error() string {
	return "not implemented: " + e.Feature
}

// Compile-time interface verification.
var (
	_ agent.Agent          = (*AgentAdapter)(nil)
	_ agent.Configurable   = (*AgentAdapter)(nil)
	_ agent.ExecutableTool = (*ToolAdapter)(nil)
)
