package middleware

import (
	"context"
	"fmt"
	"strings"

	"school-with-ai/foundation/agent"
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

func (t *AskMultipleSelectionTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: agent.TypeObject,
		Properties: map[string]*agent.Schema{
			"question": {
				Type:        agent.TypeString,
				Description: "The question to ask.",
			},
			"options": {
				Type:        agent.TypeArray,
				Description: "List of answer options.",
				Items: &agent.Schema{
					Type: agent.TypeString,
				},
			},
		},
		Required: []string{"question", "options"},
	}
}

func (t *AskMultipleSelectionTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	// Unmarshal into a flexible map to handle various types
	var raw map[string]any
	if err := args.Decode(&raw); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid args: %w", err)), nil
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
		return agent.Result{}, fmt.Errorf("question is required (use 'question' parameter)")
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
		return agent.Result{}, fmt.Errorf("at least 2 options required in 'options' (array or comma-separated string)")
	}

	schema := map[string]any{
		"type": "choose_multiple",
		"enum": options,
	}
	result, err := createMultipleChoiceResult(question, options, schema)
	if err != nil {
		return agent.ErrorResult(err), nil
	}
	return agent.NewResult(result), nil
}

// --- TOOL: YES/NO QUIZ ---
type AskTrueFalseTool struct{}

func (t *AskTrueFalseTool) Name() string { return ToolNameTrueFalse }
func (t *AskTrueFalseTool) Description() string {
	return "Ask a yes/no question."
}

func (t *AskTrueFalseTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: agent.TypeObject,
		Properties: map[string]*agent.Schema{
			"question": {
				Type:        agent.TypeString,
				Description: "The question to verify (true/false).",
			},
		},
		Required: []string{"question"},
	}
}

func (t *AskTrueFalseTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var params struct {
		Question  string `json:"question"`
		Statement string `json:"statement"`
		Text      string `json:"text"`
	}
	if err := args.Decode(&params); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid args (expected schema with question): %w", err)), nil
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
		return agent.Result{}, fmt.Errorf("question is required (use 'question' parameter)")
	}

	schema := map[string]any{
		"type": "true_false",
	}
	result, err := createTrueFalseResult(question, schema)
	if err != nil {
		return agent.ErrorResult(err), nil
	}
	return agent.NewResult(result), nil
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
// It now returns an agent.Middleware that can be used in the unified middleware chain.
type ElicitationMiddleware struct{}

func NewElicitationMiddleware() *ElicitationMiddleware {
	return &ElicitationMiddleware{}
}

// Middleware returns an agent.Middleware that injects elicitation tools.
func (m *ElicitationMiddleware) Middleware() agent.Middleware {
	return func(next agent.Handler) agent.Handler {
		return agent.HandlerFunc(func(c *agent.Context) error {
			// Get request context from metadata (set by ApplyMiddleware)
			if reqCtx, ok := c.Metadata["request_context"].(*core.RequestContext); ok {
				// Add ALL specific elicitation tools
				reqCtx.AddTool(&AskMultipleSelectionTool{})
				reqCtx.AddTool(&AskTrueFalseTool{})

				reqCtx.AddSystemPrompt(`
## Verification Tools
You MUST use these tools to verify understanding when appropriate. Arguments are MANDATORY.
1. **quiz_multiple_choice**:
   - Schema: {"question": "...", "options": ["A", "B"]}
   - Use this for multiple choice verification.
2. **quiz_yes_no**:
   - Schema: {"question": "..."}
   - Use this for true/false or yes/no verification.
`)
			}

			return next.Handle(c)
		})
	}
}
