package statistics

import (
	"context"
	"testing"
)

func TestNewStatisticsInstructor(t *testing.T) {
	ctx := context.Background()
	config, err := NewStatisticsInstructorConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to create statistics instructor config: %v", err)
	}

	if config.Name != "statistics-instructor" {
		t.Errorf("Expected agent name 'statistics-instructor', got '%s'", config.Name)
	}

	if len(config.Tools) == 0 {
		t.Error("Expected tools to be registered, got 0")
	}

	// Verify at least one granular tool is present
	foundMean := false
	for _, tool := range config.Tools {
		if tool.Name() == "mean" {
			foundMean = true
			break
		}
	}

	if !foundMean {
		t.Error("Expected 'mean' tool to be registered")
	}
}
