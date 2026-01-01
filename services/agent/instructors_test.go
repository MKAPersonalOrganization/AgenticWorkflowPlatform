package agent_test

import (
	"fmt"
	"testing"

	"school-with-ai/services/agent"
	"school-with-ai/services/llm"
)

func createMockTools() (selectTool, handoffTool, whiteboardTool, calculatorTool llm.Tool) {
	selectTool = llm.Tool{Type: "function", Name: "select_instructor", Description: "Select instructor", Parameters: map[string]interface{}{}}
	handoffTool = llm.Tool{Type: "function", Name: "handoff_to_instructor", Description: "Handoff to instructor", Parameters: map[string]interface{}{}}
	whiteboardTool = llm.Tool{Type: "function", Name: "write_whiteboard", Description: "Write to whiteboard", Parameters: map[string]interface{}{}}
	calculatorTool = llm.Tool{Type: "function", Name: "calculate", Description: "Calculate", Parameters: map[string]interface{}{}}
	return
}

func TestInstructorConfigurations(t *testing.T) {
	selectTool, handoffTool, whiteboardTool, calculatorTool := createMockTools()

	router := agent.CreateRouterAgent(selectTool)
	if router.Name != "instructor-router" {
		t.Errorf("Expected router name 'instructor-router', got '%s'", router.Name)
	}

	math := agent.CreateMathInstructor(handoffTool, whiteboardTool, calculatorTool)
	if math.Name != "math-instructor" {
		t.Errorf("Expected math name 'math-instructor', got '%s'", math.Name)
	}
}

func TestInstructorRegistry(t *testing.T) {
	selectTool, handoffTool, whiteboardTool, calculatorTool := createMockTools()
	registry := agent.NewInstructorRegistry(selectTool, handoffTool, whiteboardTool, calculatorTool)

	_, exists := registry.GetByName("math-instructor")
	if !exists {
		t.Error("Expected to find math-instructor")
	}
}

func ExampleCreateMathInstructor() {
	_, handoffTool, whiteboardTool, calculatorTool := createMockTools()
	config := agent.CreateMathInstructor(handoffTool, whiteboardTool, calculatorTool)
	fmt.Printf("Name: %s\n", config.Name)
}
