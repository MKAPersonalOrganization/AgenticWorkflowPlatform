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

func createMockTools() (whiteboardTool, readWhiteboardTool, calculatorTool, weatherTool, graphingTool, unitConversionTool, periodicTableTool, equationSolverTool, plotTool, animateTool, descriptiveStatsTool, correlationTool, distanceMetricsTool, samplingTool, dataTransformTool, normalDistributionTool, geometricDistributionTool, regressionTool, emitMathQuestionTool, emitPhysicsQuestionTool, emitChemistryQuestionTool, emitStatisticsQuestionTool, emitQuestionAnsweredTool agent.Tool) {
	whiteboardTool = &mockTool{name: "write_whiteboard", description: "Write to whiteboard", parameters: map[string]any{}}
	readWhiteboardTool = &mockTool{name: "read_whiteboard", description: "Read whiteboard", parameters: map[string]any{}}
	calculatorTool = &mockTool{name: "calculate", description: "Calculate", parameters: map[string]any{}}
	weatherTool = &mockTool{name: "get_weather", description: "Get weather", parameters: map[string]any{}}
	graphingTool = &mockTool{name: "plot_function", description: "Plot function", parameters: map[string]any{}}
	unitConversionTool = &mockTool{name: "convert_units", description: "Convert units", parameters: map[string]any{}}
	periodicTableTool = &mockTool{name: "lookup_element", description: "Lookup element", parameters: map[string]any{}}
	equationSolverTool = &mockTool{name: "solve_equation", description: "Solve equation", parameters: map[string]any{}}
	plotTool = &mockTool{name: "plot", description: "Generate plots", parameters: map[string]any{}}
	animateTool = &mockTool{name: "animate", description: "Generate animations", parameters: map[string]any{}}
	descriptiveStatsTool = &mockTool{name: "descriptive_statistics", description: "Calculate descriptive statistics", parameters: map[string]any{}}
	correlationTool = &mockTool{name: "correlation_analysis", description: "Calculate correlation", parameters: map[string]any{}}
	distanceMetricsTool = &mockTool{name: "distance_metrics", description: "Calculate distance metrics", parameters: map[string]any{}}
	samplingTool = &mockTool{name: "sampling_and_outliers", description: "Sampling and outliers", parameters: map[string]any{}}
	dataTransformTool = &mockTool{name: "data_transformation", description: "Transform data", parameters: map[string]any{}}
	normalDistributionTool = &mockTool{name: "normal_distribution", description: "Normal distribution", parameters: map[string]any{}}
	geometricDistributionTool = &mockTool{name: "geometric_distribution", description: "Geometric distribution", parameters: map[string]any{}}
	regressionTool = &mockTool{name: "regression_analysis", description: "Regression analysis", parameters: map[string]any{}}
	emitMathQuestionTool = &mockTool{name: "emit_math_question", description: "Emit math question event", parameters: map[string]any{}}
	emitPhysicsQuestionTool = &mockTool{name: "emit_physics_question", description: "Emit physics question event", parameters: map[string]any{}}
	emitChemistryQuestionTool = &mockTool{name: "emit_chemistry_question", description: "Emit chemistry question event", parameters: map[string]any{}}
	emitStatisticsQuestionTool = &mockTool{name: "emit_statistics_question", description: "Emit statistics question event", parameters: map[string]any{}}
	emitQuestionAnsweredTool = &mockTool{name: "emit_question_answered", description: "Emit question answered event", parameters: map[string]any{}}
	return
}

func TestInstructorConfigurations(t *testing.T) {
	whiteboardTool, readWhiteboardTool, calculatorTool, _, graphingTool, _, _, equationSolverTool, plotTool, animateTool, _, _, _, _, _, _, _, _, emitMathQuestionTool, emitPhysicsQuestionTool, emitChemistryQuestionTool, emitStatisticsQuestionTool, emitQuestionAnsweredTool := createMockTools()

	studentAgent := agent.CreateStudentFacingAgent(whiteboardTool, readWhiteboardTool, emitMathQuestionTool, emitPhysicsQuestionTool, emitChemistryQuestionTool, emitStatisticsQuestionTool)
	if studentAgent.Name != "student-facing-agent" {
		t.Errorf("Expected student agent name 'student-facing-agent', got '%s'", studentAgent.Name)
	}

	math := agent.CreateMathInstructor(whiteboardTool, readWhiteboardTool, calculatorTool, graphingTool, equationSolverTool, plotTool, animateTool, emitQuestionAnsweredTool, emitPhysicsQuestionTool, emitChemistryQuestionTool)
	if math.Name != "math-instructor" {
		t.Errorf("Expected math name 'math-instructor', got '%s'", math.Name)
	}
}

func TestInstructorRegistry(t *testing.T) {
	whiteboardTool, readWhiteboardTool, calculatorTool, weatherTool, graphingTool, unitConversionTool, periodicTableTool, equationSolverTool, plotTool, animateTool, descriptiveStatsTool, correlationTool, distanceMetricsTool, samplingTool, dataTransformTool, normalDistributionTool, geometricDistributionTool, regressionTool, emitMathQuestionTool, emitPhysicsQuestionTool, emitChemistryQuestionTool, emitStatisticsQuestionTool, emitQuestionAnsweredTool := createMockTools()
	registry := agent.NewInstructorRegistry(whiteboardTool, readWhiteboardTool, calculatorTool, weatherTool, graphingTool, unitConversionTool, periodicTableTool, equationSolverTool, plotTool, animateTool, descriptiveStatsTool, correlationTool, distanceMetricsTool, samplingTool, dataTransformTool, normalDistributionTool, geometricDistributionTool, regressionTool, emitMathQuestionTool, emitPhysicsQuestionTool, emitChemistryQuestionTool, emitStatisticsQuestionTool, emitQuestionAnsweredTool)

	_, exists := registry.GetByName("math-instructor")
	if !exists {
		t.Error("Expected to find math-instructor")
	}
}

func ExampleCreateMathInstructor() {
	whiteboardTool, readWhiteboardTool, calculatorTool, _, graphingTool, _, _, equationSolverTool, plotTool, animateTool, _, _, _, _, _, _, _, _, _, emitPhysicsQuestionTool, emitChemistryQuestionTool, _, emitQuestionAnsweredTool := createMockTools()
	config := agent.CreateMathInstructor(whiteboardTool, readWhiteboardTool, calculatorTool, graphingTool, equationSolverTool, plotTool, animateTool, emitQuestionAnsweredTool, emitPhysicsQuestionTool, emitChemistryQuestionTool)
	fmt.Printf("Name: %s\n", config.Name)
}
