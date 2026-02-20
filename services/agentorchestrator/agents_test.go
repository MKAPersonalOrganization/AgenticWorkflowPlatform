package agentorchestrator_test

import (
	"context"
	"testing"

	"school-with-ai/foundation/agent"
	"school-with-ai/services/agentorchestrator/agents/math"
	"school-with-ai/services/agentorchestrator/agents/statistics"
)

func TestInstructorConfigurations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Math
	mathConfig, err := math.NewMathInstructorConfig(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to create math instructor config: %v", err)
	}
	if mathConfig.Name != "math-instructor" {
		t.Errorf("Expected math name 'math-instructor', got '%s'", mathConfig.Name)
	}

	// Statistics
	statisticsConfig, err := statistics.NewStatisticsInstructorConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to create statistics instructor config: %v", err)
	}
	if statisticsConfig.Name != "statistics-instructor" {
		t.Errorf("Expected statistics name 'statistics-instructor', got '%s'", statisticsConfig.Name)
	}
}

// mockTool implements the Tool interface simply for the test
type mockTool struct {
	name string
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return "" }
func (m *mockTool) Schema() *agent.Schema     { return nil }

func TestRequirementAgentIntegration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// 1. Create a requirement that forces 'think' at step 1
	reqs := []agent.Requirement{
		agent.NewConditionalRequirement("think", agent.ForceAtStep(1)),
	}

	// 2. Create the Middleware
	middleware := agent.RequirementMiddleware(reqs...)

	// 3. Create a dummy AgentFunc (Handler)
	var finalHandler agent.HandlerFunc = func(c *agent.Context) error {
		// Verify that the 'think' tool was forced via system message
		if len(c.Chat.Messages) == 0 {
			t.Fatal("Expected system message injected for forced tool")
		}
		
		injectedMsg := c.Chat.Messages[0].Content[0].Text
		if injectedMsg == "" {
			t.Error("Expected injected message text to not be empty")
		}
		return nil
	}

	// 4. Wrap with Middleware
	wrappedHandler := middleware(finalHandler)

	// 5. Invoke the handler
	chatCtx := agent.NewChatContext(ctx, nil, []agent.Message{}, &agent.ChatOptions{
		Metadata: map[string]any{
			"tools": []agent.Tool{
				&mockTool{name: "think"},
				&mockTool{name: "search"},
			}, 
		},
	})
	
	err := wrappedHandler.Handle(chatCtx)
	if err != nil {
		t.Fatalf("Unexpected error from wrapped handler: %v", err)
	}
}
