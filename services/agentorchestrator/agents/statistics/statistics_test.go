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

	if len(config.Tools) != 0 {
		t.Errorf("Expected 0 hardcoded tools, got %d", len(config.Tools))
	}
}
