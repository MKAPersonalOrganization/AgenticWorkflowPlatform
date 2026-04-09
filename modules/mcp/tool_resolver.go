package mcp

import (
	"aicompass/foundation/agent"
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPToolResolver implements agent.ToolResolver to provide tools from
// connected MCP servers for a specific user.
type MCPToolResolver struct {
	service *Service
	userID  string
}

// NewMCPToolResolver creates a tool resolver for a user's MCP connectors.
func NewMCPToolResolver(service *Service, userID string) *MCPToolResolver {
	return &MCPToolResolver{
		service: service,
		userID:  userID,
	}
}

// Resolve returns all tools from all connected MCP servers belonging to the user.
// Tool metadata comes from the DB cache; execution uses ephemeral sessions.
func (r *MCPToolResolver) Resolve(ctx context.Context) ([]agent.Tool, error) {
	connectors, err := r.service.ListConnectors(ctx, r.userID)
	if err != nil {
		return nil, fmt.Errorf("listing connectors: %w", err)
	}

	// Collect connected connector IDs for a single batch query.
	var connectedIDs []string
	for _, conn := range connectors {
		if conn.Status != ConnectorStatusConnected {
			continue
		}
		connectedIDs = append(connectedIDs, conn.ID)
	}

	if len(connectedIDs) == 0 {
		return nil, nil
	}

	// Single query fetches tools for ALL connected connectors — avoids N+1.
	allTools, err := r.service.repo.ListToolsByConnectors(ctx, connectedIDs)
	if err != nil {
		return nil, fmt.Errorf("batch listing tools: %w", err)
	}

	var tools []agent.Tool
	for _, connID := range connectedIDs {
		for _, dt := range allTools[connID] {
			tools = append(tools, NewMCPRemoteTool(r.service, connID, r.userID, dt.Name, dt.Description, dt.InputSchema))
		}
	}

	return tools, nil
}

// mcpRemoteTool wraps a discovered MCP tool as an agent.ExecutableTool.
// It holds a reference to the Service for ephemeral session-based execution.
type mcpRemoteTool struct {
	service     *Service
	connectorID string
	userID      string // owning user — passed to CallTool for tenant isolation
	name        string
	description string
	inputSchema string // JSON Schema string
}

// NewMCPRemoteTool creates an agent.ExecutableTool that delegates to a remote
// MCP server via the Service's ephemeral session mechanism.
func NewMCPRemoteTool(service *Service, connectorID, userID, name, description, inputSchema string) agent.ExecutableTool {
	return &mcpRemoteTool{
		service:     service,
		connectorID: connectorID,
		userID:      userID,
		name:        name,
		description: description,
		inputSchema: inputSchema,
	}
}

// Name returns the tool's unique identifier, prefixed to avoid collisions.
func (t *mcpRemoteTool) Name() string {
	return t.name
}

// Description returns the tool's description.
func (t *mcpRemoteTool) Description() string {
	return t.description
}

// Schema parses the JSON Schema into an agent.Schema.
func (t *mcpRemoteTool) Schema() *agent.Schema {
	if t.inputSchema == "" || t.inputSchema == "{}" {
		return &agent.Schema{
			Type:        agent.TypeObject,
			Description: t.description,
		}
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(t.inputSchema), &raw); err != nil {
		return &agent.Schema{
			Type:        agent.TypeObject,
			Description: t.description,
		}
	}

	return convertJSONSchemaToAgentSchema(raw)
}

// Execute calls the tool on the remote MCP server via an ephemeral session.
func (t *mcpRemoteTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var arguments map[string]any
	if len(args.Raw) > 0 {
		if err := json.Unmarshal(args.Raw, &arguments); err != nil {
			return agent.ErrorResult(fmt.Errorf("invalid arguments: %w", err)), nil
		}
	}

	result, err := t.service.CallTool(ctx, t.connectorID, t.userID, t.name, arguments)
	if err != nil {
		return agent.ErrorResult(fmt.Errorf("MCP tool call failed: %w", err)), nil
	}

	if result.IsError {
		// Extract error text from content.
		var errText string
		for _, c := range result.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				errText += tc.Text
			}
		}
		return agent.Result{
			Content: []any{errText},
			Error:   fmt.Errorf("tool returned error: %s", errText),
		}, nil
	}

	// Collect all text content from the result.
	var texts []string
	for _, c := range result.Content {
		switch v := c.(type) {
		case *mcp.TextContent:
			texts = append(texts, v.Text)
		default:
			b, _ := json.Marshal(c)
			texts = append(texts, string(b))
		}
	}

	content := make([]any, len(texts))
	for i, t := range texts {
		content[i] = t
	}

	return agent.Result{Content: content}, nil
}

// convertJSONSchemaToAgentSchema converts a raw JSON Schema map to agent.Schema.
func convertJSONSchemaToAgentSchema(raw map[string]any) *agent.Schema {
	schema := &agent.Schema{}

	if t, ok := raw["type"].(string); ok {
		schema.Type = agent.SchemaType(t)
	} else {
		schema.Type = agent.TypeObject
	}

	if d, ok := raw["description"].(string); ok {
		schema.Description = d
	}

	if props, ok := raw["properties"].(map[string]any); ok {
		schema.Properties = make(map[string]*agent.Schema, len(props))
		for name, propRaw := range props {
			if propMap, ok := propRaw.(map[string]any); ok {
				schema.Properties[name] = convertJSONSchemaToAgentSchema(propMap)
			}
		}
	}

	if req, ok := raw["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	}

	if items, ok := raw["items"].(map[string]any); ok {
		schema.Items = convertJSONSchemaToAgentSchema(items)
	}

	if enum, ok := raw["enum"].([]any); ok {
		for _, e := range enum {
			if s, ok := e.(string); ok {
				schema.Enum = append(schema.Enum, s)
			}
		}
	}

	return schema
}
