//go:build integration

// memory_policy_test.go verifies the runtime promise behind the
// designer's Memory card: when memory is enabled on an agent, the
// orchestration engine indexes every completed session under the user's
// scope and later sessions can recall content from earlier ones. When
// disabled, the agent's tool list has neither load_memory nor
// preload_memory, so the LLM can't see anything from prior sessions.
//
// The tests run a seed → recall pair of messages, each on its own
// thread (contextId="" opens a fresh thread). The debug user is shared
// across tests, so markers carry a unique UUID suffix per run to avoid
// cross-test contamination in the user's indexed memory.
//
// Specs exercised:
//
//	memoryPolicy with enabled=true + preload=true recalls facts across sessions
//	  → TestAgentWithMemoryPolicyRecallsFactsAcrossSessions
//
//	memoryPolicy with enabled=false isolates the agent from prior sessions
//	  → TestAgentWithoutMemoryPolicyDoesNotRecallPriorSessions
package userrequirements

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// memoryEnabledAgent creates a fresh agent with memoryPolicy.enabled=true
// (plus preload=true so previously-indexed sessions are searched before
// every turn). Returns the new agent's id.
func memoryEnabledAgent(t *testing.T, client *http.Client, name, systemPrompt string) string {
	t.Helper()
	body := agentBody(name, systemPrompt, "openai", "gpt-4o-mini")
	body["description"] = "Agent with cross-session memory enabled."
	body["memoryPolicy"] = map[string]any{
		"enabled": true,
		"preload": true,
	}
	return createAgent(t, client, body)
}

// memoryDisabledAgent creates a fresh agent with memoryPolicy.enabled=false.
func memoryDisabledAgent(t *testing.T, client *http.Client, name, systemPrompt string) string {
	t.Helper()
	body := agentBody(name, systemPrompt, "openai", "gpt-4o-mini")
	body["description"] = "Agent with memory explicitly disabled."
	body["memoryPolicy"] = map[string]any{
		"enabled": false,
	}
	return createAgent(t, client, body)
}

func TestAgentWithMemoryPolicyRecallsFactsAcrossSessions(t *testing.T) {
	t.Parallel()
	requireOpenAIKey(t)
	client := authedClient(t)

	// Unique marker so repeated test runs under the same debug user
	// can't fake a pass by hitting an old session in the index.
	fact := "papaya-violet-" + uuid.NewString()[:8]

	agentID := memoryEnabledAgent(t, client,
		"Memory Agent",
		"Answer the user concisely. If you know a prior fact about the user, use it.",
	)

	// Seed session — a fresh thread (contextId left empty).
	_, seedEvents := sendMessage(t, client, "", agentID,
		"Please remember this for later: my favorite colour is "+fact+". Acknowledge briefly.")
	require.Empty(t, runErrorMessage(seedEvents), "seed run should not error")

	// AddSessionToMemory runs at the end of the runner loop
	// synchronously, but the new session needs a beat to be indexable.
	time.Sleep(500 * time.Millisecond)

	// Recall session — NEW thread, same agent, same user. The recall
	// prompt itself carries no colour; only a memory lookup can ground
	// the answer.
	_, recallEvents := sendMessage(t, client, "", agentID, "What is my favorite colour?")
	require.Empty(t, runErrorMessage(recallEvents), "recall run should not error")
	reply := assistantFinalText(recallEvents)
	require.NotEmpty(t, reply)

	require.Contains(t, strings.ToLower(reply), fact,
		"agent with memoryPolicy enabled should recall %q; got: %s", fact, reply)
}

func TestAgentWithoutMemoryPolicyDoesNotRecallPriorSessions(t *testing.T) {
	t.Parallel()
	requireOpenAIKey(t)
	client := authedClient(t)

	fact := "kumquat-indigo-" + uuid.NewString()[:8]

	// Seed uses the enabled agent so the fact lands in the user's
	// memory index. This proves the negative spec doesn't rely on the
	// memory store being empty — even with state there, the disabled
	// agent can't see it.
	seedAgentID := memoryEnabledAgent(t, client,
		"Seed Agent",
		"Answer the user concisely. If you know a prior fact about the user, use it.",
	)
	_, seedEvents := sendMessage(t, client, "", seedAgentID,
		"Please remember: my favorite fruit is "+fact+". Acknowledge briefly.")
	require.Empty(t, runErrorMessage(seedEvents), "seed run should not error")

	time.Sleep(500 * time.Millisecond)

	// Recall with a DISABLED-memory agent. Its tool list has no
	// load_memory / preload_memory, so the leaf's LLM call has no way
	// to reach into the user's memory index.
	disabledAgentID := memoryDisabledAgent(t, client,
		"No-Memory Agent",
		"Answer the user concisely. If you don't know, say so.",
	)
	_, recallEvents := sendMessage(t, client, "", disabledAgentID, "What is my favorite fruit?")
	require.Empty(t, runErrorMessage(recallEvents), "recall run should not error")
	reply := assistantFinalText(recallEvents)

	// The marker only exists in the seed's indexed memory. Without
	// memory tools, the model can't fetch it. Absence of the marker
	// proves enabled=false actually withholds the tools.
	require.NotContains(t, strings.ToLower(reply), fact,
		"agent with memoryPolicy disabled should not recall %q; got: %s", fact, reply)
}
