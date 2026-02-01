package agentorchestrator_test

import (
	"os"
	"strings"
	"testing"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/engine"
	"school-with-ai/foundation/logger"
)

func TestGraphGeneration(t *testing.T) {
	t.Run("Engine Graph", func(t *testing.T) {
		// Setup minimal dependencies for Engine
		log := logger.New(os.Stdout, logger.LevelInfo, "TEST", nil)
		// We need a provider that doesn't crash on nil registry if we are just testing GetGraph
		// But GetGraph doesn't use the provider.

		e := engine.New(
			log,
			nil, // chatClient
			nil, // provider
			nil, // reqCtxCapture
			nil, // toolTracer
		)

		graph := e.GetGraph()
		if graph == "" {
			t.Error("expected engine graph to be non-empty")
		}
		if !strings.HasPrefix(graph, "digraph") {
			t.Errorf("expected graph to start with 'digraph', got %s", graph)
		}
		t.Logf("Engine Graph:\n%s", graph)
	})

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
