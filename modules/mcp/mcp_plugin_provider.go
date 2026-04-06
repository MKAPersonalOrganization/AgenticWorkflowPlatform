package mcp

import (
	"context"
	"encoding/json"

	"aicompass/foundation/agent"
	"aicompass/platform/toolregistry"

	"github.com/a-h/templ"
)

// MCPPluginProvider wraps a connected MCP server as a PluginProvider,
// making it appear in the plugin sidebar alongside built-in plugins.
type MCPPluginProvider struct {
	connector       MCPConnector
	discoveredTools []MCPDiscoveredTool
	service         *Service
}

// NewMCPPluginProvider creates a plugin provider for an MCP connector.
func NewMCPPluginProvider(service *Service, connector MCPConnector, tools []MCPDiscoveredTool) *MCPPluginProvider {
	return &MCPPluginProvider{
		connector:       connector,
		discoveredTools: tools,
		service:         service,
	}
}

// ID returns the unique identifier for this MCP plugin.
func (p *MCPPluginProvider) ID() string {
	return "mcp-" + p.connector.ID
}

// DefaultSets returns a single tool set containing all discovered tools.
func (p *MCPPluginProvider) DefaultSets() []toolregistry.ToolSet {
	toolIDs := make([]string, len(p.discoveredTools))
	for i, t := range p.discoveredTools {
		toolIDs[i] = "mcp-" + p.connector.ID + "-" + t.Name
	}

	name := p.connector.Title
	if p.connector.ServerName.Valid && p.connector.ServerName.String != "" {
		name = p.connector.ServerName.String
	}

	return []toolregistry.ToolSet{
		{
			ID:          "mcp-" + p.connector.ID,
			Name:        name,
			Description: "Tools from MCP server: " + p.connector.ServerURL,
			Category:    "MCP",
			ToolIDs:     toolIDs,
			Metadata: map[string]string{
				"type":       "mcp",
				"server_url": p.connector.ServerURL,
				"user_id":    p.connector.UserID,
			},
		},
	}
}

// DefaultTools returns tool registry entries for each discovered tool.
func (p *MCPPluginProvider) DefaultTools() []toolregistry.Tool {
	tools := make([]toolregistry.Tool, len(p.discoveredTools))
	for i, dt := range p.discoveredTools {
		tools[i] = toolregistry.Tool{
			ID:          "mcp-" + p.connector.ID + "-" + dt.Name,
			Name:        dt.Name,
			Description: dt.Description,
			Category:    "MCP",
			ToolSetID:   "mcp-" + p.connector.ID,
			Metadata: map[string]string{
				"mcp_connector_id": p.connector.ID,
				"mcp_server_url":   p.connector.ServerURL,
				"input_schema":     dt.InputSchema,
			},
		}
	}
	return tools
}

// CreateExecutable creates an executable tool by delegating to the MCP service.
func (p *MCPPluginProvider) CreateExecutable(_ context.Context, toolID string, _ toolregistry.FactoryDeps) (agent.ExecutableTool, error) {
	// Find the matching discovered tool.
	for _, dt := range p.discoveredTools {
		expectedID := "mcp-" + p.connector.ID + "-" + dt.Name
		if toolID == expectedID || toolID == dt.Name {
			return NewMCPRemoteTool(p.service, p.connector.ID, p.connector.UserID, dt.Name, dt.Description, dt.InputSchema), nil
		}
	}
	return nil, nil // Not handled by this provider
}

// GetUITemplate returns nil — MCP tools use the generic tool result UI.
func (p *MCPPluginProvider) GetUITemplate() templ.Component {
	return nil
}

// UIHandledTools returns an empty list — MCP tools use the default rendering.
func (p *MCPPluginProvider) UIHandledTools() []string {
	return nil
}

// ConnectorToPluginResponse converts an MCP connector into the plugin list format
// expected by the tools sidebar JS component.
func ConnectorToPluginResponse(connector MCPConnector, tools []MCPDiscoveredTool) map[string]any {
	name := connector.Title
	if connector.ServerName.Valid && connector.ServerName.String != "" {
		name = connector.ServerName.String
	}

	toolList := make([]map[string]any, len(tools))
	for i, t := range tools {
		var schema map[string]any
		if t.InputSchema != "" {
			json.Unmarshal([]byte(t.InputSchema), &schema)
		}

		toolList[i] = map[string]any{
			"id":           "mcp-" + connector.ID + "-" + t.Name,
			"name":         t.Name,
			"description":  t.Description,
			"category":     "MCP",
			"enabled":      true,
			"input_schema": schema,
		}
	}

	return map[string]any{
		"id":          "mcp-" + connector.ID,
		"name":        name,
		"description": "MCP Server: " + connector.ServerURL,
		"category":    "MCP",
		"enabled":     connector.Status == ConnectorStatusConnected,
		"tools":       toolList,
		"metadata": map[string]any{
			"type":           "mcp",
			"server_url":     connector.ServerURL,
			"status":         string(connector.Status),
			"error_msg":      connector.ErrorMsg,
			"connector_id":   connector.ID,
			"server_name":    connector.ServerName.String,
			"server_version": connector.ServerVersion.String,
		},
	}
}
