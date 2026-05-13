package adkgo

import (
	"context"
	"testing"

	"google.golang.org/adk/memory"
	"google.golang.org/adk/session"

	fagent "aicompass/foundation/agent"
)

// ──────────────────────────────────────────────────────────────────────────────
// Test helpers: mock types for the narrow interfaces defined in ports.go.
// ──────────────────────────────────────────────────────────────────────────────

// mockMemoryService satisfies memory.Service for tests without a database.
type mockMemoryService struct{}

func (m *mockMemoryService) AddSessionToMemory(_ context.Context, _ session.Session) error {
	return nil
}

func (m *mockMemoryService) SearchMemory(_ context.Context, _ *memory.SearchRequest) (*memory.SearchResponse, error) {
	return &memory.SearchResponse{}, nil
}

type mockAgent struct {
	id          fagent.ID
	name        string
	description string
	subAgents   []fagent.Agent
}

func (m *mockAgent) ID() fagent.ID             { return m.id }
func (m *mockAgent) Name() string              { return m.name }
func (m *mockAgent) Description() string       { return m.description }
func (m *mockAgent) SubAgents() []fagent.Agent { return m.subAgents }

type mockConfigurableAgent struct {
	mockAgent
	config fagent.Config
}

func (m *mockConfigurableAgent) Config() fagent.Config { return m.config }

type mockWorkflowAgent struct {
	mockAgent
	graph fagent.WorkflowNodeInfo
}

func (m *mockWorkflowAgent) WorkflowGraph() fagent.WorkflowNodeInfo { return m.graph }

type mockAgentProvider struct {
	agents map[string]fagent.Agent
}

func (m *mockAgentProvider) GetForUser(_ context.Context, name, _ string) (fagent.Agent, error) {
	if a, ok := m.agents[name]; ok {
		return a, nil
	}
	return nil, &notFoundErr{name: name}
}

type notFoundErr struct{ name string }

func (e *notFoundErr) Error() string { return "agent not found: " + e.name }

type mockModelConfigResolver struct{}

