package toolregistry

import (
	"context"
	"fmt"

	"school-with-ai/foundation/agent"
	diagrammingtools "school-with-ai/services/agentorchestrator/agents/diagramming/tools"
	mathtools "school-with-ai/services/agentorchestrator/agents/math/tools"
	statisticstools "school-with-ai/services/agentorchestrator/agents/statistics/tools"
	"school-with-ai/services/agentorchestrator/agents/tools"
	"school-with-ai/services/artifact"
)

// ExecutableToolFactory creates executable tools from tool registry entries.
type ExecutableToolFactory struct {
	artifactStore artifact.Service
}

// NewExecutableToolFactory creates a new tool factory.
func NewExecutableToolFactory(artifactStore artifact.Service) *ExecutableToolFactory {
	return &ExecutableToolFactory{
		artifactStore: artifactStore,
	}
}

// CreateExecutableTool creates an executable tool from a tool registry entry.
func (f *ExecutableToolFactory) CreateExecutableTool(ctx context.Context, tool *Tool) (agent.ExecutableTool, error) {
	switch tool.ID {
	// Calculator tools
	case "calc-add":
		return &tools.AddTool{}, nil
	case "calc-subtract":
		return &tools.SubtractTool{}, nil
	case "calc-multiply":
		return &tools.MultiplyTool{}, nil
	case "calc-divide":
		return &tools.DivideTool{}, nil
	case "calc-sqrt":
		return &tools.SqrtTool{}, nil
	case "calc-power":
		return &tools.PowerTool{}, nil

	// Math visualization tools
	case "viz-plot":
		return mathtools.NewPlotTool(), nil
	case "viz-animate":
		return mathtools.NewAnimateTool(), nil

	// Math solving tools - temporarily removed until interface is fixed
	// case "solve-equation":
	//	return mathtools.NewEquationSolverTool(), nil

	// Statistics tools
	case "stats-descriptive":
		return &statisticstools.MeanTool{}, nil
	case "stats-regression":
		return &statisticstools.LinearRegressionTool{}, nil
	case "stats-hypothesis":
		return &statisticstools.PearsonCorrelationTool{}, nil

	// Diagramming tools
	case "diagram-mermaid":
		return diagrammingtools.NewMermaidDiagramTool(), nil

	// Document tools
	case "docs-create":
		return &tools.CreateMarkdownTool{ArtifactStore: f.artifactStore}, nil

	default:
		return nil, fmt.Errorf("unknown tool ID: %s", tool.ID)
	}
}
