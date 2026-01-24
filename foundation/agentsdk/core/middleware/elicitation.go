package middleware

import (
	"context"
	"fmt"

	"school-with-ai/foundation/agentsdk/core"
)

// CONSTANTS - Domain-Specific Educational Tool Names
const (
	ToolNameMultipleSelection = "quiz_multiple_choice"
	ToolNameNumericRange      = "quiz_numeric_scale"
	ToolNameTrueFalse         = "quiz_yes_no"
)

// --- TOOL: MULTIPLE CHOICE QUIZ ---
type AskMultipleSelectionTool struct{}

func (t *AskMultipleSelectionTool) Name() string { return ToolNameMultipleSelection }
func (t *AskMultipleSelectionTool) Description() string {
	return "Present a multiple-choice quiz question where the student can select ONE OR MORE correct answers. Use this when checking understanding of concepts with multiple valid responses."
}

func (t *AskMultipleSelectionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question_text": map[string]any{
				"type":        "string",
				"description": "The quiz question text. Frame it as a clear academic question (e.g., 'Which of the following are prime numbers?').",
				"minLength":   1,
			},
			"answer_choices": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":      "string",
					"minLength": 1,
				},
				"description": "Array of answer choices for the student to select from. Each choice should be a distinct option. Minimum 2 choices required, maximum 6 recommended for clarity.",
				"minItems":    2,
				"maxItems":    6,
			},
		},
		"required":             []string{"question_text", "answer_choices"},
		"additionalProperties": false,
	}
}

func (t *AskMultipleSelectionTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params struct {
		QuestionText  string   `json:"question_text"`
		AnswerChoices []string `json:"answer_choices"`
	}
	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse quiz parameters: %w", err)
	}

	// Validation
	if params.QuestionText == "" {
		return nil, fmt.Errorf("parameter 'question_text' is required and must not be empty. Example: {\"question_text\": \"Which are prime numbers?\", \"answer_choices\": [\"2\", \"3\", \"4\"]}")
	}
	if len(params.AnswerChoices) < 2 {
		return nil, fmt.Errorf("parameter 'answer_choices' must contain at least 2 choices. Received: %d. Example: {\"question_text\": \"...\", \"answer_choices\": [\"Option A\", \"Option B\"]}", len(params.AnswerChoices))
	}
	if len(params.AnswerChoices) > 6 {
		return nil, fmt.Errorf("parameter 'answer_choices' should contain at most 6 choices. Received: %d. Consider breaking into multiple questions.", len(params.AnswerChoices))
	}

	schema := map[string]any{
		"type": "choose_multiple",
		"enum": params.AnswerChoices,
	}
	return createMultipleChoiceResult(params.QuestionText, params.AnswerChoices, schema)
}

// --- TOOL: NUMERIC SCALE QUIZ ---
type AskNumericRangeTool struct{}

func (t *AskNumericRangeTool) Name() string { return ToolNameNumericRange }
func (t *AskNumericRangeTool) Description() string {
	return "Request a numeric rating or scale response from the student. Use for confidence ratings (1-10), Likert scales, or quantitative self-assessment."
}

func (t *AskNumericRangeTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question_text": map[string]any{
				"type":        "string",
				"description": "The question prompting the numeric response (e.g., 'On a scale of 1-5, how confident are you with quadratic equations?').",
				"minLength":   1,
			},
			"scale_minimum": map[string]any{
				"type":        "integer",
				"description": "The lowest value on the scale (inclusive). Typically 0 or 1.",
			},
			"scale_maximum": map[string]any{
				"type":        "integer",
				"description": "The highest value on the scale (inclusive). Common ranges: 1-5, 1-10, 0-100.",
			},
		},
		"required":             []string{"question_text", "scale_minimum", "scale_maximum"},
		"additionalProperties": false,
	}
}

