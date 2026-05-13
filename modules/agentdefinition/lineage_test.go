package agentdefinition

import (
	"slices"
	"testing"
)

// lineage_test.go covers the static state-flow analyzer. Each test name
// reads as a spec sentence so failures point at the contract being
// broken. The fixtures favour minimal trees so the assertions are about
// a single composition rule at a time.

// hasIssue returns true when the analyzer reported at least one issue of
// `kind` referring to the given node and (optionally) key.
func hasIssue(issues []LineageIssue, kind IssueKind, nodeID, key string) bool {
	for _, i := range issues {
		if i.Kind != kind {
			continue
		}
		if nodeID != "" && i.NodeID != nodeID {
			continue
		}
		if key != "" && i.Key != key {
			continue
		}
		return true
	}
	return false
}

func TestAnalyzeLineageSequentialDownstreamSeesUpstreamOutputKey(t *testing.T) {
	t.Parallel()

	agents := map[string]*AgentDefinition{
		"a1": {ID: "a1", Name: "Researcher", OutputKey: "research"},
		"a2": {ID: "a2", Name: "Writer", SystemPrompt: "Use {research} to draft a reply."},
	}
	root := &CompositionNode{
		ID:   "root",
		Type: NodeSequential,
		Children: []CompositionNode{
			{ID: "n1", Type: NodeAgent, AgentID: "a1"},
			{ID: "n2", Type: NodeAgent, AgentID: "a2"},
		},
	}

	got := AnalyzeLineage(root, agents)

	if !slices.Contains(got.ProducibleBefore["n2"], "research") {
		t.Fatalf("ProducibleBefore[n2] = %v, want it to include 'research'", got.ProducibleBefore["n2"])
	}
	if hasIssue(got.Issues, IssueMissingRef, "n2", "research") {
		t.Fatalf("expected no missing-ref for {research}, got issues=%+v", got.Issues)
	}
}

func TestAnalyzeLineageParallelSiblingsCannotSeeEachOther(t *testing.T) {
	t.Parallel()

	agents := map[string]*AgentDefinition{
		"a1": {ID: "a1", Name: "FetcherA", OutputKey: "data_a"},
		// a2 incorrectly tries to read a sibling's output mid-parallel.
		"a2": {ID: "a2", Name: "FetcherB", SystemPrompt: "Use {data_a} too.", OutputKey: "data_b"},
	}
	root := &CompositionNode{
		ID:   "root",
		Type: NodeParallel,
		Children: []CompositionNode{
			{ID: "n1", Type: NodeAgent, AgentID: "a1"},
			{ID: "n2", Type: NodeAgent, AgentID: "a2"},
		},
	}

	got := AnalyzeLineage(root, agents)

	if !hasIssue(got.Issues, IssueMissingRef, "n2", "data_a") {
		t.Fatalf("expected missing-ref for sibling read inside Parallel; got %+v", got.Issues)
	}
}

func TestAnalyzeLineageOuterSequentialSeesParallelMergedOutputs(t *testing.T) {
	t.Parallel()

	agents := map[string]*AgentDefinition{
		"a1": {ID: "a1", Name: "FetchA", OutputKey: "data_a"},
		"a2": {ID: "a2", Name: "FetchB", OutputKey: "data_b"},
		"a3": {ID: "a3", Name: "Combiner", SystemPrompt: "Combine {data_a} and {data_b}."},
	}
	root := &CompositionNode{
		ID:   "root",
		Type: NodeSequential,
		Children: []CompositionNode{
			{ID: "par", Type: NodeParallel, Children: []CompositionNode{
				{ID: "n1", Type: NodeAgent, AgentID: "a1"},
				{ID: "n2", Type: NodeAgent, AgentID: "a2"},
			}},
			{ID: "n3", Type: NodeAgent, AgentID: "a3"},
		},
	}

	got := AnalyzeLineage(root, agents)

	if !slices.Contains(got.ProducibleBefore["n3"], "data_a") {
		t.Fatalf("ProducibleBefore[n3] should include 'data_a' after parallel; got %v", got.ProducibleBefore["n3"])
	}
	if !slices.Contains(got.ProducibleBefore["n3"], "data_b") {
		t.Fatalf("ProducibleBefore[n3] should include 'data_b' after parallel; got %v", got.ProducibleBefore["n3"])
	}
	if len(got.Issues) > 0 {
		t.Fatalf("expected no issues, got %+v", got.Issues)
	}
}

