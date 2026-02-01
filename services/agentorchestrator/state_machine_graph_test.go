package agentorchestrator_test

import (
	"strings"
	"testing"

	"school-with-ai/domain"
)

func TestGraphGeneration(t *testing.T) {
	t.Run("Tool Graph", func(t *testing.T) {
		graph := domain.GetToolStateMachineGraph()
		if graph == "" {
			t.Error("expected tool graph to be non-empty")
		}
		if !strings.HasPrefix(graph, "digraph") {
			t.Errorf("expected graph to start with 'digraph', got %s", graph)
		}
		t.Logf("Tool Graph:\n%s", graph)
	})
}
