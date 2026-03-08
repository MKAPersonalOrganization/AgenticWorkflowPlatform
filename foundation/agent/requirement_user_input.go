package agent

import (
	"context"
	"fmt"
	"strings"

	"school-with-ai/foundation/agentsdk/io"
)

// RequireUserInput is a declarative Requirement that enables the agent
// to request structured user input during execution via the ask_user tool.
//
// Inspired by Gemini CLI's ask_user — a single unified tool that replaces
// separate quiz_multiple_choice / quiz_yes_no tools with a richer schema
// supporting choice, text, and yesno question types.
//
// It implements three interfaces:
//   - Requirement:    produces rules making ask_user always available
//   - ToolProvider:   provides the ask_user tool
//   - PromptProvider: contributes system prompt instructions about the tool
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

// NewRequireUserInput creates a new RequireUserInput with the ask_user tool.
func NewRequireUserInput() *RequireUserInput {
	return &RequireUserInput{
		tools: []ExecutableTool{
			&AskUserTool{},
		},
		prompt: `
## User Interaction Tool
You have access to the **ask_user** tool to request input from the user when needed.
Use it to verify understanding, gather clarification, or present choices.

**ask_user** supports three question types:
1. **choice**: Present options for the user to select from.
   - Schema: {"question": "...", "type": "choice", "options": ["A", "B", "C"]}
2. **text**: Request free-form text input.
   - Schema: {"question": "...", "type": "text"}
3. **yesno**: Ask a yes/no or true/false question.
   - Schema: {"question": "...", "type": "yesno"}

Guidelines:
- Use "choice" when there are clear, discrete options (quizzes, preferences, selections).
- Use "text" when you need open-ended input (explanations, names, topics).
- Use "yesno" for binary verification or true/false questions.
- Always provide clear, well-formed questions.
- For "choice" type, provide at least 2 options.
`,
	}
}

// ProvidedTools implements ToolProvider — returns the ask_user tool.
func (r *RequireUserInput) ProvidedTools() []ExecutableTool {
	return r.tools
}

// PromptAddition implements PromptProvider — returns ask_user instructions.
func (r *RequireUserInput) PromptAddition() string {
	return r.prompt
}

// Run implements Requirement — ask_user is always allowed when enabled.
func (r *RequireUserInput) Run(ctx context.Context, inv *Invocation) ([]Rule, error) {
	rules := make([]Rule, 0, len(r.tools))
	for _, t := range r.tools {
		rules = append(rules, Rule{
			TargetTool: t.Name(),
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
// AskUserTool — unified user interaction tool
// ==========================================================================

const ToolNameAskUser = "ask_user"

// AskUserQuestionType enumerates the supported question types.
type AskUserQuestionType string

const (
	QuestionTypeChoice AskUserQuestionType = "choice"
	QuestionTypeText   AskUserQuestionType = "text"
	QuestionTypeYesNo  AskUserQuestionType = "yesno"
)

// AskUserTool is a unified tool for requesting user input.
// It replaces the separate quiz_multiple_choice and quiz_yes_no tools
// with a single tool that supports multiple question types.
type AskUserTool struct{}

func (t *AskUserTool) Name() string {
	return ToolNameAskUser
}

func (t *AskUserTool) Description() string {
	return "Ask the user a question. Supports choice (multiple-choice), text (free-form), and yesno (true/false) question types."
}

func (t *AskUserTool) Schema() *Schema {
	return &Schema{
		Type: TypeObject,
		Properties: map[string]*Schema{
			"question": {
				Type:        TypeString,
				Description: "The question to present to the user.",
			},
			"type": {
				Type:        TypeString,
				Description: "The type of input expected from the user.",
				Enum:        []string{"choice", "text", "yesno"},
			},
			"options": {
				Type:        TypeArray,
				Description: "Answer options (required when type is 'choice'). At least 2 options.",
				Items: &Schema{
					Type: TypeString,
				},
			},
			"placeholder": {
				Type:        TypeString,
				Description: "Optional hint text shown in the input field (only for type 'text').",
			},
		},
		Required: []string{"question", "type"},
	}
}

func (t *AskUserTool) Execute(ctx context.Context, args Arguments) (Result, error) {
	var params struct {
		Question    string `json:"question"`
		Type        string `json:"type"`
		Options     any    `json:"options"`
		Placeholder string `json:"placeholder"`
	}
	if err := args.Decode(&params); err != nil {
		return ErrorResult(fmt.Errorf("invalid args: %w", err)), nil
	}

	question := strings.TrimSpace(params.Question)
	if question == "" {
		return ErrorResult(fmt.Errorf("question is required")), nil
	}

	qType := AskUserQuestionType(params.Type)

	switch qType {
	case QuestionTypeChoice:
		return t.executeChoice(ctx, question, params.Options)
	case QuestionTypeText:
		return t.executeText(ctx, question, params.Placeholder)
	case QuestionTypeYesNo:
		return t.executeYesNo(ctx, question)
	default:
		return ErrorResult(fmt.Errorf("unsupported question type %q (use 'choice', 'text', or 'yesno')", params.Type)), nil
	}
}

func (t *AskUserTool) executeChoice(ctx context.Context, question string, rawOptions any) (Result, error) {
	options := extractOptions(rawOptions)
	if len(options) < 2 {
		return ErrorResult(fmt.Errorf("at least 2 options required for 'choice' type")), nil
	}

	schema := map[string]any{
		"type": "choose_multiple",
		"enum": options,
	}
	response, err := io.Read(ctx, question, schema)
	if err != nil {
		return Result{}, err // sentinel error handled by engine
	}
	return TextResult(response), nil
}

func (t *AskUserTool) executeText(ctx context.Context, question string, placeholder string) (Result, error) {
	schema := map[string]any{
		"type": "short_answer",
	}
	if placeholder != "" {
		schema["placeholder"] = placeholder
	}
	response, err := io.Read(ctx, question, schema)
	if err != nil {
		return Result{}, err
	}
	return TextResult(response), nil
}

func (t *AskUserTool) executeYesNo(ctx context.Context, question string) (Result, error) {
	schema := map[string]any{
		"type": "true_false",
	}
	response, err := io.Read(ctx, question, schema)
	if err != nil {
		return Result{}, err
	}
	return TextResult(response), nil
}

// ==========================================================================
// Helpers
// ==========================================================================

// extractOptions extracts answer options from tool arguments,
// supporting arrays and comma/semicolon-separated strings.
func extractOptions(v any) []string {
	if v == nil {
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
