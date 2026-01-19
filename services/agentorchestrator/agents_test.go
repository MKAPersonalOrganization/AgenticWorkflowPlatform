package agentorchestrator_test

import (
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/services/agentorchestrator/agents/math"

	"testing"
)

func TestInstructorConfigurations(t *testing.T) {
	t.Parallel()
	// Mock notifier
	var mockNotifier core.EventNotifier = nil



	mathAgent := math.CreateMathInstructor(mockNotifier)
	if mathAgent.Name != "math-instructor" {
		t.Errorf("Expected math name 'math-instructor', got '%s'", mathAgent.Name)
	}
}
