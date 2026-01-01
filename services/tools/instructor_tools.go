package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// CallAgentInput is the unified input for calling any agent
type CallAgentInput struct {
	AgentName string `json:"agent_name"`
	Reason    string `json:"reason"`
	Context   string `json:"context"` // Optional: context to share with the agent
}

// WriteWhiteboardInput is the input for writing to the shared whiteboard
type WriteWhiteboardInput struct {
	Content string `json:"content"`
	Action  string `json:"action"` // "write", "append", or "clear"
}

// EmitEventInput is the input for emitting a custom event
type EmitEventInput struct {
	EventName   string                 `json:"event_name"`
	Payload     map[string]interface{} `json:"payload"`
	Description string                 `json:"description"`
}

// CallAgentTool implements the Tool interface for calling other agents
type CallAgentTool struct{}

func NewCallAgentTool() *CallAgentTool {
	return &CallAgentTool{}
}

func (t *CallAgentTool) Name() string {
	return "call_agent"
}

func (t *CallAgentTool) Description() string {
	return "Call another agent to help with the task. This is the primary way agents collaborate in this event-driven system. Instructors call student-facing-agent after answering to return control."
}

func (t *CallAgentTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"agent_name": map[string]any{
				"type":        "string",
				"description": "The name of the agent to call",
				"enum":        []string{"student-facing-agent", "math-instructor", "physics-instructor", "chemistry-instructor"},
			},
			"reason": map[string]any{
				"type":        "string",
				"description": "Why you're calling this agent and what you need help with",
			},
			"context": map[string]any{
				"type":        "string",
				"description": "Optional: Summary of conversation so far and what the other agent should know",
			},
		},
		"required": []string{"agent_name", "reason"},
	}
}

func (t *CallAgentTool) Invoke(ctx context.Context, arguments string) (string, error) {
	return CallAgent(arguments)
}

// WriteWhiteboardTool implements the Tool interface for writing to the whiteboard
type WriteWhiteboardTool struct{}

func NewWriteWhiteboardTool() *WriteWhiteboardTool {
	return &WriteWhiteboardTool{}
}

func (t *WriteWhiteboardTool) Name() string {
	return "write_whiteboard"
}

func (t *WriteWhiteboardTool) Description() string {
	return "Write information to the shared whiteboard that all agents can see. Use this to share key formulas, diagrams, or important points. ALWAYS use markdown format."
}

func (t *WriteWhiteboardTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"content": map[string]any{
				"type":        "string",
				"description": "The content to write to the whiteboard",
			},
			"action": map[string]any{
				"type":        "string",
				"description": "How to update the whiteboard",
				"enum":        []string{"write", "append", "clear"},
			},
		},
		"required": []string{"content", "action"},
	}
}

func (t *WriteWhiteboardTool) Invoke(ctx context.Context, arguments string) (string, error) {
	return WriteWhiteboard(arguments)
}

// EmitEventTool implements the Tool interface for emitting custom events
type EmitEventTool struct{}

func NewEmitEventTool() *EmitEventTool {
	return &EmitEventTool{}
}

func (t *EmitEventTool) Name() string {
	return "emit_event"
}

func (t *EmitEventTool) Description() string {
	return "Emit a custom event to the event store. This allows you to trigger other agents or workflows in a purely event-driven manner."
}

func (t *EmitEventTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"event_name": map[string]any{
				"type":        "string",
				"description": "Name of the event (e.g., 'task_completed', 'help_needed', 'data_ready')",
			},
			"payload": map[string]any{
				"type":        "object",
				"description": "Event payload with arbitrary data (JSON object)",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Human-readable description of what this event represents",
			},
		},
		"required": []string{"event_name", "description"},
	}
}

func (t *EmitEventTool) Invoke(ctx context.Context, arguments string) (string, error) {
	return EmitEvent(arguments)
}

// CallAgent allows any agent to call any other agent (purely event-driven)
func CallAgent(input string) (string, error) {
	var params CallAgentInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validation
	validAgents := map[string]bool{
		"student-facing-agent": true,
		"math-instructor":      true,
		"physics-instructor":   true,
		"chemistry-instructor": true,
	}

	if !validAgents[params.AgentName] {
		return "", fmt.Errorf("invalid agent: %s", params.AgentName)
	}

	// Return a signal that will be processed by the event handler
	// The actual agent switch happens via domain events
	if params.Context != "" {
		return fmt.Sprintf("Calling %s. Reason: %s. Context: %s", params.AgentName, params.Reason, params.Context), nil
	}
	return fmt.Sprintf("Calling %s. Reason: %s", params.AgentName, params.Reason), nil
}

// WriteWhiteboard allows instructors to write to the shared whiteboard
func WriteWhiteboard(input string) (string, error) {
	var params WriteWhiteboardInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate action
	validActions := map[string]bool{
		"write":  true,
		"append": true,
		"clear":  true,
	}

	if !validActions[params.Action] {
		return "", fmt.Errorf("invalid action: %s (must be write, append, or clear)", params.Action)
	}

	// Return confirmation
	return fmt.Sprintf("Whiteboard updated (%s): %s", params.Action, params.Content), nil
}

// EmitEvent allows agents to emit custom events to trigger other agents or workflows
func EmitEvent(input string) (string, error) {
	var params EmitEventInput
	if err := json.Unmarshal([]byte(input), &params); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validation
	if params.EventName == "" {
		return "", fmt.Errorf("event_name is required")
	}

	if params.Description == "" {
		return "", fmt.Errorf("description is required")
	}

	// Return confirmation - the actual event will be emitted by the agent
	return fmt.Sprintf("Event '%s' emitted: %s", params.EventName, params.Description), nil
}

// ExecuteInstructorTool executes an instructor tool by name
func ExecuteInstructorTool(name string, arguments string) (string, error) {
	switch name {
	case "call_agent":
		return CallAgent(arguments)
	case "write_whiteboard":
		return WriteWhiteboard(arguments)
	case "emit_event":
		return EmitEvent(arguments)
	default:
		return "", fmt.Errorf("unknown instructor tool: %s", name)
	}
}