func (t *AskNumericRangeTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params struct {
		QuestionText string `json:"question_text"`
		ScaleMinimum int    `json:"scale_minimum"`
		ScaleMaximum int    `json:"scale_maximum"`
	}
	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse quiz scale parameters: %w", err)
	}

	// Validation
	if params.QuestionText == "" {
		return nil, fmt.Errorf("parameter 'question_text' is required. Example: {\"question_text\": \"How do you feel?\", \"scale_minimum\": 1, \"scale_maximum\": 5}")
	}
	if params.ScaleMinimum >= params.ScaleMaximum {
		return nil, fmt.Errorf("invalid scale range: scale_minimum (%d) must be strictly less than scale_maximum (%d). Example: {\"scale_minimum\": 1, \"scale_maximum\": 5}", params.ScaleMinimum, params.ScaleMaximum)
	}

	schema := map[string]any{
		"type":    "numeric_range",
		"minimum": params.ScaleMinimum,
		"maximum": params.ScaleMaximum,
	}
	return createNumericScaleResult(params.QuestionText, params.ScaleMinimum, params.ScaleMaximum, schema)
}

// --- TOOL: YES/NO QUIZ ---
type AskTrueFalseTool struct{}

func (t *AskTrueFalseTool) Name() string { return ToolNameTrueFalse }
func (t *AskTrueFalseTool) Description() string {
	return "Present a yes/no or true/false quiz question to verify student understanding. Use for binary confirmation of concepts or facts."
}

func (t *AskTrueFalseTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"statement_to_verify": map[string]any{
				"type":        "string",
				"description": "A declarative statement for the student to confirm as true or false (e.g., 'The quadratic formula can solve all polynomial equations' or 'Photosynthesis occurs in mitochondria').",
				"minLength":   1,
			},
		},
		"required":             []string{"statement_to_verify"},
		"additionalProperties": false,
	}
}

func (t *AskTrueFalseTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params struct {
		StatementToVerify string `json:"statement_to_verify"`
	}
	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse quiz statement: %w", err)
	}

	// Validation
	if params.StatementToVerify == "" {
		return nil, fmt.Errorf("parameter 'statement_to_verify' is required and must not be empty. Example: {\"statement_to_verify\": \"The sky is blue\"}")
	}

	schema := map[string]any{
		"type": "true_false",
	}
	return createTrueFalseResult(params.StatementToVerify, schema)
}

// --- HELPERS ---

// createMultipleChoiceResult creates result for multiple choice quiz with domain-specific parameters
func createMultipleChoiceResult(questionText string, answerChoices []string, schema map[string]any) (any, error) {
	cmd := &core.Command{
		StopIteration: true,
		Elicitation: &core.ElicitationSpec{
			Question: questionText,
			Schema:   schema,
		},
	}
	status := fmt.Sprintf("Requesting input: %s", questionText)
	return core.NewToolResultWithCommand(
		map[string]any{
			"status": status,
		},
		cmd,
	), nil
}

// createNumericScaleResult creates result for numeric scale quiz with domain-specific parameters
func createNumericScaleResult(questionText string, scaleMin int, scaleMax int, schema map[string]any) (any, error) {
	cmd := &core.Command{
		StopIteration: true,
		Elicitation: &core.ElicitationSpec{
			Question: questionText,
			Schema:   schema,
		},
	}
	status := fmt.Sprintf("Requesting input: %s", questionText)
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
	reqCtx.AddTool(&AskNumericRangeTool{})
	reqCtx.AddTool(&AskTrueFalseTool{})

	reqCtx.AddSystemPrompt(`
## Student Elicitation Protocol
You MUST use the specialized quiz tools for ALL student assessments and checks for understanding. 

### Instructions:
1. **quiz_multiple_choice**: Use when checking understanding with predefined options.
   * Format: ` + "`" + `{"question_text": "...", "answer_choices": ["A", "B"]}` + "`" + `
2. **quiz_yes_no**: Use for binary factual verification.
   * Format: ` + "`" + `{"statement_to_verify": "..."}` + "`" + `
3. **quiz_numeric_scale**: Use for confidence or quantitative self-assessment.
   * Format: ` + "`" + `{"question_text": "...", "scale_minimum": 1, "scale_maximum": 5}` + "`" + `

### Rules:
- Proactive interaction: Use a tool in every turn where you expect student input.
- Structural Integrity: Ensure ALL required JSON parameters are populated correctly.
- Single Question: Ask exactly one quiz question per interaction.
`)

	return next(ctx, reqCtx)
}
