package agent_test

import (
	"fmt"
	"testing"

	"school-with-ai/infra"
	"school-with-ai/services/agent"
)

func TestInstructorConfigurations(t *testing.T) {
	// Mock notifier
	var mockNotifier agent.EventNotifier = nil

	studentAgent := agent.CreateStudentFacingAgent(mockNotifier)
	if studentAgent.Name != "student-facing-agent" {
		t.Errorf("Expected student agent name 'student-facing-agent', got '%s'", studentAgent.Name)
	}

	math := agent.CreateMathInstructor(mockNotifier)
	if math.Name != "math-instructor" {
		t.Errorf("Expected math name 'math-instructor', got '%s'", math.Name)
	}
}

func TestAgentRegistry(t *testing.T) {
	// Mock notifier
	var mockNotifier agent.EventNotifier = nil

	registry := infra.NewAgentRegistry(mockNotifier)

	// Register agents
	registry.Register("math-instructor", agent.CreateMathInstructor)
	registry.Register("physics-instructor", agent.CreatePhysicsInstructor)

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

func ExampleCreateMathInstructor() {
	// Mock notifier
	var mockNotifier agent.EventNotifier = nil

	config := agent.CreateMathInstructor(mockNotifier)
	fmt.Printf("Name: %s\n", config.Name)
}
