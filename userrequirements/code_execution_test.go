//go:build integration

// code_execution_test.go verifies the runtime promise behind the designer's
// Code Execution toggle on Gemini: when the policy is enabled, the agent
// can ask the provider's sandbox to run Python, and the source + output
// are surfaced on the event stream so the chat UI can render them.
//
// The code never runs on our servers — Google's Gemini sandbox executes
// it and returns `ExecutableCode` + `CodeExecutionResult` parts on the
// response, which our adapter translates into a `CodeExecutionRan`
// domain event (see adapters/adkgo/events.go:translateCodeExecution).
//
// Specs exercised:
//
//	An agent with code_execution_policy enabled on Gemini receives the
//	built-in executor and emits a CodeExecutionRan event with code + output.
//	  → TestAgentWithCodeExecutionPolicyRunsCodeOnGemini
package userrequirements

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const codeExecutionAgentYAML = `
name: Code Exec Agent
slug: code-exec-agent
description: "Agent with code execution enabled on Gemini."
system_prompt: |
  You are a careful calculator. When the user asks for a numeric answer, use
  your built-in code tool to compute it, then report the final integer on its
  own line.
category: Tests
icon: terminal
code_execution_policy:
  enabled: true
`

func TestAgentWithCodeExecutionPolicyRunsCodeOnGemini(t *testing.T) {
	apiKey := requireGeminiKey(t)
	temperature := 0.0

	// 87654321 * 98765432 = 8657600060481672 — a product large enough that
	// the model will not attempt to guess-multiply; it has to delegate to
	// the sandbox to get it right. Sandbox returns the exact answer; we
	// assert both the answer in the reply AND that at least one
	// CodeExecutionRan event crossed the wire.
	const expected = "8657600060481672"

	resp := postDebugRun(t, debugRequest{
		Provider:    "google",
		Model:       "gemini-2.5-flash",
		APIKey:      apiKey,
		Temperature: &temperature,
		Prompt:      "What is 87654321 times 98765432? Use your code tool to get it exact. Report only the digits.",
		AgentYAML:   codeExecutionAgentYAML,
	})

	require.Empty(t, resp.Errors, "runner reported errors: %v", resp.Errors)
	require.NotEmpty(t, resp.Response, "agent produced no response")

	// The final reply carries the correct product — proof the sandbox
	// ran and Gemini used the result.
	require.Contains(t, resp.Response, expected,
		"expected the sandbox to compute %s; reply was: %s", expected, resp.Response)

	// And there must be a CodeExecutionRan event — that's what the chat
	// UI consumes to render the inline "code card". Finding at least one
	// non-empty Code field proves the adapter translated
	// `ExecutableCode` parts into domain events.
	var sawCodeEvent bool
	for _, e := range resp.EventsEmitted {
		if e.Type != "CodeExecutionRan" {
			continue
		}
		var data map[string]any
		if err := json.Unmarshal(e.Data, &data); err != nil {
			continue
		}
		code, _ := data["code"].(string)
		if strings.TrimSpace(code) != "" {
			sawCodeEvent = true
			break
		}
	}
	require.True(t, sawCodeEvent,
		"expected at least one CodeExecutionRan event carrying source; events: %v",
		resp.EventsEmitted)
}
