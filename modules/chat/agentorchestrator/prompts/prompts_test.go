package prompts

import (
	"strings"
	"testing"
)

func TestGenerateTeacherSystemPrompt_MissionAndObjectives(t *testing.T) {
	t.Parallel()
	config := TeacherPromptConfig{
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

	prompt, err := GenerateTeacherSystemPrompt(config)
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

func TestGenerateTeacherSystemPrompt_LearningPath(t *testing.T) {
	t.Parallel()
	config := TeacherPromptConfig{
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

	prompt, err := GenerateTeacherSystemPrompt(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify learning path sections
	if !strings.Contains(prompt, "# Learning Path") {
		t.Error("missing Learning Path section")
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

func TestGenerateTeacherSystemPrompt_Instructions(t *testing.T) {
	t.Parallel()
	config := TeacherPromptConfig{
		Role:        "Physics Instructor",
		Preamble:    "You are a physics instructor.",
		Topic:       "Physics",
		Interactive: true,
	}

	prompt, err := GenerateTeacherSystemPrompt(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify pedagogical instructions
	if !strings.Contains(prompt, "# Instructions") {
		t.Error("missing Instructions section")
	}
	if !strings.Contains(prompt, "- **Active Teaching:**") {
		t.Error("missing Active Teaching instruction")
	}
	if !strings.Contains(prompt, "- **Student Interaction:**") {
		t.Error("missing Student Interaction instruction")
	}
}
