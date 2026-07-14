package adkgo

import (
	"aicompass/modules/chat/thread"
	"fmt"
	"strings"
	"time"

	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

// translateCodeExecution walks an event's content parts and emits one
// CodeExecutionRan domain event per (ExecutableCode, CodeExecutionResult)
// pair that came back from the provider. Pairs are matched by the `ID`
// field — Gemini sets the same ID on both parts so the UI can render
// the source and its output in a single card.
//
// Edge cases:
//   - An ExecutableCode without a matching result is still emitted (with
//     empty Output) so the UI can at least show the generated source.
//   - A CodeExecutionResult without a prior ExecutableCode is dropped
//     silently — it would render as output-without-context, which is
//     worse than not rendering at all.
//   - Multiple pairs in the same event are supported (the model can
//     execute several times per turn).
//
// Provider-agnostic by design: the function reads `genai.Part` fields
// (ExecutableCode, CodeExecutionResult) that exist for every provider
// in the genai types; only Gemini populates them today, but when OpenAI
// or Anthropic adapters surface their own code-interpreter outputs they
// will translate through this same path.
func translateCodeExecution(evt *session.Event, ctx EventContext) []thread.Event {
	if evt == nil || evt.Content == nil {
		return nil
	}

	codes, codeOrder, results := collectCodeParts(evt.Content.Parts)
	if len(codes) == 0 {
		return nil
	}

	now := coalesceTime(evt.Timestamp)
	events := make([]thread.Event, 0, len(codes))
	for idx, id := range codeOrder {
		events = append(events, buildCodeExecutionEvent(evt.ID, idx, id, codes[id], results[id], ctx, now))
	}
	return events
}

// codeEntry is the per-ID pair of language + source collected from a stream
// of genai parts before emission.
type codeEntry struct {
	language string
	code     string
}

// collectCodeParts walks the parts once and partitions them into ordered
// code blocks (keyed by ID) and execution results (keyed by the same ID so
// they can be paired in the emission phase).
func collectCodeParts(parts []*genai.Part) (map[string]codeEntry, []string, map[string]*genai.CodeExecutionResult) {
	codes := make(map[string]codeEntry)
	var codeOrder []string
	results := make(map[string]*genai.CodeExecutionResult)

	for _, p := range parts {
		if p == nil {
			continue
		}
		if p.ExecutableCode != nil {
			id := p.ExecutableCode.ID
			codes[id] = codeEntry{
				language: strings.ToLower(string(p.ExecutableCode.Language)),
				code:     p.ExecutableCode.Code,
			}
			codeOrder = append(codeOrder, id)
		}
		if p.CodeExecutionResult != nil {
			results[p.CodeExecutionResult.ID] = p.CodeExecutionResult
		}
	}
	return codes, codeOrder, results
}

// buildCodeExecutionEvent pairs a collected code block with its matching
// execution result (if any) and materialises a thread.CodeExecutionRan.
// When the provider omitted the part ID (synthetic Outcome-only case),
// a deterministic blockID is synthesised from evt.ID + position so replay
// dedupes correctly.
func buildCodeExecutionEvent(evtID string, idx int, id string, c codeEntry, res *genai.CodeExecutionResult, ctx EventContext, now time.Time) *thread.CodeExecutionRan {
	blockID := id
	if blockID == "" {
		blockID = fmt.Sprintf("%s-%d", evtID, idx)
	}
	out := &thread.CodeExecutionRan{
		ThreadID:  ctx.ThreadID,
		MessageID: evtID,
		BlockID:   blockID,
		Language:  c.language,
		Code:      c.code,
		AgentName: ctx.AgentName,
		Timestamp: now,
	}
	if res != nil {
		out.Output = res.Output
		out.Outcome = string(res.Outcome)
	}
	return out
}
