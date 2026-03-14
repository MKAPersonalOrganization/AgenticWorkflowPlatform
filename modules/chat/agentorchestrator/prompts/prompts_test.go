package prompts

import (
	"strings"
	"testing"
)

func TestGenerateAgentSystemPrompt_MissionAndObjectives(t *testing.T) {
	t.Parallel()
	config := AgentPromptConfig{
		Role:     "Test Expert",
		Preamble: "You are a test expert.",
		Topic:    "Testing",
		Mission:  "Ensure quality through rigorous testing.",
		Objectives: []string{
			"Verify functionality",
			"Catch edge cases",
		},
		Interactive: true,
	}

	prompt, err := GenerateAgentSystemPrompt(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify mission section exists
	if !strings.Contains(prompt, "# Mission & Objectives") {
		t.Error("missing Mission & Objectives section")
	}
	if !strings.Contains(prompt, "**Mission:** Ensure quality through rigorous testing.") {
		t.Error("missing mission statement")
	}
	if !strings.Contains(prompt, "Verify functionality") {
		t.Error("missing first objective")
	}
	if !strings.Contains(prompt, "Catch edge cases") {
		t.Error("missing second objective")
	}
}

func TestGenerateAgentSystemPrompt_ProgressionPath(t *testing.T) {
	t.Parallel()
	config := AgentPromptConfig{
		Role:     "Math Expert",
		Preamble: "You are a math expert.",
		Topic:    "Mathematics",
		ProgressionPath: &ProgressionPath{
			Prerequisites: []string{"Basic arithmetic", "Algebra fundamentals"},
			NextConcepts:  []string{"Calculus", "Linear algebra"},
			RelatedTopics: []string{"Physics", "Computer Science"},
			PracticeAreas: []string{"Word problems", "Proof exercises"},
		},
		Interactive: true,
	}

	prompt, err := GenerateAgentSystemPrompt(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify progression path sections
	if !strings.Contains(prompt, "# Progression Path") {
		t.Error("missing Progression Path section")
	}
	if !strings.Contains(prompt, "**Prerequisites:**") {
		t.Error("missing Prerequisites subsection")
	}
	if !strings.Contains(prompt, "Basic arithmetic") {
		t.Error("missing prerequisite content")
	}
	if !strings.Contains(prompt, "**Next Concepts:**") {
		t.Error("missing Next Concepts subsection")
	}
	if !strings.Contains(prompt, "Calculus") {
		t.Error("missing next concept content")
	}
}

func TestGenerateAgentSystemPrompt_Instructions(t *testing.T) {
	t.Parallel()
	config := AgentPromptConfig{
		Role:        "Physics Expert",
		Preamble:    "You are a physics expert.",
		Topic:       "Physics",
		Interactive: true,
	}

	prompt, err := GenerateAgentSystemPrompt(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify instructions
	if !strings.Contains(prompt, "# Instructions") {
		t.Error("missing Instructions section")
	}
	if !strings.Contains(prompt, "- **Active Guidance:**") {
		t.Error("missing Active Guidance instruction")
	}
	if !strings.Contains(prompt, "- **User Interaction:**") {
		t.Error("missing User Interaction instruction")
	}
}
