package agentdefinition

import (
	"aicompass/foundation/logger"
	"context"
	"io"
	"testing"
)

// available_state_keys_test.go verifies the behaviour that powers the
// designer's instruction-template picker endpoint —
// GET /v1/agent-definitions/{id}/available-state-keys.
//
// The endpoint is a thin HTTP wrapper over Service.GetAvailableStateKeys
// (see handler.go:availableStateKeys). We test the service directly here
// because the HTTP route is authenticated and the userrequirements/
// black-box suite runs against the unauthenticated debug port.
//
// The spec under test is the priority-tiered enumeration described in
// service.go and grounded in ADK's sample corpus:
//
//	self               — the agent's own OutputKey
//	direct sub-agent   — SubAgents / AgentTools of the current agent
//	graph child        — OutputKeys of agents inside this agent's Graph
//	workflow peer      — OutputKeys of peers in any workflow that embeds
//	                     this agent (reverse lookup)
//	library            — any other OutputKey declared by the user
//
// Each test name reads as a spec sentence (one claim per test) so the
// gotestdox output points directly at what regressed.

// newStateKeysService builds a Service whose repo is prepopulated with
// `defs`. It mirrors how the production handler wires up the Service,
// without touching Postgres.
func newStateKeysService(defs ...*AgentDefinition) *Service {
	byID := make(map[string]*AgentDefinition, len(defs))
	byUser := make([]AgentDefinition, 0, len(defs))
	for _, d := range defs {
		byID[d.ID] = d
		byUser = append(byUser, *d)
	}
	return &Service{
		log:  logger.New(io.Discard, logger.LevelError, "test", nil),
		repo: &stubRepo{byID: byID, byUser: byUser},
	}
}

// findKey returns the suggestion with the matching key, or nil.
func findKey(suggestions []StateKeySuggestion, key string) *StateKeySuggestion {
	for i := range suggestions {
		if suggestions[i].Key == key {
			return &suggestions[i]
		}
	}
	return nil
}

func TestGetAvailableStateKeysReturnsTheAgentsOwnOutputKeyAsSelfTier(t *testing.T) {
	t.Parallel()

	svc := newStateKeysService(&AgentDefinition{
		ID:        "a1",
		UserID:    "u1",
		Name:      "Solo",
		OutputKey: "my_output",
	})

	got, err := svc.GetAvailableStateKeys(context.Background(), "u1", "a1")
	if err != nil {
		t.Fatalf("GetAvailableStateKeys() error = %v", err)
	}

	s := findKey(got, "my_output")
	if s == nil {
		t.Fatalf("expected suggestion for 'my_output'; got %+v", got)
	}
	if s.Priority != prioritySelf {
		t.Fatalf("priority = %d, want %d (self)", s.Priority, prioritySelf)
	}
	if s.Source != "self" {
		t.Fatalf("source = %q, want %q", s.Source, "self")
	}
	if s.Scope != "session" {
		t.Fatalf("scope = %q, want %q", s.Scope, "session")
	}
}

func TestGetAvailableStateKeysIncludesOutputKeysOfDirectSubAgents(t *testing.T) {
	t.Parallel()

	svc := newStateKeysService(
		&AgentDefinition{
			ID:     "parent",
			UserID: "u1",
			Name:   "Parent",
			SubAgents: []*SubAgentRef{
				{AgentID: "child"},
			},
		},
		&AgentDefinition{
			ID:        "child",
			UserID:    "u1",
			Name:      "Child",
			OutputKey: "child_out",
		},
	)

	got, err := svc.GetAvailableStateKeys(context.Background(), "u1", "parent")
	if err != nil {
		t.Fatalf("GetAvailableStateKeys() error = %v", err)
	}

	s := findKey(got, "child_out")
	if s == nil {
		t.Fatalf("expected 'child_out' in suggestions; got %+v", got)
	}
	if s.Priority != priorityDirectChild {
		t.Fatalf("priority = %d, want %d (direct child)", s.Priority, priorityDirectChild)
	}
	if s.Source != "sub-agent output: Child" {
		t.Fatalf("source = %q, want %q", s.Source, "sub-agent output: Child")
	}
}

func TestGetAvailableStateKeysIncludesOutputKeysOfAgentTools(t *testing.T) {
	t.Parallel()

	svc := newStateKeysService(
		&AgentDefinition{
			ID:     "parent",
			UserID: "u1",
			Name:   "Parent",
			AgentTools: []*AgentToolRef{
				{AgentID: "tool"},
			},
		},
		&AgentDefinition{
			ID:        "tool",
			UserID:    "u1",
			Name:      "Tool",
			OutputKey: "tool_out",
		},
	)

	got, err := svc.GetAvailableStateKeys(context.Background(), "u1", "parent")
	if err != nil {
		t.Fatalf("GetAvailableStateKeys() error = %v", err)
	}

	s := findKey(got, "tool_out")
	if s == nil {
		t.Fatalf("expected 'tool_out' in suggestions; got %+v", got)
	}
	if s.Priority != priorityDirectChild {
		t.Fatalf("priority = %d, want %d (direct child)", s.Priority, priorityDirectChild)
	}
	if s.Source != "agent-tool output: Tool" {
		t.Fatalf("source = %q, want %q", s.Source, "agent-tool output: Tool")
	}
}

