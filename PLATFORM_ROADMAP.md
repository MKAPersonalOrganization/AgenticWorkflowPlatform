# Platform Roadmap: New Features & Refactors

Based on a comprehensive study of the underlying agent framework's capabilities versus current platform
integration. Organized by impact and effort.

---

## 1. Agent Memory Service (Cross-Session Knowledge)

**What the framework provides:**
The framework provides a `memory.Service` with two methods — `AddSessionToMemory()` and
`SearchMemory()` — plus two companion tools: `loadmemorytool` (auto-search on
each turn) and `preloadmemorytool` (pre-fetch at session start). The in-memory
implementation uses keyword-based search; the interface allows embedding-backed
implementations.

**What we have today:** Nothing. Each conversation is isolated. When a user's
agent finishes a session, the knowledge is lost.

**Proposed work:**
- Wire `memory.Service` into `RunnerAdapter` (the runner already accepts it as
  an optional field).
- Implement a persistent backend (pgvector or SQLite FTS) behind the
  `memory.Service` interface.
- Expose a per-agent toggle in the agent designer: "Remember past conversations"
  (on/off) — when enabled, `loadmemorytool` is auto-injected.
- Add a "Memory" tab in the agent settings panel where users can browse, search,
  and delete stored memories.
- State scope: memories should be scoped per-user per-agent (`user:` prefix).

**Impact:** High — agents that remember past interactions are a top differentiator
for no-code platforms. Users building tutoring agents, support agents, or
personal assistants need this.

---


## 2. Artifact Service (Versioned File Management)

**What the framework provides:**
The framework's artifact service provides Save/Load/Delete/List/Versions/GetArtifactVersion
with in-memory and GCS backends. Agents can create, read, and version files
during execution. The `loadartifactstool` lets agents pull artifacts by name.

**What we have today:** Only A2A artifact display (thread.A2AArtifactReceived).
No agent-generated artifact creation, no versioning, no user-facing file
management.

**Proposed work:**
- Implement `artifact.Service` backed by S3-compatible or local storage.
- Wire it into `RunnerAdapter` (runner already accepts it).
- Expose artifacts in the chat UI: file cards with download, preview, and
  version history.
- Allow agents to produce files (reports, CSVs, images) that persist across
  sessions.
- Add `{artifact.name}` template resolution in system prompts so agents can
  reference previously created files.

**Impact:** High — essential for agents that produce deliverables (report
generators, data analyzers, content creators).

---

## 3. Instruction Templating with Session State

**What the framework provides:**
System prompts support `{key}` placeholders that resolve from session state at
runtime. Optional keys use `{key?}` (omitted if missing). Artifact references
use `{artifact.name}`. This is handled by the framework's instruction utility.

**What we have today:** Static system prompts. No dynamic variable injection.

**Proposed work:**
- Wire `InjectSessionState` into `buildSystemPrompt()` in the adapter.
- Document the syntax in the agent designer (inline help/tooltip).
- Show available state keys in the prompt editor (autocomplete from OutputKeys
  defined in the workflow).
- Example use: a workflow where agent-1 sets `output_key: "research"`, and
  agent-2's prompt says "Based on: {research}, write a summary."

**Impact:** High — this is the glue that makes multi-agent workflows actually
useful. Currently OutputKey writes to state but prompts cannot read it
dynamically.

---

## 4. Dynamic Instruction Providers

