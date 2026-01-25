package middleware

import (
	"context"
	"fmt"
	"strings"

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
	// First unmarshal into a flexible map to handle various types
	var raw map[string]any
	if err := core.UnmarshalArgs(args, &raw); err != nil {
		return nil, fmt.Errorf("invalid args: %w", err)
	}

	// Extract question/statement/text
	var question string
	if v, ok := raw["question"].(string); ok {
		question = v
	} else if v, ok := raw["statement"].(string); ok {
		question = v
	} else if v, ok := raw["text"].(string); ok {
		question = v
	}

	if question == "" {
		return nil, fmt.Errorf("question is required (use 'question' parameter)")
	}

	// Extract options - supporting both []any/[]string and string
	var options []string
	if v, ok := raw["options"]; ok {
		switch opt := v.(type) {
		case []any:
			for _, item := range opt {
				if s, ok := item.(string); ok {
					options = append(options, s)
				}
			}
		case []string:
			options = opt
		case string:
			// Split by common delimiters if LLM sent a string
			parts := []string{}
			if strings.Contains(opt, ";") {
				parts = strings.Split(opt, ";")
			} else if strings.Contains(opt, ",") {
				parts = strings.Split(opt, ",")
			} else {
				parts = []string{opt}
			}
			for _, p := range parts {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					options = append(options, trimmed)
				}
			}
		}
	}

	if len(options) < 2 {
		return nil, fmt.Errorf("at least 2 options required in 'options' (array or comma-separated string)")
	}

	schema := map[string]any{
		"type": "choose_multiple",
		"enum": options,
	}
	return createMultipleChoiceResult(question, options, schema)
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
		Question  string `json:"question"`
		Statement string `json:"statement"`
		Text      string `json:"text"`
	}
	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("invalid args (expected schema with question): %w", err)
	}

	question := params.Question
	if question == "" {
		if params.Statement != "" {
			question = params.Statement
		} else if params.Text != "" {
			question = params.Text
		}
	}

	if question == "" {
		return nil, fmt.Errorf("question is required (use 'question' parameter)")
	}

	schema := map[string]any{
		"type": "true_false",
	}
	return createTrueFalseResult(question, schema)
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
   - Schema: {"question": "...", "options": ["A", "B"]}
   - Use this for multiple choice questions.
2. **quiz_yes_no**:
   - Schema: {"question": "..."}
   - Use this for true/false or yes/no questions.
`)

	return next(ctx, reqCtx)
}
