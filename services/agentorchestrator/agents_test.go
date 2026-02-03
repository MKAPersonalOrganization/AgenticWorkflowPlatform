package agentorchestrator_test

import (
	"context"
	"testing"

	"school-with-ai/services/agentorchestrator/agents/chemistry"
	"school-with-ai/services/agentorchestrator/agents/diagramming"
	"school-with-ai/services/agentorchestrator/agents/math"
	"school-with-ai/services/agentorchestrator/agents/physics"
	"school-with-ai/services/agentorchestrator/agents/statistics"
)

func TestInstructorConfigurations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Math
	mathAgent := math.CreateMathInstructor(ctx)
	if mathAgent.Name != "math-instructor" {
		t.Errorf("Expected math name 'math-instructor', got '%s'", mathAgent.Name)
	}

	// Physics
	physicsAgent := physics.CreatePhysicsInstructor(ctx)
	if physicsAgent.Name != "physics-instructor" {
		t.Errorf("Expected physics name 'physics-instructor', got '%s'", physicsAgent.Name)
	}

	// Chemistry
	chemistryAgent := chemistry.CreateChemistryInstructor(ctx)
	if chemistryAgent.Name != "chemistry-instructor" {
		t.Errorf("Expected chemistry name 'chemistry-instructor', got '%s'", chemistryAgent.Name)
	}

	// Statistics
	statisticsAgent := statistics.CreateStatisticsInstructor(ctx)
	if statisticsAgent.Name != "statistics-instructor" {
		t.Errorf("Expected statistics name 'statistics-instructor', got '%s'", statisticsAgent.Name)
	}

	// Diagramming
	diagrammingAgent := diagramming.CreateDiagrammingInstructor(ctx)
	if diagrammingAgent.Name != "diagramming-instructor" {
		t.Errorf("Expected diagramming name 'diagramming-instructor', got '%s'", diagrammingAgent.Name)
	}
}
