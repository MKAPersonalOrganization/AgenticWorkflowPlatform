package agentorchestrator_test

import (
	"context"
	"testing"

	"school-with-ai/services/agentorchestrator/agents/math"
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

	// Statistics
	statisticsConfig, err := statistics.NewStatisticsInstructorConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to create statistics instructor config: %v", err)
	}
	if statisticsConfig.Name != "statistics-instructor" {
		t.Errorf("Expected statistics name 'statistics-instructor', got '%s'", statisticsConfig.Name)
	}
}
