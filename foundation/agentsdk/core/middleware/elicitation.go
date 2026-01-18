package middleware

import (
	"context"
	"fmt"
	"school-with-ai/foundation/agentsdk/core"
)

const ElicitInputToolName = "ask_human"

const ElicitInputToolDescription = `Request input from the human user.
Use this tool for ANY interaction requiring user response, including:
- Asking for clarification
- Administering a quiz or test question
- Requesting approval or confirmation
- Collecting missing information
Execution will pause until the user provides the answer.`

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
				"description": "The question, quiz item, or prompt to display to the user.",
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
## Human Interaction (STRICT ENFORCEMENT)
STOP. READ THIS CAREFULLY.
You are PROHIBITED from asking questions, giving quizzes, or requesting information in your text response.
You MUST use the 'ask_human' tool for ALL interactions requiring a user response.

Violations (DO NOT DO THIS):
- "Can you clarify what you mean?" (VIOLATION: Text question)
- "Here is the first question: What is 2+2?" (VIOLATION: Text quiz)
- "Do you want to proceed?" (VIOLATION: Text confirmation)

Correct Usage (DO THIS):
- Call 'ask_human' with question="Can you clarify what you mean?"
- Call 'ask_human' with question="What is 2+2?"
- Call 'ask_human' with question="Do you want to proceed?" type="true_false"

**CRITICAL RULES**:
1. **ONE Question Per Turn**: Always ask exactly ONE question at a time using the tool.
2. **QUIZZES**: Administer quizzes one question at a time. Ask Q1 -> Wait for Answer -> Grade -> Ask Q2.
3. **Execution Pauses**: The tool will pause execution. Do not say "I will wait for your answer". Just call the tool.

To present options (quick replies or multiple choice), use 'type': 'choose_one' with 'enum':
{
  "question": "What is 2 + 2?",
  "schema": {
    "type": "choose_one",
    "enum": ["3", "4", "5"]
  }
}

For multiple selection (checkboxes), use 'type': 'choose_multiple':
{
  "question": "Select all prime numbers",
  "schema": {
    "type": "choose_multiple",
    "enum": ["2", "4", "5", "9"]
  }
}

For numeric ranges, use 'type': 'numeric_range':
{
  "question": "Rate your confidence (1-10)",
  "schema": {
    "type": "numeric_range",
    "minimum": 1,
    "maximum": 10
  }
}

For True/False, use 'type': 'true_false':
{
  "question": "The earth is flat.",
  "schema": {
    "type": "true_false"
  }
}

For short free-form answer, use 'type': 'short_answer':
{
  "question": "Explain the concept of inertia.",
  "schema": {
    "type": "short_answer"
  }
}`)

	return next(ctx, reqCtx)
}