func TestGetAvailableStateKeysIncludesOutputKeysOfChildrenInTheAgentsOwnGraph(t *testing.T) {
	t.Parallel()

	svc := newStateKeysService(
		&AgentDefinition{
			ID:     "workflow",
			UserID: "u1",
			Name:   "WF",
			Graph: &CompositionNode{
				ID:   "root",
				Type: NodeSequential,
				Children: []CompositionNode{
					{ID: "n1", Type: NodeAgent, AgentID: "step"},
				},
			},
		},
		&AgentDefinition{
			ID:        "step",
			UserID:    "u1",
			Name:      "Step",
			OutputKey: "step_out",
		},
	)

	got, err := svc.GetAvailableStateKeys(context.Background(), "u1", "workflow")
	if err != nil {
		t.Fatalf("GetAvailableStateKeys() error = %v", err)
	}

	s := findKey(got, "step_out")
	if s == nil {
		t.Fatalf("expected 'step_out' in suggestions; got %+v", got)
	}
	if s.Priority != priorityGraphChild {
		t.Fatalf("priority = %d, want %d (graph child)", s.Priority, priorityGraphChild)
	}
	if s.Source != "workflow step: Step" {
		t.Fatalf("source = %q, want %q", s.Source, "workflow step: Step")
	}
}

// Reverse lookup: editing a leaf that sits inside someone else's workflow
// should surface its workflow peers' OutputKeys. This is the gap the M6
// optimisation closes — a leaf agent's Graph is nil, so the only way to
// know its peers is to scan all workflows that contain it.
func TestGetAvailableStateKeysDetectsWorkflowPeersOfALeafAgent(t *testing.T) {
	t.Parallel()

	svc := newStateKeysService(
		&AgentDefinition{
			ID:     "pipeline",
			UserID: "u1",
			Name:   "Pipeline",
			Graph: &CompositionNode{
				ID:   "root",
				Type: NodeSequential,
				Children: []CompositionNode{
					{ID: "n1", Type: NodeAgent, AgentID: "upstream"},
					{ID: "n2", Type: NodeAgent, AgentID: "current"},
				},
			},
		},
		&AgentDefinition{
			ID:        "upstream",
			UserID:    "u1",
			Name:      "Upstream",
			OutputKey: "upstream_out",
		},
		&AgentDefinition{
			ID:     "current",
			UserID: "u1",
			Name:   "Current",
		},
	)

	got, err := svc.GetAvailableStateKeys(context.Background(), "u1", "current")
	if err != nil {
		t.Fatalf("GetAvailableStateKeys() error = %v", err)
	}

	s := findKey(got, "upstream_out")
	if s == nil {
		t.Fatalf("expected 'upstream_out' (peer in Pipeline) in suggestions; got %+v", got)
	}
	if s.Priority != priorityWorkflow {
		t.Fatalf("priority = %d, want %d (workflow peer)", s.Priority, priorityWorkflow)
	}
	if s.Source != "workflow peer: Pipeline → Upstream" {
		t.Fatalf("source = %q, want %q", s.Source, "workflow peer: Pipeline → Upstream")
	}
}

func TestGetAvailableStateKeysFallsBackToLibraryForUnrelatedAgents(t *testing.T) {
	t.Parallel()

	svc := newStateKeysService(
		&AgentDefinition{
			ID:     "focused",
			UserID: "u1",
			Name:   "Focused",
		},
		&AgentDefinition{
			ID:        "stranger",
			UserID:    "u1",
			Name:      "Stranger",
			OutputKey: "stranger_out",
		},
	)

	got, err := svc.GetAvailableStateKeys(context.Background(), "u1", "focused")
	if err != nil {
		t.Fatalf("GetAvailableStateKeys() error = %v", err)
	}

	s := findKey(got, "stranger_out")
	if s == nil {
		t.Fatalf("expected library-fallback entry for 'stranger_out'; got %+v", got)
	}
	if s.Priority != priorityLibrary {
		t.Fatalf("priority = %d, want %d (library)", s.Priority, priorityLibrary)
	}
	if s.Source != "agent: Stranger" {
		t.Fatalf("source = %q, want %q", s.Source, "agent: Stranger")
	}
}

