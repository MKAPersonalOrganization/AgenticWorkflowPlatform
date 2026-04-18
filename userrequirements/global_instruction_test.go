//go:build integration

// global_instruction_test.go verifies the runtime promise behind the
// designer's root-only Global Instruction editor: a directive declared
// on the root agent's catalog YAML is appended to its LLM call, so the
// reply respects the directive even when the agent's own system prompt
// doesn't mention it. Only the root's value is honoured — the value on
// any nested agent is silently ignored (see
// .READ/adk-go/agent/llmagent/llmagent.go:232 and
// .READ/adk-go/internal/llminternal/instruction_processor.go:48-56).
//
// Scope note on workflow roots. The orchestration engine resolves the
// root via `parents.RootAgent(ctx.Agent())` and then `asLLMAgent(root)`
// (see agent_transfer.go:186). When the root is a SequentialAgent /
// ParallelAgent / LoopAgent — i.e. every workflow we build — the type
// assertion fails and the processor falls back to the current leaf's
// own GlobalInstruction, which is empty. So a global_instruction on a
// workflow root is a no-op at runtime today. That's an ADK detail, not
// an aicompass bug — covering it here would only lock in the upstream
// limitation. If we later choose to emulate propagation in the adapter
// (by copying the workflow root's global_instruction down onto each
// leaf LlmAgent during build), add a positive test for that here.
//
// Specs exercised:
//
//	A global instruction on a single agent steers its own reply
//	  → TestAGlobalInstructionSteersTheRootAgentsOwnReply
package userrequirements

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Single LLM agent whose global_instruction forces a marker into every
// reply. The agent's own system_prompt is intentionally generic so the
// marker can only come from the global directive. Since a single agent
// is its own root, the orchestration engine honours the instruction.
const singleAgentGlobalInstructionYAML = `
name: Marker Agent
slug: marker-agent
description: "Single agent with a root-level global_instruction that injects a marker into every reply."
system_prompt: "Greet the user briefly."
global_instruction: "End every reply with the exact marker string ZZZXYZ on its own line."
category: Tests
icon: flag
`

func TestAGlobalInstructionSteersTheRootAgentsOwnReply(t *testing.T) {
	apiKey := requireOpenAIKey(t)
	temperature := 0.0

	resp := postDebugRun(t, debugRequest{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		APIKey:      apiKey,
		Temperature: &temperature,
		Prompt:      "hi",
		AgentYAML:   singleAgentGlobalInstructionYAML,
	})

	require.Empty(t, resp.Errors, "runner reported errors: %v", resp.Errors)
	require.NotEmpty(t, resp.Response, "agent produced no response")

	// Only the global_instruction requests the marker — the agent's
	// own system_prompt just asks for a greeting. Finding the marker
	// in the response proves the directive was appended to the LLM
	// call.
	require.Contains(t, strings.ToUpper(resp.Response), "ZZZXYZ",
		"root's global_instruction should have forced the marker into the reply; got: %s",
		resp.Response)
}

