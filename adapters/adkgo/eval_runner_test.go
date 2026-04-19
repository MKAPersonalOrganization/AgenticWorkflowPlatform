package adkgo

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	fagent "aicompass/foundation/agent"
	"aicompass/modules/evaluation"
)

// requireRealGeminiKey skips the test unless a real Gemini API key is
// available in the environment. These EvalRunner tests drive the ADK
// runner end-to-end — they need a live model to produce a response,
// otherwise the assertions (non-empty FinalResponse, captured ToolCalls)
// can't be satisfied. Mirrors the gating pattern in userrequirements/.
func requireRealGeminiKey(t *testing.T) {
	t.Helper()
	for _, name := range []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"} {
		if key := os.Getenv(name); key != "" {
			return
		}
	}
	t.Skip("GEMINI_API_KEY / GOOGLE_API_KEY not set — skipping live-LLM EvalRunner test")
}

// geminiModelConfig returns a config using the real key from the
// environment, for tests that actually need to call Gemini.
func geminiModelConfig() *fagent.ModelConfig {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		key = os.Getenv("GOOGLE_API_KEY")
	}
	return &fagent.ModelConfig{
		Provider: "gemini",
		Model:    "gemini-2.5-flash",
		APIKey:   key,
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Test helpers: mock agent evaluator provider
// ──────────────────────────────────────────────────────────────────────────────

type mockEvalAgentProvider struct {
	agents map[string]fagent.Agent
}

func (m *mockEvalAgentProvider) GetForUser(_ context.Context, agentID, _ string) (fagent.Agent, error) {
	if a, ok := m.agents[agentID]; ok {
		return a, nil
	}
	return nil, &notFoundErr{name: agentID}
}

type mockEvalConfigResolver struct {
	cfg *fagent.ModelConfig
}

func (m *mockEvalConfigResolver) GetModelConfigForUser(_ context.Context, _ string) (*fagent.ModelConfig, error) {
	if m.cfg != nil {
		return m.cfg, nil
	}
	return &fagent.ModelConfig{
		Provider: "gemini",
		Model:    "gemini-2.5-flash",
		APIKey:   "test-key",
	}, nil
}

// mockAPIKeyResolver returns nil for all requests (no overrides)
type mockAPIKeyResolver struct{}

func (m *mockAPIKeyResolver) GetAPIKeyForProvider(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Tests
// ──────────────────────────────────────────────────────────────────────────────

func TestEvalRunner_SimpleAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireRealGeminiKey(t)

	t.Parallel()

	// Create a simple test agent
	testAgent := &mockConfigurableAgent{
		mockAgent: mockAgent{
			id:          "test-agent-1",
			name:        "TestAgent",
			description: "A simple test agent",
		},
		config: fagent.Config{
			SystemPrompt: "You are a helpful test agent. When asked what is 2+2, respond with '4'.",
		},
	}

	provider := &mockEvalAgentProvider{
		agents: map[string]fagent.Agent{
			"test-agent-1": testAgent,
		},
	}

	runner := NewEvalRunner(provider, &mockEvalConfigResolver{cfg: geminiModelConfig()}, &mockAPIKeyResolver{})

	ctx := context.Background()
	req := evaluation.EvalInvokeRequest{
		AgentID: "test-agent-1",
		UserID:  "test-user",
		Input:   "What is 2+2?",
	}

	result, err := runner.InvokeForEval(ctx, req)
	if err != nil {
		t.Fatalf("InvokeForEval failed: %v", err)
	}

	// Verify that we got a result
	if result == nil {
		t.Error("expected non-nil result")
	}

	// The response should contain text (we can't assert exact content since LLM is non-deterministic)
	if result.FinalResponse == "" {
		t.Error("expected non-empty FinalResponse")
	}

	// Verify JudgeAPIKey and JudgeModel are set
	if result.JudgeAPIKey == "" {
		t.Error("expected non-empty JudgeAPIKey")
	}
	if result.JudgeModel == "" {
		t.Error("expected non-empty JudgeModel")
	}
}

func TestEvalRunner_AgentNotFound(t *testing.T) {
	t.Parallel()

	provider := &mockEvalAgentProvider{agents: make(map[string]fagent.Agent)}
	runner := NewEvalRunner(provider, &mockEvalConfigResolver{}, &mockAPIKeyResolver{})

	ctx := context.Background()
	req := evaluation.EvalInvokeRequest{
		AgentID: "nonexistent-agent",
		UserID:  "test-user",
		Input:   "Hello",
	}

	_, err := runner.InvokeForEval(ctx, req)
	if err == nil {
		t.Error("expected error for nonexistent agent")
	}
}

func TestEvalRunner_WithModelOverride(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set — skipping live-LLM model-override test")
	}

	t.Parallel()

	// Create an agent with a model override
	testAgent := &mockConfigurableAgent{
		mockAgent: mockAgent{
			id:          "override-agent",
			name:        "OverrideAgent",
			description: "Agent with model override",
		},
		config: fagent.Config{
			SystemPrompt: "You are a helpful assistant.",
			ModelOverride: &fagent.AgentModelOverride{
				Provider:  "openai",
				ModelName: "gpt-4",
			},
		},
	}

	provider := &mockEvalAgentProvider{
		agents: map[string]fagent.Agent{
			"override-agent": testAgent,
		},
	}

	runner := NewEvalRunner(provider, &mockEvalConfigResolver{}, &mockAPIKeyResolver{})

	ctx := context.Background()
	req := evaluation.EvalInvokeRequest{
		AgentID: "override-agent",
		UserID:  "test-user",
		Input:   "Hello",
	}

	result, err := runner.InvokeForEval(ctx, req)
	if err != nil {
		t.Fatalf("InvokeForEval failed: %v", err)
	}

	if result == nil {
		t.Error("expected non-nil result")
	}
}

