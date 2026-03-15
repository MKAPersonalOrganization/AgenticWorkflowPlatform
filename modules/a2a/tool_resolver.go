package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"school-with-ai/foundation/agent"
)

// A2ARemoteTool wraps an entire A2A agent as a single agent.ExecutableTool.
//
// Following the ADK-Go pattern (agenttool), a remote A2A agent is exposed as
// ONE tool — not one per skill. The LLM sends it a natural-language message
// and the remote agent decides which internal skill(s) to use. Enable/disable
// is at the agent (connector) level, never per-skill.
type A2ARemoteTool struct {
	service   *Service
	connector A2AConnector
	userID    string
	// skills are embedded in the description so the LLM knows what the agent
	// is capable of, but they are NOT individual tools.
	skills []A2ADiscoveredSkill
}

// NewA2ARemoteTool creates an agent.ExecutableTool that represents one remote
// A2A agent. The entire agent is a single callable tool.
func NewA2ARemoteTool(service *Service, connector A2AConnector, userID string, skills []A2ADiscoveredSkill) agent.ExecutableTool {
	return &A2ARemoteTool{
		service:   service,
		connector: connector,
		userID:    userID,
		skills:    skills,
	}
}

// Name returns a stable tool name derived from the connector ID.
func (t *A2ARemoteTool) Name() string {
	return "a2a_" + strings.ReplaceAll(t.connector.ID, "-", "_")
}

// Description returns a rich description that includes the agent's
// capabilities and discovered skills so the LLM can decide when to use it.
func (t *A2ARemoteTool) Description() string {
	name := t.connector.Title
	if t.connector.AgentName != "" {
		name = t.connector.AgentName
	}

	desc := t.connector.AgentDescription
	if desc == "" {
		desc = fmt.Sprintf("Remote A2A agent: %s", name)
	}

	if len(t.skills) > 0 {
		var sb strings.Builder
		sb.WriteString(desc)
		sb.WriteString("\n\nAvailable skills:")
		for _, s := range t.skills {
			sb.WriteString("\n- ")
			sb.WriteString(s.Name)
			if s.Description != "" {
				sb.WriteString(": ")
				sb.WriteString(s.Description)
			}
		}
		return sb.String()
	}
	return desc
}

// Schema returns a schema with a single "request" string parameter,
// matching ADK-Go's agenttool pattern.
func (t *A2ARemoteTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type:        agent.TypeObject,
		Description: t.Description(),
		Properties: map[string]*agent.Schema{
			"request": {
				Type:        agent.TypeString,
				Description: "The request to send to the remote A2A agent. Describe what you need in natural language.",
			},
		},
		Required: []string{"request"},
	}
}

// Execute sends a message to the remote A2A agent and returns the response.
func (t *A2ARemoteTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var params struct {
		Request string `json:"request"`
	}
	if len(args.Raw) > 0 {
		if err := json.Unmarshal(args.Raw, &params); err != nil {
			return agent.ErrorResult(fmt.Errorf("invalid arguments: %w", err)), nil
		}
	}

	if params.Request == "" {
		return agent.ErrorResult(fmt.Errorf("request parameter is required")), nil
	}

	response, err := t.service.SendMessage(ctx, t.connector.ID, t.userID, params.Request)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("A2A agent call failed: %w", err)), nil
	}

	if response == "" {
		response = "(Remote agent returned an empty response)"
	}

	return agent.Result{Content: []any{response}}, nil
}
