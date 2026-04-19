//go:build integration

// output_schema_test.go verifies the "Output Schema" feature end-to-end:
// an agent with an outputSchema returns JSON that conforms to the
// schema, and — composed in a workflow — downstream agents can read that
// structured result from session state via {outputKey} templates.
//
// Specs exercised:
//
//	An agent can return structured data matching a schema
//	  → TestAnAgentCanReturnStructuredDataMatchingASchema
//
//	A downstream agent can receive the output of an upstream agent
//	  → TestADownstreamAgentCanReceiveOutputFromAnUpstreamAgent
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

func TestAnAgentCanReturnStructuredDataMatchingASchema(t *testing.T) {
	apiKey := requireOpenAIKey(t)

	client := authedClient(t)
	upsertAISettings(t, client, "openai", apiKey)

	body := agentBody(
		"city-facts",
		"Return facts about the given city. "+
			"population_millions is the metro-area population in millions (a decimal). "+
			"is_capital is true iff the city is the capital of its country.",
		"openai", "gpt-4o-mini",
	)
	body["outputSchema"] = outputSchemaCityFacts

	agentID := createAgent(t, client, body)

	_, events := sendMessage(t, client, "", agentID, "Tokyo")

	require.Empty(t, runErrorMessage(events), "run should not error")
	reply := assistantFinalText(events)
	require.NotEmpty(t, reply, "agent produced no text reply")

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(reply), &got),
		"response must be valid JSON; got:\n%s", reply)

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

func TestADownstreamAgentCanReceiveOutputFromAnUpstreamAgent(t *testing.T) {
	apiKey := requireOpenAIKey(t)

	client := authedClient(t)
	upsertAISettings(t, client, "openai", apiKey)

	// Extractor: emits city facts as JSON into session state under
	// `city_facts` via outputKey. Downstream consumers read it via the
	// {city_facts} template in their systemPrompt.
	extractorBody := agentBody(
		"CityExtractor",
		"Return facts about the city the user names as JSON.\n"+
			"population_millions is the metro-area population as a decimal number.\n"+
			"is_capital is true iff the city is the capital of its country.",
		"openai", "gpt-4o-mini",
	)
	extractorBody["description"] = "Extract city facts as JSON."
	extractorBody["outputKey"] = "city_facts"
	extractorID := createAgent(t, client, extractorBody)

	// Formatter: reads the structured city_facts from state and renders
	// a single-sentence summary. Only state-template expansion can
	// deliver the structured value into the prompt.
	formatterBody := agentBody(
		"CityFormatter",
		"You are given structured city facts as JSON: {city_facts}\n"+
			"Produce exactly one English sentence of the form:\n"+
			`"<city> is the capital of <country>." if is_capital is true,`+"\n"+
			`or "<city> is a city in <country>." if is_capital is false.`+"\n"+
			"Output the sentence and nothing else.",
		"openai", "gpt-4o-mini",
	)
	formatterBody["description"] = "Render the extracted facts as a one-line sentence."
	formatterID := createAgent(t, client, formatterBody)

	workflowBody := agentBody(
		"Leaf Schema",
		"Orchestrates extraction then formatting.",
		"openai", "gpt-4o-mini",
	)
	workflowBody["description"] = "Tests outputSchema as a workflow leaf with downstream state passing."
	workflowBody["icon"] = "file-json"
	workflowBody["graph"] = map[string]any{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]any{
			{"id": "n1", "type": "agent", "agent_id": extractorID},
			{"id": "n2", "type": "agent", "agent_id": formatterID},
		},
	}
	workflowID := createAgent(t, client, workflowBody)

	_, events := sendMessage(t, client, "", workflowID, "Paris")

	require.Empty(t, runErrorMessage(events), "run should not error")
	reply := assistantFinalText(events)
	require.NotEmpty(t, reply, "workflow produced no final reply")

	final := strings.ToLower(strings.TrimSpace(reply))
	assert.Contains(t, final, "paris",
		"formatter output must reference the prompt subject; state was not passed: %q", final)
	assert.Contains(t, final, "france",
		"formatter output must reference the country from the extractor; state was not passed: %q", final)
	assert.True(t,
		strings.Contains(final, "capital") || strings.Contains(final, "city"),
		"formatter output must pick the is_capital branch; state was not passed: %q", final)
}
