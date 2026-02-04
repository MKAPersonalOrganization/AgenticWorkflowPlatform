package tools

import (
	"testing"

	"school-with-ai/foundation/agent"
)

// TestMathTools verifies the new math tool definitions.
func TestMathTools(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		toolFunc func() agent.Tool
	}{
		{"add", GetAddTool},
		{"subtract", GetSubtractTool},
		{"multiply", GetMultiplyTool},
		{"divide", GetDivideTool},
		{"power", GetPowerTool},
		{"sqrt", GetSqrtTool},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tool := tc.toolFunc()
			if tool.Name() != tc.name {
				t.Errorf("Expected name %q, got %v", tc.name, tool.Name())
			}
			if tool.Description() == "" {
				t.Errorf("Tool %q missing description", tc.name)
			}
			if tool.InputSchema() == nil {
				t.Errorf("Tool %q missing input schema", tc.name)
			}
		})
	}
}

// TestGetAllTools verifies all tools are returned.
func TestGetAllTools(t *testing.T) {
	t.Parallel()
	tools := GetAllTools()

	expectedTools := map[string]bool{
		"add":      false,
		"subtract": false,
		"multiply": false,
		"divide":   false,
		"power":    false,
		"sqrt":     false,
	}

	for _, tool := range tools {
		if _, exists := expectedTools[tool.Name()]; exists {
			expectedTools[tool.Name()] = true
		}
	}

	for name, found := range expectedTools {
		if !found {
			t.Errorf("Tool %q not found in GetAllTools()", name)
		}
	}
}
