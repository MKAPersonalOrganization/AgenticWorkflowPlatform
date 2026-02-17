package toolregistry

import (
	"context"
	"fmt"

	"school-with-ai/foundation/agent"
	mathtools "school-with-ai/services/agentorchestrator/agents/math/tools"
	statisticstools "school-with-ai/services/agentorchestrator/agents/statistics/tools"
	"school-with-ai/services/agentorchestrator/agents/tools"
	kbtools "school-with-ai/services/agentorchestrator/agents/tools/knowledgebase"
	"school-with-ai/services/artifact"
	"school-with-ai/services/memory"
	memorytools "school-with-ai/services/memory/tools"
)

// ExecutableToolFactory creates executable tools from tool registry entries and tool sets.
type ExecutableToolFactory struct {
	artifactStore    artifact.Service
	toolRegistry     *Registry
	edgequakeBaseURL string
	openAIAPIKey     string
	modelID          string
}

// NewExecutableToolFactory creates a new tool factory.
func NewExecutableToolFactory(artifactStore artifact.Service) *ExecutableToolFactory {
	return &ExecutableToolFactory{
		artifactStore: artifactStore,
	}
}

// SetEdgequakeConfig sets the edgequake base URL and LLM config for knowledge base tools.
// The openAIAPIKey and modelID are used by the KB sub-agent to synthesize answers.
func (f *ExecutableToolFactory) SetEdgequakeConfig(edgequakeBaseURL, openAIAPIKey, modelID string) {
	f.edgequakeBaseURL = edgequakeBaseURL
	f.openAIAPIKey = openAIAPIKey
	f.modelID = modelID
}

// SetToolRegistry sets the tool registry for the factory (used for tool set expansion).
func (f *ExecutableToolFactory) SetToolRegistry(registry *Registry) {
	f.toolRegistry = registry
}

// CreateExecutableToolsFromSet creates all executable tools from a tool set.
func (f *ExecutableToolFactory) CreateExecutableToolsFromSet(ctx context.Context, toolSetID string) ([]agent.ExecutableTool, error) {
	if f.toolRegistry == nil {
		return nil, fmt.Errorf("tool registry not set")
	}

	// Get tools in the set
	tools, err := f.toolRegistry.GetToolsInSet(ctx, toolSetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tools in set %s: %w", toolSetID, err)
	}

	var executableTools []agent.ExecutableTool
	for _, tool := range tools {
		executableTool, err := f.CreateExecutableTool(ctx, &tool)
		if err != nil {
			// Skip tools that can't be created rather than failing the entire set
			continue
		}
		executableTools = append(executableTools, executableTool)
	}

	return executableTools, nil
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
	case "math-solve":
		return mathtools.NewEquationSolverTool(), nil

	// Math visualization tools
	case "viz-plot":
		return mathtools.NewPlotTool(f.artifactStore), nil
	case "viz-animate":
		return mathtools.NewAnimateTool(f.artifactStore), nil

	// Statistics tools
	case "stats-mean":
		return &statisticstools.MeanTool{}, nil
	case "stats-median":
		return &statisticstools.MedianTool{}, nil
	case "stats-mode":
		return &statisticstools.ModeTool{}, nil
	case "stats-variance":
		return &statisticstools.VarianceTool{}, nil
	case "stats-stddev":
		return &statisticstools.StandardDeviationTool{}, nil
	case "stats-correlation":
		return &statisticstools.PearsonCorrelationTool{}, nil
	case "stats-regression":
		return &statisticstools.LinearRegressionTool{}, nil

	// Document tools
	case "doc-create":
		return &tools.CreateMarkdownTool{ArtifactStore: f.artifactStore}, nil
	case "doc-update":
		return &tools.UpdateMarkdownTool{ArtifactStore: f.artifactStore}, nil
	case "doc-delete":
		return &tools.DeleteMarkdownTool{ArtifactStore: f.artifactStore}, nil

	// Memory tools
	case "mem-remember":
		return memorytools.NewMemoryTool(memory.NewArtifactMemoryService(f.artifactStore)), nil
	case "mem-forget":
		return memorytools.NewForgetMemoryTool(memory.NewArtifactMemoryService(f.artifactStore)), nil
	case "mem-list":
		return memorytools.NewListMemoriesTool(memory.NewArtifactMemoryService(f.artifactStore)), nil
	case "mem-clear":
		return memorytools.NewClearMemoriesTool(memory.NewArtifactMemoryService(f.artifactStore)), nil

	// Knowledge base tools
	case "kb-query":
		if f.edgequakeBaseURL == "" {
			return nil, fmt.Errorf("edgequake base URL not configured for knowledge base tool")
		}
		if f.openAIAPIKey == "" {
			return nil, fmt.Errorf("OpenAI API key not configured for knowledge base tool")
		}
		return kbtools.NewQueryKnowledgeBaseTool(f.edgequakeBaseURL, f.openAIAPIKey, f.modelID), nil

	default:
		return nil, fmt.Errorf("unknown tool ID: %s", tool.ID)
	}
}
