package a2a

import (
	"aicompass/foundation/agent"
	"aicompass/platform/toolregistry"
	"context"
	"encoding/json"

	"github.com/a-h/templ"
)

// A2APluginProvider wraps a connected A2A agent as a PluginProvider,
// making it appear in the plugin sidebar alongside built-in plugins and MCP servers.
//
// Following the ADK-Go agenttool pattern, each A2A agent is ONE tool —
// skills are metadata for the UI only, never individual tools.
type A2APluginProvider struct {
	connector        A2AConnector
	discoveredSkills []A2ADiscoveredSkill
	service          *Service
}

// NewA2APluginProvider creates a plugin provider for an A2A connector.
func NewA2APluginProvider(service *Service, connector A2AConnector, skills []A2ADiscoveredSkill) *A2APluginProvider {
	return &A2APluginProvider{
		connector:        connector,
		discoveredSkills: skills,
		service:          service,
	}
}

// ID returns the unique identifier for this A2A plugin.
func (p *A2APluginProvider) ID() string {
	return "a2a-" + p.connector.ID
}

func (p *A2APluginProvider) agentName() string {
	if p.connector.AgentName != "" {
		return p.connector.AgentName
	}
	return p.connector.Title
}

// DefaultSets returns a single tool set with a single tool (the agent itself).
func (p *A2APluginProvider) DefaultSets() []toolregistry.ToolSet {
	toolSetID := "a2a-" + p.connector.ID

	return []toolregistry.ToolSet{
		{
			ID:          toolSetID,
			Name:        p.agentName(),
			Description: "A2A Agent: " + p.connector.AgentURL,
			Category:    "A2A",
			ToolIDs:     []string{toolSetID}, // one tool = the agent
			Metadata: map[string]string{
				"type":      "a2a",
				"agent_url": p.connector.AgentURL,
				"user_id":   p.connector.UserID,
			},
		},
	}
}

// DefaultTools returns a SINGLE tool representing the entire A2A agent.
func (p *A2APluginProvider) DefaultTools() []toolregistry.Tool {
	toolSetID := "a2a-" + p.connector.ID
	return []toolregistry.Tool{
		{
			ID:          toolSetID, // same as set — one tool per agent
			Name:        p.agentName(),
			Description: p.connector.AgentDescription,
			Category:    "A2A",
			ToolSetID:   toolSetID,
			Metadata: map[string]string{
				"a2a_connector_id": p.connector.ID,
				"a2a_agent_url":    p.connector.AgentURL,
			},
		},
	}
}

// CreateExecutable creates the single agent-level executable tool.
func (p *A2APluginProvider) CreateExecutable(_ context.Context, toolID string, _ toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	toolSetID := "a2a-" + p.connector.ID
	if toolID == toolSetID {
		return NewA2ARemoteTool(p.service, p.connector, p.connector.UserID, p.discoveredSkills), nil
	}
	return nil, nil
}

// GetUITemplate returns nil — A2A tools use the generic tool result UI.
func (p *A2APluginProvider) GetUITemplate() templ.Component {
	return nil
}

// UIHandledTools returns an empty list — A2A tools use default rendering.
func (p *A2APluginProvider) UIHandledTools() []string {
	return nil
}

// ConnectorToPluginResponse converts an A2A connector into the plugin list format
// expected by the tools dialog JS component. Skills are listed as informational
// metadata only — they are NOT individual tools.
func ConnectorToPluginResponse(connector A2AConnector, skills []A2ADiscoveredSkill) map[string]any {
	name := connector.Title
	if connector.AgentName != "" {
		name = connector.AgentName
	}

	// Skills are exposed as informational display items in the UI,
	// but the entire agent is ONE tool with enable/disable at agent level.
	skillList := make([]map[string]any, len(skills))
	for i, s := range skills {
		var tags []string
		json.Unmarshal([]byte(s.Tags), &tags) //nolint:errcheck

		skillList[i] = map[string]any{
			"id":          s.SkillID,
			"name":        s.Name,
			"description": s.Description,
			"tags":        tags,
		}
	}

	return map[string]any{
		"id":          "a2a-" + connector.ID,
		"name":        name,
		"description": "A2A Agent: " + connector.AgentURL,
		"category":    "A2A",
		"enabled":     connector.Status == ConnectorStatusConnected,
		"skills":      skillList, // informational only — not tools
		"metadata": map[string]any{
			"type":              "a2a",
			"agent_url":         connector.AgentURL,
			"status":            string(connector.Status),
			"error_msg":         connector.ErrorMsg,
			"connector_id":      connector.ID,
			"agent_name":        connector.AgentName,
			"agent_description": connector.AgentDescription,
			"agent_version":     connector.AgentVersion,
			"icon_url":          connector.IconURL,
		},
	}
}