func TestAnalyzeLineageLoopChildrenSeeOtherIterationsOutputs(t *testing.T) {
	t.Parallel()

	// Loop body: child1 reads child2's output. In iteration 1 child1
	// can't see it, but the analyzer models steady state — runs > 1.
	agents := map[string]*AgentDefinition{
		"a1": {ID: "a1", Name: "Refine", SystemPrompt: "Improve {draft}.", OutputKey: "draft"},
		"a2": {ID: "a2", Name: "Critique", SystemPrompt: "Critique {draft}.", OutputKey: "critique"},
	}
	root := &CompositionNode{
		ID:            "root",
		Type:          NodeLoop,
		MaxIterations: 3,
		Children: []CompositionNode{
			{ID: "n1", Type: NodeAgent, AgentID: "a1"},
			{ID: "n2", Type: NodeAgent, AgentID: "a2"},
		},
	}

	got := AnalyzeLineage(root, agents)

	// Both nodes should see {draft} at steady state.
	if !slices.Contains(got.ProducibleBefore["n1"], "draft") {
		t.Fatalf("ProducibleBefore[n1] should include 'draft' in steady-state loop; got %v", got.ProducibleBefore["n1"])
	}
	if hasIssue(got.Issues, IssueMissingRef, "n1", "draft") {
		t.Fatalf("expected no missing-ref inside loop; got %+v", got.Issues)
	}
}

func TestAnalyzeLineageConditionalMissingEvaluatorOutputKeyIsReported(t *testing.T) {
	t.Parallel()

	agents := map[string]*AgentDefinition{
		"a1": {ID: "a1", Name: "Classifier"}, // no OutputKey
		"a2": {ID: "a2", Name: "BranchA"},
	}
	root := &CompositionNode{
		ID:   "root",
		Type: NodeConditional,
		Children: []CompositionNode{
			{ID: "eval", Type: NodeAgent, AgentID: "a1"},
			{ID: "br", Type: NodeAgent, AgentID: "a2", Condition: "*"},
		},
	}

	got := AnalyzeLineage(root, agents)

	if !hasIssue(got.Issues, IssueConditionalNoEvaluatorOutput, "eval", "") {
		t.Fatalf("expected conditional_no_evaluator_output issue; got %+v", got.Issues)
	}
}

func TestAnalyzeLineageOptionalReferenceDoesNotEmitMissingRef(t *testing.T) {
	t.Parallel()

	agents := map[string]*AgentDefinition{
		"a1": {ID: "a1", Name: "Solo", SystemPrompt: "Continue from {prior?} if any."},
	}
	root := &CompositionNode{
		ID:   "root",
		Type: NodeSequential,
		Children: []CompositionNode{
			{ID: "n1", Type: NodeAgent, AgentID: "a1"},
		},
	}

	got := AnalyzeLineage(root, agents)

	if hasIssue(got.Issues, IssueMissingRef, "n1", "prior") {
		t.Fatalf("expected no missing-ref for {prior?}; got %+v", got.Issues)
	}
	if len(got.References["n1"]) != 1 {
		t.Fatalf("expected one reference recorded; got %+v", got.References["n1"])
	}
	if !got.References["n1"][0].Optional {
		t.Fatalf("expected reference marked optional; got %+v", got.References["n1"])
	}
}

func TestAnalyzeLineageScopedReferencesAreSkipped(t *testing.T) {
	t.Parallel()

	agents := map[string]*AgentDefinition{
		"a1": {
			ID:           "a1",
			Name:         "Solo",
			SystemPrompt: "{app:flag} {user:name} {temp:scratch} {artifact.note}",
		},
	}
	root := &CompositionNode{
		ID:   "root",
		Type: NodeSequential,
		Children: []CompositionNode{
			{ID: "n1", Type: NodeAgent, AgentID: "a1"},
		},
	}

	got := AnalyzeLineage(root, agents)

	if len(got.References["n1"]) != 0 {
		t.Fatalf("expected scoped/artifact refs to be ignored; got %+v", got.References["n1"])
	}
	if len(got.Issues) != 0 {
		t.Fatalf("expected no issues for scoped refs; got %+v", got.Issues)
	}
}

func TestAnalyzeLineageMissingReferenceIsReported(t *testing.T) {
	t.Parallel()

	agents := map[string]*AgentDefinition{
		"a1": {ID: "a1", Name: "Solo", SystemPrompt: "Use {totally_undefined}."},
	}
	root := &CompositionNode{
		ID:   "root",
		Type: NodeSequential,
		Children: []CompositionNode{
			{ID: "n1", Type: NodeAgent, AgentID: "a1"},
		},
	}

	got := AnalyzeLineage(root, agents)

	if !hasIssue(got.Issues, IssueMissingRef, "n1", "totally_undefined") {
		t.Fatalf("expected missing-ref for {totally_undefined}; got %+v", got.Issues)
	}
}

