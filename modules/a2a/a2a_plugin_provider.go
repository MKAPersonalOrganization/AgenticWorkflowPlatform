package a2a

import (
	"context"
	"encoding/json"

	"school-with-ai/foundation/agent"
	"school-with-ai/platform/toolregistry"

	"github.com/a-h/templ"
)

// A2APluginProvider wraps a connected A2A agent as a PluginProvider,
// making it appear in the plugin sidebar alongside built-in plugins and MCP servers.
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

// DefaultSets returns a single tool set containing all discovered skills.
func (p *A2APluginProvider) DefaultSets() []toolregistry.ToolSet {
	toolIDs := make([]string, len(p.discoveredSkills))
	for i, s := range p.discoveredSkills {
		toolIDs[i] = "a2a-" + p.connector.ID + "-" + s.SkillID
	}

	name := p.connector.Title
	if p.connector.AgentName != "" {
		name = p.connector.AgentName
	}

	return []toolregistry.ToolSet{
		{
			ID:          "a2a-" + p.connector.ID,
			Name:        name,
			Description: "Skills from A2A agent: " + p.connector.AgentURL,
			Category:    "A2A",
			ToolIDs:     toolIDs,
			Metadata: map[string]string{
				"type":      "a2a",
				"agent_url": p.connector.AgentURL,
				"user_id":   p.connector.UserID,
			},
		},
	}
}

// DefaultTools returns tool registry entries for each discovered skill.
func (p *A2APluginProvider) DefaultTools() []toolregistry.Tool {
	tools := make([]toolregistry.Tool, len(p.discoveredSkills))
	for i, s := range p.discoveredSkills {
		tools[i] = toolregistry.Tool{
			ID:          "a2a-" + p.connector.ID + "-" + s.SkillID,
			Name:        s.Name,
			Description: s.Description,
			Category:    "A2A",
			ToolSetID:   "a2a-" + p.connector.ID,
			Metadata: map[string]string{
				"a2a_connector_id": p.connector.ID,
				"a2a_agent_url":    p.connector.AgentURL,
				"skill_id":         s.SkillID,
				"tags":             s.Tags,
			},
		}
	}
	return tools
}

// CreateExecutable creates an executable tool by delegating to the A2A service.
func (p *A2APluginProvider) CreateExecutable(_ context.Context, toolID string, _ toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	for _, s := range p.discoveredSkills {
		expectedID := "a2a-" + p.connector.ID + "-" + s.SkillID
		if toolID == expectedID || toolID == s.SkillID {
			return NewA2ARemoteTool(p.service, p.connector.ID, p.connector.UserID, s), nil
		}
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
// expected by the tools dialog JS component.
func ConnectorToPluginResponse(connector A2AConnector, skills []A2ADiscoveredSkill) map[string]any {
	name := connector.Title
	if connector.AgentName != "" {
		name = connector.AgentName
	}

	skillList := make([]map[string]any, len(skills))
	for i, s := range skills {
		var tags []string
		json.Unmarshal([]byte(s.Tags), &tags) //nolint:errcheck

		skillList[i] = map[string]any{
			"id":          "a2a-" + connector.ID + "-" + s.SkillID,
			"name":        s.Name,
			"description": s.Description,
			"category":    "A2A",
			"enabled":     true,
			"tags":        tags,
		}
	}

	return map[string]any{
		"id":          "a2a-" + connector.ID,
		"name":        name,
		"description": "A2A Agent: " + connector.AgentURL,
		"category":    "A2A",
		"enabled":     connector.Status == ConnectorStatusConnected,
		"tools":       skillList,
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
