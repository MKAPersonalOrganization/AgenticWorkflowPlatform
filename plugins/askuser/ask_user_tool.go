package askuser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agent/io"
)

// AskUserQuestionType enumerates the supported question types.
type AskUserQuestionType string

const (
	QuestionTypeChoice AskUserQuestionType = "choice"
	QuestionTypeText   AskUserQuestionType = "text"
	QuestionTypeYesNo  AskUserQuestionType = "yesno"
)

// AskUserOption represents a selectable option with label and description.
type AskUserOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// AskUserQuestion represents a single question in the ask_user tool.
type AskUserQuestion struct {
	Question    string              `json:"question"`
	Header      string              `json:"header"`
	Type        AskUserQuestionType `json:"type"`
	Options     []AskUserOption     `json:"options,omitempty"`
	MultiSelect bool                `json:"multiSelect,omitempty"`
	Placeholder string              `json:"placeholder,omitempty"`
}

// AskUserTool is a Gemini-style tool for requesting user input.
// It supports 1-4 questions per invocation, each with rich options,
// header chips, and multiple question types.
type AskUserTool struct{}

func (t *AskUserTool) Name() string {
	return "ask_user"
}

func (t *AskUserTool) Description() string {
	return "Ask the user one or more questions to gather preferences, clarify requirements, or make decisions. Supports choice (with rich labeled options), text, and yesno question types."
}

func (t *AskUserTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type: agent.TypeObject,
		Properties: map[string]*agent.Schema{
			"questions": {
				Type:        agent.TypeArray,
				Description: "Array of 1-4 questions to ask the user.",
				Items: &agent.Schema{
					Type: agent.TypeObject,
					Properties: map[string]*agent.Schema{
						"question": {
							Type:        agent.TypeString,
							Description: "The complete question to ask. Should be clear, specific, and end with a question mark.",
						},
						"header": {
							Type:        agent.TypeString,
							Description: "Very short label displayed as a chip/tag. Use abbreviations: \"Auth\" not \"Authentication\". Examples: \"Auth method\", \"Library\", \"Approach\", \"Database\".",
						},
						"type": {
							Type:        agent.TypeString,
							Description: "Question type: 'choice' for multiple-choice with options, 'text' for free-form input, 'yesno' for Yes/No confirmation.",
							Enum:        []string{"choice", "text", "yesno"},
						},
						"options": {
							Type:        agent.TypeArray,
							Description: "Selectable choices for 'choice' type questions. Provide 2-4 options. An 'Other' option is automatically added by the UI. Not needed for 'text' or 'yesno' types.",
							Items: &agent.Schema{
								Type: agent.TypeObject,
								Properties: map[string]*agent.Schema{
									"label": {
										Type:        agent.TypeString,
										Description: "Display text for this option (1-5 words). Example: \"OAuth 2.0\"",
									},
									"description": {
										Type:        agent.TypeString,
										Description: "Brief explanation of this option. Example: \"Industry standard, supports SSO\"",
									},
								},
								Required: []string{"label", "description"},
							},
						},
						"multiSelect": {
							Type:        agent.TypeBoolean,
							Description: "Only applies when type='choice'. Set to true to allow selecting multiple options.",
						},
						"placeholder": {
							Type:        agent.TypeString,
							Description: "Hint text shown in the input field. For type='text', shown in the main input. For type='choice', shown in the 'Other' custom input.",
						},
					},
					Required: []string{"question", "header", "type"},
				},
			},
		},
		Required: []string{"questions"},
	}
}

func (t *AskUserTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var params struct {
		Questions []json.RawMessage `json:"questions"`
	}
	if err := args.Decode(&params); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid args: %w", err)), nil
	}

	if len(params.Questions) == 0 {
		return agent.ErrorResult(fmt.Errorf("at least one question is required")), nil
	}
	if len(params.Questions) > 4 {
		return agent.ErrorResult(fmt.Errorf("maximum 4 questions per invocation")), nil
	}

	// Parse and validate all questions.
	questions := make([]AskUserQuestion, 0, len(params.Questions))
	for i, raw := range params.Questions {
		var q AskUserQuestion
		if err := json.Unmarshal(raw, &q); err != nil {
			return agent.ErrorResult(fmt.Errorf("question %d: invalid format: %w", i+1, err)), nil
		}
		if strings.TrimSpace(q.Question) == "" {
			return agent.ErrorResult(fmt.Errorf("question %d: 'question' is required", i+1)), nil
		}
		if strings.TrimSpace(q.Header) == "" {
			return agent.ErrorResult(fmt.Errorf("question %d: 'header' is required", i+1)), nil
		}
		switch q.Type {
		case QuestionTypeChoice:
			if len(q.Options) < 2 {
				return agent.ErrorResult(fmt.Errorf("question %d: type='choice' requires 2-4 options", i+1)), nil
			}
			if len(q.Options) > 4 {
				return agent.ErrorResult(fmt.Errorf("question %d: options array must have at most 4 items", i+1)), nil
			}
			for j, opt := range q.Options {
				if strings.TrimSpace(opt.Label) == "" {
					return agent.ErrorResult(fmt.Errorf("question %d, option %d: 'label' is required", i+1, j+1)), nil
				}
			}
		case QuestionTypeText, QuestionTypeYesNo:
			// Valid types — no additional validation needed.
		default:
			return agent.ErrorResult(fmt.Errorf("question %d: unsupported type %q (use 'choice', 'text', or 'yesno')", i+1, q.Type)), nil
		}
		questions = append(questions, q)
	}

	// Build a summary prompt from all question texts.
	var promptParts []string
	for _, q := range questions {
		promptParts = append(promptParts, q.Question)
	}
	summaryPrompt := strings.Join(promptParts, " | ")

	// Build the rich schema with all questions embedded.
	questionsData := make([]map[string]any, len(questions))
	for i, q := range questions {
		qMap := map[string]any{
			"question": q.Question,
			"header":   q.Header,
			"type":     string(q.Type),
		}
		if len(q.Options) > 0 {
			opts := make([]map[string]string, len(q.Options))
			for j, opt := range q.Options {
				opts[j] = map[string]string{
					"label":       opt.Label,
					"description": opt.Description,
				}
			}
			qMap["options"] = opts
		}
		if q.MultiSelect {
			qMap["multiSelect"] = true
		}
		if q.Placeholder != "" {
			qMap["placeholder"] = q.Placeholder
		}
		questionsData[i] = qMap
	}

	schema := map[string]any{
		"type":      "ask_user",
		"questions": questionsData,
	}

	// Single io.Read() call — pauses the engine, shows the rich multi-question UI.
	response, err := io.Read(ctx, summaryPrompt, schema)
	if err != nil {
		return agent.Result{}, err // Sentinel error (ErrInputRequired) handled by engine.
	}

	// Parse the JSON response: {"0": "answer", "1": "answer", ...}
	var answers map[string]string
	if err := json.Unmarshal([]byte(response), &answers); err != nil {
		// Fallback: treat as a single plain-text answer to the first question.
		return agent.TextResult(response), nil
	}

	// Build structured result for the LLM.
	var resultParts []string
	for i, q := range questions {
		key := fmt.Sprintf("%d", i)
		answer, ok := answers[key]
		if !ok || strings.TrimSpace(answer) == "" {
			answer = "(no answer)"
		}
		resultParts = append(resultParts, fmt.Sprintf("%s: %s", q.Header, answer))
	}

	return agent.TextResult(strings.Join(resultParts, "\n")), nil
}

// Verify interface compliance.
var _ agent.ExecutableTool = (*AskUserTool)(nil)
