//go:build integration

// instruction_template_variables_test.go verifies the runtime promise
// behind the designer's "{"-triggered instruction-template picker: if the
// picker suggests a key, ADK must actually substitute it at runtime.
//
// The picker itself lives at
// GET /v1/agent-definitions/{id}/available-state-keys and is authenticated,
// so we don't hit it directly from this black-box suite. Instead we
// exercise the runtime contract: compose a workflow via /debug/agent:run,
// let ADK's instruction processor expand {key} / {key?} against session
// state, and assert the observable outcome.
//
// Template syntax under test is defined in
// .READ/adk-go/internal/llminternal/instruction_processor.go —
// `replaceMatch` expands {name}, {name?}, {scope:name}, {artifact.name}
// against session state and the artifact store. The dominant read
// pattern in real ADK samples (see
// .READ/adk-samples/python/agents/academic-research/academic_research
// and .READ/adk-samples/python/agents/small-business-loan-agent) is a
// downstream sub-agent referencing an upstream peer's `output_key` —
// which is exactly what the picker elevates to its highest tiers.
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

// Sequential(Producer, Consumer). Producer sets extracted_secret via
// its output_key; Consumer's instruction references {extracted_secret}.
// ADK's InjectSessionState expands that template before calling the
// LLM, so the Consumer can only echo the Producer's value if the
// substitution actually fired.
const peerOutputKeyWorkflowYAML = `
name: State Flow Workflow
slug: state-flow-workflow
description: "Proves ADK substitutes upstream {output_key} references at runtime."
system_prompt: "Run a two-step pipeline."
category: Tests
icon: workflow

workflow:
  id: root
  type: sequential
  children:
    - id: n1
      type: agent
      agent_id: producer
    - id: n2
      type: agent
      agent_id: consumer

llm_agents:
  - id: producer
    name: Producer
    description: Emits the word following SECRET= in the user prompt.
    output_key: extracted_secret
    system_prompt: |
      Reply with ONLY the word immediately after "SECRET=" in the user's
      message. Output a single lowercase word. No quotes, no punctuation,
      no prefix, no suffix, no sentence.

  - id: consumer
    name: Consumer
    description: Uses the extracted secret from session state.
    system_prompt: |
      The value in session state is: {extracted_secret}

      Reply with exactly this sentence, substituting <value> with the
      session-state value above:

      "The secret is <value>."
`

func TestAWorkflowPeerOutputKeyFlowsIntoDownstreamSiblingPrompt(t *testing.T) {
	apiKey := requireOpenAIKey(t)
	temperature := 0.0

	resp := postDebugRun(t, debugRequest{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		APIKey:      apiKey,
		Temperature: &temperature,
		Prompt:      "Process the following: SECRET=avocado please proceed.",
		AgentYAML:   peerOutputKeyWorkflowYAML,
	})

	require.Empty(t, resp.Errors, "runner reported errors: %v", resp.Errors)
	require.NotEmpty(t, resp.Response, "agent produced no response")

	// The Consumer can only emit "avocado" if ADK substituted the
	// Producer's output_key value into the Consumer's instruction
	// before the model call. A literal "{extracted_secret}" in the
	// prompt would not produce the word.
	require.Contains(t, strings.ToLower(resp.Response), "avocado",
		"Consumer should have echoed the Producer's output_key value; got: %s",
		resp.Response)
}

// Single agent with {nothing_here?}. ADK's `?` suffix swallows the
// missing-key error and emits an empty string so the run proceeds.
const optionalTemplateVariableYAML = `
name: Optional Variable Agent
slug: optional-variable-agent
description: "Uses {key?} syntax for an intentionally missing key."
system_prompt: |
  Marker: [{nothing_here?}]

  Reply with exactly this sentence and nothing else:

  "Optional variable ran."
category: Tests
icon: flask
`

func TestAnOptionalTemplateVariableIsSilentWhenMissing(t *testing.T) {
	apiKey := requireOpenAIKey(t)
	temperature := 0.0

	resp := postDebugRun(t, debugRequest{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		APIKey:      apiKey,
		Temperature: &temperature,
		Prompt:      "Run.",
		AgentYAML:   optionalTemplateVariableYAML,
	})

	require.Empty(t, resp.Errors,
		"optional {nothing_here?} must not error when the key is missing; got: %v",
		resp.Errors)
	require.Contains(t, strings.ToLower(resp.Response), "optional variable ran",
		"agent should have completed successfully; got: %s", resp.Response)
}

// Single agent with {this_does_not_exist} and no `?` suffix. ADK's
// instruction processor raises; the runner surfaces that error as a
// non-empty resp.Errors entry before the LLM is ever called.
const requiredTemplateVariableYAML = `
name: Required Variable Agent
slug: required-variable-agent
description: "References a required but missing key — expected to error."
system_prompt: |
  Value: {this_does_not_exist}

  Reply anything.
category: Tests
icon: flask
`

func TestARequiredTemplateVariableFailsWhenMissing(t *testing.T) {
	apiKey := requireOpenAIKey(t)
	temperature := 0.0

	resp := postDebugRun(t, debugRequest{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		APIKey:      apiKey,
		Temperature: &temperature,
		Prompt:      "Run.",
		AgentYAML:   requiredTemplateVariableYAML,
	})

	require.NotEmpty(t, resp.Errors,
		"required {this_does_not_exist} must cause an error when the key is missing")

	// ADK wraps the missing-key error with "failed to inject session
	// state into instruction" (see appendInstructions in
	// .READ/adk-go/internal/llminternal/instruction_processor.go). We
	// accept any of several keywords so minor wording changes in ADK
	// don't flap the test.
	joined := strings.ToLower(strings.Join(resp.Errors, "|"))
	require.True(t,
		strings.Contains(joined, "inject") ||
			strings.Contains(joined, "session state") ||
			strings.Contains(joined, "this_does_not_exist"),
		"error should reference instruction/state injection; got: %v",
		resp.Errors)
}
