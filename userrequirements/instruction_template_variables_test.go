//go:build integration

// instruction_template_variables_test.go verifies the runtime promise
// behind the designer's "{"-triggered instruction-template picker: if the
// picker suggests a key, the orchestration engine must actually
// substitute it at runtime.
//
// Template syntax under test is defined in the orchestration engine's
// instruction processor — `replaceMatch` expands {name}, {name?},
// {scope:name}, {artifact.name} against session state and the artifact
// store. The dominant read pattern is a downstream sub-agent referencing
// an upstream peer's output_key.
//
// Specs exercised:
//
//	A workflow peer's output_key flows into a downstream sibling's prompt
//	  → TestAWorkflowPeerOutputKeyFlowsIntoDownstreamSiblingPrompt
//
//	An optional template variable ({key?}) is silent when the key is missing
//	  → TestAnOptionalTemplateVariableIsSilentWhenMissing
//
//	A required template variable ({key}) fails the run when the key is missing
//	  → TestARequiredTemplateVariableFailsWhenMissing
package userrequirements

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAWorkflowPeerOutputKeyFlowsIntoDownstreamSiblingPrompt(t *testing.T) {
	apiKey := requireOpenAIKey(t)

	client := authedClient(t)
	upsertAISettings(t, client, "openai", apiKey)

	// Producer: emits the word after SECRET= into session state under
	// `extracted_secret`. The single-lowercase-word contract is tight so
	// the model can't slip in a sentence that coincidentally happens to
	// contain "avocado".
	producerBody := agentBody(
		"Producer",
		`Reply with ONLY the word immediately after "SECRET=" in the user's message. `+
			`Output a single lowercase word. No quotes, no punctuation, no prefix, no suffix, no sentence.`,
		"openai", "gpt-4o-mini",
	)
	producerBody["description"] = "Emits the word following SECRET= in the user prompt."
	producerBody["outputKey"] = "extracted_secret"
	producerID := createAgent(t, client, producerBody)

	// Consumer: references {extracted_secret}. If the orchestration
	// engine's InjectSessionState expanded the template, the model sees
	// the actual word and echoes it. If not, the model sees the literal
	// "{extracted_secret}" token and can't produce the marker.
	consumerBody := agentBody(
		"Consumer",
		"The value in session state is: {extracted_secret}\n\n"+
			"Reply with exactly this sentence, substituting <value> with the "+
			"session-state value above:\n\n"+
			`"The secret is <value>."`,
		"openai", "gpt-4o-mini",
	)
	consumerBody["description"] = "Uses the extracted secret from session state."
	consumerID := createAgent(t, client, consumerBody)

	// Sequential workflow root referencing both children via their DB
	// UUIDs. The graph's agent_id fields must match the ids returned by
	// the two creates above.
	workflowBody := agentBody(
		"State Flow Workflow",
		"Run a two-step pipeline.",
		"openai", "gpt-4o-mini",
	)
	workflowBody["description"] = "Proves the orchestration engine substitutes upstream {output_key} references at runtime."
	workflowBody["graph"] = map[string]any{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]any{
			{"id": "n1", "type": "agent", "agent_id": producerID},
			{"id": "n2", "type": "agent", "agent_id": consumerID},
		},
	}
	workflowID := createAgent(t, client, workflowBody)

	_, events := sendMessage(t, client, "", workflowID, "Process the following: SECRET=avocado please proceed.")

	require.Empty(t, runErrorMessage(events), "run should not error")
	reply := assistantFinalText(events)

	// Only template expansion can route "avocado" from the Producer into
	// the Consumer's reply. If the literal "{extracted_secret}" survived,
	// the Consumer output would either contain the braces or omit the
	// word entirely.
	require.Contains(t, strings.ToLower(reply), "avocado",
		"Consumer should echo the Producer's output_key value; got: %s", reply)
}

func TestAnOptionalTemplateVariableIsSilentWhenMissing(t *testing.T) {
	apiKey := requireOpenAIKey(t)

	client := authedClient(t)
	upsertAISettings(t, client, "openai", apiKey)

	// {nothing_here?} — the `?` suffix tells the orchestration engine's
	// instruction processor to swallow missing-key errors and substitute
	// an empty string instead of failing the run.
	body := agentBody(
		"Optional Variable Agent",
		"Marker: [{nothing_here?}]\n\n"+
			"Reply with exactly this sentence and nothing else:\n\n"+
			`"Optional variable ran."`,
		"openai", "gpt-4o-mini",
	)
	body["description"] = "Uses {key?} syntax for an intentionally missing key."

	agentID := createAgent(t, client, body)

	_, events := sendMessage(t, client, "", agentID, "Run.")

	require.Empty(t, runErrorMessage(events),
		"{nothing_here?} must not error when the key is missing")

	reply := assistantFinalText(events)
	require.Contains(t, strings.ToLower(reply), "optional variable ran",
		"agent should have completed successfully; got: %s", reply)
}

func TestARequiredTemplateVariableFailsWhenMissing(t *testing.T) {
	apiKey := requireOpenAIKey(t)

	client := authedClient(t)
	upsertAISettings(t, client, "openai", apiKey)

	// {this_does_not_exist} — no `?` suffix means the orchestration
	// engine's instruction processor raises when the key is absent. The
	// run fails before the LLM is called and surfaces a RUN_ERROR frame.
	body := agentBody(
		"Required Variable Agent",
		"Value: {this_does_not_exist}\n\nReply anything.",
		"openai", "gpt-4o-mini",
	)
	body["description"] = "References a required but missing key — expected to error."

	agentID := createAgent(t, client, body)

	_, events := sendMessage(t, client, "", agentID, "Run.")

	errMsg := runErrorMessage(events)
	require.NotEmpty(t, errMsg,
		"required {this_does_not_exist} must cause RUN_ERROR when the key is missing")

	// The engine wraps the missing-key error with "failed to inject
	// session state into instruction". We accept any of several keywords
	// so minor wording changes upstream don't flap the test.
	lower := strings.ToLower(errMsg)
	require.True(t,
		strings.Contains(lower, "inject") ||
			strings.Contains(lower, "session state") ||
			strings.Contains(lower, "this_does_not_exist"),
		"RUN_ERROR should reference instruction/state injection; got: %s", errMsg)
}