**What the framework provides:**
An instruction provider function(ctx ReadonlyContext) string` generates instructions at
runtime based on session state, user ID, time of day, etc. Separate from the
static instruction field.

**What we have today:** `InstructionFunc` in foundation Config, but it is not
exposed to no-code users.

**Proposed work:**
- Add a "Dynamic Prompt" mode in the agent designer where users write a prompt
  template with conditionals (e.g., Handlebars/Go template syntax).
- The template has access to: `{{.UserID}}`, `{{.SessionID}}`,
  `{{.State "key"}}`, `{{.Time}}`, `{{.DayOfWeek}}`.
- Compile the template into an `InstructionProvider` at agent build time.
- This lets users build agents whose personality or behavior varies by context
  without code.

**Impact:** Medium — power feature for advanced users building context-aware
agents.

---

## 5. State Scopes in the UI

**What the framework provides:**
Three state scopes via key prefixes:
- `app:` — shared across all users and sessions (app-wide config)
- `user:` — shared across sessions for one user (user preferences)
- `temp:` — discarded after the invocation (scratch space)

**What we have today:** OutputKey writes to default (session) scope. No UI
exposure of scopes.

**Proposed work:**
- In the workflow tree builder, add a scope dropdown next to the OutputKey field:
  Session (default), User, App, Temporary.
- Auto-prefix the key at build time (`user:research` → persists across sessions).
- Show scope badges in the state inspector during chat.
- Document scope semantics in the workflow builder help panel.

**Impact:** Medium — enables persistent user preferences and app-wide shared
state (e.g., a knowledge base that all users' agents can access).

---

## 6. Toolsets (Dynamic Tool Collections)

**What the framework provides:**
`tool.Toolset` interface returns different tools based on context (agent name,
session state, user role). Tools can appear/disappear dynamically per turn.

**What we have today:** Static tool assignment per agent. MCP tools are resolved
once. No per-turn dynamic tool availability.

**Proposed work:**
- Implement a `ConditionalToolset` that enables/disables tools based on session
  state keys (e.g., "only show `submit_order` tool when state `cart_items` > 0").
- Expose this in the agent designer as "Tool Conditions" — a simple rules UI:
  "Show tool X when state key Y equals Z."
- Wire through the agent configuration's dynamic toolsets instead of static tools.

**Impact:** Medium — makes agents smarter about when to offer capabilities.
Prevents LLMs from calling tools in wrong contexts.

---

## 7. Long-Running Tool Support

**What the framework provides:**
Tools can set `IsLongRunning: true`. The framework returns a resource ID
immediately, tracks the tool in `LongRunningToolIDs`, and the client polls for
completion. This enables async operations (file processing, API calls that take
minutes).

**What we have today:** All tools are synchronous. Long operations block the
stream.

**Proposed work:**
- Add an `IsLongRunning` flag to the tool configuration in the agent designer.
- In the chat UI, show a progress indicator for long-running tools with a
  polling mechanism.
- Map long-running tool IDs in event translation to a "pending tools" UI state.
- Backend: implement a simple job queue (Redis or DB) for async tool execution.

**Impact:** Medium — enables agents to trigger slow operations (data pipelines,
document generation, external API calls) without freezing the conversation.

---

## 8. Plugin System for Users

**What the framework provides:**
Full plugin lifecycle: `OnUserMessage`, `BeforeRun`, `AfterRun`, `OnEvent`,
plus all agent/model/tool callbacks. Plugins execute sequentially; first
non-nil result wins.

**What we have today:** Only `retryandreflect` plugin is wired. No user-facing
plugin configuration.

**Proposed work:**
- Define a curated set of built-in plugins users can toggle per agent:
  - **Retry & Reflect** (already wired) — auto-retry on tool errors.
  - **Conversation Summarizer** — compress history after N turns.
  - **Content Filter** — block/redact sensitive output patterns.
  - **Rate Limiter** — cap model calls per session/minute.
  - **Audit Logger** — log all agent actions to external sink.
- Expose as checkboxes in agent settings with per-plugin configuration.
- Wire through the runner's plugin configuration at build time.

**Impact:** Medium — provides guardrails and observability without code.

---

## 9. Agent Evaluation Framework

**What the framework provides:**
Stub routes exist (`ListEvalSets`, `ListEvalResults`) but the framework is
unimplemented. The conformance testing infrastructure (replay plugin, YAML
test cases) provides a foundation.

**What we have today:** Nothing. Users cannot test their agents systematically.

**Proposed work:**
- Build an evaluation module:
  - Users define test cases: input message + expected behavior (contains text,
    calls tool, matches regex, sentiment).
  - Run test suite against agent, collect pass/fail results.
  - Show results in a dashboard with per-test-case breakdown.
- Leverage the framework's replay infrastructure for deterministic testing.
- Add a "Test" tab in the agent designer alongside "Chat" and "Settings."
- Support batch evaluation for comparing agent versions.

**Impact:** High — critical for users iterating on prompts and workflows.
Without evaluation, prompt engineering is guesswork.

---

## 10. BeforeAgentCallback as Guardrails

**What the framework provides:**
`BeforeAgentCallback` runs before agent execution and can short-circuit with
content or an error. If it returns content, the agent is skipped entirely.

**What we have today:** `BeforeRunHooks` from requirements, but no user-facing
guardrail configuration.

**Proposed work:**
- Build a "Guardrails" section in agent settings:
  - Input guardrails: block/redirect if user message matches patterns (regex,
    keyword list, or classifier agent).
  - Output guardrails: post-process agent response (via AfterAgentCallback).
- Implement as `BeforeAgentCallback` that checks rules before each agent runs.
- For workflow agents, guardrails can be set per-node (e.g., the "writer" agent
  has a PII filter but the "researcher" does not).

**Impact:** High — safety and compliance are table stakes for enterprise
no-code platforms.

---

## 11. OpenTelemetry Observability

**What the framework provides:**
Built-in OpenTelemetry integration with spans for `invoke_agent`,
`generate_content`, `execute_tool`. Attributes include agent name, model, token
usage, finish reasons.

**What we have today:** Custom logging callbacks (toolLoggingCallbacks,
modelLoggingCallbacks). No distributed tracing.

**Proposed work:**
- Enable the framework's telemetry layer and export spans to a tracing backend
  (Jaeger, Tempo, or Cloud Trace).
- Build a "Trace View" in the platform that shows the execution timeline of a
  workflow: which agent ran, which tools were called, token counts, latencies.
- Surface per-agent cost estimates based on token usage.

**Impact:** Medium — essential for debugging complex workflows and managing
LLM costs.

---

## Priority Matrix

| # | Feature | Impact | Effort | Priority |
|---|---------|--------|--------|----------|
| 1 | Agent Memory Service | High | Medium | P0 |
| 3 | Instruction Templating | High | Low | P0 |
| 9 | Evaluation Framework | High | High | P0 |
| 10 | Guardrails (BeforeAgentCallback) | High | Medium | P0 |
| 2 | Artifact Service | High | Medium | P1 |
| 5 | State Scopes in UI | Medium | Low | P1 |
| 6 | Dynamic Toolsets | Medium | Medium | P2 |
| 7 | Long-Running Tools | Medium | High | P2 |
| 8 | Plugin System | Medium | High | P2 |
| 11 | OpenTelemetry | Medium | Medium | P2 |
| 4 | Dynamic Instruction Providers | Medium | Medium | P2 |