func TestAnalyzeLineageUnusedOutputIsReportedForNonTerminalProducer(t *testing.T) {
	t.Parallel()

	// a1 sets `dangling` but nobody reads it. a2 is the terminal step
	// in the sequential and is exempt from the unused-output check.
	agents := map[string]*AgentDefinition{
		"a1": {ID: "a1", Name: "Producer", OutputKey: "dangling"},
		"a2": {ID: "a2", Name: "Final", OutputKey: "answer"},
	}
	root := &CompositionNode{
		ID:   "root",
		Type: NodeSequential,
		Children: []CompositionNode{
			{ID: "n1", Type: NodeAgent, AgentID: "a1"},
			{ID: "n2", Type: NodeAgent, AgentID: "a2"},
		},
	}

	got := AnalyzeLineage(root, agents)

	if !hasIssue(got.Issues, IssueUnusedOutput, "n1", "dangling") {
		t.Fatalf("expected unused-output for n1.dangling; got %+v", got.Issues)
	}
	if hasIssue(got.Issues, IssueUnusedOutput, "n2", "answer") {
		t.Fatalf("expected terminal n2.answer to be exempt; got %+v", got.Issues)
	}
}

func TestAnalyzeLineageConditionalIntersectionLimitsDownstreamVisibility(t *testing.T) {
	t.Parallel()

	// Conditional with two branches: branchA writes both `shared` and
	// `only_a`; branchB writes only `shared`. Downstream of the
	// conditional, only `shared` should be guaranteed.
	agents := map[string]*AgentDefinition{
		"eval": {ID: "eval", Name: "Eval", OutputKey: "decision"},
		"a":    {ID: "a", Name: "BranchA", OutputKey: "shared"},
		"a2":   {ID: "a2", Name: "BranchAExtra", OutputKey: "only_a"},
		"b":    {ID: "b", Name: "BranchB", OutputKey: "shared"},
		"down": {ID: "down", Name: "Downstream", SystemPrompt: "Read {shared} and {only_a}."},
	}
	root := &CompositionNode{
		ID:   "root",
		Type: NodeSequential,
		Children: []CompositionNode{
			{ID: "cond", Type: NodeConditional, Children: []CompositionNode{
				{ID: "evalNode", Type: NodeAgent, AgentID: "eval"},
				{ID: "brA", Type: NodeSequential, Condition: "x", Children: []CompositionNode{
					{ID: "brA1", Type: NodeAgent, AgentID: "a"},
					{ID: "brA2", Type: NodeAgent, AgentID: "a2"},
				}},
				{ID: "brB", Type: NodeAgent, AgentID: "b", Condition: "*"},
			}},
			{ID: "downNode", Type: NodeAgent, AgentID: "down"},
		},
	}

	got := AnalyzeLineage(root, agents)

	// `shared` produced by both branches → must appear downstream.
	if !slices.Contains(got.ProducibleBefore["downNode"], "shared") {
		t.Fatalf("downstream should see 'shared' (intersection of branches); got %v", got.ProducibleBefore["downNode"])
	}
	// `only_a` produced by only one branch → must NOT appear and must
	// raise a missing-ref on the downstream consumer.
	if slices.Contains(got.ProducibleBefore["downNode"], "only_a") {
		t.Fatalf("downstream should not guarantee 'only_a' (single branch); got %v", got.ProducibleBefore["downNode"])
	}
	if !hasIssue(got.Issues, IssueMissingRef, "downNode", "only_a") {
		t.Fatalf("expected missing-ref for branch-only key; got %+v", got.Issues)
	}
}

func TestAnalyzeLineageReferencesIncludeGlobalInstruction(t *testing.T) {
	t.Parallel()

	agents := map[string]*AgentDefinition{
		"a1": {
			ID:                "a1",
			Name:              "Solo",
			GlobalInstruction: "Always cite {sources}.",
		},
	}
	root := &CompositionNode{
		ID:   "root",
		Type: NodeSequential,
		Children: []CompositionNode{
			{ID: "n1", Type: NodeAgent, AgentID: "a1"},
		},
	}

	got := AnalyzeLineage(root, agents)

	if len(got.References["n1"]) != 1 || got.References["n1"][0].Source != "global_instruction" {
		t.Fatalf("expected one global_instruction reference; got %+v", got.References["n1"])
	}
}

func TestAnalyzeLineageNilRootReturnsEmptyAnalysis(t *testing.T) {
	t.Parallel()

	got := AnalyzeLineage(nil, nil)
	if len(got.Issues) != 0 || len(got.References) != 0 {
		t.Fatalf("expected empty analysis; got %+v", got)
	}
}
