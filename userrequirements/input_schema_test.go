//go:build integration

// input_schema_test.go verifies the user-facing behavior of the
// "Input Schema" feature. Each test reads as a spec sentence.
//
// Specs exercised:
//
//	A coordinator can call a sub-agent as a tool
//	  → ACoordinatorCanCallASubAgentAsATool
//
//	A callable without input schema receives plain text input
//	  → ACallableWithoutSchemaReceivesPlainTextInput
package userrequirements

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Root agent exposes a sub-agent as a tool with input_schema on the tool ref.
const agentAsToolYAML = `
name: Parent Agent
slug: parent-agent
description: Parent that calls sub-agent as tool.
system_prompt: |
  You are a weather assistant. Use the GetWeather tool to answer any weather question.
category: Tests
icon: cloud

llm_agents:
  - id: parent-agent
    name: ParentAgent
    description: Parent that orchestrates.
    system_prompt: You are a parent agent. Use the GetWeather tool.

  - id: weather-agent
    name: GetWeather
    description: Get weather for a city.
    system_prompt: Return the weather for the given city in one sentence.

agent_tools:
  - agent_id: weather-agent
    input_schema: |
      {
        "type": "object",
        "properties": {
          "city": { "type": "string", "description": "The city name" }
        },
        "required": ["city"]
      }
`

func TestACoordinatorCanCallASubAgentAsATool(t *testing.T) {
	apiKey := requireOpenAIKey(t)
	temperature := 0.0

	resp := postDebugRun(t, debugRequest{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		APIKey:      apiKey,
		Temperature: &temperature,
		Prompt:      "What's the weather in Tokyo?",
		AgentYAML:   agentAsToolYAML,
	})

	require.Empty(t, resp.Errors, "runner reported errors: %v", resp.Errors)
	require.NotEmpty(t, resp.Response, "agent produced no response")

	eventTypes := make([]string, 0, len(resp.EventsEmitted))
	for _, e := range resp.EventsEmitted {
		eventTypes = append(eventTypes, e.Type)
	}

	toolEvents := findEventsByType(resp.EventsEmitted, "ToolStart")
	require.NotEmpty(t, toolEvents, "agent-as-tool pattern must emit ToolStart event, got: %v", eventTypes)

	// Verify the tool name matches the sub-agent name
	var toolName string
	if len(toolEvents) > 0 && len(toolEvents[0].Data) > 0 {
		var toolData map[string]any
		if err := json.Unmarshal(toolEvents[0].Data, &toolData); err == nil {
			if name, ok := toolData["toolName"].(string); ok {
				toolName = name
			}
		}
	}
	require.Equal(t, "GetWeather", toolName, "tool name should be the sub-agent name 'GetWeather'")
}

// Agent without input_schema but exposed as tool - should use "request" parameter.
const defaultInputSchemaYAML = `
name: Simple Agent
slug: simple-agent
description: Agent without input_schema, used as tool.
system_prompt: Repeat the user's request back to them.
category: Tests
icon: echo

llm_agents:
  - id: echo-agent
    name: EchoAgent
    description: Echoes the request.
    system_prompt: Repeat exactly what the user says.

agent_tools:
  - agent_id: echo-agent
`

func TestACallableWithoutSchemaReceivesPlainTextInput(t *testing.T) {
	apiKey := requireOpenAIKey(t)
	temperature := 0.0

	resp := postDebugRun(t, debugRequest{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		APIKey:      apiKey,
		Temperature: &temperature,
		Prompt:      "Say hello world",
		AgentYAML:   defaultInputSchemaYAML,
	})

	require.Empty(t, resp.Errors, "runner reported errors: %v", resp.Errors)
	require.NotEmpty(t, resp.Response, "agent produced no response")

	toolEvents := findEventsByType(resp.EventsEmitted, "ToolStart")
	require.NotEmpty(t, toolEvents, "must emit ToolStart event")

	// Verify arguments contain "request" key (default parameter name)
	var args string
	if len(toolEvents) > 0 && len(toolEvents[0].Data) > 0 {
		var toolData map[string]any
		if err := json.Unmarshal(toolEvents[0].Data, &toolData); err == nil {
			if a, ok := toolData["arguments"].(string); ok {
				args = a
			}
		}
	}
	require.True(t, strings.Contains(args, "request"), "default input schema should use 'request' parameter, got: %s", args)
}
