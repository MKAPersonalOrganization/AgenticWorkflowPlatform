package agent_test

import (
	"fmt"
	"testing"

	"school-with-ai/services/agent"
	"school-with-ai/services/llm"
)

func createMockTools() (handoffTool, whiteboardTool, calculatorTool llm.Tool) {
	handoffTool = llm.Tool{Type: "function", Name: "call_agent", Description: "Call another agent", Parameters: map[string]interface{}{}}
	whiteboardTool = llm.Tool{Type: "function", Name: "write_whiteboard", Description: "Write to whiteboard", Parameters: map[string]interface{}{}}
	calculatorTool = llm.Tool{Type: "function", Name: "calculate", Description: "Calculate", Parameters: map[string]interface{}{}}
	return
}

func TestInstructorConfigurations(t *testing.T) {
	handoffTool, whiteboardTool, calculatorTool := createMockTools()

	studentAgent := agent.CreateStudentFacingAgent(handoffTool, whiteboardTool)
	if studentAgent.Name != "student-facing-agent" {
		t.Errorf("Expected student agent name 'student-facing-agent', got '%s'", studentAgent.Name)
	}

	math := agent.CreateMathInstructor(handoffTool, whiteboardTool, calculatorTool)
	if math.Name != "math-instructor" {
		t.Errorf("Expected math name 'math-instructor', got '%s'", math.Name)
	}
}

func TestInstructorRegistry(t *testing.T) {
	handoffTool, whiteboardTool, calculatorTool := createMockTools()
	registry := agent.NewInstructorRegistry(handoffTool, whiteboardTool, calculatorTool)

	_, exists := registry.GetByName("math-instructor")
	if !exists {
		t.Error("Expected to find math-instructor")
	}
}

func ExampleCreateMathInstructor() {
	handoffTool, whiteboardTool, calculatorTool := createMockTools()
	config := agent.CreateMathInstructor(handoffTool, whiteboardTool, calculatorTool)
	fmt.Printf("Name: %s\n", config.Name)
}
