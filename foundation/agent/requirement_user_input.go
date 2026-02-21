package agent

import (
	"context"
	"fmt"
	"strings"
)

// RequireUserInput is a declarative Requirement that enables the agent
// to request structured user input (quizzes, true/false) during execution.
//
// It implements three interfaces:
//   - Requirement:    produces rules making quiz tools always available
//   - ToolProvider:   provides quiz tools (quiz_multiple_choice, quiz_yes_no)
//   - PromptProvider: contributes system prompt instructions about the tools
//
// Usage (declarative on Config):
//
//	agent.Config{
//	    Requirements: []agent.Requirement{
//	        agent.NewRequireUserInput(),
//	    },
//	}
type RequireUserInput struct {
	tools  []ExecutableTool
	prompt string
}

// NewRequireUserInput creates a new RequireUserInput with the standard quiz tools.
func NewRequireUserInput() *RequireUserInput {
	return &RequireUserInput{
		tools: []ExecutableTool{
			&QuizMultipleChoiceTool{},
			&QuizTrueFalseTool{},
		},
		prompt: `
## Verification Tools
You MUST use these tools to verify understanding when appropriate. Arguments are MANDATORY.
1. **quiz_multiple_choice**:
   - Schema: {"question": "...", "options": ["A", "B"]}
   - Use this for multiple choice verification.
2. **quiz_yes_no**:
   - Schema: {"question": "..."}
   - Use this for true/false or yes/no verification.
`,
	}
}

// ProvidedTools implements ToolProvider — returns the quiz tools.
func (r *RequireUserInput) ProvidedTools() []ExecutableTool {
	return r.tools
}

// PromptAddition implements PromptProvider — returns quiz tool instructions.
func (r *RequireUserInput) PromptAddition() string {
	return r.prompt
}

// Run implements Requirement — quiz tools are always allowed when enabled.
func (r *RequireUserInput) Run(ctx context.Context, inv *Invocation) ([]Rule, error) {
	rules := make([]Rule, 0, len(r.tools))
	for _, t := range r.tools {
		rules = append(rules, Rule{
			TargetTool: t.Name(),
			// Blocked: false (zero value) — tools are available.
		})
	}
	return rules, nil
}

// Verify interface compliance.
var (
	_ Requirement    = (*RequireUserInput)(nil)
	_ ToolProvider   = (*RequireUserInput)(nil)
	_ PromptProvider = (*RequireUserInput)(nil)
)

// ==========================================================================
// Quiz Tools
// ==========================================================================

const (
	ToolNameQuizMultipleChoice = "quiz_multiple_choice"
	ToolNameQuizTrueFalse      = "quiz_yes_no"
)

// QuizMultipleChoiceTool presents a multiple-choice question to the user.
// It returns a result that pauses agent execution until the user responds.
type QuizMultipleChoiceTool struct{}

func (t *QuizMultipleChoiceTool) ID() ID { return ID(ToolNameQuizMultipleChoice) }
func (t *QuizMultipleChoiceTool) Name() string {
	return ToolNameQuizMultipleChoice
}

func (t *QuizMultipleChoiceTool) Description() string {
	return "Ask a multiple-choice question."
}

func (t *QuizMultipleChoiceTool) Schema() *Schema {
	return &Schema{
		Type: TypeObject,
		Properties: map[string]*Schema{
			"question": {
				Type:        TypeString,
				Description: "The question to ask.",
			},
			"options": {
				Type:        TypeArray,
				Description: "List of answer options.",
				Items: &Schema{
					Type: TypeString,
				},
			},
		},
		Required: []string{"question", "options"},
	}
}

func (t *QuizMultipleChoiceTool) Execute(ctx context.Context, args Arguments) (Result, error) {
	var raw map[string]any
	if err := args.Decode(&raw); err != nil {
		return ErrorResult(fmt.Errorf("invalid args: %w", err)), nil
	}

	question := extractQuestion(raw)
	if question == "" {
		return Result{}, fmt.Errorf("question is required (use 'question' parameter)")
	}

	options := extractOptions(raw)
	if len(options) < 2 {
		return Result{}, fmt.Errorf("at least 2 options required in 'options' (array or comma-separated string)")
	}

	schema := map[string]any{
		"type": "choose_multiple",
		"enum": options,
	}

	// Return a special result format that the adapter layer recognizes
	// and converts to the engine's Command{Elicitation} format.
	// The adapter handles converting agent.Result → core.ToolResult with Command.
	return Result{
		Content: []any{map[string]any{
			"status": fmt.Sprintf("Requesting input: %s", question),
		}},
		Metadata: map[string]any{
			"elicitation": map[string]any{
				"question": question,
				"schema":   schema,
			},
			"stop_iteration": true,
		},
	}, nil
}

// QuizTrueFalseTool presents a true/false question to the user.
type QuizTrueFalseTool struct{}

func (t *QuizTrueFalseTool) ID() ID { return ID(ToolNameQuizTrueFalse) }
func (t *QuizTrueFalseTool) Name() string {
	return ToolNameQuizTrueFalse
}

func (t *QuizTrueFalseTool) Description() string {
	return "Ask a yes/no question."
}

func (t *QuizTrueFalseTool) Schema() *Schema {
	return &Schema{
		Type: TypeObject,
		Properties: map[string]*Schema{
			"question": {
				Type:        TypeString,
				Description: "The question to verify (true/false).",
			},
		},
		Required: []string{"question"},
	}
}

func (t *QuizTrueFalseTool) Execute(ctx context.Context, args Arguments) (Result, error) {
	var params struct {
		Question  string `json:"question"`
		Statement string `json:"statement"`
		Text      string `json:"text"`
	}
	if err := args.Decode(&params); err != nil {
		return ErrorResult(fmt.Errorf("invalid args (expected schema with question): %w", err)), nil
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
		return Result{}, fmt.Errorf("question is required (use 'question' parameter)")
	}

	schema := map[string]any{
		"type": "true_false",
	}

	return Result{
		Content: []any{map[string]any{
			"status": fmt.Sprintf("Requesting input: %s", question),
		}},
		Metadata: map[string]any{
			"elicitation": map[string]any{
				"question": question,
				"schema":   schema,
			},
			"stop_iteration": true,
		},
	}, nil
}

// ==========================================================================
// Helpers
// ==========================================================================

// extractQuestion extracts the question text from tool arguments,
// supporting multiple field names for LLM flexibility.
func extractQuestion(args map[string]any) string {
	for _, key := range []string{"question", "statement", "text"} {
		if v, ok := args[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// extractOptions extracts answer options from tool arguments,
// supporting arrays and comma/semicolon-separated strings.
func extractOptions(args map[string]any) []string {
	v, ok := args["options"]
	if !ok {
		return nil
	}

	switch opt := v.(type) {
	case []any:
		var out []string
		for _, item := range opt {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return opt
	case string:
		var parts []string
		if strings.Contains(opt, ";") {
			parts = strings.Split(opt, ";")
		} else if strings.Contains(opt, ",") {
			parts = strings.Split(opt, ",")
		} else {
			parts = []string{opt}
		}
		var out []string
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}
	return nil
}
