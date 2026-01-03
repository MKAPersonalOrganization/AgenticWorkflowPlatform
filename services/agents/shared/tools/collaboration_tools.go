package tools

import (
	"encoding/json"
	"fmt"

	"school-with-ai/domain"
	"school-with-ai/services/agents"
)

// WhiteboardStateGetter is a function that retrieves the whiteboard state for a thread
type WhiteboardStateGetter func(threadID string) string

// IsCollaborationTool checks if a tool is a collaboration/routing tool
func IsCollaborationTool(toolName string) bool {
	return toolName == "emit_math_question" ||
		toolName == "emit_physics_question" ||
		toolName == "emit_chemistry_question" ||
		toolName == "emit_statistics_question" ||
		toolName == "emit_linearalgebra_question" ||
		toolName == "emit_question_answered"
}

// HandleCollaborationToolCall processes a collaboration tool call and returns the domain event
func HandleCollaborationToolCall(threadID string, agentName string, toolCall domain.ToolCall, getWhiteboardState WhiteboardStateGetter) (*agents.DomainEvent, error) {
	toolName := toolCall.Function.Name

	switch toolName {
	case "emit_math_question":
		var input struct {
			Question string `json:"question"`
			Context  string `json:"context"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return nil, fmt.Errorf("failed to parse emit_math_question arguments: %w", err)
		}
		return agents.NewMathQuestionEvent(threadID, input.Question, agentName, input.Context, getWhiteboardState(threadID)), nil

	case "emit_physics_question":
		var input struct {
			Question string `json:"question"`
			Context  string `json:"context"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return nil, fmt.Errorf("failed to parse emit_physics_question arguments: %w", err)
		}
		return agents.NewPhysicsQuestionEvent(threadID, input.Question, agentName, input.Context, getWhiteboardState(threadID)), nil

	case "emit_chemistry_question":
		var input struct {
			Question string `json:"question"`
			Context  string `json:"context"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return nil, fmt.Errorf("failed to parse emit_chemistry_question arguments: %w", err)
		}
		return agents.NewChemistryQuestionEvent(threadID, input.Question, agentName, input.Context, getWhiteboardState(threadID)), nil

	case "emit_statistics_question":
		var input struct {
			Question string `json:"question"`
			Context  string `json:"context"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return nil, fmt.Errorf("failed to parse emit_statistics_question arguments: %w", err)
		}
		return agents.NewStatisticsQuestionEvent(threadID, input.Question, agentName, input.Context, getWhiteboardState(threadID)), nil

	case "emit_linearalgebra_question":
		var input struct {
			Question string `json:"question"`
			Context  string `json:"context"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return nil, fmt.Errorf("failed to parse emit_linearalgebra_question arguments: %w", err)
		}
		return agents.NewLinearAlgebraQuestionEvent(threadID, input.Question, agentName, input.Context, getWhiteboardState(threadID)), nil

	case "emit_question_answered":
		var input struct {
			Subject    string `json:"subject"`
			IsComplete bool   `json:"is_complete"`
			Summary    string `json:"summary"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return nil, fmt.Errorf("failed to parse emit_question_answered arguments: %w", err)
		}
		return agents.NewQuestionAnsweredEvent(threadID, input.Subject, input.Summary, agentName, input.IsComplete, getWhiteboardState(threadID)), nil

	case "write_whiteboard":
		var input struct {
			Content string `json:"content"`
			Action  string `json:"action"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return nil, fmt.Errorf("failed to parse write_whiteboard arguments: %w", err)
		}
		return agents.NewWhiteboardUpdatedEvent(threadID, agentName, input.Content, input.Action), nil

	default:
		return nil, fmt.Errorf("unknown collaboration tool: %s", toolName)
	}
}

// GetHandoffEventForTool creates an agent handoff event for collaboration tools
// Returns nil if the tool doesn't represent an agent handoff
func GetHandoffEventForTool(threadID string, fromAgent string, toolName string) *agents.DomainEvent {
	var toAgent, reason string

	switch toolName {
	case "emit_math_question":
		toAgent = "math-instructor"
		reason = "Math question requires specialist"
	case "emit_physics_question":
		toAgent = "physics-instructor"
		reason = "Physics question requires specialist"
	case "emit_chemistry_question":
		toAgent = "chemistry-instructor"
		reason = "Chemistry question requires specialist"
	case "emit_statistics_question":
		toAgent = "statistics-instructor"
		reason = "Statistics question requires specialist"
	case "emit_linearalgebra_question":
		toAgent = "linearalgebra-instructor"
		reason = "Linear algebra question requires specialist"
	case "emit_question_answered":
		toAgent = "student-facing-agent"
		reason = "Question answered, returning to coordinator"
	default:
		return nil
	}

	return agents.NewAgentHandoffEvent(threadID, fromAgent, toAgent, reason)
}