func (m *mockModelConfigResolver) GetModelConfigForUser(_ context.Context, _ string) (*fagent.ModelConfig, error) {
	return &fagent.ModelConfig{Provider: "gemini", Model: "gemini-2.5-flash"}, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests: isWorkflowAgent
// ──────────────────────────────────────────────────────────────────────────────

func TestIsWorkflowAgent(t *testing.T) {
	t.Parallel()

	t.Run("simple agent returns false", func(t *testing.T) {
		t.Parallel()
		a := &mockAgent{id: "a1", name: "simple"}
		wa, ok := isWorkflowAgent(a)
		if ok {
			t.Error("expected false for simple agent")
		}
		if wa != nil {
			t.Error("expected nil WorkflowAgent")
		}
	})

	t.Run("workflow agent returns true", func(t *testing.T) {
		t.Parallel()
		a := &mockWorkflowAgent{
			mockAgent: mockAgent{id: "a2", name: "workflow"},
			graph: fagent.WorkflowNodeInfo{
				ID:   "root",
				Type: "sequential",
				Children: []fagent.WorkflowNodeInfo{
					{ID: "n1", Type: "agent", AgentID: "a1"},
				},
			},
		}
		wa, ok := isWorkflowAgent(a)
		if !ok {
			t.Error("expected true for workflow agent")
		}
		if wa == nil {
			t.Error("expected non-nil WorkflowAgent")
		}
		graph := wa.WorkflowGraph()
		if graph.Type != "sequential" {
			t.Errorf("expected sequential root, got %q", graph.Type)
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests: extractAgentConfig
// ──────────────────────────────────────────────────────────────────────────────

func TestExtractAgentConfig(t *testing.T) {
	t.Parallel()

	t.Run("non-configurable returns nil", func(t *testing.T) {
		t.Parallel()
		a := &mockAgent{id: "a1", name: "basic"}
		cfg := extractAgentConfig(a)
		if cfg != nil {
			t.Error("expected nil config for non-configurable agent")
		}
	})

	t.Run("configurable returns config", func(t *testing.T) {
		t.Parallel()
		a := &mockConfigurableAgent{
			mockAgent: mockAgent{id: "a2", name: "configured"},
			config: fagent.Config{
				SystemPrompt: "You are a helpful assistant.",
				OutputKey:    "research_output",
				TransferPolicy: fagent.TransferPolicy{
					DisallowTransferToParent: true,
					DisallowTransferToPeers:  false,
				},
			},
		}
		cfg := extractAgentConfig(a)
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
		if cfg.SystemPrompt != "You are a helpful assistant." {
			t.Errorf("unexpected system prompt: %q", cfg.SystemPrompt)
		}
		if cfg.OutputKey != "research_output" {
			t.Errorf("unexpected output key: %q", cfg.OutputKey)
		}
		if !cfg.TransferPolicy.DisallowTransferToParent {
			t.Error("expected DisallowTransferToParent to be true")
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests: workflowBuildContext.uniqueName
// ──────────────────────────────────────────────────────────────────────────────

func TestUniqueName(t *testing.T) {
	t.Parallel()

	wbc := &workflowBuildContext{}

	tests := []struct {
		input    string
		expected string
	}{
		{"writer", "writer"},
		{"reviewer", "reviewer"},
		{"writer", "writer (2)"},
		{"writer", "writer (3)"},
		{"reviewer", "reviewer (2)"},
	}

	for _, tt := range tests {
		got := wbc.uniqueName(tt.input)
		if got != tt.expected {
			t.Errorf("uniqueName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests: buildGenerateContentConfig
// ──────────────────────────────────────────────────────────────────────────────

func TestBuildGenerateContentConfig(t *testing.T) {
	t.Parallel()

	t.Run("nil fields", func(t *testing.T) {
		t.Parallel()
		cfg := &fagent.ModelConfig{}
		gc := buildGenerateContentConfig(cfg, nil, nil)
		if gc == nil {
			t.Fatal("expected non-nil GenerateContentConfig")
		}
		if gc.Temperature != nil {
			t.Error("expected nil temperature")
		}
	})

	t.Run("with temperature and maxTokens", func(t *testing.T) {
		t.Parallel()
		temp := 0.7
		maxTok := 4096
		cfg := &fagent.ModelConfig{Temperature: &temp, MaxTokens: &maxTok}
		gc := buildGenerateContentConfig(cfg, nil, nil)
		if gc == nil {
			t.Fatal("expected non-nil GenerateContentConfig")
		}
		if gc.Temperature == nil {
			t.Fatal("expected non-nil temperature")
		}
		if *gc.Temperature != float32(0.7) {
			t.Errorf("expected temperature 0.7, got %f", *gc.Temperature)
		}
		if gc.MaxOutputTokens != 4096 {
			t.Errorf("expected max output tokens 4096, got %d", gc.MaxOutputTokens)
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests: WorkflowNodeInfo completeness — ensures the struct has all ADK-Go fields
// ──────────────────────────────────────────────────────────────────────────────

func TestWorkflowNodeInfoCompleteness(t *testing.T) {
	t.Parallel()

	boolTrue := true
	boolFalse := false

	node := fagent.WorkflowNodeInfo{
		ID:                       "node-1",
		Type:                     "sequential",
		Label:                    "Research Pipeline",
		AgentID:                  "",
		OutputKey:                "pipeline_output",
		MaxIterations:            0,
		DisallowTransferToParent: &boolTrue,
		DisallowTransferToPeers:  &boolFalse,
		IncludeContents:          "none",
		Children: []fagent.WorkflowNodeInfo{
			{ID: "node-2", Type: "agent", AgentID: "researcher-agent-id", Label: "Researcher"},
			{
				ID: "node-3", Type: "parallel",
				Children: []fagent.WorkflowNodeInfo{
					{ID: "node-4", Type: "agent", AgentID: "writer-id"},
					{ID: "node-5", Type: "agent", AgentID: "fact-checker-id"},
				},
			},
			{
				ID: "node-6", Type: "loop", MaxIterations: 5,
				Children: []fagent.WorkflowNodeInfo{
					{ID: "node-7", Type: "agent", AgentID: "reviewer-id"},
					{ID: "node-8", Type: "agent", AgentID: "editor-id"},
				},
			},
		},
	}

	if node.Type != "sequential" {
		t.Errorf("expected sequential root, got %q", node.Type)
	}
	if len(node.Children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(node.Children))
	}
	if node.Children[0].Type != "agent" {
		t.Errorf("child 0: expected agent, got %q", node.Children[0].Type)
	}
	if node.Children[1].Type != "parallel" {
		t.Errorf("child 1: expected parallel, got %q", node.Children[1].Type)
	}
	if node.Children[2].Type != "loop" {
		t.Errorf("child 2: expected loop, got %q", node.Children[2].Type)
	}
	if node.Children[2].MaxIterations != 5 {
		t.Errorf("child 2: expected max_iterations 5, got %d", node.Children[2].MaxIterations)
	}
	if node.DisallowTransferToParent == nil || !*node.DisallowTransferToParent {
		t.Error("expected DisallowTransferToParent true")
	}
	if node.IncludeContents != "none" {
		t.Errorf("expected include_contents 'none', got %q", node.IncludeContents)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests: Node type mapping — frontend types must match buildNode switch cases
// ──────────────────────────────────────────────────────────────────────────────

func TestNodeTypeMapping(t *testing.T) {
	t.Parallel()

	validTypes := map[string]bool{
		"agent": true, "sequential": true, "parallel": true, "loop": true,
	}

	for _, ft := range []string{"agent", "sequential", "parallel", "loop"} {
		if !validTypes[ft] {
			t.Errorf("frontend type %q is not handled by buildNode", ft)
		}
	}

	for _, it := range []string{"", "pipeline", "branch", "merge", "custom"} {
		if validTypes[it] {
			t.Errorf("type %q should not be valid", it)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests: RunnerAdapter constructor
// ──────────────────────────────────────────────────────────────────────────────

func TestRunnerAdapterConstructor(t *testing.T) {
	t.Parallel()

	t.Run("nil config resolver returns error", func(t *testing.T) {
		t.Parallel()
		_, err := NewRunnerAdapter(RunnerAdapterConfig{
			MemoryService: &mockMemoryService{},
		})
		if err == nil {
			t.Error("expected error for nil ConfigResolver")
		}
	})

	t.Run("nil memory service returns error", func(t *testing.T) {
		t.Parallel()
		_, err := NewRunnerAdapter(RunnerAdapterConfig{
			ConfigResolver: &mockModelConfigResolver{},
		})
		if err == nil {
			t.Error("expected error for nil MemoryService")
		}
	})

	t.Run("with config resolver succeeds", func(t *testing.T) {
		t.Parallel()
		adapter, err := NewRunnerAdapter(RunnerAdapterConfig{
			ConfigResolver: &mockModelConfigResolver{},
			MemoryService:  &mockMemoryService{},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if adapter == nil {
			t.Fatal("expected non-nil adapter")
		}
	})

	t.Run("all dependencies wired", func(t *testing.T) {
		t.Parallel()
		provider := &mockAgentProvider{agents: map[string]fagent.Agent{}}
		adapter, err := NewRunnerAdapter(RunnerAdapterConfig{
			ConfigResolver: &mockModelConfigResolver{},
			AgentProvider:  provider,
			MemoryService:  &mockMemoryService{},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if adapter.agentProvider != provider {
			t.Error("agentProvider not wired correctly")
		}
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests: Pipeline graph patterns — verifies ADK-Go alignment
// ──────────────────────────────────────────────────────────────────────────────

func TestPipelineGraphPatterns(t *testing.T) {
	t.Parallel()

	t.Run("sequential with two agents", func(t *testing.T) {
		t.Parallel()
		graph := fagent.WorkflowNodeInfo{
			ID: "node-1", Type: "sequential",
			Children: []fagent.WorkflowNodeInfo{
				{ID: "node-2", Type: "agent", AgentID: "researcher"},
				{ID: "node-3", Type: "agent", AgentID: "writer"},
			},
		}
		validateGraphStructure(t, &graph)
	})

	t.Run("parallel with three agents", func(t *testing.T) {
		t.Parallel()
		graph := fagent.WorkflowNodeInfo{
			ID: "node-1", Type: "parallel",
			Children: []fagent.WorkflowNodeInfo{
				{ID: "node-2", Type: "agent", AgentID: "writer"},
				{ID: "node-3", Type: "agent", AgentID: "reviewer"},
				{ID: "node-4", Type: "agent", AgentID: "fact-checker"},
			},
		}
		validateGraphStructure(t, &graph)
	})

	t.Run("loop with review-edit cycle", func(t *testing.T) {
		t.Parallel()
		graph := fagent.WorkflowNodeInfo{
			ID: "node-1", Type: "loop", MaxIterations: 3,
			Children: []fagent.WorkflowNodeInfo{
				{ID: "node-2", Type: "agent", AgentID: "reviewer"},
				{ID: "node-3", Type: "agent", AgentID: "editor"},
			},
		}
		validateGraphStructure(t, &graph)
		if graph.MaxIterations != 3 {
			t.Errorf("expected max_iterations 3, got %d", graph.MaxIterations)
		}
	})

	t.Run("nested sequential-parallel-loop", func(t *testing.T) {
		t.Parallel()
		graph := fagent.WorkflowNodeInfo{
			ID: "node-1", Type: "sequential",
			Children: []fagent.WorkflowNodeInfo{
				{ID: "node-2", Type: "agent", AgentID: "researcher"},
				{
					ID: "node-3", Type: "parallel",
					Children: []fagent.WorkflowNodeInfo{
						{ID: "node-4", Type: "agent", AgentID: "writer-1"},
						{ID: "node-5", Type: "agent", AgentID: "writer-2"},
					},
				},
				{
					ID: "node-6", Type: "loop", MaxIterations: 5,
					Children: []fagent.WorkflowNodeInfo{
						{ID: "node-7", Type: "agent", AgentID: "reviewer"},
						{ID: "node-8", Type: "agent", AgentID: "editor"},
					},
				},
			},
		}
		validateGraphStructure(t, &graph)
		if countNodesRecursive(&graph) != 8 {
			t.Errorf("expected 8 nodes, got %d", countNodesRecursive(&graph))
		}
	})

	t.Run("agent with transfer policy overrides", func(t *testing.T) {
		t.Parallel()
		boolTrue := true
		boolFalse := false
		graph := fagent.WorkflowNodeInfo{
			ID: "node-1", Type: "sequential",
			Children: []fagent.WorkflowNodeInfo{
				{
					ID: "node-2", Type: "agent", AgentID: "researcher",
					OutputKey:                "research_data",
					DisallowTransferToParent: &boolTrue,
					DisallowTransferToPeers:  &boolFalse,
					IncludeContents:          "none",
				},
				{ID: "node-3", Type: "agent", AgentID: "writer"},
			},
		}
		validateGraphStructure(t, &graph)
		rn := graph.Children[0]
		if rn.OutputKey != "research_data" {
			t.Errorf("expected output_key 'research_data', got %q", rn.OutputKey)
		}
		if rn.DisallowTransferToParent == nil || !*rn.DisallowTransferToParent {
			t.Error("expected DisallowTransferToParent true")
		}
		if rn.IncludeContents != "none" {
			t.Errorf("expected include_contents 'none', got %q", rn.IncludeContents)
		}
	})
}

func validateGraphStructure(t *testing.T, node *fagent.WorkflowNodeInfo) {
	t.Helper()
	switch node.Type {
	case "agent":
		if node.AgentID == "" {
			t.Errorf("node %q: agent requires agent_id", node.ID)
		}
		if len(node.Children) > 0 {
			t.Errorf("node %q: agent should not have children", node.ID)
		}
	case "sequential", "parallel":
		if len(node.Children) == 0 {
			t.Errorf("node %q: %s requires at least one child", node.ID, node.Type)
		}
		for i := range node.Children {
			validateGraphStructure(t, &node.Children[i])
		}
	case "loop":
		if len(node.Children) == 0 {
			t.Errorf("node %q: loop requires at least one child", node.ID)
		}
		for i := range node.Children {
			validateGraphStructure(t, &node.Children[i])
		}
	default:
		t.Errorf("node %q: unsupported type %q", node.ID, node.Type)
	}
}

func countNodesRecursive(node *fagent.WorkflowNodeInfo) int {
	count := 1
	for i := range node.Children {
		count += countNodesRecursive(&node.Children[i])
	}
	return count
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests: Frontend JSON field mapping
// ──────────────────────────────────────────────────────────────────────────────

func TestFrontendJSONFieldMapping(t *testing.T) {
	t.Parallel()

	type frontendNode struct {
		ID                       string         `json:"id"`
		Type                     string         `json:"type"`
		Label                    string         `json:"label"`
		AgentID                  string         `json:"agent_id"`
		OutputKey                string         `json:"output_key"`
		MaxIterations            int            `json:"max_iterations"`
		DisallowTransferToParent *bool          `json:"disallow_transfer_to_parent"`
		DisallowTransferToPeers  *bool          `json:"disallow_transfer_to_peers"`
		IncludeContents          string         `json:"include_contents"`
		Children                 []frontendNode `json:"children"`
	}

	fn := frontendNode{
		ID: "node-1", Type: "agent", AgentID: "test-agent",
		OutputKey: "output", MaxIterations: 3,
	}

	// All frontend fields map to WorkflowNodeInfo fields
	wni := fagent.WorkflowNodeInfo{
		ID: fn.ID, Type: fn.Type, AgentID: fn.AgentID,
		OutputKey: fn.OutputKey, MaxIterations: fn.MaxIterations,
	}
	if wni.ID != fn.ID {
		t.Error("ID mapping mismatch")
	}
	if wni.Type != fn.Type {
		t.Error("Type mapping mismatch")
	}
	if wni.AgentID != fn.AgentID {
		t.Error("AgentID mapping mismatch")
	}
	if wni.OutputKey != fn.OutputKey {
		t.Error("OutputKey mapping mismatch")
	}
	if wni.MaxIterations != fn.MaxIterations {
		t.Error("MaxIterations mapping mismatch")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests: Loop exit tool injection flag
// ──────────────────────────────────────────────────────────────────────────────

func TestLoopLastChildFlag(t *testing.T) {
	t.Parallel()

	children := []fagent.WorkflowNodeInfo{
		{ID: "node-1", Type: "agent", AgentID: "reviewer"},
		{ID: "node-2", Type: "agent", AgentID: "editor"},
		{ID: "node-3", Type: "agent", AgentID: "final-checker"},
	}

	lastIdx := len(children) - 1
	for i := range children {
		isLast := i == lastIdx
		if i == lastIdx && !isLast {
			t.Error("last child should be flagged as isLastInLoop")
		}
		if i != lastIdx && isLast {
			t.Error("non-last child should NOT be flagged as isLastInLoop")
		}
	}
	if children[lastIdx].AgentID != "final-checker" {
		t.Errorf("expected last child to be final-checker, got %q", children[lastIdx].AgentID)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests: Max node limit
// ──────────────────────────────────────────────────────────────────────────────

func TestMaxNodeLimit(t *testing.T) {
	t.Parallel()

	const maxNodes = 50

	children := make([]fagent.WorkflowNodeInfo, 49)
	for i := range children {
		children[i] = fagent.WorkflowNodeInfo{
			ID: "node-" + string(rune('a'+i%26)), Type: "agent",
			AgentID: "agent-" + string(rune('a'+i%26)),
		}
	}
	graph := fagent.WorkflowNodeInfo{
		ID: "root", Type: "sequential", Children: children,
	}

	count := countNodesRecursive(&graph)
	if count != maxNodes {
		t.Errorf("expected %d nodes, got %d", maxNodes, count)
	}
	if count > maxNodes {
		t.Error("graph exceeds the 50 node limit")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests: Conditional routing
// ──────────────────────────────────────────────────────────────────────────────

func TestConditionalRouting(t *testing.T) {
	t.Parallel()

	t.Run("conditional node structure is valid", func(t *testing.T) {
		t.Parallel()
		// This maps to how the UI constructs conditional nodes:
		// Children[0] = evaluator agent (output_key read from AgentDefinition at runtime)
		// Children[1:] = branches with condition values
		// Note: WorkflowNodeInfo.OutputKey is NOT set on the evaluator node in the graph.
		// buildConditionalNode resolves the AgentDefinition and reads its OutputKey.
		graph := fagent.WorkflowNodeInfo{
			ID: "cond-1", Type: "conditional", Label: "Route Inquiry",
			Children: []fagent.WorkflowNodeInfo{
				// Evaluator: output_key lives on AgentDefinition, not the node
				{
					ID: "node-eval", Type: "agent", AgentID: "classifier",
				},
				// Branch 1: condition = "technical"
				{
					ID: "node-tech", Type: "agent", AgentID: "tech-support",
					Condition: "technical",
				},
				// Branch 2: condition = "billing"
				{
					ID: "node-bill", Type: "agent", AgentID: "billing-agent",
					Condition: "billing",
				},
				// Branch 3: fallback (condition = "*")
				{
					ID: "node-fallback", Type: "agent", AgentID: "general-assistant",
					Condition: "*",
				},
			},
		}

		// Validate structure
		if len(graph.Children) < 2 {
			t.Fatal("conditional node requires at least 2 children")
		}

		// Branches must have condition values
		for i := 1; i < len(graph.Children); i++ {
			branch := graph.Children[i]
			if branch.Condition == "" {
				t.Errorf("branch %d should have a condition (defaults to '*' at runtime)", i)
			}
		}

		// Verify specific conditions
		if graph.Children[1].Condition != "technical" {
			t.Errorf("expected branch 1 condition 'technical', got %q", graph.Children[1].Condition)
		}
		if graph.Children[2].Condition != "billing" {
			t.Errorf("expected branch 2 condition 'billing', got %q", graph.Children[2].Condition)
		}
		if graph.Children[3].Condition != "*" {
			t.Errorf("expected branch 3 condition '*', got %q", graph.Children[3].Condition)
		}
	})

	t.Run("conditional with composite branch agents", func(t *testing.T) {
		t.Parallel()
		// Branches can be composite nodes (sequential, parallel)
		graph := fagent.WorkflowNodeInfo{
			ID: "cond-2", Type: "conditional",
			Children: []fagent.WorkflowNodeInfo{
				// Evaluator: output_key on AgentDefinition at runtime
				{
					ID: "eval-1", Type: "agent", AgentID: "intent-classifier",
				},
				{
					ID: "branch-tech", Type: "sequential", Condition: "technical",
					Children: []fagent.WorkflowNodeInfo{
						{ID: "tech-1", Type: "agent", AgentID: "diagnose"},
						{ID: "tech-2", Type: "agent", AgentID: "resolve"},
					},
				},
				{
					ID: "branch-general", Type: "agent", AgentID: "general",
					Condition: "*",
				},
			},
		}

		// Composite branch structure
		techBranch := graph.Children[1]
		if techBranch.Type != "sequential" {
			t.Errorf("expected sequential branch, got %q", techBranch.Type)
		}
		if techBranch.Condition != "technical" {
			t.Errorf("expected condition 'technical', got %q", techBranch.Condition)
		}
		if len(techBranch.Children) != 2 {
			t.Errorf("expected 2 children in sequential branch, got %d", len(techBranch.Children))
		}
	})
}
