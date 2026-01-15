package tools

import (
	"encoding/json"
	"errors"
	"fmt"
	"school-with-ai/foundation/agentsdk/core"
)

var (
	ErrInvalidAction         = errors.New("invalid action")
	ErrUnknownInstructorTool = errors.New("unknown instructor tool")
)

// WriteWhiteboardInput is the input for writing to the shared whiteboard.
type WriteWhiteboardInput struct {
	Content string `json:"content"`
	Action  string `json:"action"` // "write", "append", or "clear"
}

// WriteWhiteboardTool implements the Tool interface for writing to the whiteboard.
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

func (t *WriteWhiteboardTool) Invoke(ctx core.ToolContext, arguments string) (core.ToolResult, error) {
	res, err := WriteWhiteboard(arguments)
	if err != nil {
		return core.ToolResult{}, err
	}
	return core.NewToolResult(res), nil
}

// WriteWhiteboard allows instructors to write to the shared whiteboard.
func WriteWhiteboard(input string) (string, error) {
	var params WriteWhiteboardInput
	err := json.Unmarshal([]byte(input), &params)
	if err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate action
	validActions := map[string]bool{
		"write":  true,
		"append": true,
		"clear":  true,
	}

	if !validActions[params.Action] {
		return "", fmt.Errorf("%w: %s (must be write, append, or clear)", ErrInvalidAction, params.Action)
	}

	// Return confirmation
	return fmt.Sprintf("Whiteboard updated (%s): %s", params.Action, params.Content), nil
}

// ExecuteInstructorTool executes an instructor tool by name.
func ExecuteInstructorTool(name string, arguments string) (string, error) {
	switch name {
	case "write_whiteboard":
		return WriteWhiteboard(arguments)
	default:
		return "", fmt.Errorf("%w: %s", ErrUnknownInstructorTool, name)
	}
}
