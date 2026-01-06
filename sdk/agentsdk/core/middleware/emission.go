package middleware

import (
	"context"
	"encoding/json"
	"fmt"

	"school-with-ai/sdk/agentsdk/core"
)

// EmissionToolDescription is the description for emission tools
const EmissionToolDescription = `Emit a question to a specialist instructor. Use this when you need expertise from another subject area.

This is a fire-and-forget event - the specialist will automatically receive and process it.

Available specialists:
- emit_math_question: Mathematical calculations, equations, algebra
- emit_physics_question: Forces, motion, energy, mechanics
- emit_chemistry_question: Reactions, molecules, compounds
- emit_linearalgebra_question: Matrix operations, vectors, eigenvalues`

const EmissionSystemPrompt = `## Specialist Delegation

You have access to emission tools to delegate questions to other specialist instructors:
- emit_math_question
- emit_physics_question  
- emit_chemistry_question
- emit_linearalgebra_question

Use these when you need expertise from another subject. After emitting, your work is done - the specialist will handle it.`

// EmissionTool is a generic tool for emitting questions to specialists
type EmissionTool struct {
	name        string
	subject     string
	description string
	reqCtx      *core.RequestContext
}

// NewEmissionTool creates a new emission tool
func NewEmissionTool(name, subject string, reqCtx *core.RequestContext) *EmissionTool {
	return &EmissionTool{
		name:    name,
		subject: subject,
		description: fmt.Sprintf(
			"Emit a %s question event. This is a fire-and-forget event - "+
				"agents subscribed to %s events will automatically receive and process it. "+
				"Use this when the question requires %s expertise.",
			subject, subject, subject,
		),
		reqCtx: reqCtx,
	}
}

func (t *EmissionTool) Name() string {
	return t.name
}

func (t *EmissionTool) Description() string {
	return t.description
}

func (t *EmissionTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"question": map[string]interface{}{
				"type":        "string",
				"description": fmt.Sprintf("The %s question or request that needs to be addressed", t.subject),
			},
			"context": map[string]interface{}{
				"type":        "string",
				"description": "Optional context or additional information to help the receiving agent",
			},
		},
		"required": []string{"question"},
	}
}

func (t *EmissionTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var params struct {
		Question string `json:"question"`
		Context  string `json:"context"`
	}
	if err := json.Unmarshal([]byte(arguments), &params); err != nil {
		return "", fmt.Errorf("failed to parse arguments: %w", err)
	}

	// Store emission request in state for later processing
	emissions := GetEmissions(t.reqCtx)
	emissions = append(emissions, Emission{
		ToolName: t.name,
		Subject:  t.subject,
		Question: params.Question,
		Context:  params.Context,
	})
	SetEmissions(t.reqCtx, emissions)

	return fmt.Sprintf("%s event emitted. Question: %s", t.subject, params.Question), nil
}

// Emission represents an emitted question
type Emission struct {
	ToolName string
	Subject  string
	Question string
	Context  string
}

// EmissionMiddleware provides emission tools to agents
type EmissionMiddleware struct {
	systemPrompt string
	tools        []string // List of emission tool names to add
}

// NewEmissionMiddleware creates a new EmissionMiddleware with all emission tools
func NewEmissionMiddleware() *EmissionMiddleware {
	return &EmissionMiddleware{
		systemPrompt: EmissionSystemPrompt,
		tools: []string{
			"emit_math_question",
			"emit_physics_question",
			"emit_chemistry_question",
			"emit_linearalgebra_question",
		},
	}
}

// NewEmissionMiddlewareWithTools creates a new EmissionMiddleware with specific tools
func NewEmissionMiddlewareWithTools(toolNames []string) *EmissionMiddleware {
	return &EmissionMiddleware{
		systemPrompt: EmissionSystemPrompt,
		tools:        toolNames,
	}
}

// Process implements the Middleware interface
func (m *EmissionMiddleware) Process(ctx context.Context, reqCtx *core.RequestContext, next core.NextHandler) error {
	// Map of tool names to subjects
	toolSubjects := map[string]string{
		"emit_math_question":          "math",
		"emit_physics_question":       "physics",
		"emit_chemistry_question":     "chemistry",
		"emit_linearalgebra_question": "linear algebra",
	}

	// Add emission tools
	for _, toolName := range m.tools {
		if subject, ok := toolSubjects[toolName]; ok {
			reqCtx.AddTool(NewEmissionTool(toolName, subject, reqCtx))
		}
	}

	reqCtx.AddSystemPrompt(m.systemPrompt)

	// Initialize emissions state if not present
	if _, exists := reqCtx.State["emissions"]; !exists {
		reqCtx.State["emissions"] = []Emission{}
	}

	return next(ctx, reqCtx)
}

// GetEmissions retrieves emissions from the request context
func GetEmissions(reqCtx *core.RequestContext) []Emission {
	if emissions, ok := reqCtx.State["emissions"].([]Emission); ok {
		return emissions
	}
	return []Emission{}
}

// SetEmissions updates emissions in the request context
func SetEmissions(reqCtx *core.RequestContext, emissions []Emission) {
	reqCtx.State["emissions"] = emissions
}

// ClearEmissions clears emissions from the request context
func ClearEmissions(reqCtx *core.RequestContext) {
	reqCtx.State["emissions"] = []Emission{}
}
