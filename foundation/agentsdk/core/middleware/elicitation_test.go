package middleware_test

import (
	"context"
	"encoding/json"
	"testing"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core/middleware"

	"github.com/stretchr/testify/assert"
)

func TestAskMultipleSelectionTool_Invoke(t *testing.T) {
	tool := &middleware.AskMultipleSelectionTool{}
	ctx := context.Background()

	tests := []struct {
		name      string
		args      any
		wantError bool
	}{
		{
			name: "Valid args with domain-specific parameters",
			args: map[string]any{
				"question": "Which of the following are primary colors?",
				"options":  []any{"Red", "Blue", "Green", "Yellow"},
			},
			wantError: false,
		},
		{
			name: "Missing answer_choices parameter",
			args: map[string]any{
				"question": "Pick a color",
			},
			wantError: true,
		},
		{
			name: "Insufficient answer_choices (only 1)",
			args: map[string]any{
				"question": "Pick a color",
				"options":  []any{"Red"},
			},
			wantError: true,
		},

		{
			name: "Empty question_text",
			args: map[string]any{
				"question": "",
				"options":  []any{"Red", "Blue"},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := agent.Arguments{Data: tt.args}
			result, err := tool.Execute(ctx, args)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Result should contain command with elicitation

				data, ok := resStruct.Result.(map[string]any)
				assert.True(t, ok)
				assert.Contains(t, data["status"], "Requesting input")
				assert.NotContains(t, data, "schema")
				assert.NotContains(t, data, "question")

				assert.NotNil(t, resStruct.Command)
				assert.NotNil(t, resStruct.Command.Elicitation)
				assert.Equal(t, tt.args.(map[string]any)["question"], resStruct.Command.Elicitation.Question)
				assert.Equal(t, "choose_multiple", resStruct.Command.Elicitation.Schema["type"])
			}
		})
	}
}

func TestAskTrueFalseTool_Invoke(t *testing.T) {
	tool := &middleware.AskTrueFalseTool{}
	ctx := context.Background()

	tests := []struct {
		name      string
		args      any
		wantError bool
	}{
		{
			name: "Valid statement with domain-specific parameter",
			args: map[string]any{
				"question": "Photosynthesis occurs in the chloroplasts",
			},
			wantError: false,
		},
		{
			name: "Empty statement",
			args: map[string]any{
				"question": "",
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
			// Marshal args to JSON for agent.Arguments
			argsJSON, _ := json.Marshal(tt.args)
			args := agent.Arguments{Raw: argsJSON}
			_, err := tool.Execute(ctx, args)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
