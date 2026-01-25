package core

import (
	"bytes"
	"strings"
	"text/template"
)

// Specialty defines an academic specialist for the learning coordinator.
type Specialty struct {
	Name        string
	Description string
}

// Skill defines a specialized agent capability.
type Skill struct {
	Name        string
	Description string
	Location    string
}

// StudentProfile defines the learning context of the student.
type StudentProfile struct {
	Name          string
	LearningStyle string
	Struggles     []string
	Strengths     []string
}

// LearningPath defines related concepts and resources for progressive learning.
type LearningPath struct {
	Prerequisites []string // Concepts the student should know first
	NextConcepts  []string // Logical next topics to explore
	RelatedTopics []string // Cross-domain connections
	PracticeAreas []string // Suggested practice problems or exercises
}

// PromptConfig defines data for the academic system prompt template.
type PromptConfig struct {
	Role           string
	Preamble       string
	Topic          string
	Mission        string   // Clear mission statement for the agent
	Objectives     []string // Specific objectives to achieve
	Specialties    string   // raw text for backward compatibility or simple agents
	Specialists    []Specialty
	Skills         []Skill
	StudentProfile *StudentProfile
	LearningPath   *LearningPath // Guidance for progressive learning
	ExtraRules     string
	Interactive    bool
}

// BaseSystemPromptTemplate uses Go template define blocks for modularity.
const BaseSystemPromptTemplate = `
{{define "Preamble"}}
# Role: {{.Role}}
{{.Preamble}}
{{- if .Interactive}}
You are a supportive and rigorous Secondary School Teacher. Your goal is to help the student learn and master the material through clear explanations and active verification. Balance extreme conciseness with pedagogical clarity.
{{- else}}
You are a non-interactive academic agent. Do your best to complete the task at hand and avoid asking user for any additional information.
{{- end}}
{{- end}}

{{define "Mission"}}
{{- if or .Mission .Objectives}}
# Mission & Objectives
{{- if .Mission}}
**Mission:** {{.Mission}}
{{- end}}
{{- if .Objectives}}
**Objectives:**
{{range .Objectives}}- {{.}}
{{end}}
{{- end}}
{{- end}}
{{- end}}

{{define "StudentProfile"}}
{{- if .StudentProfile}}
# Student Profile
- **Name:** {{.StudentProfile.Name}}
- **Learning Style:** {{.StudentProfile.LearningStyle}}
{{- if .StudentProfile.Struggles}}
- **Current Struggles:** {{range $i, $s := .StudentProfile.Struggles}}{{if $i}}, {{end}}{{$s}}{{end}}
{{- end}}
{{- if .StudentProfile.Strengths}}
- **Strengths:** {{range $i, $s := .StudentProfile.Strengths}}{{if $i}}, {{end}}{{$s}}{{end}}
{{- end}}
{{- end}}
{{- end}}



{{define "Instructions"}}
# Instructions
- **Active Teaching:** You are a teacher, not a search engine. You must frequently pause to VERIFY student understanding by administering short quizzes using the available tools.
- **Student Interaction:** You MUST use the appropriate tools for ANY interaction requiring student response. **STOP and use the tool.** You are PROHIBITED from asking questions directly in your text response.
{{- end}}

{{template "Preamble" .}}
{{template "Mission" .}}
{{template "StudentProfile" .}}
{{template "Instructions" .}}

{{if .Skills}}
# Available Agent Skills
You have access to the following specialized skills.
<available_skills>
{{range .Skills}}  <skill>
    <name>{{.Name}}</name>
    <description>{{.Description}}</description>
    <location>{{.Location}}</location>
  </skill>
{{end}}</available_skills>
{{end}}

{{if .Specialists}}
# Available Specialists
<specialists>
{{range .Specialists}}  <specialist>
    <name>{{.Name}}</name>
    <description>{{.Description}}</description>
  </specialist>
{{end}}</specialists>
{{end}}

{{if .Specialties}}
# Your Specialties
{{.Specialties}}
{{end}}

{{if .ExtraRules}}
# Additional Instructions
{{.ExtraRules}}
{{end}}


`

// GenerateSystemPrompt generates a system prompt from the template and configuration.
func GenerateSystemPrompt(config PromptConfig) (string, error) {
	tmpl, err := template.New("system_prompt").Parse(strings.TrimSpace(BaseSystemPromptTemplate))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// GenerateCompressionPrompt returns the XML-based snapshot prompt.
func GenerateCompressionPrompt() string {
	return strings.TrimSpace(`
<state_snapshot>
    <overall_learning_goal>
        <!-- A single, concise sentence describing the student's high-level academic objective. -->
    </overall_learning_goal>

    <key_academic_knowledge>
        <!-- Crucial facts, formulas, and student misconceptions identified. -->
    </key_academic_knowledge>

    <pedagogical_state>
        <!-- Current stage of explanation, tools used, and logical progress. -->
    </pedagogical_state>

    <current_plan>
        <!-- The agent's step-by-step plan. Mark completed steps. -->
    </current_plan>
</state_snapshot>
`)
}