func TestEvalRunner_CapturesToolCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	requireRealGeminiKey(t)

	t.Parallel()

	// Create an agent that might make tool calls
	testAgent := &mockConfigurableAgent{
		mockAgent: mockAgent{
			id:          "tool-agent",
			name:        "ToolAgent",
			description: "Agent that uses tools",
		},
		config: fagent.Config{
			SystemPrompt: "You are a weather helper. When asked about weather, use the weather_api tool.",
		},
	}

	provider := &mockEvalAgentProvider{
		agents: map[string]fagent.Agent{
			"tool-agent": testAgent,
		},
	}

	runner := NewEvalRunner(provider, &mockEvalConfigResolver{cfg: geminiModelConfig()}, &mockAPIKeyResolver{})

	ctx := context.Background()
	req := evaluation.EvalInvokeRequest{
		AgentID: "tool-agent",
		UserID:  "test-user",
		Input:   "What's the weather in Tokyo?",
	}

	result, err := runner.InvokeForEval(ctx, req)
	if err != nil {
		t.Fatalf("InvokeForEval failed: %v", err)
	}

	// Result should contain ToolCalls (may be empty if agent doesn't call tools)
	if result.ToolCalls == nil {
		t.Error("expected ToolCalls slice, got nil")
	}
}

func TestEvalInvokeRequest_JSON(t *testing.T) {
	t.Parallel()

	req := evaluation.EvalInvokeRequest{
		AgentID: "agent-123",
		UserID:  "user-456",
		Input:   "What is 2+2?",
	}

	// Verify the request can be marshaled
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	// Verify it can be unmarshaled
	var unmarshaled evaluation.EvalInvokeRequest
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if unmarshaled.AgentID != req.AgentID {
		t.Errorf("expected AgentID %s, got %s", req.AgentID, unmarshaled.AgentID)
	}
}

func TestEvalInvokeResult_JSON(t *testing.T) {
	t.Parallel()

	result := &evaluation.EvalInvokeResult{
		FinalResponse: "The answer is 4.",
		ToolCalls: []evaluation.EvalToolCall{
			{
				Name: "calculator",
				Args: map[string]any{"expr": "2+2"},
			},
		},
		JudgeAPIKey: "test-judge-key",
		JudgeModel:  "gemini-2.5-flash",
	}

	// Verify the result can be marshaled
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	// Verify it can be unmarshaled
	var unmarshaled evaluation.EvalInvokeResult
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if unmarshaled.FinalResponse != result.FinalResponse {
		t.Errorf("expected FinalResponse %q, got %q", result.FinalResponse, unmarshaled.FinalResponse)
	}
	if len(unmarshaled.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(unmarshaled.ToolCalls))
	}
}
