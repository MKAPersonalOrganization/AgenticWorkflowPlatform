package middleware_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
)

func TestAskMultipleSelectionTool_Invoke(t *testing.T) {
	tool := &middleware.AskMultipleSelectionTool{}
	ctx := context.Background()
	tCtx := core.ToolContext{Context: ctx}

	tests := []struct {
		name      string
		args      any
		wantError bool
	}{
		{
			name: "Valid args with domain-specific parameters",
			args: map[string]any{
				"question_text":  "Which of the following are primary colors?",
				"answer_choices": []any{"Red", "Blue", "Green", "Yellow"},
			},
			wantError: false,
		},
		{
			name: "Missing answer_choices parameter",
			args: map[string]any{
				"question_text": "Pick a color",
			},
			wantError: true,
		},
		{
			name: "Insufficient answer_choices (only 1)",
			args: map[string]any{
				"question_text":  "Pick a color",
				"answer_choices": []any{"Red"},
			},
			wantError: true,
		},
		{
			name: "Too many answer_choices (more than 6)",
			args: map[string]any{
				"question_text":  "Pick colors",
				"answer_choices": []any{"A", "B", "C", "D", "E", "F", "G"},
			},
			wantError: true,
		},
		{
			name: "Empty question_text",
			args: map[string]any{
				"question_text":  "",
				"answer_choices": []any{"Red", "Blue"},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Invoke(tCtx, tt.args)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				resStruct, ok := result.(core.ToolResult)
				assert.True(t, ok)

				data, ok := resStruct.Result.(map[string]any)
				assert.True(t, ok)
				assert.Contains(t, data["status"], "Requesting input")
				assert.NotContains(t, data, "schema")
				assert.NotContains(t, data, "question_text")

				assert.NotNil(t, resStruct.Command)
				assert.NotNil(t, resStruct.Command.Elicitation)
				assert.Equal(t, tt.args.(map[string]any)["question_text"], resStruct.Command.Elicitation.Question)
				assert.Equal(t, "choose_multiple", resStruct.Command.Elicitation.Schema["type"])
			}
		})
	}
}

func TestAskNumericRangeTool_Invoke(t *testing.T) {
	tool := &middleware.AskNumericRangeTool{}
	ctx := context.Background()
	tCtx := core.ToolContext{Context: ctx}

	tests := []struct {
		name      string
		args      any
		wantError bool
	}{
		{
			name: "Valid scale range with domain-specific parameters",
			args: map[string]any{
				"question_text": "How confident are you with algebra (1-5)?",
				"scale_minimum": 1,
				"scale_maximum": 5,
			},
			wantError: false,
		},
		{
			name: "Scale minimum greater than maximum",
			args: map[string]any{
				"question_text": "Invalid scale",
				"scale_minimum": 5,
				"scale_maximum": 1,
			},
			wantError: true,
		},
		{
			name: "Scale minimum equal to maximum",
			args: map[string]any{
				"question_text": "Single point scale",
				"scale_minimum": 5,
				"scale_maximum": 5,
			},
			wantError: true,
		},
		{
			name: "Empty question_text",
			args: map[string]any{
				"question_text": "",
				"scale_minimum": 1,
				"scale_maximum": 5,
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Invoke(tCtx, tt.args)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAskTrueFalseTool_Invoke(t *testing.T) {
	tool := &middleware.AskTrueFalseTool{}
	ctx := context.Background()
	tCtx := core.ToolContext{Context: ctx}

	tests := []struct {
		name      string
		args      any
		wantError bool
	}{
		{
			name: "Valid statement with domain-specific parameter",
			args: map[string]any{
				"statement_to_verify": "Photosynthesis occurs in the chloroplasts",
			},
			wantError: false,
		},
		{
			name: "Empty statement",
			args: map[string]any{
				"statement_to_verify": "",
			},
			wantError: true,
		},
		{
			name: "Missing statement parameter",
			args: map[string]any{
				// Missing 'statement_to_verify' key results in empty string due to zero value
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Invoke(tCtx, tt.args)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
