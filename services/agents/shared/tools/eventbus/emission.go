package eventbus

import (
	"context"
	"encoding/json"
	"fmt"

	"school-with-ai/services/agents"
	"school-with-ai/sdk/agent-sdk"
)

// EmissionTool is a generic tool for emitting agent activation events.
// It is auto-generated from an agent's Emits declaration.
type EmissionTool struct {
	eventType agents.EventType
}

// NewEmissionTool creates an emission tool for the given event type
func NewEmissionTool(eventType agents.EventType) sdk.Tool {
	return &EmissionTool{eventType: eventType}
}

// Name returns the tool name, e.g., "emit_math_question"
func (t *EmissionTool) Name() string {
	return EmissionToolName(t.eventType)
}

// Description returns a description of what this tool does
func (t *EmissionTool) Description() string {
	slug := EventTypeSlug(t.eventType)
	return fmt.Sprintf(
		"Emit a %s question event. This is a fire-and-forget event - "+
			"agents subscribed to %s events will automatically receive and process it. "+
			"Use this when the question requires %s expertise.",
		slug,
		slug,
		slug,
	)
}

// Parameters returns the JSON schema for the tool's parameters
func (t *EmissionTool) Parameters() map[string]any {
	slug := EventTypeSlug(t.eventType)
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": fmt.Sprintf("The %s question or request that needs to be addressed", slug),
			},
			"context": map[string]any{
				"type":        "string",
				"description": "Optional context or additional information to help the receiving agent",
			},
		},
		"required": []string{"question"},
	}
}

// Invoke executes the tool. The actual event emission is handled by the SDK
// when it recognizes this as a collaboration tool.
func (t *EmissionTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var params struct {
		Question string `json:"question"`
		Context  string `json:"context"`
	}
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	slug := EventTypeSlug(t.eventType)
	return fmt.Sprintf("%s event emitted. Question: %s", slug, params.Question), nil
}

// GenerateEmissionTools creates emission tools for all event types in the Emits list.
// This is called during agent setup to auto-generate tools from the contract.
func GenerateEmissionTools(emits []agents.EventType) []sdk.Tool {
	tools := make([]sdk.Tool, 0, len(emits))
	for _, eventType := range emits {
		// Skip whiteboard.updated - this is handled differently
		if eventType == agents.EventTypeWhiteboardUpdated {
			continue
		}
		// Skip question.answered - this has a custom tool with is_complete parameter
		if eventType == agents.EventTypeQuestionAnswered {
			continue
		}
		tools = append(tools, NewEmissionTool(eventType))
	}
	return tools
}

// Verify EmissionTool implements sdk.Tool
var _ sdk.Tool = (*EmissionTool)(nil)
