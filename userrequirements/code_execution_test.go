//go:build integration

// code_execution_test.go verifies the runtime promise behind the designer's
// Code Execution toggle: when an agent enables it, the agent delegates
// math to the LLM provider's built-in sandbox, the chat stream carries
// CODE_EXECUTION_RAN events, and — crucially — the result survives a
// browser refresh.
//
// The tests drive the app through the same public REST endpoints the UI
// uses:
//
//	POST /auth/dev-login         → session cookie
//	PUT  /api/ai-settings        → provider + api_key
//	POST /v1/agent-definitions   → create agent with code_execution_policy
//	POST /v1/message:send        → SSE: parse CODE_EXECUTION_RAN frames
//	GET  /api/agui/threads/{id}/messages → assert the code card is in history
//	DELETE /v1/agent-definitions/{id}   (auto-cleanup)
//	DELETE /api/agui/threads/{id}       (auto-cleanup)
//
// The product is 87654321 * 98765432 = 8657216880231672. That value is big
// enough that a model won't guess it — it has to delegate to the sandbox.
package userrequirements

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const codeExecProduct = "8657216880231672"
const codeExecPrompt = "What is 87654321 times 98765432? Use your code tool to get it exact. Report only the digits."

// codeExecAgentBody returns the /v1/agent-definitions request body for a
// code-exec-enabled agent. modelOverride lets a test pin provider+model
// per call (e.g. "gemini"/"gemini-2.5-flash" vs "openai"/"gpt-4o-mini")
// while sharing the rest of the definition. temperature=0 is baked into
// model_config — same column the UI would write — so the run is
// deterministic without any per-request override on the endpoint.
func codeExecAgentBody(name, provider, model string) map[string]any {
	return map[string]any{
		"name":         name,
		"description":  "Agent with code execution enabled.",
		"systemPrompt": "You are a careful calculator. When the user asks for a numeric answer, use your built-in code tool to compute it, then report the final integer on its own line.",
		"icon":         "terminal",
		"codeExecutionPolicy": map[string]any{
			"enabled": true,
		},
		"modelConfig": map[string]any{
			"temperature": 0.0,
		},
		"modelOverride": map[string]any{
			"provider":  provider,
			"modelName": model,
		},
	}
}

// TestAgentWithCodeExecutionPolicyRunsCodeOnGemini — Gemini built-in sandbox.
func TestAgentWithCodeExecutionPolicyRunsCodeOnGemini(t *testing.T) {
	t.Parallel()
	requireGeminiKey(t)
	client := authedClient(t)

	agentID := createAgent(t, client, codeExecAgentBody("code-exec-gemini", "gemini", "gemini-2.5-flash"))

	threadID, events := sendMessage(t, client, "", agentID, codeExecPrompt)
	require.NotEmpty(t, threadID, "SESSION_START did not carry a contextId")

	assertCodeExecutionStream(t, events, codeExecProduct)

	// Refresh survival — the chat card MUST reappear after a reload.
	// The UI reads /api/agui/threads/{id}/messages on refresh; the same
	// endpoint here lets us assert the code card persisted. The AG-UI
	// projection is eventually-consistent (updater runs in a background
	// loop), so we poll briefly — mirrors what a user would see if they
	// refreshed a second or two after the run completes.
	state := waitForCodeExecutionSidecar(t, client, threadID, 5*time.Second)
	assertCodeExecutionInSidecar(t, state)
}

// TestAgentWithCodeExecutionPolicyRunsCodeOnOpenAI — OpenAI Responses API
// code_interpreter. If the adapter's Responses-API translation drops the
// tool, the model will either guess-multiply (wrong digits) or refuse —
// either way this test fails.
func TestAgentWithCodeExecutionPolicyRunsCodeOnOpenAI(t *testing.T) {
	t.Parallel()
	requireOpenAIKey(t)
	client := authedClient(t)

	agentID := createAgent(t, client, codeExecAgentBody("code-exec-openai", "openai", "gpt-4o-mini"))

	threadID, events := sendMessage(t, client, "", agentID, codeExecPrompt)
	require.NotEmpty(t, threadID)

	assertCodeExecutionStream(t, events, codeExecProduct)

	state := waitForCodeExecutionSidecar(t, client, threadID, 5*time.Second)
	assertCodeExecutionInSidecar(t, state)
}

// waitForCodeExecutionSidecar polls /api/agui/threads/{id}/messages until
// the codeExecutions sidecar is populated or the deadline is hit. The
// AG-UI projection processes events off the store asynchronously via a
// background updater loop — fresh events can take a short moment to
// appear in the read API even after SSE closes. This matches the UX: a
// user refreshing milliseconds after a run might see the card still
// loading; waiting a second or two is realistic.
func waitForCodeExecutionSidecar(t *testing.T, client *http.Client, threadID string, timeout time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last map[string]any
	for {
		last = readThreadMessages(t, client, threadID)
		if ce, ok := last["codeExecutions"].(map[string]any); ok && len(ce) > 0 {
			return last
		}
		if time.Now().After(deadline) {
			return last
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// assertCodeExecutionStream checks (a) the assistant's final text contains
// the expected product (proof the sandbox returned it), and (b) at least
// one CODE_EXECUTION_RAN frame carried non-empty source (proof the
// adapter translated the provider's ExecutableCode parts into a domain
// event and the SSE layer re-emitted it).
func assertCodeExecutionStream(t *testing.T, events []sseEvent, expected string) {
	t.Helper()

	var finalText strings.Builder
	sawCodeEvent := false

	for _, e := range events {
		switch e.Name() {
		case "TEXT_MESSAGE_CONTENT":
			// Top-level delta — not a CUSTOM event, so not wrapped in `value`.
			var payload struct {
				Delta string `json:"delta"`
			}
			_ = json.Unmarshal(e.Data, &payload)
			finalText.WriteString(payload.Delta)
		case "CODE_EXECUTION_RAN":
			// CUSTOM event — payload nested under `value`.
			var payload struct {
				Code string `json:"code"`
			}
			_ = e.Value(&payload)
			if strings.TrimSpace(payload.Code) != "" {
				sawCodeEvent = true
			}
		}
	}

	require.Contains(t, finalText.String(), expected,
		"expected sandbox to compute %s; assistant said: %s", expected, finalText.String())
	require.True(t, sawCodeEvent,
		"expected at least one CODE_EXECUTION_RAN frame with non-empty source")
}

// assertCodeExecutionInSidecar verifies the AG-UI read API includes at
// least one code-execution entry in the `codeExecutions` sidecar — the
// object the frontend feeds into the chat card on page load. Refresh
// survival hinges on this landing in agui_messages, which in turn hinges
// on the CodeExecutionRan domain event being persisted by the
// orchestrator. No DB access, by design — we assert the same thing the
// UI would see.
func assertCodeExecutionInSidecar(t *testing.T, state map[string]any) {
	t.Helper()
	ce, ok := state["codeExecutions"].(map[string]any)
	if !ok || len(ce) == 0 {
		t.Fatalf("expected codeExecutions sidecar to contain at least one entry; got: %v", state["codeExecutions"])
	}
	// Spot-check one entry has a language or output — anything to prove
	// it's not an empty shell.
	for key, raw := range ce {
		m, ok := raw.(map[string]any)
		require.Truef(t, ok, "codeExecutions[%s] is not an object: %v", key, raw)
		if m["language"] != nil || m["output"] != nil {
			return
		}
	}
	t.Fatalf("codeExecutions sidecar entries are all empty: %v", ce)
}
