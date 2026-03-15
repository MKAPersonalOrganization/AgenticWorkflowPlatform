package a2a

import (
	"context"
	"encoding/json"
	"fmt"

	"school-with-ai/foundation/agent"
)

// A2ARemoteTool wraps a discovered A2A agent skill as an agent.ExecutableTool.
// It holds a reference to the Service for remote task execution.
type A2ARemoteTool struct {
	service     *Service
	connectorID string
	userID      string
	skill       A2ADiscoveredSkill
}

// NewA2ARemoteTool creates an agent.ExecutableTool that delegates to a remote
// A2A agent via the Service's SendMessage mechanism.
func NewA2ARemoteTool(service *Service, connectorID, userID string, skill A2ADiscoveredSkill) agent.ExecutableTool {
	return &A2ARemoteTool{
		service:     service,
		connectorID: connectorID,
		userID:      userID,
		skill:       skill,
	}
}

// Name returns the tool's unique identifier, prefixed with a2a to avoid collisions.
func (t *A2ARemoteTool) Name() string {
	return "a2a_" + t.skill.SkillID
}

// Description returns the tool's description from the agent card skill.
func (t *A2ARemoteTool) Description() string {
	desc := t.skill.Description
	if desc == "" {
		desc = fmt.Sprintf("Remote A2A agent skill: %s", t.skill.Name)
	}
	return desc
}

// Schema returns a generic schema accepting a "message" string parameter.
// A2A skills don't have JSON Schema — they accept natural language messages.
func (t *A2ARemoteTool) Schema() *agent.Schema {
	return &agent.Schema{
		Type:        agent.TypeObject,
		Description: t.Description(),
		Properties: map[string]*agent.Schema{
			"message": {
				Type:        agent.TypeString,
				Description: "The message to send to the remote A2A agent. Describe what you need in natural language.",
			},
		},
		Required: []string{"message"},
	}
}

// Execute sends a message to the remote A2A agent and returns the response.
func (t *A2ARemoteTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var params struct {
		Message string `json:"message"`
	}
	if len(args.Raw) > 0 {
		if err := json.Unmarshal(args.Raw, &params); err != nil {
			return agent.ErrorResult(fmt.Errorf("invalid arguments: %w", err)), nil
		}
	}

	if params.Message == "" {
		return agent.ErrorResult(fmt.Errorf("message parameter is required")), nil
	}

	response, err := t.service.SendMessage(ctx, t.connectorID, t.userID, params.Message)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("A2A agent call failed: %w", err)), nil
	}

	if response == "" {
		response = "(Remote agent returned an empty response)"
	}

	return agent.Result{Content: []any{response}}, nil
}
