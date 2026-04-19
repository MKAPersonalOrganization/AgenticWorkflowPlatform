//go:build integration

// streaming_policy_test.go verifies the runtime promise behind the
// designer's Streaming toggle: StreamingPolicy flips
// RunConfig.StreamingMode on ADK's runner, and the SSE the frontend
// sees reflects that choice.
//
// Mechanism (see adapters/adkgo/runner.go — resolveStreamingMode, and
// adapters/adkgo/runner_loop.go — onAssistantContentStreamed /
// onAssistantMessageAdded):
//
//   - Streaming ON  : ADK emits many AssistantContentStreamed events
//     during the LLM call. Each becomes a TEXT_MESSAGE_CONTENT frame
//     with a small delta, plus exactly one AssistantMessageAdded at
//     the end. The frontend accumulates the deltas into the final
//     bubble.
//   - Streaming OFF : ADK emits zero AssistantContentStreamed during
//     the call; a single AssistantMessageAdded arrives at the end.
//     The runner loop synthesises one Started + one Streamed (whole
//     text as a single delta) + one Completed so the frontend still
//     has a lifecycle to render — crucially this means exactly one
//     TEXT_MESSAGE_CONTENT frame carries the whole reply.
//
// Tests run against OpenAI's Responses API (gpt-4o-mini) — the same
// provider the designer defaults to in Chat Completions mode, now
// driven through adapters/adkgo/openai_responses.go.
package userrequirements

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// streamingPrompt is deliberately long enough that OpenAI's Responses
// API will split the reply across multiple output_text.delta events —
// short replies occasionally arrive in a single frame even with
// streaming on, since the provider batches small responses.
const streamingPrompt = "List the planets of the solar system from Mercury outward in one short paragraph, mentioning each by name with one distinguishing feature."

func streamingAgentBody(name, provider, model string, streamingEnabled bool) map[string]any {
	body := map[string]any{
		"name":         name,
		"description":  "Agent used to assert StreamingPolicy → RunConfig.StreamingMode wiring.",
		"systemPrompt": "You answer every question with a single short paragraph.",
		"icon":         "zap",
		"modelConfig":  map[string]any{"temperature": 0.0},
		"modelOverride": map[string]any{
			"provider":  provider,
			"modelName": model,
		},
	}
	if !streamingEnabled {
		// Only materialise a policy when turning streaming OFF — matches
		// the designer's payload shape, where ON is represented by a nil
		// policy so the backend falls back to ADK's SSE default.
		body["streamingPolicy"] = map[string]any{"enabled": false}
	}
	return body
}

// countTextFrames reports how many TEXT_MESSAGE_CONTENT frames carried
// non-empty delta text. TEXT_MESSAGE_CONTENT is a top-level AG-UI event
// (not CUSTOM), so the delta lives at the root of the data object —
// mirrors assistantFinalText in shared_test.go.
func countTextFrames(events []sseEvent) int {
	n := 0
	for _, e := range events {
		if e.Name() != "TEXT_MESSAGE_CONTENT" {
			continue
		}
		var p struct {
			Delta string `json:"delta"`
		}
		if err := json.Unmarshal(e.Data, &p); err != nil {
			continue
		}
		if p.Delta != "" {
			n++
		}
	}
	return n
}

// countTextMessageStarts reports how many TEXT_MESSAGE_START frames
// appeared. Exactly one assistant bubble should open per run regardless
// of streaming mode — that invariant is part of the runner-loop
// contract and we assert it in both cases so a regression in the
// lifecycle synthesiser is caught.
func countTextMessageStarts(events []sseEvent) int {
	n := 0
	for _, e := range events {
		if e.Name() == "TEXT_MESSAGE_START" {
			n++
		}
	}
	return n
}

// countTextMessageEnds reports the number of TEXT_MESSAGE_END frames.
// Mirrors countTextMessageStarts — every opened bubble must close.
func countTextMessageEnds(events []sseEvent) int {
	n := 0
	for _, e := range events {
		if e.Name() == "TEXT_MESSAGE_END" {
			n++
		}
	}
	return n
}

// TestAgentWithStreamingPolicyOffCollapsesToOneFrameOnOpenAI — with
// StreamingPolicy.Enabled=false the adapter passes
// agent.StreamingModeNone to ADK; the runner loop synthesises a single
// Started + single Streamed + single Completed lifecycle. Assert:
//
//   - exactly one TEXT_MESSAGE_START / END pair (bubble lifecycle),
//   - exactly one TEXT_MESSAGE_CONTENT frame carrying the whole reply,
//   - the reply contains the expected content.
func TestAgentWithStreamingPolicyOffCollapsesToOneFrameOnOpenAI(t *testing.T) {
	t.Parallel()
	requireOpenAIKey(t)
	client := authedClient(t)

	agentID := createAgent(t, client, streamingAgentBody("streaming-off-openai", "openai", "gpt-4o-mini", false))

	_, events := sendMessage(t, client, "", agentID, streamingPrompt)
	text := assistantFinalText(events)
	require.Contains(t, text, "Mercury",
		"expected the agent to answer the planet question; got: %s", text)

	require.Equal(t, 1, countTextMessageStarts(events),
		"streaming off should open exactly one assistant bubble; got %d TEXT_MESSAGE_START frames",
		countTextMessageStarts(events))
	require.Equal(t, 1, countTextMessageEnds(events),
		"streaming off should close exactly one assistant bubble; got %d TEXT_MESSAGE_END frames",
		countTextMessageEnds(events))

	got := countTextFrames(events)
	require.Equal(t, 1, got,
		"streaming off should deliver the reply in exactly one TEXT_MESSAGE_CONTENT frame; got %d", got)
}

// TestAgentWithStreamingPolicyOnEmitsMultipleDeltasOnOpenAI — with no
// policy set (nil → default) the adapter passes
// agent.StreamingModeSSE; ADK emits per-chunk AssistantContentStreamed
// events which the AG-UI pipeline translates to multiple
// TEXT_MESSAGE_CONTENT frames. Assert:
//
//   - exactly one TEXT_MESSAGE_START / END pair (one bubble), AND
//   - strictly more than one TEXT_MESSAGE_CONTENT frame (i.e.
//     streaming genuinely fanned the reply into deltas), AND
//   - the concatenated text still matches the expected content.
func TestAgentWithStreamingPolicyOnEmitsMultipleDeltasOnOpenAI(t *testing.T) {
	t.Parallel()
	requireOpenAIKey(t)
	client := authedClient(t)

	agentID := createAgent(t, client, streamingAgentBody("streaming-on-openai", "openai", "gpt-4o-mini", true))

	_, events := sendMessage(t, client, "", agentID, streamingPrompt)
	text := assistantFinalText(events)
	require.Contains(t, text, "Mercury",
		"expected the agent to answer the planet question; got: %s", text)

	require.Equal(t, 1, countTextMessageStarts(events),
		"streaming on should open exactly one assistant bubble; got %d TEXT_MESSAGE_START frames",
		countTextMessageStarts(events))
	require.Equal(t, 1, countTextMessageEnds(events),
		"streaming on should close exactly one assistant bubble; got %d TEXT_MESSAGE_END frames",
		countTextMessageEnds(events))

	got := countTextFrames(events)
	require.Greater(t, got, 1,
		"streaming on should produce more than one TEXT_MESSAGE_CONTENT frame for a paragraph reply; got %d (text: %q)", got, text)
}
