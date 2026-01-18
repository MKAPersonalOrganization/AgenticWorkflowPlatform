package core

import (
	"strings"
	"testing"
)

func TestGenerateSystemPrompt_MissionAndObjectives(t *testing.T) {
	t.Parallel()
	config := PromptConfig{
		Role:     "Test Instructor",
		Preamble: "You are a test instructor.",
		Topic:    "Testing",
		Mission:  "Ensure quality through rigorous testing.",
		Objectives: []string{
			"Verify functionality",
			"Catch edge cases",
		},
		Interactive: true,
	}

	prompt, err := GenerateSystemPrompt(config)
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

func TestGenerateSystemPrompt_LearningPath(t *testing.T) {
	t.Parallel()
	config := PromptConfig{
		Role:     "Math Instructor",
		Preamble: "You are a math instructor.",
		Topic:    "Mathematics",
		LearningPath: &LearningPath{
			Prerequisites: []string{"Basic arithmetic", "Algebra fundamentals"},
			NextConcepts:  []string{"Calculus", "Linear algebra"},
			RelatedTopics: []string{"Physics", "Computer Science"},
			PracticeAreas: []string{"Word problems", "Proof exercises"},
		},
		Interactive: true,
	}

	prompt, err := GenerateSystemPrompt(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify learning path sections
	if !strings.Contains(prompt, "# Learning Path Guidance") {
		t.Error("missing Learning Path Guidance section")
	}
	if !strings.Contains(prompt, "## Prerequisites") {
		t.Error("missing Prerequisites subsection")
	}
	if !strings.Contains(prompt, "Basic arithmetic") {
		t.Error("missing prerequisite content")
	}
	if !strings.Contains(prompt, "## Suggested Next Topics") {
		t.Error("missing Next Topics subsection")
	}
	if !strings.Contains(prompt, "Calculus") {
		t.Error("missing next concept content")
	}
	if !strings.Contains(prompt, "## Cross-Domain Connections") {
		t.Error("missing Cross-Domain Connections subsection")
	}
	if !strings.Contains(prompt, "## Practice Recommendations") {
		t.Error("missing Practice Recommendations subsection")
	}
}

func TestGenerateSystemPrompt_NoHallucinationPolicy(t *testing.T) {
	t.Parallel()
	config := PromptConfig{
		Role:        "Chemistry Instructor",
		Preamble:    "You are a chemistry instructor.",
		Topic:       "Chemistry",
		Interactive: true,
	}

	prompt, err := GenerateSystemPrompt(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no-hallucination policy
	if !strings.Contains(prompt, "### No Hallucination Policy") {
		t.Error("missing No Hallucination Policy section")
	}
	if !strings.Contains(prompt, "CRITICAL") {
		t.Error("missing CRITICAL emphasis")
	}
	if !strings.Contains(prompt, "Never invent") {
		t.Error("missing 'Never invent' constraint")
	}
}

func TestGenerateSystemPrompt_PedagogicalDecisionTrees(t *testing.T) {
	t.Parallel()
	config := PromptConfig{
		Role:        "Physics Instructor",
		Preamble:    "You are a physics instructor.",
		Topic:       "Physics",
		Interactive: true,
	}

	prompt, err := GenerateSystemPrompt(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify pedagogical decision trees
	if !strings.Contains(prompt, "## Pedagogical Decision Trees") {
		t.Error("missing Pedagogical Decision Trees section")
	}
	if !strings.Contains(prompt, "### When Student Asks") {
		t.Error("missing 'When Student Asks' decision tree")
	}
	if !strings.Contains(prompt, "### When Student Gets Answer Wrong") {
		t.Error("missing 'When Student Gets Answer Wrong' decision tree")
	}
	if !strings.Contains(prompt, "### When Student Says \"I don't understand\"") {
		t.Error("missing confusion handling decision tree")
	}
	if !strings.Contains(prompt, "### When Recommending Next Steps") {
		t.Error("missing next steps decision tree")
	}
}

func TestGenerateSystemPrompt_CognitivePrioritization(t *testing.T) {
	t.Parallel()
	config := PromptConfig{
		Role:        "Statistics Instructor",
		Preamble:    "You are a statistics instructor.",
		Topic:       "Statistics",
		Interactive: true,
	}

	prompt, err := GenerateSystemPrompt(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify cognitive prioritization pattern
	if !strings.Contains(prompt, "PAY EXTREMELY CLOSE ATTENTION") {
		t.Error("missing cognitive prioritization emphasis")
	}
	if !strings.Contains(prompt, "## Foundational Constraints (NON-NEGOTIABLE)") {
		t.Error("missing foundational constraints section")
	}
}

func TestGenerateSystemPrompt_SourceGrounding(t *testing.T) {
	t.Parallel()
	config := PromptConfig{
		Role:        "Math Instructor",
		Preamble:    "You are a math instructor.",
		Topic:       "Mathematics",
		Interactive: true,
	}

	prompt, err := GenerateSystemPrompt(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify source grounding
	if !strings.Contains(prompt, "### Source Grounding (Mandatory)") {
		t.Error("missing Source Grounding section")
	}
	if !strings.Contains(prompt, "Prioritize provided book resources") {
		t.Error("missing book resources guidance")
	}
}
