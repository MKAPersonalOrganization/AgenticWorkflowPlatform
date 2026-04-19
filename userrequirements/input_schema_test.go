//go:build integration

// input_schema_test.go verifies the runtime promise behind the designer's
// Agent-Tools section: a parent agent can expose a sub-agent as a callable
// tool, and the tool's JSON schema controls the parameter shape the
// orchestration engine announces to the LLM. Without a schema, the engine
// synthesises a default "request" string parameter.
//
// Specs exercised:
//
//	A coordinator can call a sub-agent as a tool
//	  → TestACoordinatorCanCallASubAgentAsATool
//
//	A callable without input schema receives plain text input
//	  → TestACallableWithoutSchemaReceivesPlainTextInput
package userrequirements

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const weatherInputSchema = `{
  "type": "object",
  "properties": {
    "city": { "type": "string", "description": "The city name" }
  },
  "required": ["city"]
}`

func TestACoordinatorCanCallASubAgentAsATool(t *testing.T) {
	apiKey := requireOpenAIKey(t)

	client := authedClient(t)
	upsertAISettings(t, client, "openai", apiKey)

	// Sub-agent first so we can reference its id from the parent. It
	// lives as a standalone agent and is exposed as a tool via the
	// parent's agentTools list (not subAgents — subAgents is ADK's
	// coordinator pattern; agentTools is the agent-as-tool pattern).
	subBody := agentBody(
		"GetWeather",
		"Return the weather for the given city in one sentence.",
		"openai", "gpt-4o-mini",
	)
	subBody["description"] = "Get weather for a city."
	subID := createAgent(t, client, subBody)

	parentBody := agentBody(
		"Parent Agent",
		"You are a weather assistant. Use the GetWeather tool to answer any weather question.",
		"openai", "gpt-4o-mini",
	)
	parentBody["agentTools"] = []map[string]any{
		{
			"agent_id":     subID,
			"input_schema": weatherInputSchema,
		},
	}
	parentID := createAgent(t, client, parentBody)

	_, events := sendMessage(t, client, "", parentID, "What's the weather in Tokyo?")

	require.Empty(t, runErrorMessage(events), "run should not error")
	reply := assistantFinalText(events)
	require.NotEmpty(t, reply, "agent produced no text reply")

	tools := findToolStarts(events)
	require.NotEmpty(t, tools,
		"agent-as-tool pattern must emit a TOOL_START frame; reply was: %s", reply)

	// The tool name the engine announces to the model is the sub-agent's
	// display name. The LLM calls it with that name, so the TOOL_START
	// frame's toolName must match "GetWeather".
	toolName, _ := tools[0]["toolName"].(string)
	require.Equal(t, "GetWeather", toolName,
		"tool name should be the sub-agent's name; got %q", toolName)
}

func TestACallableWithoutSchemaReceivesPlainTextInput(t *testing.T) {
	apiKey := requireOpenAIKey(t)

	client := authedClient(t)
	upsertAISettings(t, client, "openai", apiKey)

	subBody := agentBody(
		"EchoAgent",
		"Repeat exactly what the user says.",
		"openai", "gpt-4o-mini",
	)
	subBody["description"] = "Echoes the request."
	subID := createAgent(t, client, subBody)

	parentBody := agentBody(
		"Simple Agent",
		"Repeat the user's request back to them by calling the EchoAgent tool.",
		"openai", "gpt-4o-mini",
	)
	// No input_schema → orchestration engine synthesises a default
	// single-parameter "request" string input.
	parentBody["agentTools"] = []map[string]any{
		{"agent_id": subID},
	}
	parentID := createAgent(t, client, parentBody)

	_, events := sendMessage(t, client, "", parentID, "Say hello world")

	require.Empty(t, runErrorMessage(events), "run should not error")

	tools := findToolStarts(events)
	require.NotEmpty(t, tools, "must emit TOOL_START")

	// The engine announced a single "request" parameter; the model's
	// arguments field is a JSON string containing that key.
	args, _ := tools[0]["arguments"].(string)
	require.Contains(t, args, "request",
		"default input schema should use 'request' parameter; got arguments=%s", args)
}
