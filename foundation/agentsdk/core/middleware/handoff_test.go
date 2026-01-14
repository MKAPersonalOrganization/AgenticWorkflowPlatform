package middleware

import (
	"context"
	"encoding/json"
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

func TestNewHandoffMiddleware(t *testing.T) {
	t.Parallel()
	agents := map[string]string{
		"math":    "Mathematics specialist",
		"physics": "Physics specialist",
	}

	middleware := NewHandoffMiddleware(agents)

	if middleware == nil {
		t.Fatal("Expected middleware to be created, got nil")
	}

	if len(middleware.availableAgents) != 2 {
		t.Errorf("Expected 2 agents, got %d", len(middleware.availableAgents))
	}

	if middleware.systemPrompt == "" {
		t.Error("Expected system prompt to be set")
	}
}

func TestNewHandoffMiddlewareWithPrompt(t *testing.T) {
	t.Parallel()
	agents := map[string]string{
		"math": "Mathematics specialist",
	}
	customPrompt := "Custom handoff prompt"

	middleware := NewHandoffMiddlewareWithPrompt(agents, customPrompt)

	if middleware == nil {
		t.Fatal("Expected middleware to be created, got nil")
	}

	if middleware.systemPrompt != customPrompt {
		t.Errorf("Expected custom prompt %q, got %q", customPrompt, middleware.systemPrompt)
	}
}

func TestHandoffMiddleware_Process(t *testing.T) {
	t.Parallel()
	agents := map[string]string{
		"math":    "Mathematics specialist",
		"physics": "Physics specialist",
	}

	middleware := NewHandoffMiddleware(agents)
	reqCtx := newTestRequestContext()

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
	tool := NewTransferToAgentTool(map[string]string{}, nil)

	if tool.Name() != "transfer_to_agent" {
		t.Errorf("Expected name 'transfer_to_agent', got %q", tool.Name())
	}
}

func TestTransferToAgentTool_Description(t *testing.T) {
	t.Parallel()
	agents := map[string]string{
		"math":    "Mathematics specialist for calculations",
		"physics": "Physics specialist for mechanics",
	}

	tool := NewTransferToAgentTool(agents, nil)
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
	agents := map[string]string{
		"math":    "Mathematics specialist",
		"physics": "Physics specialist",
	}

	tool := NewTransferToAgentTool(agents, nil)
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

	// question is optional for return handoffs
	if containsString(required, "question") {
		t.Error("Expected question to be optional (not required)")
	}

	// Check properties
	props, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatal("Expected properties to be map")
	}

	if _, exists := props["agent_name"]; !exists {
		t.Error("Expected agent_name property")
	}

	if _, exists := props["question"]; !exists {
		t.Error("Expected question property")
	}

	if _, exists := props["context"]; !exists {
		t.Error("Expected context property")
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
	agents := map[string]string{
		"math": "Mathematics specialist",
	}

	config := &core.AgentConfig{
		Name: "test-agent",
	}
	reqCtx := core.NewRequestContext("", nil, "", nil, config)
	tool := NewTransferToAgentTool(agents, reqCtx)

	args := map[string]any{
		"agent_name": "math",
		"question":   "What is 2 + 2?",
		"context":    "Basic arithmetic question",
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Result should be a Command
	cmd, isCmd := result.Command, result.Command != nil
	if !isCmd {
		t.Fatal("Expected result to be a Command")
	}

	if cmd.Goto != "math" {
		t.Errorf("Expected Command.Goto to be 'math', got %q", cmd.Goto)
	}

	// Check that handoff was stored in state
	handoff, ok := GetHandoff(reqCtx)
	if !ok {
		t.Fatal("Expected handoff to be stored in state")
	}

	if handoff.TargetAgent != "math" {
		t.Errorf("Expected target agent 'math', got %q", handoff.TargetAgent)
	}

	if handoff.Question != "What is 2 + 2?" {
		t.Errorf("Expected question 'What is 2 + 2?', got %q", handoff.Question)
	}

	if handoff.Context != "Basic arithmetic question" {
		t.Errorf("Expected context 'Basic arithmetic question', got %q", handoff.Context)
	}
}

func TestTransferToAgentTool_Invoke_MissingAgentName(t *testing.T) {
	t.Parallel()
	agents := map[string]string{
		"math": "Mathematics specialist",
	}

	reqCtx := newTestRequestContext()
	tool := NewTransferToAgentTool(agents, reqCtx)

	args := map[string]any{
		"question": "What is 2 + 2?",
	}

	argsJSON, _ := json.Marshal(args)
	_, err := tool.Invoke(context.Background(), string(argsJSON))

	if err == nil {
		t.Error("Expected error for missing agent_name")
	}

	if !contains(err.Error(), "agent_name") {
		t.Errorf("Expected error to mention agent_name, got %v", err)
	}
}

func TestTransferToAgentTool_Invoke_MissingQuestion(t *testing.T) {
	t.Parallel()
	agents := map[string]string{
		"math": "Mathematics specialist",
	}

	reqCtx := newTestRequestContext()
	tool := NewTransferToAgentTool(agents, reqCtx)

	args := map[string]any{
		"agent_name": "math",
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	// question is optional, so this should succeed
	if err != nil {
		t.Errorf("Expected no error for missing question (it's optional), got %v", err)
	}

	// Result should be a Command
	cmd, isCmd := result.Command, result.Command != nil
	if !isCmd {
		t.Fatal("Expected result to be a Command")
	}

	if cmd.Goto != "math" {
		t.Errorf("Expected Command.Goto to be 'math', got %q", cmd.Goto)
	}

	// Verify handoff was stored
	handoff, ok := GetHandoff(reqCtx)
	if !ok || handoff == nil {
		t.Fatal("Expected handoff to be stored")
	}

	if handoff.TargetAgent != "math" {
		t.Errorf("Expected ToAgent to be math, got %s", handoff.TargetAgent)
	}

	if handoff.Question != "" {
		t.Errorf("Expected Question to be empty, got %s", handoff.Question)
	}
}

func TestTransferToAgentTool_Invoke_UnknownAgent(t *testing.T) {
	t.Parallel()
	agents := map[string]string{
		"math": "Mathematics specialist",
	}

	reqCtx := newTestRequestContext()
	tool := NewTransferToAgentTool(agents, reqCtx)

	args := map[string]any{
		"agent_name": "biology", // Not in available agents
		"question":   "What is photosynthesis?",
	}

	argsJSON, _ := json.Marshal(args)
	_, err := tool.Invoke(context.Background(), string(argsJSON))

	if err == nil {
		t.Error("Expected error for unknown agent")
	}

	if !contains(err.Error(), "unknown agent") {
		t.Errorf("Expected error to mention unknown agent, got %v", err)
	}

	if !contains(err.Error(), "biology") {
		t.Errorf("Expected error to mention 'biology', got %v", err)
	}
}

func TestTransferToAgentTool_Invoke_WithoutContext(t *testing.T) {
	t.Parallel()
	agents := map[string]string{
		"math": "Mathematics specialist",
	}

	reqCtx := newTestRequestContext()
	tool := NewTransferToAgentTool(agents, reqCtx)

	// Context is optional, should work without it
	args := map[string]any{
		"agent_name": "math",
		"question":   "What is 2 + 2?",
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Result should be a Command
	_, isCmd := result.Command, result.Command != nil
	if !isCmd {
		t.Fatal("Expected result to be a Command")
	}

	// Check that handoff was stored with empty context
	handoff, ok := GetHandoff(reqCtx)
	if !ok {
		t.Fatal("Expected handoff to be stored in state")
	}

	if handoff.Context != "" {
		t.Errorf("Expected empty context, got %q", handoff.Context)
	}
}

func TestTransferToAgentTool_Invoke_InvalidJSON(t *testing.T) {
	t.Parallel()
	agents := map[string]string{
		"math": "Mathematics specialist",
	}

	reqCtx := newTestRequestContext()
	tool := NewTransferToAgentTool(agents, reqCtx)

	_, err := tool.Invoke(context.Background(), "invalid json")

	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestGetHandoff_NotSet(t *testing.T) {
	t.Parallel()
	reqCtx := newTestRequestContext()

	handoff, ok := GetHandoff(reqCtx)

	if ok {
		t.Error("Expected GetHandoff to return false when not set")
	}

	if handoff != nil {
		t.Error("Expected handoff to be nil when not set")
	}
}

func TestSetHandoff_AndGet(t *testing.T) {
	t.Parallel()
	reqCtx := newTestRequestContext()

	expectedHandoff := Handoff{
		TargetAgent: "math",
		Question:    "Calculate 2 + 2",
		Context:     "Basic arithmetic",
	}

	SetHandoff(reqCtx, expectedHandoff)

	handoff, ok := GetHandoff(reqCtx)

	if !ok {
		t.Fatal("Expected GetHandoff to return true after SetHandoff")
	}

	if handoff == nil {
		t.Fatal("Expected handoff to be non-nil")
	}

	if handoff.TargetAgent != expectedHandoff.TargetAgent {
		t.Errorf("Expected TargetAgent %q, got %q", expectedHandoff.TargetAgent, handoff.TargetAgent)
	}

	if handoff.Question != expectedHandoff.Question {
		t.Errorf("Expected Question %q, got %q", expectedHandoff.Question, handoff.Question)
	}

	if handoff.Context != expectedHandoff.Context {
		t.Errorf("Expected Context %q, got %q", expectedHandoff.Context, handoff.Context)
	}
}

func TestClearHandoff(t *testing.T) {
	t.Parallel()
	reqCtx := newTestRequestContext()

	// Set handoff
	SetHandoff(reqCtx, Handoff{
		TargetAgent: "math",
		Question:    "Test",
	})

	// Verify it's set
	if !HasHandoff(reqCtx) {
		t.Fatal("Expected handoff to be set")
	}

	// Clear it
	ClearHandoff(reqCtx)

	// Verify it's cleared
	if HasHandoff(reqCtx) {
		t.Error("Expected handoff to be cleared")
	}

	_, ok := GetHandoff(reqCtx)
	if ok {
		t.Error("Expected GetHandoff to return false after clear")
	}
}

func TestHasHandoff(t *testing.T) {
	t.Parallel()
	reqCtx := newTestRequestContext()

	// Initially should not have handoff
	if HasHandoff(reqCtx) {
		t.Error("Expected HasHandoff to return false initially")
	}

	// After setting should have handoff
	SetHandoff(reqCtx, Handoff{TargetAgent: "math", Question: "Test"})

	if !HasHandoff(reqCtx) {
		t.Error("Expected HasHandoff to return true after setting")
	}

	// After clearing should not have handoff
	ClearHandoff(reqCtx)

	if HasHandoff(reqCtx) {
		t.Error("Expected HasHandoff to return false after clearing")
	}
}

func TestBuildSystemPromptWithAgents(t *testing.T) {
	t.Parallel()
	agents := map[string]string{
		"math":    "Mathematics specialist",
		"physics": "Physics specialist for mechanics",
	}

	middleware := NewHandoffMiddleware(agents)
	prompt := middleware.buildSystemPromptWithAgents()

	if prompt == "" {
		t.Error("Expected non-empty system prompt")
	}

	// Should contain base prompt
	if !contains(prompt, "transfer_to_agent") {
		t.Error("Expected prompt to mention transfer_to_agent tool")
	}

	// Should contain available agents
	if !contains(prompt, "math") {
		t.Error("Expected prompt to contain 'math'")
	}

	if !contains(prompt, "physics") {
		t.Error("Expected prompt to contain 'physics'")
	}

	// Should contain descriptions
	if !contains(prompt, "Mathematics specialist") {
		t.Error("Expected prompt to contain math description")
	}

	if !contains(prompt, "Physics specialist") {
		t.Error("Expected prompt to contain physics description")
	}
}

func TestHandoffMiddleware_MultipleAgents(t *testing.T) {
	t.Parallel()
	agents := map[string]string{
		"math":          "Mathematics specialist",
		"physics":       "Physics specialist",
		"chemistry":     "Chemistry specialist",
		"linearalgebra": "Linear algebra specialist",
	}

	middleware := NewHandoffMiddleware(agents)
	reqCtx := newTestRequestContext()

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
	for agent := range agents {
		if !containsString(enum, agent) {
			t.Errorf("Expected %q to be in agent enum", agent)
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
