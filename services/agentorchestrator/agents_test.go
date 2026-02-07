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
	mathConfig, err := math.NewMathInstructorConfig(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to create math instructor config: %v", err)
	}
	if mathConfig.Name != "math-instructor" {
		t.Errorf("Expected math name 'math-instructor', got '%s'", mathConfig.Name)
	}

	// Physics
	physicsConfig, err := physics.NewPhysicsInstructorConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to create physics instructor config: %v", err)
	}
	if physicsConfig.Name != "physics-instructor" {
		t.Errorf("Expected physics name 'physics-instructor', got '%s'", physicsConfig.Name)
	}

	// Chemistry
	chemistryConfig, err := chemistry.NewChemistryInstructorConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to create chemistry instructor config: %v", err)
	}
	if chemistryConfig.Name != "chemistry-instructor" {
		t.Errorf("Expected chemistry name 'chemistry-instructor', got '%s'", chemistryConfig.Name)
	}

	// Statistics
	statisticsConfig, err := statistics.NewStatisticsInstructorConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to create statistics instructor config: %v", err)
	}
	if statisticsConfig.Name != "statistics-instructor" {
		t.Errorf("Expected statistics name 'statistics-instructor', got '%s'", statisticsConfig.Name)
	}

	// Diagramming
	diagrammingConfig, err := diagramming.NewDiagrammingInstructorConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to create diagramming instructor config: %v", err)
	}
	if diagrammingConfig.Name != "diagramming-instructor" {
		t.Errorf("Expected diagramming name 'diagramming-instructor', got '%s'", diagrammingConfig.Name)
	}
}
