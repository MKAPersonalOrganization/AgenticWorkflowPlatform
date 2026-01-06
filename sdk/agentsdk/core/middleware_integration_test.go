package core

import (
	"context"
	"school-with-ai/sdk/agentsdk/core/middleware"
	"school-with-ai/sdk/agentsdk/llm"
	"testing"
)

// TestMiddlewareToolsAvailableInChatOptions verifies that middleware tools
// are included in the effective tools list
func TestMiddlewareToolsAvailableInChatOptions(t *testing.T) {
	// Create a config with TodoList middleware
	config := AgentConfig{
		Name:         "test-agent",
		SystemPrompt: "You are a test agent.",
		Tools:        []Tool{}, // No tools initially
		Middleware:   []Middleware{middleware.NewTodoListMiddleware()},
	}

	// Create some dummy messages
	messages := []llm.ChatMessage{
		llm.NewUserMessage("test message"),
	}

	// Apply middleware
	chatOpts, reqCtx, err := ToChatOptionsWithMiddleware(context.Background(), config, messages, DefaultRunOptions())
	if err != nil {
		t.Fatalf("ToChatOptionsWithMiddleware failed: %v", err)
	}

	// Verify write_todos tool was added
	effectiveTools := reqCtx.GetEffectiveTools()
	foundWriteTodos := false
	for _, tool := range effectiveTools {
		if tool.Name() == "write_todos" {
			foundWriteTodos = true
			break
		}
	}

	if !foundWriteTodos {
		t.Errorf("write_todos tool not found in effective tools")
	}

	// Verify the tool is in the chat options
	if len(chatOpts.Tools) == 0 {
		t.Errorf("Expected tools in ChatOptions, got none")
	}

	foundInChatOpts := false
	for _, aiTool := range chatOpts.Tools {
		if aiTool.Name == "write_todos" {
			foundInChatOpts = true
			break
		}
	}

	if !foundInChatOpts {
		t.Errorf("write_todos tool not found in ChatOptions.Tools")
	}
}
