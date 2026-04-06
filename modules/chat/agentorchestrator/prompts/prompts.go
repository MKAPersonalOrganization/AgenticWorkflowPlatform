package prompts

import (
	"bytes"
	"strings"
	"text/template"

	"aicompass/modules/chat/chatcontracts"
)

// Specialty defines a domain specialist for the coordinator agent.
type Specialty struct {
	Name        string
	Description string
}

// ProgressionPath defines related concepts and resources for progressive guidance.
type ProgressionPath struct {
	Prerequisites []string // Concepts the user should know first
	NextConcepts  []string // Logical next topics to explore
	RelatedTopics []string // Cross-domain connections
	PracticeAreas []string // Suggested practice problems or exercises
}

// AgentPromptConfig defines data for the agent system prompt template.
type AgentPromptConfig struct {
	Role            string
	Preamble        string
	Topic           string
	Mission         string   // Clear mission statement for the agent
	Objectives      []string // Specific objectives to achieve
	Specialists     []Specialty
	UserProfile     *chatcontracts.UserProfile
	ProgressionPath *ProgressionPath // Guidance for progressive exploration
	ExtraRules      string
	Interactive     bool
}

// AgentSystemPromptTemplate uses Go template define blocks for modularity.
const AgentSystemPromptTemplate = `
{{define "Preamble"}}
# Role: {{.Role}}
{{.Preamble}}
{{- if .Interactive}}
You are a knowledgeable and supportive AI Assistant. Your goal is to help the user understand and master the material through clear explanations and active verification. Balance conciseness with clarity.
{{- else}}
You are a non-interactive AI agent. Do your best to complete the task at hand and avoid asking user for any additional information.
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

{{define "UserProfile"}}
{{- if .UserProfile}}
# User Profile
- **Name:** {{.UserProfile.UserID}}
- **Preferred Style:** {{.UserProfile.LearningStyle}}
{{- if .UserProfile.Struggles}}
- **Current Challenges:** {{range $i, $s := .UserProfile.Struggles}}{{if $i}}, {{end}}{{$s}}{{end}}
{{- end}}
{{- if .UserProfile.Strengths}}
- **Strengths:** {{range $i, $s := .UserProfile.Strengths}}{{if $i}}, {{end}}{{$s}}{{end}}
{{- end}}
{{- end}}
{{- end}}

{{define "ProgressionPath"}}
{{- if .ProgressionPath}}
# Progression Path
{{- if .ProgressionPath.Prerequisites}}
**Prerequisites:** {{range $i, $s := .ProgressionPath.Prerequisites}}{{if $i}}, {{end}}{{$s}}{{end}}
{{- end}}
{{- if .ProgressionPath.NextConcepts}}
**Next Concepts:** {{range $i, $s := .ProgressionPath.NextConcepts}}{{if $i}}, {{end}}{{$s}}{{end}}
{{- end}}
{{- end}}
{{- end}}

{{define "Instructions"}}
# Instructions
- **Active Guidance:** You are an expert assistant, not a search engine. You must frequently pause to VERIFY user understanding by administering short quizzes using the available tools.
- **User Interaction:** You MUST use the appropriate tools for ANY interaction requiring user response. **STOP and use the tool.** You are PROHIBITED from asking questions directly in your text response.
{{- end}}

{{template "Preamble" .}}
{{template "Mission" .}}
{{template "UserProfile" .}}
{{template "ProgressionPath" .}}
{{template "Instructions" .}}

{{if .Specialists}}
# Available Specialists
<specialists>
{{range .Specialists}}  <specialist>
    <name>{{.Name}}</name>
    <description>{{.Description}}</description>
  </specialist>
{{end}}</specialists>
{{end}}

{{if .ExtraRules}}
# Additional Instructions
{{.ExtraRules}}
{{end}}
`

// GenerateAgentSystemPrompt generates a system prompt from the agent template.
func GenerateAgentSystemPrompt(config AgentPromptConfig) (string, error) {
	tmpl, err := template.New("agent_system_prompt").Parse(strings.TrimSpace(AgentSystemPromptTemplate))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return "", err
	}

	return buf.String(), nil
}
