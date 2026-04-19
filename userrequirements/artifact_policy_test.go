//go:build integration

// artifact_policy_test.go verifies the runtime promise behind the
// designer's Artifacts card: when artifacts are enabled on an agent, the
// orchestration engine exposes the `load_artifacts` tool so the agent
// can see what's been saved. When disabled, the tool is NOT wired and
// the model has no way to advertise or call it.
//
// Specs exercised:
//
//	An agent with artifactPolicy.enabled=true gets load_artifacts in its tool list
//	  → TestAgentWithArtifactPolicyReceivesLoadArtifactsTool
//
//	An agent with artifactPolicy.enabled=false does NOT get load_artifacts
//	  → TestAgentWithoutArtifactPolicyDoesNotReceiveLoadArtifacts
package userrequirements

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// toolListPrompt is deliberately short and directive: the model is asked
// to emit the names of every tool it has access to. The agent has no
// agentTools / subAgents / graph, so the only tool that could appear is
// the one injected by artifactPolicy (load_artifacts).
const toolListPrompt = "List your available tools by name, one per line. No prose."

// hasLoadArtifacts is true if either (a) the model invoked the tool
// (TOOL_START with toolName="load_artifacts") or (b) the model named the
// tool in its reply text. Models frequently just list tools in prose on
// a "what tools do you have" prompt without actually calling any, so the
// text-match fallback matters.
func hasLoadArtifacts(events []sseEvent, reply string) bool {
	for _, ts := range findToolStarts(events) {
		if name, _ := ts["toolName"].(string); name == "load_artifacts" {
			return true
		}
	}
	return strings.Contains(strings.ToLower(reply), "load_artifacts")
}

func TestAgentWithArtifactPolicyReceivesLoadArtifactsTool(t *testing.T) {
	apiKey := requireOpenAIKey(t)

	client := authedClient(t)
	upsertAISettings(t, client, "openai", apiKey)

	body := agentBody(
		"Artifact Agent",
		"List every tool you have access to by name, one per line. No prose, no commentary.",
		"openai", "gpt-4o-mini",
	)
	body["artifactPolicy"] = map[string]any{"enabled": true}

	agentID := createAgent(t, client, body)

	_, events := sendMessage(t, client, "", agentID, toolListPrompt)

	require.Empty(t, runErrorMessage(events), "run should not error")
	reply := assistantFinalText(events)
	require.NotEmpty(t, reply, "agent produced no text reply")

	require.True(t,
		hasLoadArtifacts(events, reply),
		"artifactPolicy.enabled=true should expose load_artifacts; reply was: %s", reply)
}

func TestAgentWithoutArtifactPolicyDoesNotReceiveLoadArtifacts(t *testing.T) {
	apiKey := requireOpenAIKey(t)

	client := authedClient(t)
	upsertAISettings(t, client, "openai", apiKey)

	body := agentBody(
		"No-Artifact Agent",
		"List every tool you have access to by name, one per line. No prose, no commentary.",
		"openai", "gpt-4o-mini",
	)
	body["artifactPolicy"] = map[string]any{"enabled": false}

	agentID := createAgent(t, client, body)

	_, events := sendMessage(t, client, "", agentID, toolListPrompt)

	require.Empty(t, runErrorMessage(events), "run should not error")

	// No TOOL_START frames should reference load_artifacts.
	for _, ts := range findToolStarts(events) {
		if name, _ := ts["toolName"].(string); name == "load_artifacts" {
			t.Fatalf("disabled agent should not invoke load_artifacts; saw TOOL_START: %v", ts)
		}
	}

	// And the model's reply shouldn't advertise it by name.
	reply := assistantFinalText(events)
	require.NotContains(t, strings.ToLower(reply), "load_artifacts",
		"artifactPolicy.enabled=false should not expose load_artifacts; reply: %s", reply)
}
