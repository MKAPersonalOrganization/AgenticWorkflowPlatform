package middleware

import (
	"context"
	"fmt"

	"school-with-ai/foundation/agentsdk/core"
)

// CONSTANTS - Domain-Specific Educational Tool Names
const (
	ToolNameMultipleSelection = "quiz_multiple_choice"
	ToolNameTrueFalse         = "quiz_yes_no"
)

// --- TOOL: MULTIPLE CHOICE QUIZ ---
type AskMultipleSelectionTool struct{}

func (t *AskMultipleSelectionTool) Name() string { return ToolNameMultipleSelection }
func (t *AskMultipleSelectionTool) Description() string {
	return "Ask a multiple-choice question."
}

func (t *AskMultipleSelectionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "The question to ask.",
			},
			"options": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "List of answer options.",
				"minItems":    2,
			},
		},
		"required": []string{"question", "options"},
	}
}

func (t *AskMultipleSelectionTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params struct {
		Question string   `json:"question"`
		Options  []string `json:"options"`
	}
	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}

	if params.Question == "" {
		return nil, fmt.Errorf("question is required")
	}
	if len(params.Options) < 2 {
		return nil, fmt.Errorf("at least 2 options required")
	}

	schema := map[string]any{
		"type": "choose_multiple",
		"enum": params.Options,
	}
	return createMultipleChoiceResult(params.Question, params.Options, schema)
}


// --- TOOL: YES/NO QUIZ ---
type AskTrueFalseTool struct{}

func (t *AskTrueFalseTool) Name() string { return ToolNameTrueFalse }
func (t *AskTrueFalseTool) Description() string {
	return "Ask a yes/no question."
}

func (t *AskTrueFalseTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "The question to verify (true/false).",
			},
		},
		"required": []string{"question"},
	}
}

func (t *AskTrueFalseTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params struct {
		Question string `json:"question"`
	}
	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}

	if params.Question == "" {
		return nil, fmt.Errorf("question is required")
	}

	schema := map[string]any{
		"type": "true_false",
	}
	return createTrueFalseResult(params.Question, schema)
}

// --- HELPERS ---

// createMultipleChoiceResult creates result for multiple choice quiz
func createMultipleChoiceResult(question string, options []string, schema map[string]any) (any, error) {
	cmd := &core.Command{
		StopIteration: true,
		Elicitation: &core.ElicitationSpec{
			Question: question,
			Schema:   schema,
		},
	}
	status := fmt.Sprintf("Requesting input: %s", question)
	return core.NewToolResultWithCommand(
		map[string]any{
			"status": status,
		},
		cmd,
	), nil
}


// createTrueFalseResult creates result for true/false quiz with domain-specific parameters
func createTrueFalseResult(statement string, schema map[string]any) (any, error) {
	cmd := &core.Command{
		StopIteration: true,
		Elicitation: &core.ElicitationSpec{
			Question: statement,
			Schema:   schema,
		},
	}
	status := fmt.Sprintf("Requesting input: %s", statement)
	return core.NewToolResultWithCommand(
		map[string]any{
			"status": status,
		},
		cmd,
	), nil
}

// --- MIDDLEWARE ---

// ElicitationMiddleware adds the elicitation tools to the agent.
type ElicitationMiddleware struct{}

func NewElicitationMiddleware() *ElicitationMiddleware {
	return &ElicitationMiddleware{}
}

func (m *ElicitationMiddleware) Process(ctx context.Context, reqCtx *core.RequestContext, next core.NextHandler) error {
	// Add ALL specific elicitation tools
	reqCtx.AddTool(&AskMultipleSelectionTool{})
	reqCtx.AddTool(&AskTrueFalseTool{})

	reqCtx.AddSystemPrompt(`
## Quiz Tools
You MUST use these tools to quiz the student. Arguments are MANDATORY.
1. **quiz_multiple_choice**:
   - arguments: {"question": "...", "options": ["A", "B"]}
2. **quiz_yes_no**:
   - arguments: {"question": "..."}
`)

	return next(ctx, reqCtx)
}
