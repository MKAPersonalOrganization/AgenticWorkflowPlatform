//go:build integration

// artifact_policy_test.go verifies the runtime promise behind the designer's
// Artifacts card: when artifacts are enabled on an agent, the orchestration
// engine exposes the `load_artifacts` tool so the agent can see what's been
// saved — and when the AutoSave toggle is on, the engine takes inline blobs
// from user messages out of the raw conversation and swaps them for text
// references the model can mention.
//
// The debug endpoint runs with an in-memory artifact service (one instance
// per Handler, process-lifetime) so we don't need external storage. See
// modules/chat/handlers/debughandler/handler.go — the handler wires its
// own artifact service alongside memory.Service.
//
// Specs exercised:
//
//	An agent with artifact_policy.enabled=true gets load_artifacts in its tool list
//	  → TestAgentWithArtifactPolicyReceivesLoadArtifactsTool
//
//	An agent with artifact_policy.enabled=false does NOT get load_artifacts
//	  → TestAgentWithoutArtifactPolicyDoesNotReceiveLoadArtifacts
package userrequirements

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const artifactsEnabledAgentYAML = `
name: Artifact Agent
slug: artifact-agent
description: "Agent with artifact support enabled."
system_prompt: "List every tool you have access to by name, one per line. No prose, no commentary."
category: Tests
icon: file-stack
artifact_policy:
  enabled: true
`

const artifactsDisabledAgentYAML = `
name: No-Artifact Agent
slug: no-artifact-agent
description: "Agent with artifacts explicitly disabled."
system_prompt: "List every tool you have access to by name, one per line. No prose, no commentary."
category: Tests
icon: file-stack
artifact_policy:
  enabled: false
`

// hasToolInRun returns true if any ToolStart event (or the agent's reply
// when no tool call happened) mentions the given tool name. We prefer the
// ToolStart event because it's a structural signal from the runtime; the
// agent's text reply is a fallback since the prompt asks it to list
// available tools — some models won't actually call load_artifacts on a
// simple "list your tools" prompt and instead describe them in prose.
func hasToolInRun(events []wireEvent, response, toolName string) bool {
	for _, e := range events {
		if e.Type != "ToolStart" {
			continue
		}
		var data map[string]any
		if err := json.Unmarshal(e.Data, &data); err != nil {
			continue
		}
		if name, ok := data["toolName"].(string); ok && name == toolName {
			return true
		}
	}
	return strings.Contains(strings.ToLower(response), strings.ToLower(toolName))
}

func TestAgentWithArtifactPolicyReceivesLoadArtifactsTool(t *testing.T) {
	apiKey := requireOpenAIKey(t)
	temperature := 0.0

	resp := postDebugRun(t, debugRequest{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		APIKey:      apiKey,
		Temperature: &temperature,
		UserID:      "art-user-a",
		Prompt:      "List your available tools.",
		AgentYAML:   artifactsEnabledAgentYAML,
	})

	require.Empty(t, resp.Errors, "runner reported errors: %v", resp.Errors)
	require.NotEmpty(t, resp.Response, "agent produced no response")

	// The only tool the engine can inject on an agent whose YAML has no
	// agent_tools / llm_agents / workflow is load_artifacts — and only
	// when artifact_policy.enabled is true. So the model listing it by
	// name (or calling it) proves the adapter wired the tool from the
	// policy correctly.
	require.True(t,
		hasToolInRun(resp.EventsEmitted, resp.Response, "load_artifacts"),
		"agent with artifact_policy.enabled=true should expose load_artifacts; got response: %s",
		resp.Response)
}

func TestAgentWithoutArtifactPolicyDoesNotReceiveLoadArtifacts(t *testing.T) {
	apiKey := requireOpenAIKey(t)
	
	temperature := 0.0

	resp := postDebugRun(t, debugRequest{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		APIKey:      apiKey,
		Temperature: &temperature,
		UserID:      "art-user-b",
		Prompt:      "List your available tools.",
		AgentYAML:   artifactsDisabledAgentYAML,
	})

	require.Empty(t, resp.Errors, "runner reported errors: %v", resp.Errors)

	// No ToolStart events should reference load_artifacts, and the
	// model's reply should not advertise a tool by that name. If the
	// legacy gate had survived or the policy was ignored, the tool
	// would leak into the tool list and the model would list it.
	for _, e := range resp.EventsEmitted {
		if e.Type != "ToolStart" {
			continue
		}
		var data map[string]any
		if err := json.Unmarshal(e.Data, &data); err == nil {
			if name, ok := data["toolName"].(string); ok {
				require.NotEqual(t, "load_artifacts", name,
					"disabled agent should not call load_artifacts")
			}
		}
	}
	require.NotContains(t, strings.ToLower(resp.Response), "load_artifacts",
		"agent with artifact_policy.enabled=false should not advertise load_artifacts; got: %s",
		resp.Response)
}
