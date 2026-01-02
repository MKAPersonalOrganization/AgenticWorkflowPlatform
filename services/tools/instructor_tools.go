package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

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
	case "write_whiteboard":
		return WriteWhiteboard(arguments)
	case "emit_event":
		return EmitEvent(arguments)
	default:
		return "", fmt.Errorf("unknown instructor tool: %s", name)
	}
}
