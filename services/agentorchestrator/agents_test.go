package agentorchestrator_test

import (
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/services/agentorchestrator/agents/math"
	"school-with-ai/services/agentorchestrator/agents/studentfacing"
	"testing"
)

func TestInstructorConfigurations(t *testing.T) {
	t.Parallel()
	// Mock notifier
	var mockNotifier core.EventNotifier = nil

	studentAgent := studentfacing.CreateStudentFacingAgent(mockNotifier)
	if studentAgent.Name != "student-facing-agent" {
		t.Errorf("Expected student agent name 'student-facing-agent', got '%s'", studentAgent.Name)
	}

	mathAgent := math.CreateMathInstructor(mockNotifier)
	if mathAgent.Name != "math-instructor" {
		t.Errorf("Expected math name 'math-instructor', got '%s'", mathAgent.Name)
	}
}
