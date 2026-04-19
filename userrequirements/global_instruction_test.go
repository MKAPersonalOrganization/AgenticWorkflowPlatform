//go:build integration

// global_instruction_test.go verifies the runtime promise behind the
// designer's root-only Global Instruction editor: a directive declared on
// the root agent's globalInstruction field is appended to its LLM call, so
// the reply respects the directive even when the agent's own system
// prompt doesn't mention it.
//
// Scope note on workflow roots. The orchestration engine resolves the
// root via `parents.RootAgent(ctx.Agent())` and then `asLLMAgent(root)`
// (see agent_transfer.go:186). When the root is a Sequential/Parallel/
// Loop workflow the type assertion fails and the processor falls back to
// the current leaf's own GlobalInstruction, which is empty — so a
// global_instruction on a workflow root is a no-op at runtime today.
// That's an ADK detail, not an aicompass bug.
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

func TestAGlobalInstructionSteersTheRootAgentsOwnReply(t *testing.T) {
	t.Parallel()
	requireOpenAIKey(t)
	client := authedClient(t)

	// Single LLM agent whose globalInstruction forces a marker into every
	// reply. The agent's own systemPrompt is deliberately generic so the
	// marker can only come from the global directive. A single agent is
	// its own root, so the orchestration engine honours the instruction.
	body := agentBody(
		"Marker Agent",
		"Greet the user briefly.",
		"openai", "gpt-4o-mini",
	)
	body["globalInstruction"] = "End every reply with the exact marker string ZZZXYZ on its own line."

	agentID := createAgent(t, client, body)

	_, events := sendMessage(t, client, "", agentID, "hi")

	require.Empty(t, runErrorMessage(events), "run should not error")
	reply := assistantFinalText(events)
	require.NotEmpty(t, reply, "agent produced no text reply")

	// Only the globalInstruction requests the marker — the systemPrompt
	// just asks for a greeting. Finding the marker in the reply proves
	// the directive was appended to the LLM call.
	require.Contains(t, strings.ToUpper(reply), "ZZZXYZ",
		"globalInstruction should have forced the marker into the reply; got: %s", reply)
}
