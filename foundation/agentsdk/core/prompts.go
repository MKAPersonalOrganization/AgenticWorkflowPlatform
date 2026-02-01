package core

import (
	"bytes"
	"strings"
	"text/template"
)

// PromptConfig defines data for the system prompt template.
type PromptConfig struct {
	Role     string
	Preamble string
	// Context allows passing arbitrary data to the template.
	Context map[string]any
}

// BaseSystemPromptTemplate is a generic template.
const BaseSystemPromptTemplate = `
{{define "Preamble"}}
# Role: {{.Role}}
{{.Preamble}}
{{end}}

{{template "Preamble" .}}

{{if .Context}}
# Context
{{range $k, $v := .Context}}
## {{$k}}
{{$v}}
{{end}}
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
