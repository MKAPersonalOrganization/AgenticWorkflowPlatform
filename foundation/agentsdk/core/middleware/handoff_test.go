package middleware

import (
	"context"
	"school-with-ai/foundation/agentsdk/core"
	"slices"
	"testing"
)

// Helper to create a test request context with proper agent config.
func newTestRequestContext() *core.RequestContext {
	config := &core.AgentConfig{
		Name: "test-agent",
	}

	return core.NewRequestContext("", nil, "", nil, config)
}

// Helper to create a test request context with handoffs configured.
func newTestRequestContextWithHandoffs(handoffs []core.Handoff) *core.RequestContext {
	config := &core.AgentConfig{
		Name:     "test-agent",
		Handoffs: handoffs,
	}

	return core.NewRequestContext("", nil, "", nil, config)
}

func TestNewHandoffMiddleware(t *testing.T) {
	t.Parallel()

	middleware := NewHandoffMiddleware()

	if middleware == nil {
		t.Fatal("Expected middleware to be created, got nil")
	}
}

func TestHandoffMiddleware_Process(t *testing.T) {
	t.Parallel()
	handoffs := []core.Handoff{
		{AgentName: "math", Description: "Mathematics specialist"},
		{AgentName: "physics", Description: "Physics specialist"},
	}

	middleware := NewHandoffMiddleware()
	reqCtx := newTestRequestContextWithHandoffs(handoffs)

	called := false
	next := func(ctx context.Context, rc *core.RequestContext) error {
		called = true

		return nil
	}

	err := middleware.Process(context.Background(), reqCtx, next)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !called {
		t.Error("Expected next handler to be called")
	}

	// Check that transfer_to_agent tool was added
	tools := reqCtx.GetEffectiveTools()
	found := false
	for _, tool := range tools {
		if tool.Name() == "transfer_to_agent" {
			found = true

			break
		}
	}

	if !found {
		t.Error("Expected transfer_to_agent tool to be added")
	}

	// Check that system prompt was added
	if len(reqCtx.SystemPromptAdditions) == 0 {
		t.Error("Expected system prompt additions")
	}
}

func TestTransferToAgentTool_Name(t *testing.T) {
	t.Parallel()
	tool := NewTransferToAgentTool([]core.Handoff{}, nil)

	if tool.Name() != "transfer_to_agent" {
		t.Errorf("Expected name 'transfer_to_agent', got %q", tool.Name())
	}
}

func TestTransferToAgentTool_Description(t *testing.T) {
	t.Parallel()
	handoffs := []core.Handoff{
		{AgentName: "math", Description: "Mathematics specialist for calculations"},
		{AgentName: "physics", Description: "Physics specialist for mechanics"},
	}

	tool := NewTransferToAgentTool(handoffs, nil)
	desc := tool.Description()

	if desc == "" {
		t.Error("Expected non-empty description")
	}

	// Check that available agents are listed
	if !contains(desc, "math") {
		t.Error("Expected description to contain 'math'")
	}

	if !contains(desc, "physics") {
		t.Error("Expected description to contain 'physics'")
	}

	if !contains(desc, "Mathematics specialist") {
		t.Error("Expected description to contain math description")
	}

	if !contains(desc, "Physics specialist") {
		t.Error("Expected description to contain physics description")
	}
}

func TestTransferToAgentTool_Parameters(t *testing.T) {
	t.Parallel()
	handoffs := []core.Handoff{
		{AgentName: "math", Description: "Mathematics specialist"},
		{AgentName: "physics", Description: "Physics specialist"},
	}

	tool := NewTransferToAgentTool(handoffs, nil)
	params := tool.Parameters()

	if params == nil {
		t.Fatal("Expected parameters to be non-nil")
	}

	// Check required fields
	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("Expected required field to be []string")
	}

	if len(required) != 1 {
		t.Errorf("Expected 1 required field, got %d", len(required))
	}

	if !containsString(required, "agent_name") {
		t.Error("Expected agent_name to be required")
	}

	// Check properties
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties to be map")
	}

	if _, exists := props["agent_name"]; !exists {
		t.Error("Expected agent_name property")
	}

	// Check agent_name has enum
	agentNameProp := props["agent_name"].(map[string]any)
	enum, ok := agentNameProp["enum"].([]string)
	if !ok {
		t.Fatal("Expected agent_name to have enum")
	}

	if len(enum) != 2 {
		t.Errorf("Expected 2 agents in enum, got %d", len(enum))
	}
}

