package infra_test

import (
	"testing"

	"school-with-ai/services/agents/math"
	"school-with-ai/services/agents/studentfacing"
	"school-with-ai/services/sdk"
)

func TestInstructorConfigurations(t *testing.T) {
	// Mock notifier
	var mockNotifier sdk.EventNotifier = nil

	studentAgent := studentfacing.CreateStudentFacingAgent(mockNotifier)
	if studentAgent.Name != "student-facing-agent" {
		t.Errorf("Expected student agent name 'student-facing-agent', got '%s'", studentAgent.Name)
	}

	mathAgent := math.CreateMathInstructor(mockNotifier)
	if mathAgent.Name != "math-instructor" {
		t.Errorf("Expected math name 'math-instructor', got '%s'", mathAgent.Name)
	}
}
