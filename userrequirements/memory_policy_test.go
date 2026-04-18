//go:build integration

// memory_policy_test.go verifies the runtime promise behind the designer's
// Memory card: when memory is enabled on an agent, the orchestration engine
// indexes every completed session under the user's scope and later sessions
// can recall content from earlier ones.
//
// How the test works (two /debug/agent:run calls):
//
//  1. Seed call — fresh thread, same user, agent declares
//     memory_policy.enabled=true + memory_policy.preload=true, prompt
//     tells the agent a unique fact ("my favorite colour is teal").
//     The runner indexes this session in the process-wide InMemoryService
//     held by the debug handler (see modules/chat/handlers/debughandler/
//     handler.go — Handler.memoryService).
//
//  2. Recall call — NEW thread, same user, same agent definition, prompt
//     asks "what is my favorite colour?". preload_memory runs before
//     the model is called, searches the persistent memory service for
//     the user's current query, finds the seed session's text, and
//     injects it into the system instruction. The model answers "teal"
//     — observable proof the cross-session path works.
//
// If either preload_memory injection or AddSessionToMemory is broken, the
// second call has no grounding and the model will either refuse or guess.
//
// Specs exercised:
//
//	memory_policy with enabled=true + preload=true recalls facts across sessions
//	  → TestAgentWithMemoryPolicyRecallsFactsAcrossSessions
//
//	memory_policy with enabled=false isolates the agent from prior sessions
//	  → TestAgentWithoutMemoryPolicyDoesNotRecallPriorSessions
package userrequirements

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Shared agent definition — differs only in memory_policy between the two
// specs. Deliberately generic system_prompt so any recall must come from
// the injected memory context, not prompt engineering.
const memoryEnabledAgentYAML = `
name: Memory Agent
slug: memory-agent
description: "Agent with cross-session memory enabled for the user."
system_prompt: "Answer the user concisely. If you know a prior fact about the user, use it."
category: Tests
icon: brain
memory_policy:
  enabled: true
  preload: true
`

const memoryDisabledAgentYAML = `
name: No-Memory Agent
slug: no-memory-agent
description: "Agent with memory explicitly disabled."
system_prompt: "Answer the user concisely. If you don't know, say so."
category: Tests
icon: brain
memory_policy:
  enabled: false
`

func TestAgentWithMemoryPolicyRecallsFactsAcrossSessions(t *testing.T) {
	apiKey := requireOpenAIKey(t)
	temperature := 0.0

	// Unique per-test-run user so other runs don't pollute the memory
	// service that the debug handler holds process-wide. Unique fact so
	// cache-hit reasoning can't fake a pass.
	userID := "mem-user-" + uuid.NewString()
	fact := "papaya-violet"

	seedThread := "mem-seed-" + uuid.NewString()
	seed := postDebugRun(t, debugRequest{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		APIKey:      apiKey,
		Temperature: &temperature,
		UserID:      userID,
		ThreadID:    seedThread,
		Prompt:      "Please remember this for later: my favorite colour is " + fact + ". Acknowledge briefly.",
		AgentYAML:   memoryEnabledAgentYAML,
	})
	require.Empty(t, seed.Errors, "seed run reported errors: %v", seed.Errors)
	require.NotEmpty(t, seed.Response, "seed run produced no response")

	// Small gap so the seed session is fully indexed before the recall
	// call. AddSessionToMemory is called at the end of the runner loop
	// synchronously (see adapters/adkgo/runner.go around line 772), so
	// this is strictly defensive, not required for correctness.
	time.Sleep(200 * time.Millisecond)

	recallThread := "mem-recall-" + uuid.NewString()
	recall := postDebugRun(t, debugRequest{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		APIKey:      apiKey,
		Temperature: &temperature,
		UserID:      userID,
		ThreadID:    recallThread,
		Prompt:      "What is my favorite colour?",
		AgentYAML:   memoryEnabledAgentYAML,
	})
	require.Empty(t, recall.Errors, "recall run reported errors: %v", recall.Errors)
	require.NotEmpty(t, recall.Response, "recall run produced no response")

	// Only preload_memory's retrieval of the seed session can ground the
	// answer — the recall prompt itself contains no colour. If the
	// marker shows up here, the full cycle worked: AddSessionToMemory
	// indexed the seed session, SearchMemory found it, preload_memory
	// injected it into the leaf's instruction, the model used it.
	require.Contains(t, strings.ToLower(recall.Response), fact,
		"agent with memory_policy enabled should recall %q from the prior session; got: %s",
		fact, recall.Response)
}

func TestAgentWithoutMemoryPolicyDoesNotRecallPriorSessions(t *testing.T) {
	apiKey := requireOpenAIKey(t)
	temperature := 0.0

	userID := "no-mem-user-" + uuid.NewString()
	fact := "kumquat-indigo"

	// Seed as though memory were on. This proves the NEGATIVE spec
	// doesn't depend on the memory service being empty — even if
	// something is in memory for this user, the second agent's
	// enabled=false policy means its leaves don't get load_memory /
	// preload_memory and can't see anything from before.
	_ = postDebugRun(t, debugRequest{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		APIKey:      apiKey,
		Temperature: &temperature,
		UserID:      userID,
		ThreadID:    "seed-" + uuid.NewString(),
		Prompt:      "Please remember: my favorite fruit is " + fact + ". Acknowledge briefly.",
		AgentYAML:   memoryEnabledAgentYAML,
	})

	time.Sleep(200 * time.Millisecond)

	recall := postDebugRun(t, debugRequest{
		Provider:    "openai",
		Model:       "gpt-4o-mini",
		APIKey:      apiKey,
		Temperature: &temperature,
		UserID:      userID,
		ThreadID:    "recall-" + uuid.NewString(),
		Prompt:      "What is my favorite fruit?",
		AgentYAML:   memoryDisabledAgentYAML,
	})
	require.Empty(t, recall.Errors, "recall run reported errors: %v", recall.Errors)

	// The unique marker only exists in the seed session's indexed
	// memory. The disabled agent has no memory tools in its tool list,
	// so the leaf's LLM call can't fetch it. Absence of the marker
	// proves enabled=false actually withholds the tools — the model's
	// natural answer to "what is my favorite fruit?" without grounding
	// will not spontaneously hit "kumquat-indigo".
	require.NotContains(t, strings.ToLower(recall.Response), fact,
		"agent with memory_policy disabled should not see %q from prior sessions; got: %s",
		fact, recall.Response)
}
