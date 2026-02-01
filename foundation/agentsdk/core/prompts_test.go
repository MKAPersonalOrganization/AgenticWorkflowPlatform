package core

import (
	"strings"
	"testing"
)

func TestGenerateSystemPrompt_Generic(t *testing.T) {
	t.Parallel()
	config := PromptConfig{
		Role:     "Test Role",
		Preamble: "Test Preamble",
		Context: map[string]any{
			"Mission": "Obtain coffee",
		},
	}

	prompt, err := GenerateSystemPrompt(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(prompt, "# Role: Test Role") {
		t.Error("missing role")
	}
	if !strings.Contains(prompt, "Test Preamble") {
		t.Error("missing preamble")
	}
	if !strings.Contains(prompt, "# Context") {
		t.Error("missing context header")
	}
	if !strings.Contains(prompt, "## Mission") {
		t.Error("missing context key")
	}
	if !strings.Contains(prompt, "Obtain coffee") {
		t.Error("missing context value")
	}
}
