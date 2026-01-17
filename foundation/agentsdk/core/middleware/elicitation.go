package middleware

import (
	"context"
	"fmt"
	"school-with-ai/foundation/agentsdk/core"
)

const ElicitInputToolName = "elicit_user_input"

const ElicitInputToolDescription = `Request structured input from the user.
Use this tool when you need specific information, confirmation, or choices from the user.
Execution will pause until the user provides the requested input.`

// ElicitInputTool allows agents to request structured input from users.
type ElicitInputTool struct{}

func NewElicitInputTool() *ElicitInputTool {
	return &ElicitInputTool{}
}

func (t *ElicitInputTool) Name() string {
	return ElicitInputToolName
}

func (t *ElicitInputTool) Description() string {
	return ElicitInputToolDescription
}

func (t *ElicitInputTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "The question or prompt to display to the user.",
			},
			"schema": map[string]any{
				"type":        "object",
				"description": "Optional JSON schema describing the expected response structure.",
				"properties": map[string]any{
					"enum": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "List of valid options for multiple-choice questions.",
					},
					"type": map[string]any{
						"type":        "string",
						"description": "The type of input expected (e.g., 'string').",
					},
					"description": map[string]any{
						"type": "string",
					},
					"items": map[string]any{
						"type":        "object",
						"description": "For array types, describes the items (e.g., enum options).",
						"properties": map[string]any{
							"enum": map[string]any{
								"type":        "array",
								"items":       map[string]any{"type": "string"},
								"description": "List of valid options.",
							},
							"type": map[string]any{
								"type": "string",
							},
						},
					},
				},
			},
		},
		"required": []string{"question"},
	}
}

func (t *ElicitInputTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params struct {
		Question string         `json:"question"`
		Schema   map[string]any `json:"schema"`
	}

	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse elicitation arguments: %w", err)
	}

	// Create command to stop iteration and request input
	cmd := &core.Command{
		StopIteration: true,
		Elicitation: &core.ElicitationSpec{
			Question: params.Question,
			Schema:   params.Schema,
		},
	}

	status := fmt.Sprintf("Requesting input: %s", params.Question)

	// Return a result that indicates we are waiting
	return core.NewToolResultWithCommand(
		map[string]any{
			"status":   status,
			"question": params.Question,
			"schema":   params.Schema,
		},
		cmd,
	), nil
}

// ElicitationMiddleware adds the elicitation tool to the agent.
type ElicitationMiddleware struct{}

func NewElicitationMiddleware() *ElicitationMiddleware {
	return &ElicitationMiddleware{}
}

func (m *ElicitationMiddleware) Process(ctx context.Context, reqCtx *core.RequestContext, next core.NextHandler) error {
	// Add the elicitation tool
	reqCtx.AddTool(NewElicitInputTool())

	// Add system prompt instruction
	reqCtx.AddSystemPrompt(`
## User Input
You MUST use the 'elicit_user_input' tool whenever you need to ask the user a question.
NEVER ask the user a question directly in your response text. **Always ask exactly ONE question at a time.** Never ask multiple questions in a single response.
Use this tool for:
- Clarification on a request
- Selection from a list of options
- Approval for an action
- Specific missing information
Execution will pause until the user answers.

To present options (quick replies), use 'type': 'choose_one' with 'enum':
{
  "question": "Which subject?",
  "schema": {
    "type": "choose_one",
    "enum": ["Math", "Physics", "Chemistry"]
  }
}

For multiple selection (checkboxes), use 'type': 'choose_multiple':
{
  "question": "Select all that apply",
  "schema": {
    "type": "choose_multiple",
    "enum": ["Red", "Green", "Blue"]
  }
}

For numeric ranges, use 'type': 'numeric_range':
{
  "question": "Rate from 1 to 5",
  "schema": {
    "type": "numeric_range",
    "minimum": 1,
    "maximum": 5
  }
}

For True/False (single checkbox), use 'type': 'true_false':
{
  "question": "I agree to the terms",
  "schema": {
    "type": "true_false"
  }
}

For short free-form answer, use 'type': 'short_answer':
{
  "question": "What is your name?",
  "schema": {
    "type": "short_answer"
  }
}`)

	return next(ctx, reqCtx)
}