// When the same OutputKey appears in several tiers (e.g. also in the
// library fallback), the highest-priority tier's entry must win. The
// picker should never show duplicates, and the chosen source label has
// to match the tier actually displayed.
func TestGetAvailableStateKeysDeduplicatesAcrossTiersKeepingHighestPriority(t *testing.T) {
	t.Parallel()

	// Agent A has its own OutputKey "shared_key" AND the same key is
	// declared on another agent in the library. Self (100) must beat
	// library (20).
	svc := newStateKeysService(
		&AgentDefinition{
			ID:        "a",
			UserID:    "u1",
			Name:      "A",
			OutputKey: "shared_key",
		},
		&AgentDefinition{
			ID:        "b",
			UserID:    "u1",
			Name:      "B",
			OutputKey: "shared_key",
		},
	)

	got, err := svc.GetAvailableStateKeys(context.Background(), "u1", "a")
	if err != nil {
		t.Fatalf("GetAvailableStateKeys() error = %v", err)
	}

	count := 0
	for _, s := range got {
		if s.Key == "shared_key" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("'shared_key' appeared %d times in suggestions; want 1 (deduped)", count)
	}

	s := findKey(got, "shared_key")
	if s.Priority != prioritySelf {
		t.Fatalf("priority = %d, want %d (self should beat library)", s.Priority, prioritySelf)
	}
	if s.Source != "self" {
		t.Fatalf("source = %q, want %q", s.Source, "self")
	}
}

// The picker relies on the backend ordering to render tier group
// headers in the right sequence. Verify ordering is
// (priority DESC, then key ASC) regardless of insertion order.
func TestGetAvailableStateKeysSortsByPriorityDescendingThenByKey(t *testing.T) {
	t.Parallel()

	svc := newStateKeysService(
		&AgentDefinition{
			ID:     "editing",
			UserID: "u1",
			Name:   "Editing",
			SubAgents: []*SubAgentRef{
				{AgentID: "direct-child"},
			},
		},
		&AgentDefinition{
			ID:        "direct-child",
			UserID:    "u1",
			Name:      "Child",
			OutputKey: "zebra_key", // direct-child tier (80)
		},
		&AgentDefinition{
			ID:        "library-a",
			UserID:    "u1",
			Name:      "LibA",
			OutputKey: "alpha_key", // library tier (20)
		},
		&AgentDefinition{
			ID:        "library-b",
			UserID:    "u1",
			Name:      "LibB",
			OutputKey: "beta_key", // library tier (20)
		},
	)

	got, err := svc.GetAvailableStateKeys(context.Background(), "u1", "editing")
	if err != nil {
		t.Fatalf("GetAvailableStateKeys() error = %v", err)
	}

	var keys []string
	for _, s := range got {
		keys = append(keys, s.Key)
	}
	// zebra_key (direct child, 80) must come before alpha_key and
	// beta_key (library, 20) despite being alphabetically last.
	// Within the library tier the alphabetical order holds.
	want := []string{"zebra_key", "alpha_key", "beta_key"}
	if len(keys) != len(want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	for i, k := range want {
		if keys[i] != k {
			t.Fatalf("keys[%d] = %q, want %q (full list: %v)", i, keys[i], k, keys)
		}
	}
}

func TestGetAvailableStateKeysReturnsEmptyWhenNoOutputKeysAreDeclared(t *testing.T) {
	t.Parallel()

	svc := newStateKeysService(
		&AgentDefinition{ID: "a", UserID: "u1", Name: "A"},
		&AgentDefinition{ID: "b", UserID: "u1", Name: "B"},
	)

	got, err := svc.GetAvailableStateKeys(context.Background(), "u1", "a")
	if err != nil {
		t.Fatalf("GetAvailableStateKeys() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d suggestions, want 0: %+v", len(got), got)
	}
}

// A SubAgentRef can override the child's OutputKey at the call site
// (SubAgentRef.OutputKey), but the child may also declare its own
// OutputKey on its definition. Both are valid at runtime, so the picker
// surfaces both names — each carrying the direct-child priority.
func TestGetAvailableStateKeysIncludesBothTheSubAgentRefOverrideAndTheChildsOwnOutputKey(t *testing.T) {
	t.Parallel()

	svc := newStateKeysService(
		&AgentDefinition{
			ID:     "parent",
			UserID: "u1",
			Name:   "Parent",
			SubAgents: []*SubAgentRef{
				{AgentID: "child", OutputKey: "override_name"},
			},
		},
		&AgentDefinition{
			ID:        "child",
			UserID:    "u1",
			Name:      "Child",
			OutputKey: "child_native_name",
		},
	)

	got, err := svc.GetAvailableStateKeys(context.Background(), "u1", "parent")
	if err != nil {
		t.Fatalf("GetAvailableStateKeys() error = %v", err)
	}

	if s := findKey(got, "override_name"); s == nil || s.Priority != priorityDirectChild {
		t.Fatalf("expected direct-child entry for 'override_name'; got %+v", got)
	}
	if s := findKey(got, "child_native_name"); s == nil || s.Priority != priorityDirectChild {
		t.Fatalf("expected direct-child entry for 'child_native_name'; got %+v", got)
	}
}
