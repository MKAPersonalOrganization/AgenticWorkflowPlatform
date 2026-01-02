package infra_test

import (
	"testing"

	"school-with-ai/infra"
	"school-with-ai/services/agents/math"
	"school-with-ai/services/agents/physics"
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

func TestAgentRegistry(t *testing.T) {
	// Mock notifier
	var mockNotifier sdk.EventNotifier = nil

	registry := infra.NewAgentRegistry(mockNotifier)

	// Register agents
	registry.Register("math-instructor", math.CreateMathInstructor)
	registry.Register("physics-instructor", physics.CreatePhysicsInstructor)

	_, exists := registry.GetByName("math-instructor")
	if !exists {
		t.Error("Expected to find math-instructor")
	}

	_, exists = registry.GetByName("physics-instructor")
	if !exists {
		t.Error("Expected to find physics-instructor")
	}

	_, exists = registry.GetByName("nonexistent")
	if exists {
		t.Error("Expected not to find nonexistent agent")
	}
}