func TestTransferToAgentTool_Invoke_Success(t *testing.T) {
	t.Parallel()
	handoffs := []core.Handoff{
		{AgentName: "math", Description: "Mathematics specialist"},
	}

	config := &core.AgentConfig{
		Name: "test-agent",
	}
	reqCtx := core.NewRequestContext("", nil, "", nil, config)
	tool := NewTransferToAgentTool(handoffs, reqCtx)

	args := map[string]any{
		"agent_name": "math",
	}

	result, err := tool.Invoke(core.NewToolContext(context.Background(), "", "", "", nil), args)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	tr, ok := result.(core.ToolResult)
	if !ok {
		t.Fatalf("Expected core.ToolResult result, got %T", result)
	}

	if tr.Command == nil || tr.Command.Goto != "math" || !tr.Command.StopIteration {
		t.Errorf("Expected Command with Goto: 'math' and StopIteration: true, got %+v", tr.Command)
	}

	// Check that handoff was stored in state
	if targetAgent, ok := reqCtx.State["handoff"].(string); !ok || targetAgent != "math" {
		t.Fatal("Expected handoff to be stored in state")
	}
}

func TestTransferToAgentTool_Invoke_MissingAgentName(t *testing.T) {
	t.Parallel()
	handoffs := []core.Handoff{
		{AgentName: "math", Description: "Mathematics specialist"},
	}

	reqCtx := newTestRequestContext()
	tool := NewTransferToAgentTool(handoffs, reqCtx)

	args := map[string]any{}

	_, err := tool.Invoke(core.NewToolContext(context.Background(), "", "", "", nil), args)

	if err == nil {
		t.Error("Expected error for missing agent_name")
	}

	if !contains(err.Error(), "agent_name") {
		t.Errorf("Expected error to mention agent_name, got %v", err)
	}
}

func TestTransferToAgentTool_Invoke_WithoutContext(t *testing.T) {
	t.Parallel()
	handoffs := []core.Handoff{
		{AgentName: "math", Description: "Mathematics specialist"},
	}

	reqCtx := newTestRequestContext()
	tool := NewTransferToAgentTool(handoffs, reqCtx)

	// Context is optional, should work without it
	args := map[string]any{
		"agent_name": "math",
	}

	_, err := tool.Invoke(core.NewToolContext(context.Background(), "", "", "", nil), args)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	// Check that handoff was stored with empty context
	if targetAgent, ok := reqCtx.State["handoff"].(string); !ok || targetAgent != "math" {
		t.Fatal("Expected handoff to be stored in state")
	}
}

func TestTransferToAgentTool_Invoke_InvalidJSON(t *testing.T) {
	t.Parallel()
	handoffs := []core.Handoff{
		{AgentName: "math", Description: "Mathematics specialist"},
	}

	reqCtx := newTestRequestContext()
	tool := NewTransferToAgentTool(handoffs, reqCtx)

	_, err := tool.Invoke(core.NewToolContext(context.Background(), "", "", "", nil), "invalid json")

	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestHandoffMiddleware_MultipleAgents(t *testing.T) {
	t.Parallel()
	handoffs := []core.Handoff{
		{AgentName: "math", Description: "Mathematics specialist"},
		{AgentName: "physics", Description: "Physics specialist"},
		{AgentName: "chemistry", Description: "Chemistry specialist"},
		{AgentName: "linearalgebra", Description: "Linear algebra specialist"},
	}

	middleware := NewHandoffMiddleware()
	reqCtx := newTestRequestContextWithHandoffs(handoffs)

	next := func(ctx context.Context, rc *core.RequestContext) error {
		return nil
	}

	err := middleware.Process(context.Background(), reqCtx, next)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Get the tool and check its parameters
	tools := reqCtx.GetEffectiveTools()
	var transferTool core.Tool
	for _, tool := range tools {
		if tool.Name() == "transfer_to_agent" {
			transferTool = tool

			break
		}
	}

	if transferTool == nil {
		t.Fatal("Expected transfer_to_agent tool to be added")
	}

	params := transferTool.Parameters()
	props := params["properties"].(map[string]any)
	agentNameProp := props["agent_name"].(map[string]any)
	enum := agentNameProp["enum"].([]string)

	if len(enum) != 4 {
		t.Errorf("Expected 4 agents in enum, got %d", len(enum))
	}

	// Check all agents are in enum
	for _, handoff := range handoffs {
		if !containsString(enum, handoff.AgentName) {
			t.Errorf("Expected %q to be in agent enum", handoff.AgentName)
		}
	}
}

// Helper functions

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

func containsString(slice []string, s string) bool {
	return slices.Contains(slice, s)
}
