package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// EmitMathQuestionTool allows agents to emit a math question event to the event log
type EmitMathQuestionTool struct{}

func NewEmitMathQuestionTool() *EmitMathQuestionTool {
	return &EmitMathQuestionTool{}
}

func (t *EmitMathQuestionTool) Name() string {
	return "emit_math_question"
}

func (t *EmitMathQuestionTool) Description() string {
	return "Emit a math question event to activate the math instructor. The math instructor subscribes to these events and will be automatically invoked to answer the question."
}

func (t *EmitMathQuestionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "The math question that needs to be answered",
			},
			"context": map[string]any{
				"type":        "string",
				"description": "Optional context or additional information about the question",
			},
		},
		"required": []string{"question"},
	}
}

func (t *EmitMathQuestionTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var params struct {
		Question string `json:"question"`
		Context  string `json:"context"`
	}
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	// The actual event emission happens in agent.go:emitCollaborationEvents()
	// This tool just returns a signal that will trigger the event emission
	return fmt.Sprintf("MathQuestion event emitted. Question: %s", params.Question), nil
}

// EmitPhysicsQuestionTool allows agents to emit a physics question event
type EmitPhysicsQuestionTool struct{}

func NewEmitPhysicsQuestionTool() *EmitPhysicsQuestionTool {
	return &EmitPhysicsQuestionTool{}
}

func (t *EmitPhysicsQuestionTool) Name() string {
	return "emit_physics_question"
}

func (t *EmitPhysicsQuestionTool) Description() string {
	return "Emit a physics question event to activate the physics instructor. The physics instructor subscribes to these events and will be automatically invoked to answer the question."
}

func (t *EmitPhysicsQuestionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "The physics question that needs to be answered",
			},
			"context": map[string]any{
				"type":        "string",
				"description": "Optional context or additional information about the question",
			},
		},
		"required": []string{"question"},
	}
}

func (t *EmitPhysicsQuestionTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var params struct {
		Question string `json:"question"`
		Context  string `json:"context"`
	}
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	return fmt.Sprintf("PhysicsQuestion event emitted. Question: %s", params.Question), nil
}

// EmitChemistryQuestionTool allows agents to emit a chemistry question event
type EmitChemistryQuestionTool struct{}

func NewEmitChemistryQuestionTool() *EmitChemistryQuestionTool {
	return &EmitChemistryQuestionTool{}
}

func (t *EmitChemistryQuestionTool) Name() string {
	return "emit_chemistry_question"
}

func (t *EmitChemistryQuestionTool) Description() string {
	return "Emit a chemistry question event to activate the chemistry instructor. The chemistry instructor subscribes to these events and will be automatically invoked to answer the question."
}

func (t *EmitChemistryQuestionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "The chemistry question that needs to be answered",
			},
			"context": map[string]any{
				"type":        "string",
				"description": "Optional context or additional information about the question",
			},
		},
		"required": []string{"question"},
	}
}

func (t *EmitChemistryQuestionTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var params struct {
		Question string `json:"question"`
		Context  string `json:"context"`
	}
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	return fmt.Sprintf("ChemistryQuestion event emitted. Question: %s", params.Question), nil
}

// EmitStatisticsQuestionTool emits statistics question events
type EmitStatisticsQuestionTool struct{}

func NewEmitStatisticsQuestionTool() *EmitStatisticsQuestionTool {
	return &EmitStatisticsQuestionTool{}
}

func (t *EmitStatisticsQuestionTool) Name() string {
	return "emit_statistics_question"
}

func (t *EmitStatisticsQuestionTool) Description() string {
	return "Emit a statistics question event to activate the statistics instructor. The statistics instructor subscribes to these events and will be automatically invoked to analyze data and answer the question."
}

func (t *EmitStatisticsQuestionTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "The statistics/data analysis question that needs to be answered",
			},
			"context": map[string]any{
				"type":        "string",
				"description": "Optional context or additional information about the question",
			},
		},
		"required": []string{"question"},
	}
}

func (t *EmitStatisticsQuestionTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var params struct {
		Question string `json:"question"`
		Context  string `json:"context"`
	}
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	return fmt.Sprintf("StatisticsQuestion event emitted. Question: %s", params.Question), nil
}

// EmitQuestionAnsweredTool allows instructors to signal they've answered a question
type EmitQuestionAnsweredTool struct{}

func NewEmitQuestionAnsweredTool() *EmitQuestionAnsweredTool {
	return &EmitQuestionAnsweredTool{}
}

func (t *EmitQuestionAnsweredTool) Name() string {
	return "emit_question_answered"
}

func (t *EmitQuestionAnsweredTool) Description() string {
	return "Emit an event signaling that you've answered a question. This notifies the student-facing agent that the answer is ready and triggers it to review the whiteboard and respond to the student."
}

func (t *EmitQuestionAnsweredTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"subject": map[string]any{
				"type":        "string",
				"description": "The subject of the question answered",
				"enum":        []string{"math", "physics", "chemistry"},
			},
			"is_complete": map[string]any{
				"type":        "boolean",
				"description": "Whether the answer is complete or if additional specialists are needed",
			},
			"summary": map[string]any{
				"type":        "string",
				"description": "Brief summary of what was answered",
			},
		},
		"required": []string{"subject", "is_complete"},
	}
}

func (t *EmitQuestionAnsweredTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var params struct {
		Subject    string `json:"subject"`
		IsComplete bool   `json:"is_complete"`
		Summary    string `json:"summary"`
	}
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	validSubjects := map[string]bool{
		"math":      true,
		"physics":   true,
		"chemistry": true,
	}

	if !validSubjects[params.Subject] {
		return "", fmt.Errorf("invalid subject: %s", params.Subject)
	}

	status := "complete"
	if !params.IsComplete {
		status = "partial (additional specialists needed)"
	}

	return fmt.Sprintf("QuestionAnswered event emitted. Subject: %s, Status: %s", params.Subject, status), nil
}
