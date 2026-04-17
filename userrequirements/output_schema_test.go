//go:build integration

// output_schema_test.go verifies the user-facing behavior of the
// "Output Schema" feature. Each test reads as a spec sentence.
//
// Specs exercised:
//
//	An agent with output_schema returns JSON that conforms to the schema
//	  → ReturnsSchemaConformantJSONInDirectChat
//
//	An agent with output_schema writes JSON to state via output_key
//	  → OutputsJSONToStateAndPassesToDownstreamAgent
package userrequirements

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const outputSchemaCityFacts = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["city", "country", "population_millions", "is_capital"],
  "properties": {
    "city":                { "type": "string",  "minLength": 1 },
    "country":             { "type": "string",  "minLength": 1 },
    "population_millions": { "type": "number",  "minimum": 0, "maximum": 100 },
    "is_capital":          { "type": "boolean" }
  }
}`

// An agent with output_schema returns JSON that conforms to the schema in direct chat.
func TestReturnsSchemaConformantJSONInDirectChat(t *testing.T) {
	apiKey := requireOpenAIKey(t)
	temperature := 0.0

	agent, err := json.Marshal(map[string]any{
		"name": "city-facts",
		"system_prompt": "Return facts about the given city. " +
			"population_millions is the metro-area population in millions (a decimal). " +
			"is_capital is true iff the city is the capital of its country.",
		"output_schema": outputSchemaCityFacts,
	})
	require.NoError(t, err)

	resp := postDebugRun(t, debugRequest{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		APIKey:      apiKey,
		Temperature: &temperature,
		Prompt:      "Tokyo",
		Agent:       agent,
	})

	require.Empty(t, resp.Errors, "runner reported errors: %v", resp.Errors)
	require.NotEmpty(t, resp.Response, "agent produced no response")

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(resp.Response), &got),
		"response must be valid JSON; got:\n%s", resp.Response)

	validateAgainstSchema(t, outputSchemaCityFacts, got)

	city, _ := got["city"].(string)
	assert.Equal(t, "tokyo", strings.ToLower(strings.TrimSpace(city)),
		"city should be Tokyo; got %q", city)

	country, _ := got["country"].(string)
	assert.Equal(t, "japan", strings.ToLower(strings.TrimSpace(country)),
		"country should be Japan; got %q", country)

	pop, _ := got["population_millions"].(float64)
	assert.GreaterOrEqual(t, pop, 9.0, "Tokyo population_millions too low: %.2f", pop)
	assert.LessOrEqual(t, pop, 40.0, "Tokyo population_millions too high: %.2f", pop)

	isCapital, _ := got["is_capital"].(bool)
	assert.True(t, isCapital, "Tokyo is the capital of Japan; is_capital must be true")
}

// A sequential workflow where the first agent has output_schema writes JSON
// to state via output_key, and the second agent reads it via {key} template.
const workflowLeafYAML = `
name: Leaf Schema
slug: leaf-schema
description: Tests output_schema as a workflow leaf with downstream state passing.
system_prompt: Orchestrates extraction then formatting.
category: Tests
icon: file-json

workflow:
  id: root
  type: sequential
  children:
    - id: n1
      type: agent
      agent_id: sub-extractor
    - id: n2
      type: agent
      agent_id: sub-formatter

llm_agents:
  - id: sub-extractor
    name: CityExtractor
    description: Extract city facts as JSON.
    output_key: city_facts
    system_prompt: |
      Return facts about the city the user names as JSON.
      population_millions is the metro-area population as a decimal number.
      is_capital is true iff the city is the capital of its country.

  - id: sub-formatter
    name: CityFormatter
    description: Render the extracted facts as a one-line sentence.
    system_prompt: |
      You are given structured city facts as JSON: {city_facts}
      Produce exactly one English sentence of the form:
      "<city> is the capital of <country>." if is_capital is true,
      or "<city> is a city in <country>." if is_capital is false.
      Output the sentence and nothing else.
`

func TestOutputsJSONToStateAndPassesToDownstreamAgent(t *testing.T) {
	apiKey := requireOpenAIKey(t)
	temperature := 0.0

	resp := postDebugRun(t, debugRequest{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		APIKey:      apiKey,
		Temperature: &temperature,
		Prompt:      "Paris",
		AgentYAML:   workflowLeafYAML,
	})

	require.Empty(t, resp.Errors, "runner reported errors: %v", resp.Errors)
	require.NotEmpty(t, resp.Response, "workflow produced no final response")

	final := strings.ToLower(strings.TrimSpace(resp.Response))
	assert.Contains(t, final, "paris",
		"formatter output must reference the prompt subject — state was not passed: %q", final)
	assert.Contains(t, final, "france",
		"formatter output must reference the country from the extractor — state was not passed: %q", final)
	assert.True(t,
		strings.Contains(final, "capital") || strings.Contains(final, "city"),
		"formatter output must pick the is_capital branch — state was not passed: %q", final)

	msgs := findEventsByType(resp.EventsEmitted, "AssistantMessageAdded")
	require.NotEmpty(t, msgs, "no AssistantMessageAdded events were emitted")
}
