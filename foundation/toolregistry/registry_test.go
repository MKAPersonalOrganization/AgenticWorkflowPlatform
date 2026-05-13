package toolregistry_test

import (
	"aicompass/foundation/logger"
	"aicompass/foundation/toolregistry"
	"aicompass/foundation/toolregistry/mocks"
	"context"
	"os"
	"testing"
)

func TestToolRegistry(t *testing.T) {
	t.Parallel()

	// Create logger for tests
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)

	// Register required plugins for the test
	toolregistry.RegisterPlugin(&testPlugin{})

	// Create registry with in-memory repository
	registry := toolregistry.NewRegistry(log, mocks.NewInMemoryRepository())

	ctx := context.Background()

	// Test initialization
	if err := registry.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	// Test listing tools
	tools, err := registry.ListTools(ctx)
	if err != nil {
		t.Fatalf("Failed to list tools: %v", err)
	}

	if len(tools) == 0 {
		t.Fatal("Expected default tools to be initialized")
	}

	t.Logf("Found %d default tools", len(tools))

	// Test getting a specific tool
	testToolID := "power"
	tool, err := registry.GetTool(ctx, testToolID)
	if err != nil {
		t.Fatalf("Failed to get tool: %v", err)
	}

	if tool.ID != testToolID {
		t.Fatalf("Expected tool ID %s, got %s", testToolID, tool.ID)
	}

	t.Logf("Successfully retrieved tool: %s - %s", tool.ID, tool.Name)

	// Test registering a new tool
	newTool := &toolregistry.Tool{
		ID:          "test-tool",
		Name:        "Test Tool",
		Description: "A tool for testing",
		Category:    "testing",
		Metadata:    map[string]string{"type": "test"},
	}

	if err := registry.RegisterTool(ctx, newTool); err != nil {
		t.Fatalf("Failed to register new tool: %v", err)
	}

	// Verify new tool was registered
	registeredTool, err := registry.GetTool(ctx, newTool.ID)
	if err != nil {
		t.Fatalf("Failed to get registered tool: %v", err)
	}

	if registeredTool.Name != newTool.Name {
		t.Fatalf("Expected tool name %s, got %s", newTool.Name, registeredTool.Name)
	}

	t.Log("Tool registry tests passed!")
}
