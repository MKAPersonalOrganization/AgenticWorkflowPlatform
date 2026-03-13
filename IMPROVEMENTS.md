# Improvement Backlog

Findings from the ADK-Go integration cleanup and the `plugins/websearch` decoupling work.
Each item is self-contained and can be tackled independently.

---

## 1. `Invocation.History` is never populated by the ADK adapter

**Where:** `adapters/adkgo/runner.go` — the `Invocation` built in `resolveTools()` and
`buildBeforeRunHooks()` never sets `History`.

**Impact:** `ConditionalRequirement.extractToolHistory` scans `inv.History` to count
steps and detect which tools were used last. Because the slice is always empty, every
turn looks like "step 1 / no tool used yet". This means:

- `ForceAtStep(1)` fires on every turn, not just the first.
- `ForceAfter`, `ForceAfterAny`, `MaxInvocations`, `MinInvocations`, `OnlyAfter`, and
  `PreventConsecutive` never work correctly.

**Fix:** In `RunnerAdapter.Run`, after `BuildSessionFromThread` succeeds, translate
`agg.Messages` into `[]*fagent.Message` and assign to `inv.History` before the first
`Invocation` is constructed. The thread aggregate is already in memory at that point.

---

## 2. Requirements are evaluated twice per invocation

**Where:** `resolveTools()` (line ~468) and `buildBeforeRunHooks()` (line ~905) both
call `requirement.Run(ctx, inv)` on the same set of requirements for the same request.

**Impact:** Double execution cost; any requirement with side-effects (e.g. a DB lookup)
runs twice. Also, both use a fresh `Invocation` with no `History` (see item 1), so the
results are identical and the second evaluation adds no value over the first.

**Fix:** Evaluate requirements once in `Run()`, capture the resulting `[]Rule`, then
pass the pre-computed rules into both `resolveTools()` (for Blocked/Hidden filtering)
and `buildBeforeRunHooks()` (for Forced/PreventStop guidance injection). This also makes
it easier to fix item 1, because History only needs to be populated in one place.

---

## 3. `buildBeforeRunHooks` guidance is written to session state but never read

**Where:** `runner.go:941` — `ctx.State().Set("_adk_requirement_guidance", guidance)`.

**Impact:** The guidance text telling the LLM which tools are forced or blocked is stored
in the ADK session state, but neither the system prompt nor the ADK event loop reads that
key back and surfaces it to the model. The LLM never sees the guidance.

**Fix (option A — simpler):** Return a `*genai.Content` from the `BeforeAgentCallback`
with the guidance as a `"system"` role part. ADK injects it into the current turn.

**Fix (option B — cleaner):** Inject guidance directly into the session's current message
instead of a named state key, following the pattern used by ADK's built-in planner.

---

## 4. `dbAgent` should implement `Configurable` directly

**Where:** `adapters.go:374-400` (`dbAgent`) and `runner.go:724-751` (`extractAgentConfig`).

**Impact:** `extractAgentConfig` uses a two-strategy fallback: first it tries
`agent.Configurable`, then it tries the bespoke `embeddedAgentAccessor` interface to
unwrap `dbAgent → configAgent → Config()`. This is fragile — a new wrapper type that
forgets to implement `EmbeddedAgent()` would silently receive a nil config.

**Fix:** Make `dbAgent` implement `agent.Configurable` directly:

```go
func (a *dbAgent) Config() agent.Config { return a.Agent.(agent.Configurable).Config() }
```

Then `extractAgentConfig` only needs the single `agent.Configurable` check and
`EmbeddedAgent()` / `embeddedAgentAccessor` can be removed.

---

## 5. `websearch.globalLimiter` is a package-level singleton

**Where:** `plugins/websearch/web_search_tool.go:133`.

**Impact:** The rate limiter is shared across all tests and all instances of
`WebSearchTool`, so tests that exhaust the rate limit for a user ID bleed into
subsequent tests (or into production instances created after test setup).

**Fix:** Move `rateLimiter` into `WebSearchTool` itself. `NewWebSearchTool` can accept
an optional `rateLimiter` parameter (or just always construct one internally). Tests that
need to verify rate-limiting behaviour create a tool with their own limiter.

---

## 6. `adapters/adkgo/runner.go` still imports `google.golang.org/genai` directly

**Where:** `runner.go:17` — imported for `genai.GenerateContentConfig`,
`genai.Content`, and `genai.Part` in `buildGenerateContentConfig` and `buildUserContent`;
and for `genai.Content` in `buildBeforeRunHooks`.

**Context:** The `adapters/adkgo` package is explicitly the framework adapter layer, so
genai imports here are more acceptable than they were in `plugins/websearch`. However,
`buildGenerateContentConfig` and `buildUserContent` are pure translation functions that
could live in a narrower internal file so the genai surface is contained in one place
rather than spread across runner.go.

**Fix:** Move `buildGenerateContentConfig`, `buildUserContent`, and the
`BeforeAgentCallback` content-building helpers into `adapters/adkgo/model.go` (already
the home for model-specific code). `runner.go` then imports only `google.golang.org/adk`
types, not `google.golang.org/genai` directly.

---

## 7. Go linter suggestions (low-effort, high-cleanliness)

The following are flagged by the IDE linter and have straightforward mechanical fixes:

| File | Line | Suggestion |
|------|------|------------|
| `plugins/websearch/web_search_tool.go` | 205 | `for i := range len(sorted)` (range-over-int) |
| `plugins/websearch/web_search_tool.go` | 221 | `fmt.Fprintf(&marker, "[%d]", idx)` instead of `WriteString(fmt.Sprintf(...))` |
| `plugins/websearch/web_search_tool.go` | 225 | `min(cit.EndIndex, len(runes))` instead of explicit if |
| `plugins/websearch/web_search_tool_test.go` | 165, 174, 184 | `for range rateLimitMax` (range-over-int) |

All require Go 1.22+ (range-over-int) and 1.21+ (`min` builtin), both of which this
module already targets.

---

## 8. `FunctionMiddlewareProvider` is undocumented in the public interface

**Where:** `adapters/adkgo/runner.go:796` references `agent.FunctionMiddlewareProvider`
but this interface is not mentioned in `foundation/agent/requirement.go` alongside
`ToolProvider`, `PromptProvider`, and `Enableable`.

**Impact:** Developers adding a new Requirement that wants to inject middleware have no
discovery path — they must read runner.go to find out the fourth optional interface.

**Fix:** Add `FunctionMiddlewareProvider` to the optional-interfaces block comment in
`requirement.go`, matching the style of `ToolProvider` / `PromptProvider` / `Enableable`.

---

## 9. `RunnerAdapter.ListAgents` uses a hidden type assertion

**Where:** `runner.go:602-615` — casts `AgentProvider` to an unnamed local `lister`
interface to call `List()`.

**Impact:** `AgentProvider` in `ports.go` only declares `GetForUser`. Any provider that
has a `List()` method works, but this is invisible at the call site and untestable via the
interface.

**Fix:** Add `List() []agent.Agent` to `AgentProvider` in `ports.go`, or create a
separate `AgentLister` interface and document the opt-in pattern, matching the style of
the other narrow interfaces in `ports.go`.
