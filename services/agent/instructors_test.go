package agent_test

import (
	"context"
	"fmt"
	"testing"

	"school-with-ai/services/agent"
)

// mockTool is a simple mock implementation of agent.Tool for testing
type mockTool struct {
	name        string
	description string
	parameters  map[string]any
}

func (m *mockTool) Name() string                                            { return m.name }
func (m *mockTool) Description() string                                     { return m.description }
func (m *mockTool) Parameters() map[string]any                              { return m.parameters }
func (m *mockTool) Invoke(ctx context.Context, args string) (string, error) { return "{}", nil }

func createMockTools() (handoffTool, whiteboardTool, calculatorTool, weatherTool, graphingTool, unitConversionTool, periodicTableTool, equationSolverTool, plotTool agent.Tool) {
	handoffTool = &mockTool{name: "call_agent", description: "Call another agent", parameters: map[string]any{}}
	whiteboardTool = &mockTool{name: "write_whiteboard", description: "Write to whiteboard", parameters: map[string]any{}}
	calculatorTool = &mockTool{name: "calculate", description: "Calculate", parameters: map[string]any{}}
	weatherTool = &mockTool{name: "get_weather", description: "Get weather", parameters: map[string]any{}}
	graphingTool = &mockTool{name: "plot_function", description: "Plot function", parameters: map[string]any{}}
	unitConversionTool = &mockTool{name: "convert_units", description: "Convert units", parameters: map[string]any{}}
	periodicTableTool = &mockTool{name: "lookup_element", description: "Lookup element", parameters: map[string]any{}}
	equationSolverTool = &mockTool{name: "solve_equation", description: "Solve equation", parameters: map[string]any{}}
	plotTool = &mockTool{name: "plot", description: "Generate plots", parameters: map[string]any{}}
	return
}

func TestInstructorConfigurations(t *testing.T) {
	handoffTool, whiteboardTool, calculatorTool, _, graphingTool, _, _, equationSolverTool, plotTool := createMockTools()

	studentAgent := agent.CreateStudentFacingAgent(handoffTool, whiteboardTool)
	if studentAgent.Name != "student-facing-agent" {
		t.Errorf("Expected student agent name 'student-facing-agent', got '%s'", studentAgent.Name)
	}

	math := agent.CreateMathInstructor(handoffTool, whiteboardTool, calculatorTool, graphingTool, equationSolverTool, plotTool)
	if math.Name != "math-instructor" {
		t.Errorf("Expected math name 'math-instructor', got '%s'", math.Name)
	}
}

func TestInstructorRegistry(t *testing.T) {
	handoffTool, whiteboardTool, calculatorTool, weatherTool, graphingTool, unitConversionTool, periodicTableTool, equationSolverTool, plotTool := createMockTools()
	registry := agent.NewInstructorRegistry(handoffTool, whiteboardTool, calculatorTool, weatherTool, graphingTool, unitConversionTool, periodicTableTool, equationSolverTool, plotTool)

	_, exists := registry.GetByName("math-instructor")
	if !exists {
		t.Error("Expected to find math-instructor")
	}
}

func ExampleCreateMathInstructor() {
	handoffTool, whiteboardTool, calculatorTool, _, graphingTool, _, _, equationSolverTool, plotTool := createMockTools()
	config := agent.CreateMathInstructor(handoffTool, whiteboardTool, calculatorTool, graphingTool, equationSolverTool, plotTool)
	fmt.Printf("Name: %s\n", config.Name)
}
