package agentorchestrator_test

import (
	"context"
	"testing"

	"school-with-ai/foundation/agent"
	"school-with-ai/modules/chat/agentorchestrator/agents/math"
)

func TestAgentConfigurations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Math
	mathConfig, err := math.NewMathExpertConfig(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to create math expert config: %v", err)
	}
	if mathConfig.Name != "math-expert" {
		t.Errorf("Expected math name 'math-expert', got '%s'", mathConfig.Name)
	}
}

// mockTool implements the Tool interface simply for the test
type mockTool struct {
	name string
}

func (m *mockTool) Name() string          { return m.name }
func (m *mockTool) Description() string   { return "" }
func (m *mockTool) Schema() *agent.Schema { return nil }

func TestRequirementAgentIntegration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// 1. Create requirements declaratively
	reqs := []agent.Requirement{
		agent.NewConditionalRequirement("think", agent.ForceAtStep(1)),
	}

	// 2. Create the RequirementMiddleware (Reasoner)
	middleware := agent.RequirementMiddleware(reqs...)

	// 3. Create a handler that verifies the reasoner's enforcement
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

	// 5. Create a Chat context with tools on c.Chat.Tools
	chatCtx := agent.NewChatContext(ctx, nil, []agent.Message{}, &agent.ChatOptions{})
	chatCtx.Chat.Tools = []agent.Tool{
		&mockTool{name: "think"},
		&mockTool{name: "search"},
	}

	err := wrappedHandler.Handle(chatCtx)
	if err != nil {
		t.Fatalf("Unexpected error from wrapped handler: %v", err)
	}
}

// TestRequirementFunc tests the functional shorthand for requirements.
func TestRequirementFunc(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Create a requirement using Require() functional shorthand
	req := agent.Require("block-search", func(ctx context.Context, inv *agent.Invocation) ([]agent.Rule, error) {
		return []agent.Rule{{TargetTool: "search", Blocked: true, Reason: "Think first"}}, nil
	})

	middleware := agent.RequirementMiddleware(req)
	var finalHandler agent.HandlerFunc = func(c *agent.Context) error {
		// Verify search was removed from available tools
		for _, t := range c.Chat.Tools {
			if t.Name() == "search" {
				t2 := t // avoid shadowing in error
				_ = t2
				return nil // will be checked below
			}
		}
		return nil
	}

	wrappedHandler := middleware(finalHandler)
	chatCtx := agent.NewChatContext(ctx, nil, []agent.Message{}, &agent.ChatOptions{})
	chatCtx.Chat.Tools = []agent.Tool{
		&mockTool{name: "think"},
		&mockTool{name: "search"},
	}

	err := wrappedHandler.Handle(chatCtx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// After middleware, search should be filtered out
	for _, tool := range chatCtx.Chat.Tools {
		if tool.Name() == "search" {
			t.Error("Expected 'search' to be filtered out by requirement")
		}
	}
}

// TestPriorityConflictResolution tests that higher-priority rules win.
func TestPriorityConflictResolution(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Low-priority: force think
	lowPriority := agent.Require("low", func(ctx context.Context, inv *agent.Invocation) ([]agent.Rule, error) {
		return []agent.Rule{{TargetTool: "think", Forced: true, Priority: 1}}, nil
	})

	// High-priority: also force think (should win the reason)
	highPriority := agent.Require("high", func(ctx context.Context, inv *agent.Invocation) ([]agent.Rule, error) {
		return []agent.Rule{{TargetTool: "think", Forced: true, Priority: 10, Reason: "high-priority reason"}}, nil
	})

	middleware := agent.RequirementMiddleware(lowPriority, highPriority)

	var capturedChoice string
	var finalHandler agent.HandlerFunc = func(c *agent.Context) error {
		capturedChoice = c.Chat.Options.ToolChoice
		return nil
	}

	wrappedHandler := middleware(finalHandler)
	chatCtx := agent.NewChatContext(ctx, nil, []agent.Message{}, &agent.ChatOptions{})
	chatCtx.Chat.Tools = []agent.Tool{
		&mockTool{name: "think"},
		&mockTool{name: "search"},
	}

	err := wrappedHandler.Handle(chatCtx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if capturedChoice != "think" {
		t.Errorf("Expected ToolChoice='think', got '%s'", capturedChoice)
	}
}
