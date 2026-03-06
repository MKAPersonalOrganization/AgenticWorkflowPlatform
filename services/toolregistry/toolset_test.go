package toolregistry_test

import (
	"context"
	"os"
	"school-with-ai/foundation/logger"
	"school-with-ai/plugins/calculator"
	"school-with-ai/plugins/memory"
	"school-with-ai/plugins/visualization"
	"school-with-ai/services/toolregistry"
	"school-with-ai/services/toolregistry/mocks"
	"testing"
)

func TestToolSetFunctionality(t *testing.T) {
	ctx := context.Background()
	log := logger.NewText(os.Stdout, logger.LevelInfo, "TEST", nil)

	// Register required plugins for the test
	toolregistry.RegisterPlugin(&calculator.Provider{})
	toolregistry.RegisterPlugin(&visualization.Provider{})
	toolregistry.RegisterPlugin(&memory.Provider{})

	// Create in-memory repository and registry
	repo := mocks.NewInMemoryRepository()
	registry := toolregistry.NewRegistry(log, repo)

	// Initialize with default tools and tool sets
	if err := registry.Initialize(ctx); err != nil {
		t.Fatalf("Failed to initialize registry: %v", err)
	}

	// Test: List all tool sets
	toolSets, err := registry.ListToolSets(ctx)
	if err != nil {
		t.Fatalf("Failed to list tool sets: %v", err)
	}

	if len(toolSets) == 0 {
		t.Errorf("Expected default tool sets, but got none")
	}

	t.Logf("Found %d tool sets", len(toolSets))

	// Test: Verify specific tool sets exist
	expectedToolSets := []string{"calculator-set", "visualization-set", "memory-set"}
	for _, expectedID := range expectedToolSets {
		toolSet, err := registry.GetToolSet(ctx, expectedID)
		if err != nil {
			t.Errorf("Expected tool set %s to exist, but got error: %v", expectedID, err)
			continue
		}

		if len(toolSet.ToolIDs) == 0 {
			t.Errorf("Expected tool set %s to contain tools, but it's empty", expectedID)
		}

		t.Logf("Tool set %s contains %d tools: %v", expectedID, len(toolSet.ToolIDs), toolSet.ToolIDs)
	}

	// Test: Verify tools are associated with correct tool sets
	calculatorSet, err := registry.GetToolSet(ctx, "calculator-set")
	if err != nil {
		t.Fatalf("Failed to get calculator set: %v", err)
	}

	// Verify calculator tools are in the calculator set
	expectedCalculatorTools := []string{"calc-add", "calc-subtract", "calc-multiply", "calc-divide", "calc-power", "calc-sqrt"}
	for _, expectedTool := range expectedCalculatorTools {
		found := false
		for _, toolID := range calculatorSet.ToolIDs {
			if toolID == expectedTool {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected calculator set to contain %s, but it doesn't", expectedTool)
		}
	}

	// Test: Get tools in set
	toolsInCalculatorSet, err := registry.GetToolsInSet(ctx, "calculator-set")
	if err != nil {
		t.Fatalf("Failed to get tools in calculator set: %v", err)
	}

	if len(toolsInCalculatorSet) != len(calculatorSet.ToolIDs) {
		t.Errorf("Expected %d tools in calculator set, but got %d", len(calculatorSet.ToolIDs), len(toolsInCalculatorSet))
	}

	// Verify all tools have the calculator set ID
	for _, tool := range toolsInCalculatorSet {
		if tool.ToolSetID != "calculator-set" {
			t.Errorf("Expected tool %s to have ToolSetID 'calculator-set', but got '%s'", tool.ID, tool.ToolSetID)
		}
	}

	// Test: Create a custom tool set
	customToolSet := &toolregistry.ToolSet{
		ID:          "custom-test-set",
		Name:        "Custom Test Set",
		Description: "A test tool set for validation",
		Category:    "Testing",
		ToolIDs:     []string{"calc-add", "viz-plot"},
	}

	if err := registry.RegisterToolSet(ctx, customToolSet); err != nil {
		t.Fatalf("Failed to register custom tool set: %v", err)
	}

	// Verify the custom tool set was saved
	retrievedCustomSet, err := registry.GetToolSet(ctx, "custom-test-set")
	if err != nil {
		t.Fatalf("Failed to retrieve custom tool set: %v", err)
	}

	if retrievedCustomSet.Name != customToolSet.Name {
		t.Errorf("Expected custom set name '%s', but got '%s'", customToolSet.Name, retrievedCustomSet.Name)
	}

	if len(retrievedCustomSet.ToolIDs) != 2 {
		t.Errorf("Expected custom set to have 2 tools, but got %d", len(retrievedCustomSet.ToolIDs))
	}

	t.Log("Tool set functionality tests passed!")
}
