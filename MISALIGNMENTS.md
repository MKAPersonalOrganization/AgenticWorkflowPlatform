# MISALIGNMENTS — Agent Designer vs ADK-Go

> **Goal.** Our agent designer (the UI where non-programmers graphically build agents) should expose, as faithfully as practical, the feature surface offered by [`adk-go`](./.READ/adk-go). This document catalogs **orthogonal** gaps so multiple developers can work in parallel without stepping on each other.
>
> **Guiding principles.** Orthogonality, simplicity, reusability, avoid over-engineering. Prefer ADK-native primitives over bespoke ones. If a field is too raw for non-programmers, wrap it in a preset — but don't hide it behind an abstraction that drifts from ADK's model.

---

## Table of contents

1. [Modular architecture recap](#1-modular-architecture-recap)
2. [End-to-end workflow](#2-end-to-end-workflow)
3. [Orthogonal misalignments (parallelizable work items)](#3-orthogonal-misalignments)
   - [M1. Agent-as-Tool node type](#m1-agent-as-tool-node-type) ✅ **done**
   - [M2. Planner preset (ReAct / Plan-then-Act / None)](#m2-planner-preset)
   - [M3. Model parameter controls (the rest of `GenerateContentConfig`)](#m3-model-parameter-controls)
   - [M4. Input-schema visual builder (remove dead code or ship it)](#m4-input-schema-visual-builder) ✅ **done**
   - [M5. Output-schema visual builder (replace raw JSON textarea)](#m5-output-schema-visual-builder) ✅ **done**
   - [M6. Instruction template variable picker](#m6-instruction-template-variable-picker) ✅ **done**
   - [M7. Global-instruction UX (root-only surfacing)](#m7-global-instruction-ux) ✅ **done**
   - [M8. Memory service toggle + backend selector](#m8-memory-service-toggle) ✅ **done**
   - [M9. Artifact service — surface the `SaveInputBlobsAsArtifacts` toggle](#m9-artifact-service) ✅ **done**
   - [M10. Built-in code-execution toggle](#m10-code-execution-toggle) ✅ **done**
   - [M11. Long-running-tool surfacing in chat timeline](#m11-long-running-tools)
   - [M12. A2A remote-agent node configuration](#m12-a2a-remote-agent)
   - [M13. MCP toolset picker at agent level](#m13-mcp-toolset-picker)
   - [M14. Durable session service (swap `InMemoryService` → Postgres)](#m14-durable-session-service)
   - [M15. Evaluation-set editor + runner](#m15-evaluation-sets)
   - [M16. Callback presets (non-programmer-friendly)](#m16-callback-presets)
   - [M17. Built-in ADK tool catalog visibility](#m17-builtin-adk-tool-catalog)
   - [M18. Streaming-mode toggle](#m18-streaming-mode-toggle) ✅ **done**
   - [M19. Agent-card export (expose our agent as A2A)](#m19-agent-card-export)
   - [M20. Tool-confirmation hint UX](#m20-tool-confirmation-hint-ux)
   - [M21. OpenAI Responses API migration (unlocks code interpreter, file search, web search, computer use)](#m21-openai-responses-api-migration) ✅ **done**
4. [Custom patterns we should replace with ADK natives](#4-custom-vs-adk-native)
5. [Cross-cutting UI cleanup](#5-cross-cutting-ui-cleanup)
6. [Scope guards — things we deliberately are *not* building](#6-scope-guards)
7. [Suggested dependency graph for scheduling](#7-suggested-dependency-graph)

---

## 1. Modular architecture recap

Everyone working on this doc needs the same mental model. Keep this chart handy.

```
┌─────────────────────────── FRONTEND ────────────────────────────┐
│ frontend/ui/agenteditor/agenteditor.templ        ← designer UI  │
│ frontend/ui/agents/agents.templ                  ← agent picker │
│ frontend/ui/agentcapabilities/agentcapabilities.templ           │
│ assets/js/components/agent-designer-dialog.js   ← Alpine store  │
│ assets/js/components/workflow-tree-builder.js   ← tree renderer │
└──────────────────────────┬──────────────────────────────────────┘
                           │  HTTP (JSON) + SSE (AG-UI events)
┌──────────────────────────▼──────────────────────────────────────┐
│ HTTP LAYER  (modules/chat/handlers/, modules/agentdefinition/)  │
│   - messagehandler     → runs an agent                          │
│   - agenthandler       → CRUD for agent definitions             │
│   - agent-catalog      → fork-a-template                        │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│ DOMAIN / APPLICATION                                            │
│   modules/agentdefinition/   ← persisted `Config` per user      │
│   modules/chat/agentorchestrator/  ← run orchestration          │
│   foundation/agent/config.go ← framework-agnostic `Config`      │
│   foundation/toolregistry/   ← tools (built-in + MCP + A2A)     │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│ ADAPTER LAYER                                                   │
│   adapters/adkgo/runner.go        ← translates Config → ADK     │
│   adapters/adkgo/agent_builder.go ← builds llmagent.Config      │
│   adapters/adkgo/workflow.go      ← seq/parallel/loop wiring    │
│   adapters/adkgo/tools.go         ← wraps ExecutableTool        │
│   adapters/adkgo/callbacks.go     ← confirmation, logging, block│
│   adapters/adkgo/session.go       ← thread → session replay     │
│   adapters/adkgo/events.go        ← ADK Event → domain event    │
└──────────────────────────┬──────────────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────────────┐
│ ADK-GO   (.READ/adk-go/…)                                       │
│   agent/llmagent, workflowagents, remoteagent                   │
│   tool/functiontool, agenttool, mcptoolset, geminitool, …       │
│   session, memory, artifact, plugin, runner, telemetry          │
└─────────────────────────────────────────────────────────────────┘
```

Rule of thumb for every misalignment below:

- **Config model** lives in `foundation/agent/config.go`.
- **Persistence** lives in `modules/agentdefinition/` (repo + service + handler).
- **Translation to ADK** lives in `adapters/adkgo/`.
- **Designer UI** lives in `frontend/ui/agenteditor/` + `assets/js/components/agent-designer-dialog.js`.
- **Chat rendering** lives in `frontend/ui/chat*` + `modules/chat/`.

If your change crosses none of those, you're probably in the wrong place.

---

## 2. End-to-end workflow

One pass through the system so any dev knows where their piece fits.

```
Designer UI (agent-designer-dialog.js)
   │  user fills form → POST /v1/agent-definitions (JSON payload)
   ▼
modules/agentdefinition/handler.go
   │  validates & persists to `agent_definitions` table
   ▼
modules/agentdefinition/service.go → foundation/agent.Config
   │
   ├─ (runtime) user sends message → POST /v1/threads/.../messages
   ▼
modules/chat/handlers/messagehandler/handler.go
   │  loads thread + user + agent def
   ▼
modules/chat/agentorchestrator/service.go
   │  orchestrates event persistence + SSE stream
   ▼
adapters/adkgo/runner.go
   │  1. resolveModel      (aisettings + ModelOverride)
   │  2. evaluateRequirements
   │  3. resolveTools      (static + requirements + toolreqs + MCP + A2A)
   │  4. buildSystemPrompt (SystemPrompt + Requirements + KB + user ctx)
   │  5. buildLLMAgent     (llmagent.New)
   │  6. attachCallbacks   (logging, block, confirmation)
   │  7. injectAdkTools    (preload_memory, load_memory, load_artifacts)
   │  8. runner.New + RetryAndReflect plugin
   │  9. runner.Run        (streams session.Event)
   ▼
adapters/adkgo/events.go → domain thread events
   │
   ▼
modules/chat/projection/agui → AG-UI SSE frames
   │
   ▼
Frontend (assets/js/chat/*) → renders live messages, tool calls, confirmations
```

Everything in §3 slots somewhere along this pipeline. Each item names its slots explicitly.

---

## 3. Orthogonal misalignments

Each item below is a self-contained work package. It names:

- **What ADK-Go offers** (so you know the target)
- **Where we are today** (so you know the gap)
- **Workflow deltas** (back ↔ front)
- **Files to touch** (scoped so items don't collide)
- **UI deltas** (what the designer UI needs)
- **Acceptance criteria**
- **Does-not-touch** (what to keep your hands off so parallel work stays orthogonal)

---

### M1. Agent-as-Tool node type

> **Status: ✅ Done.** Shipped as a peer list on the agent definition (not a composition-tree node). Docs updated in `foundation/agent/config.go`, `frontend/ui/docs/agents.templ`, and the landing pages (`agents.templ`, `multiagent.templ`, `features.templ`). See the commit touching `adapters/adkgo/agent_tools.go` + migration `035_add_agent_tools.sql`.

**ADK.** `tool/agenttool/agenttool.go` — `agenttool.New(agent, *Config)` wraps an `agent.Agent` as a `tool.Tool`. Config: `SkipSummarization`. The wrapped agent's `InputSchema` becomes the tool's signature (defaults to `{request: string}`).

**Today.** We have `SubAgents` (transfer-based delegation) but no way to expose another saved agent *as a callable tool* from the designer. Non-programmers can't say "when agent A decides to do X, call agent B as a tool and keep control."

**Workflow.**

```
Designer → selects "Agent as Tool" node in composition tree
         → picks a target agentDefinitionID from the user's own agents
         → toggles "Skip summarization"
Backend  → resolves target agent at build time
         → wraps via agenttool.New() and appends to parent's Tools
```

**Files.**
- Backend: `foundation/agent/config.go` (new `AgentToolRef` struct on `Config`), `adapters/adkgo/agent_builder.go` (wrap via `agenttool.New`), `adapters/adkgo/tools.go` (register).
- Persistence: `modules/agentdefinition/types.go` + repo migration (new column `agent_tool_refs` JSONB).
- Frontend: new case in `workflow-tree-builder.js` (`renderCompositionNode`) + a new form in `agenteditor.templ`.

**UI delta.** In the composition tree node-type selector (currently: LLM / Sequential / Parallel / Loop / Remote), add **"Agent-as-Tool"**. Picker: dropdown of user's other agents + `Skip summarization` toggle.

**Acceptance.** A non-programmer can, without writing code, make Agent A expose Agent B as a callable tool, and see B's tool-call show up in the chat timeline as a tool execution (not as a sub-agent transfer).

**Does-not-touch.** Sub-agent transfer behaviour, workflow-agent orchestration (M12 handles A2A).

---

### M2. Planner preset

**ADK.** ADK-Go has no first-class `Planner` type in the public API (confirmed by scanning `.READ/adk-go/agent` — the ReAct pattern is expressed via instruction + tool ordering). But **users see "planner" as a conceptual primitive** (ReAct, Plan-and-Execute).

**Today.** No planner concept at all. Non-programmers have to hand-craft a ReAct-style prompt.

**Workflow.**

```
Designer → selects from a "Reasoning style" dropdown:
           • None  • ReAct (think → act → observe loop)  • Plan-then-Act
Backend  → planner choice maps to:
           - a canned Requirement set (see foundation/agent/requirement_conditional.go)
           - and/or an appended instruction block
         → appended to SystemPrompt at buildSystemPrompt()
```

**Files.**
- Backend: `foundation/agent/config.go` (new `Planner string`), `foundation/agent/requirement.go` (canned ReAct requirement), `adapters/adkgo/agent_builder.go`.
- Frontend: `agenteditor.templ` (dropdown), `agent-designer-dialog.js` (form field + payload).

**UI delta.** One dropdown in "Behaviour" section, below `System Prompt`. Tooltip: one-sentence explanation of each.

**Acceptance.** Selecting "ReAct" produces the same observable behaviour as the canonical ReAct example in `.READ/adk-go/examples/` — model emits a `think` step before tool calls.

**Does-not-touch.** System-prompt editor (M6), model params (M3), callbacks (M16).

---

### M3. Model parameter controls

**ADK.** `genai.GenerateContentConfig` supports `Temperature`, `TopP`, `TopK`, `MaxOutputTokens`, `StopSequences`, `FrequencyPenalty`, `PresencePenalty`, `ResponseMimeType`, `SafetySettings`. Our `AgentModelConfig` (`foundation/agent/config.go:37-52`) covers temp/topP/topK/maxTokens/stopSequences **but the UI only exposes some of them**.

**Today.** `agent-designer-dialog.js` collects `modelConfig.temperature, topP, topK, maxOutputTokens, stopSequences[]`. No frequency/presence penalty, no safety settings, no response MIME type (the output-schema field implicitly sets it, which is confusing). Some controls may not be rendered (verify in `agenteditor.templ`).

**Workflow.**

```
Designer slider → payload → AgentModelConfig (+ new fields) → adapters/adkgo/model.go
       → buildGenerateContentConfig() merges into genai.GenerateContentConfig
```

**Files.**
- Backend: add `FrequencyPenalty *float64`, `PresencePenalty *float64`, `SafetySettings []SafetySetting` to `AgentModelConfig`. Update `adapters/adkgo/model.go` (`buildGenerateContentConfig`). Guard OpenAI-reasoning models (already done for `Temperature` — apply same guard).
- Frontend: collapsible **"Generation"** section with labelled sliders, all with sensible defaults and reset buttons. Stop-sequences as chip-input.

**UI delta.** One clean "Generation parameters" card. Sliders with numeric displays. Safety settings as four dropdowns (HARASSMENT / HATE / SEXUALLY_EXPLICIT / DANGEROUS_CONTENT × BLOCK_NONE / LOW / MED / HIGH).

**Acceptance.** Each slider's value survives a save/reload round-trip and is visibly reflected in the generated `genai.GenerateContentConfig` (add a unit test in `adapters/adkgo/model_config_test.go`).

**Does-not-touch.** Model *selection* (that's `ModelOverride`, already wired).

---

### M4. Input-schema visual builder

> **Status: ✅ Done.** Reframed after reading ADK source (`agent/llmagent/llmagent.go:253` — "The input schema when agent is used as a tool"): InputSchema is an **inbound** signature that only matters when the agent is wrapped via `agenttool.New` (M1). Moved it out of the mixed "Data Contracts" block into its own **"Called as a Tool"** section peer to "Agents as Tools" (outbound). Replaced the raw JSON textarea with a row-based builder (name / type / required / description) plus a JSON-fallback view for complex schemas. OutputSchema is now a separate standalone section since it applies universally. See `frontend/ui/agenteditor/agenteditor.templ` (Called-as-a-Tool section) + `assets/js/components/agent-designer-dialog.js` (`hydrateInputSchemaRows` / `serializeInputSchema`). Backend untouched — round-trips through the existing `InputSchemaJSON` string.

**ADK.** `llmagent.Config.InputSchema (*genai.Schema)` — defines the function-call signature when an agent is invoked as a tool (see M1).

**Today.** `agent-designer-dialog.js` has an `inputSchema` field in `form` but the exploration found it **never rendered** (`agenteditor.templ:519` is dead code). That's a fork in the road: either ship the UI or delete the field.

**Workflow.**

```
Designer (row-based builder) → JSON schema object → persisted on Config.InputSchema
   used only when agent is wrapped via agenttool (M1) or exported as A2A (M19)
```

**Files.**
- Frontend: new component `frontend/ui/agenteditor/schemabuilder.templ` + a small JS helper (rows with name / type / required / description). Shared between M4 and M5.
- Backend: no change — `InputSchema *Schema` already exists in `foundation/agent/config.go:130`.

**UI delta.** Row-based editor: each row = `{name, type: string|number|bool|array|object, required, description}`. Hide behind a "Define input arguments" toggle so simple agents don't see it.

**Acceptance.** Non-programmer can define `{topic: string, depth: number}` without typing JSON. Round-trips to `*genai.Schema` via helper in a new `foundation/agent/io/schema_builder.go`.

**Does-not-touch.** Output schema (M5 ships the same builder for output).

---

### M5. Output-schema visual builder

> **Status: ✅ Done.** Extracted the M4 row builder into a shared `@schemaBuilder(fieldKey, emptyMessage)` templ and a generic JS helper family (`hydrateSchemaRows / serializeSchema / addSchemaRow / removeSchemaRow / switchSchemaToVisual / switchSchemaToJSON`, all parameterised by field name). Replaced the raw-JSON Output Schema textarea with `@schemaBuilder("outputSchema", ...)` inside a new collapsible section (matching the Input Schema UX). Corrected the misleading "disables tool use" copy — the new helper text explains the actual ADK behaviour (native ResponseSchema on most providers; `set_model_response` workaround tool on Gemini API ≤ 2.5). See `frontend/ui/agenteditor/agenteditor.templ` (`schemaBuilder` helper + both schema sections) and `assets/js/components/agent-designer-dialog.js` (renamed `inputSchema*` methods → generic `Schema*` family keyed by field). Backend untouched — both schemas continue to round-trip through their existing `*SchemaJSON` strings.

> **Reframe (after reading ADK source).** The original framing of this item was wrong about tools being disabled — that belief came from the `llmagent.go:256-258` docstring but is contradicted by the runtime. Keeping the item, tightening the scope, and correcting the copy we ship to users.

**ADK.** `llmagent.Config.OutputSchema (*genai.Schema)` — forces the agent's reply to conform to a JSON Schema. Enforcement path (verified in `internal/llminternal/basic_processor.go:46-51` + `outputschema_processor.go:41-65`):

1. **No tools attached** → the schema is passed as `genai.GenerateContentConfig.ResponseSchema` + `ResponseMIMEType = "application/json"`. Trivial case.
2. **Tools attached, provider supports the native combo** (non-Gemini, Vertex Gemini, or a future Gemini API variant that lifts the restriction) → same native path; tools and transfer remain fully functional.
3. **Tools attached, Gemini API + Gemini ≤ 2.5** (`googlellm/variant.go:71-76`, `NeedsOutputSchemaProcessor`) → `ResponseSchema` is NOT set on the native config. Instead ADK injects a synthetic `set_model_response` tool and a system instruction telling the model: *"use real tools for the work, then always call set_model_response with the final schema-shaped JSON"*. Tools and transfer still work — the schema is enforced at the final step via that tool.

So the widely-repeated claim "OutputSchema disables tools and transfers" is only true for case 1 (where there are no tools to disable anyway). The source docstring is outdated; our UI copy currently mirrors it and is therefore wrong.

**Today.** `frontend/ui/agenteditor/agenteditor.templ` exposes `outputSchema` as a raw JSON textarea (post-M4 unbundling). Warning copy is present but inaccurate — the helper text *"When set, the agent cannot use tools or transfer"* and the subtitle *"disables tool use"* contradict ADK's actual behaviour (see above). Non-programmers also can't reliably author raw JSON Schema.

**Workflow.** Two parts, both purely frontend:

1. Replace the textarea with the row-based builder shipped in M4 (`hydrateInputSchemaRows` / `serializeInputSchema` in `agent-designer-dialog.js`). Rename the pair to a generic `hydrateSchemaRows(field)` / `serializeSchema(field)` and call it for both `inputSchema` and `outputSchema`.
2. Fix the warning copy so it reflects reality.

**Files.**
- Frontend: `agenteditor.templ` (replace textarea + rewrite helper copy); `assets/js/components/agent-designer-dialog.js` (promote the schema helpers to work on either field). No new component — reuse `@schemaBuilder` extracted from M4.
- Backend: none. `OutputSchemaJSON` already round-trips.

**UI delta.**
- Replace the raw textarea with the row-based builder from M4.
- Rewrite helper text. Suggested copy: *"Every reply must match this JSON Schema. Tools still work on most providers; on the Gemini API with Gemini 2.5 or earlier, ADK injects a workaround tool so your tools remain usable — the model uses them first, then returns the final schema-shaped JSON."*
- Drop the "red banner" from the previous scope — the scary warning is based on the outdated docstring.

**Acceptance.**
- A non-programmer can define `{summary: string, tags: array<string>}` without typing JSON.
- With tools attached and Gemini 2.5 selected, running the agent produces both tool calls and a valid schema-shaped final reply (the `set_model_response` workaround kicks in).
- With tools attached and a non-Gemini provider, schema and tools both work via the native path.
- Helper copy matches the observed behaviour.

**Does-not-touch.** Input schema (M4 — already shipped), model config (M3). Also: do NOT touch `foundationSchemaToGenAI` or `parseAgentSchema` — M4 verified the round-trip already works.

---

### M6. Instruction template variable picker

> **Status: ✅ Done.** System-prompt textarea triggers a `{`-anchored autocomplete popover listing live session-state keys — OutputKey values aggregated across the user's agents — plus a single `{key?}` optional-suffix hint. Scoped prefixes and artifact refs are intentionally omitted from the picker to keep the UI approachable for non-programmers (they still resolve at runtime if typed). Backend: `GET /v1/agent-definitions/{id}/available-state-keys` → `Service.GetAvailableStateKeys` with priority-tiered suggestions (self / direct sub-agents / workflow children / workflow peers / library) grounded in ADK sample frequency. Reverse-lookup detects when the current agent is a leaf inside someone else's workflow and elevates that workflow's peers. Priority tiers render as group headers in the popover. Undefined `{foo}` references surface as a warning card below the textarea. Regex mirrors `.READ/adk-go/internal/llminternal/instruction_processor.go`; inline comments cite the file so the alignment survives. Docs: "Output Keys and Template Variables" section in `frontend/ui/docs/agents.templ`. Tests: `modules/agentdefinition/available_state_keys_test.go` (tier priorities + dedup + sort) and `userrequirements/instruction_template_variables_test.go` (runtime substitution via `/debug/agent:run`). See also `modules/agentdefinition/{service,handler,module}.go`, `assets/js/components/agent-designer-dialog.js`, `frontend/ui/agenteditor/agenteditor.templ`.

**ADK.** `llmagent` instruction templates support `{state_key}`, `{state_key?}` (optional), `{artifact.name}`, `{artifact.name?}`. The regex for keys is `^[a-zA-Z_][a-zA-Z0-9_]*$` — anything else is treated as literal.

**Today.** The `System Prompt` field is a plain textarea. Non-programmers have no discoverability for template variables, nor any warning when they type an undefined key.

**Workflow.**

```
Designer types "{" → autocomplete dropdown appears:
   - session state keys (from other agents' OutputKey values — reachable via workflow graph)
   - artifact names (from user's artifact list)
   - user-context placeholders
```

**Files.**
- Frontend: enhance system-prompt textarea in `agenteditor.templ` with a small Alpine autocomplete. No templ component change — inline directive + JS.
- Backend: add `GET /v1/agents/{id}/available-state-keys` (aggregates OutputKey of upstream agents in the workflow graph). Small new endpoint in `modules/agentdefinition/handler.go`.

**UI delta.** "{"-triggered popover listing available keys, with scope icons (session / user / app / artifact). "Insert" on selection.

**Acceptance.** Typing `{` opens suggestions; selecting inserts `{key_name}`; typing a non-existent key shows a warning underline.

**Does-not-touch.** System-prompt storage (still a single string).

---

### M7. Global-instruction UX

> **Status: ✅ Done.** Global Instruction editor is now root-only in both dialogs: `frontend/ui/agenteditor/agenteditor.templ` (new designer — section didn't exist before, added as a collapsible emerald card with a "Root only" pill) and `frontend/ui/agents/agents.templ` (legacy AgentsDialog — existing block wrapped in `x-show="isRootNode()"`). Both stores expose an `isRootNode()` helper that returns `true` when the currently edited agent has no `parentId`, matching ADK's runtime rule verified at `.READ/adk-go/agent/llmagent/llmagent.go:232` and `.READ/adk-go/internal/llminternal/instruction_processor.go:48-56` (the processor walks `parents.RootAgent()` and uses that agent's value). Backend exposes `parentId` on `agentResponse` (`modules/agentdefinition/handler.go`). No adapter change — ADK already ignores non-root values at runtime, so explicit zeroing would be cosmetic. Docs updated in `frontend/ui/docs/agents.templ` with a "Root only" tag on the Global Instructions heading + an info box explaining which agent counts as root.

**ADK.** `GlobalInstruction` only takes effect on the **root** agent (see `llmagent` doc). Child agents' values are silently ignored.

**Today.** The field is surfaced identically on every agent node in the composition tree. A user editing a sub-agent can write a GlobalInstruction that will silently do nothing.

**Workflow.** Pure UI change — hide/disable the field unless the current node is the root.

**Files.**
- Frontend: `agenteditor.templ` — wrap Global Instruction section in `x-show="isRootNode()"`. Add a helper in `agent-designer-dialog.js` that checks whether the currently-edited node has no parent in the composition graph.
- Backend: no change, but consider zero-ing it in `agent_builder.go` when building non-root nodes (defensive — already a no-op per ADK, but makes intent explicit).

**UI delta.** Section hidden by default; visible only when editing the root. Add a "Root-only" tag next to the header.

**Acceptance.** Editing a sub-agent doesn't show the Global Instruction field. Editing the root does.

**Does-not-touch.** System-prompt editor (M6), root-finding in the persisted graph.

---

### M8. Memory service toggle

> **Status: ✅ Done.** `plugins/agentmemory` PluginProvider stub removed; per-agent `Config.MemoryPolicy { Enabled, Preload }` drives tool injection exclusively; the legacy `memory-set` toolset-permission fallback was deleted in the M9 refactor. Designer has a collapsible Memory card; catalog YAML accepts `memory_policy:`; integration tests cover cross-session recall end-to-end.

**ADK.** `memory.Service` is a runner-level service; `preload_memory` / `load_memory` are in-tree tools the app explicitly adds to agents that should have them. Memory is NOT a plugin in ADK.

**Today.** Memory is an agent feature driven by `Config.MemoryPolicy`. The orchestration engine's service is wired at the runner level; per-agent choice determines tool injection.

**Workflow.**

```
Designer → new "Memory" section in edit view
   [✓] Use long-term memory
   [ ] Auto-preload relevant memories at start of each run
Backend → maps to toolreqs permissions for the agent-memory plugin
       → optionally sets llmagent.Config to include preload_memory at position 0
```

**Files.**
- Backend: small addition in `adapters/adkgo/runner.go` (read `Config.MemoryPolicy`) and `foundation/agent/config.go` (new struct `MemoryPolicy { Enabled bool; Preload bool }`).
- Frontend: new collapsible "Memory" section in `agenteditor.templ`.

**UI delta.** Two toggles plus a small explanatory blurb ("The agent can remember across sessions for this user.").

**Acceptance.** Toggling on → `load_memory` tool appears in the agent's tool list at runtime; preload → `preload_memory` runs at turn 1.

**Does-not-touch.** Tool-requirements UI (that stays the power-user view).

---

### M9. Artifact service

> **Status: ✅ Done.** `plugins/artifacts` PluginProvider stub removed; per-agent `Config.ArtifactPolicy { Enabled, AutoSaveUserBlobs }` drives both tool injection and the runner's `SaveInputBlobsAsArtifacts` knob; the legacy `artifacts-set` toolset-permission gate is gone. Designer has a collapsible Artifacts card; catalog YAML accepts `artifact_policy:`; integration tests exercise the tool-injection surface end-to-end. S3-backed `foundation/artifact/adkartifact` service unchanged — that was always the real implementation.

**ADK.** `runner.Config.RunConfig.SaveInputBlobsAsArtifacts (bool)` — automatically stores user-uploaded blobs as artifacts (they then become referenceable via `{artifact.name}`).

**Today.** Artifacts are an agent feature driven by `Config.ArtifactPolicy`. Setting `AutoSaveUserBlobs=true` surfaces inline blobs into the artifact store at runtime so the model sees text placeholders instead of raw bytes.

**Workflow.**

```
Designer toggle → Config.ArtifactPolicy.AutoSaveInputs
         → RunConfig passed to runner.Run()
```

**Files.**
- Backend: `foundation/agent/config.go` (new `ArtifactPolicy` struct), `adapters/adkgo/runner.go` (pass through to `RunConfig`).
- Frontend: one checkbox in the "Memory" section (they're conceptually neighboring — long-term state).

**UI delta.** One toggle: "Keep user-uploaded files as agent artifacts".

**Acceptance.** Toggled on → uploaded file appears in `GET /v1/threads/.../artifacts` with `CustomMetadata.AutoSaved=true`.

**Does-not-touch.** Upload pipeline, MinIO configuration.

---

### M10. Code-execution toggle

> **Status: ✅ Done.** Per-agent `Config.CodeExecutionPolicy { Enabled }` drives a provider-agnostic toggle in the designer's new Code Execution card. Adapter dispatches by `modelCfg.Provider`: Gemini appends `&genai.Tool{CodeExecution: &genai.ToolCodeExecution{}}` to the request's `GenerateContentConfig.Tools`; OpenAI and Anthropic fall through silently with a TODO pointing at the future wiring. Response parts (`ExecutableCode` + `CodeExecutionResult`) are normalised into a new `CodeExecutionRan` domain event, projected to AG-UI as `CODE_EXECUTION_RAN`, and rendered as a read-only code + output card (`CodeExecutionCard` in `frontend/ui/messagelist/messagelist.templ`). Code never runs on our infrastructure — the provider sandboxes it. Integration test seeds a large multiplication on Gemini, asserts the exact product appears in the reply AND a `CodeExecutionRan` event carries the source.

**ADK.** Gemini models can enable built-in code execution via `GenerateContentConfig` — results arrive as `genai.Part.CodeExecutionResult`.

**Today.** Not exposed. Non-programmers who'd benefit most ("Make the agent able to compute things reliably") can't enable it.

**Workflow.**

```
Designer toggle → Config.CodeExecution.Enabled
         → adapters/adkgo/model.go appends code-execution tool in genai config
Frontend chat → render genai.Part.CodeExecutionResult as a code block with output
```


**Files.**
- Backend: new bool on `Config`, guarded by provider capability in `adapters/adkgo/model.go` (silently ignored for non-supporting providers).
- Frontend: new event type `CODE_EXECUTED` in `events.go`. Chat renderer: syntax-highlighted block + stdout/stderr.

**UI delta.** Toggle in a new "Capabilities" section (shares real estate with M8 / M9). Disabled + tooltip when provider doesn't support it.

**Acceptance.** With Gemini 2.5 + toggle on, asking "what is 1234 × 5678" causes model to emit a code-exec block which the UI renders.

**Does-not-touch.** Other providers' code-exec routes (we don't have a container executor wired; don't add one).

---

### M11. Long-running tools

**ADK.** `functiontool.Config.IsLongRunning (bool)` + `session.Event.LongRunningToolIDs []string` — tool returns status immediately, result later. ADK marks the event final so the LLM doesn't re-call.

**Today.** We don't propagate `LongRunningToolIDs` to AG-UI. Non-programmers can't mark a tool as long-running from the designer, and the chat UI doesn't show pending status.

**Workflow.**

```
Tool definition (plugin) declares IsLongRunning
 → agent designer shows a "⏱ long-running" pill on such tools
 → at runtime, LongRunningToolIDs propagate to chat timeline as a pending state
 → final result arrives later as a separate event and updates the timeline
```

**Files.**
- Backend: `adapters/adkgo/events.go` — surface `LongRunningToolIDs` on tool events. `modules/chat/projection/agui` — new AG-UI frame `TOOL_PENDING`.
- Frontend: chat renderer — pending spinner on tool-call card. Designer: read-only pill on plugin list in the Tools section (`agenteditor.templ`).

**UI delta.** Chat: spinner + "running…" label on tool cards until the result event arrives. Designer: pill indicator.

**Acceptance.** A long-running tool (e.g. a background web-scrape) shows "running…" in the UI and updates to the final result when done, without the agent thinking it finished prematurely.

**Does-not-touch.** Plugin implementations — each plugin already declares its own `IsLongRunning`.

---

### M12. A2A remote-agent node configuration

**ADK.** `remoteagent.NewA2A(A2AConfig)` — remote agent via Agent-Card URL or local file. Config includes `MessageSendConfig`, part converters, HITL callbacks.

**Today.** The composition tree has a "Remote (A2A)" node type (confirmed in `workflow-tree-builder.js`), and there's a `modules/a2a/` module. But the **designer form for configuring an A2A node is thin** — verify in `agenteditor.templ`. It needs at least: agent-card URL *or* picker of already-registered A2A connectors; timeout; retries.

**Workflow.**

```
Designer configures A2A node:
   • Source: [ URL ] [ Saved connector (dropdown) ]
   • Name, Description (local aliases)
   • Timeout, retries
Backend → adapters/adkgo/workflow.go wraps the node via remoteagent.NewA2A()
```

**Files.**
- Backend: `adapters/adkgo/workflow.go` (extend A2A branch), `modules/a2a/` (connector picker endpoint if missing).
- Frontend: expanded form for the A2A node type in `agenteditor.templ`.

**UI delta.** When the user picks "Remote (A2A)", show: Source dropdown (Connector / URL), Name, Description, Timeout (seconds), Max retries.

**Acceptance.** Adding a remote A2A node with a local demo agent card → chat turn delegates to remote and surfaces its response in the timeline.

**Does-not-touch.** The A2A server itself (`modules/a2a/` server-side code).

---

### M13. MCP toolset picker

**ADK.** `mcptoolset.New({Transport, ToolFilter, RequireConfirmation})` — connects to external MCP servers. Tools are discovered dynamically.

**Today.** `modules/mcp/` exists; we persist MCP connectors per user. But in the designer there is **no picker to attach MCP connectors to a specific agent** — it's all-or-nothing via tool-requirements.

**Workflow.**

```
Designer (in Tools section) → "MCP Servers" sub-panel
  list of user's configured MCP connectors with per-agent toggles
  + per-tool filter (ToolFilter)
Backend → mcptoolset attached to llmagent.Config.Toolsets at build time
```

**Files.**
- Backend: `adapters/adkgo/tools.go` (resolve per-agent MCP attachments), `modules/agentdefinition/types.go` (store `MCPAttachments []MCPRef`).
- Frontend: new sub-panel in `agenteditor.templ`.

**UI delta.** Under the existing Tools section, add "MCP servers" list with: enable toggle per connector + "choose tools" drawer (multi-select of tool names, default: all).

**Acceptance.** Attaching a filesystem-MCP connector → the `fs_read` tool becomes callable by only this agent.

**Does-not-touch.** MCP connector CRUD (owned by `modules/mcp/`).

---

### M14. Durable session service

**ADK.** `session.Service` has `InMemoryService()`, `vertexai/`, and `database/` backends. We use `session.InMemoryService()` hardcoded in `adapters/adkgo/runner.go:27` → sessions die on restart.

**Today.** We reconstruct session by replaying thread events from the event store on every run (`adapters/adkgo/session.go`). That works, but it's a **custom mechanism where ADK already offers a `database/` backend**. We could either (a) replace the replay with ADK's DB session service, or (b) keep the replay and document why.

**Recommendation.** Keep the replay. Our thread aggregate is the source of truth; ADK's DB service would be a second durable store. But **surface the choice in the designer** per-agent: "Session lifetime: per-thread (default) | per-user | per-app". Internally this controls whether `KeyPrefixUser` / `KeyPrefixApp` keys are used.

**Files.**
- Backend: `foundation/agent/config.go` (new `SessionScope`), `adapters/adkgo/session.go` (initial state population respects the scope).
- Frontend: single dropdown in "Advanced" section.

**UI delta.** Dropdown: "Agent remembers across → Only this conversation / This user / Everyone (app-wide)". Default first option.

**Acceptance.** An agent with "This user" scope sees the user-scoped state keys from any of their threads.

**Does-not-touch.** Thread replay logic, event store.

---

### M15. Evaluation sets

**ADK.** We have `adapters/adkgo/eval_runner.go` — this already wraps ADK's eval surface. The designer shell has `evalPanelActive/evalSets/evalRunning` in Alpine state but **the flows to author, save, and run eval sets from the UI are incomplete**. A root-level `test-weather-evalset.json` suggests the format is known.

**Today.** Left-panel placeholder UI exists; needs wiring.

**Workflow.**

```
Designer → "Tests" panel → "New test case"
   • Input message
   • Expected tool calls (names + optional arg patterns)
   • Expected final state keys
   • Assertion: substring / regex / JSON-path on final output
→ saved to DB, runnable from the panel
Backend → modules/evaluation/ hosts CRUD + runner
       → uses adapters/adkgo/eval_runner.go
Bottom panel → streams results (pass/fail/diff)
```

**Files.**
- Backend: `modules/evaluation/` (already exists — finish the handler/service for eval-set CRUD), wire to `adapters/adkgo/eval_runner.go`.
- Frontend: flesh out the left Test Explorer panel + bottom Output tab in `agenteditor.templ` + `agent-designer-dialog.js`.

**UI delta.** VS-Code-style left Tests explorer + a results pane. Green/red row per case.

**Acceptance.** `test-weather-evalset.json` imports cleanly and runs to green.

**Does-not-touch.** Chat runtime — eval runner has its own session service.

---

### M16. Callback presets (non-programmer-friendly)

**ADK.** `BeforeModelCallback`, `AfterModelCallback`, `BeforeToolCallback`, `AfterToolCallback`, `OnModelErrorCallback`, `OnToolErrorCallback`, plus Runner-level plugin callbacks. We already wire `retryandreflect` (tool retries), `logging`, `confirmation`, and `block` callbacks in `adapters/adkgo/callbacks.go`.

**Today.** Callbacks are invisible in the designer. Non-programmers benefit from **presets**, not a code editor.

**Workflow.**

```
Designer → "Guardrails" section
   [ ] Retry failed tools up to 3 times      (already on — make it a toggle)
   [ ] Require confirmation for external writes (bulk-set confirmation hint)
   [ ] Redact emails and phone numbers from model input
   [ ] Cap total LLM calls per run at N
Backend → each preset maps to a known callback wired in adapters/adkgo/callbacks.go
```

**Files.**
- Backend: `foundation/agent/config.go` (new `Guardrails` struct — just booleans + numbers), `adapters/adkgo/callbacks.go` (attach based on flags). Reuse `functioncallmodifier` where useful.
- Frontend: new section + a help tooltip per preset.

**UI delta.** A "Guardrails" card in the edit view. **Do not** expose raw code — that's not the audience.

**Acceptance.** Toggling PII redaction produces observable redaction in the LLM request logs (visible in the dev console).

**Does-not-touch.** The existing `RetryAndReflect` plugin wiring (just add a toggle).

---

### M17. Built-in ADK tool catalog visibility

**ADK.** Trivial tools with no config: `geminitool.GoogleSearch`, `loadartifactstool.Tool`, `loadmemorytool.Tool`, `preloadmemorytool.Tool`, `exitlooptool.Tool`, `exampletool.Tool`.

**Today.** The tools panel in `agenteditor.templ` lists our plugins (`plugins/*`) but ADK-native tools are attached implicitly by `runner.go`. Users don't see them, can't turn them off per agent.

**Workflow.**

```
toolregistry → registers ADK-native tools alongside plugin tools
Designer → they appear in the same Tools list as plugins, with a "built-in" badge
```

**Files.**
- Backend: `foundation/toolregistry/registry.go` — register ADK natives on startup. `adapters/adkgo/runner.go` — stop auto-injecting them; respect the registry.
- Frontend: no new UI; tools just appear in the existing list.

**UI delta.** No new controls; list gets richer. Add a "built-in" pill.

**Acceptance.** In the Tools section, users see `google_search`, `load_artifacts`, etc. with toggles. Turning off `load_memory` stops the model from seeing it.

**Does-not-touch.** Custom-plugin tool registration.

---

### M18. Streaming-mode toggle

> **Status: ✅ Done.** Per-agent `Config.StreamingPolicy { Enabled }` drives a new Streaming card in the designer. Nil policy preserves ADK's historical default (`StreamingModeSSE`); an explicit `{ enabled: false }` flips the run to `StreamingModeNone`, and `runner_loop.go:onAssistantMessageAdded` already synthesises a single Started + Streamed (whole text) + Completed lifecycle, so the frontend sees the reply as exactly one `TEXT_MESSAGE_CONTENT` frame with no changes required in the AG-UI pipeline. Adapter dispatch lives in `adapters/adkgo/runner.go` (`resolveStreamingMode` + the `RunConfig.StreamingMode` call site). Persistence follows the same workflow-A recipe as the other policies: foundation type (`foundation/agent/config.go` — `AgentStreamingPolicy`), DB-side mirror (`modules/agentdefinition/types.go` — marshal/unmarshal helpers), migration `039_add_streaming_policy.sql`, repo wiring (`agent_definition_repo.go` — INSERT/UPDATE/hydrate), provider conversion (`provider.go` — `toFoundationStreamingPolicy`), REST surface (`handler.go` — `streamingPolicy` on create / update / read), designer UI (`frontend/ui/agenteditor/agenteditor.templ` — collapsible sky card peer to Code Execution; Alpine getter/setter in `agent-designer-dialog.js`). Integration tests (`userrequirements/streaming_policy_test.go`) assert the exactly-one-frame collapse for streaming off and the ≥ 2-frame fan-out for the default on the Gemini provider. Copy: "Stream responses (recommended)" with plain-English on/off explanations in the card body.

**ADK.** `RunConfig.StreamingMode` ∈ `"none" | "sse"`. We currently always use `sse` (`adapters/adkgo/runner.go:540`).

**Today.** No designer control. Some use cases (structured-output agents, evals) would prefer non-streaming.

**Workflow.**

```
Designer → "Streaming" toggle in Advanced
         → Config.Streaming
Backend → passed to RunConfig
```

**Files.**
- Backend: `foundation/agent/config.go` + `adapters/adkgo/runner.go`.
- Frontend: toggle in Advanced section.

**UI delta.** One toggle: "Stream responses (recommended)". Default on.

**Acceptance.** With streaming off, the chat shows the full message at once rather than delta-by-delta.

**Does-not-touch.** AG-UI event pipeline (it already handles both cases — the final event is always non-partial).

---

### M19. Agent-card export

**ADK.** `.READ/adk-go/examples/a2a` shows how to export an agent as an A2A-compliant endpoint via an `AgentCard`.

**Today.** We have an `a2a` module and we support **consuming** remote A2A agents (M12). We don't let a user **publish** their own agent as an A2A endpoint. That completes the loop — other ADK consumers can now call our agents.

**Workflow.**

```
Designer → "Share" action on an agent
         → shows generated AgentCard (URL + JSON preview)
Backend → /a2a/cards/{agentID} serves the card
       → /a2a/run/{agentID} accepts A2A messages and runs the agent
```

**Files.**
- Backend: `modules/a2a/` — add "server-side publish" handlers. Reuse `adapters/adkgo/runner.go`.
- Frontend: a "Share as agent" button in the designer; a copy-URL modal.

**UI delta.** Share action on each agent card + modal with copyable URL and "download card" button.

**Acceptance.** A second instance of this app can register the URL as an A2A connector (M12) and use the agent.

**Does-not-touch.** Authentication — for v1, cards are public; scope to signed-in users later.

---

### M20. Tool-confirmation hint UX

**ADK.** `tool.Context.RequestConfirmation(hint, payload)` — the `hint` surfaces to the user. `functiontool.Config.RequireConfirmationProvider` lets the provider decide dynamically.

**Today.** Confirmation is wired (`adapters/adkgo/callbacks.go` — buildConfirmationCallback). The per-tool requirement config already has `confirmation_hint`. But:
- The **hint** input in the designer is hidden away in the requirement config dialog.
- There's no UI to author **conditional** confirmation (e.g. "only confirm when `path` starts with `/etc`") — only on/off.

**Workflow.**

```
Designer → per-tool config dialog:
   • Confirmation: Always / Never / When condition…
   • Hint to show the user
   • (advanced) Predicate expression: simple DSL evaluated against tool args
Backend → maps to RequireConfirmation + RequireConfirmationProvider
```

**Files.**
- Backend: extend existing per-tool requirement persisted shape + `adapters/adkgo/callbacks.go` predicate evaluation.
- Frontend: revamp the small dialog that currently sets requirements — merge toolset-level and per-tool paths (duplicated today per `agent-designer-dialog.js`).

**UI delta.** Cleaner per-tool modal with three radio options + a simple arg-predicate builder for "When condition".

**Acceptance.** A `file_delete` tool configured with `path starts-with /etc → confirm, else → allow` behaves accordingly.

**Does-not-touch.** Tool plugin implementations.

---

### M21. OpenAI Responses API migration

> **Status: ✅ Done.** OpenAI provider migrated off Chat Completions onto the Responses API (new `openAIResponsesModel` in `adapters/adkgo/openai_responses.go`). `applyCodeExecutionTool` gains an `openai` branch emitting the same provider-agnostic `genai.Tool.CodeExecution` marker, which the Responses adapter translates to OpenAI's native `code_interpreter` tool with an auto-provisioned container. Structured-output translation wired too (`Text.Format.JSONSchema` with strict mode). `ResponseCodeInterpreterToolCall` output items are unpacked into `genai.Part.ExecutableCode` + `genai.Part.CodeExecutionResult`, which `events.go:translateCodeExecution` already normalises into `CodeExecutionRan` domain events — same chat card renders for both providers. Anthropic stays on Chat Completions (its OpenAI-compatible endpoint doesn't implement Responses). Integration tests verify `87654321 × 98765432` round-trips on both Gemini and OpenAI with the exact product `8657216880231672` in the reply; all 14 userrequirements specs pass. File search / web search / computer_use / image_generation / remote MCP are now unlocked on the OpenAI path but not yet surfaced in the designer — each can follow as its own Mxx entry using the existing toggle pattern.

**ADK / SDK.** OpenAI's `/v1/chat/completions` endpoint (what we use today) only supports **custom function tools**. Every built-in OpenAI tool — `code_interpreter`, `file_search`, `web_search`, `computer_use`, `image_generation`, remote MCP — lives exclusively on the **Responses API** (`/v1/responses`). The official `openai-go` SDK has full first-class support for it: `client.Responses.New(...)`, `client.Responses.NewStreaming(...)`, typed union `ToolUnionParam` with a `OfCodeInterpreter *ToolCodeInterpreterParam` case, typed streaming events (`ResponseCodeInterpreterCallCodeDeltaEvent`, `…CallCodeDoneEvent`, `…CallCompletedEvent`), and a typed `ResponseCodeInterpreterToolCall` response item with `Code`, `Outputs`, `ContainerID`, `Status`.

**Today.** `adapters/adkgo/model.go:newOpenAIModel` goes through `client.Chat.Completions.New` / `.NewStreaming` exclusively. That rules out code interpreter (M10's OpenAI path), and every other built-in OpenAI tool by extension. The M10 infrastructure (`AgentCodeExecutionPolicy`, `CodeExecutionRan` domain event, `applyCodeExecutionTool` dispatch switch, `translateCodeExecution` parser) is already provider-agnostic and has a `default:` branch with a `TODO(M10-followups)` comment pointing at this migration.

**Workflow.** Migrate the OpenAI provider off Chat Completions to Responses. The foundation / persistence / designer UI / domain event don't change — only the provider adapter. Once the provider is on Responses, wiring `code_interpreter` is a three-line addition to the existing `applyCodeExecutionTool` switch, and the OpenAI integration test mirrors the Gemini M10 spec.

**Files.**
- `adapters/adkgo/model.go` — rewrite `newOpenAIModel`'s Run path: replace `chat.Completions.New` with `responses.New`; replace the `ChatCompletionChoice.Message` parse with a walk over `Response.Output []ResponseOutputItemUnion`; replace the streaming delta-accumulator with a switch over `ResponseStreamEventUnion` types.
- `adapters/adkgo/runner.go:applyCodeExecutionTool` — new `openai` / `openai-compatible` case in the provider switch that appends `{Type: "code_interpreter", Container: {Type: "auto"}}` to the request's tool list (adapted to whatever we pass to the Responses API; likely lives in a small OpenAI-specific config object rather than `genai.GenerateContentConfig`).
- `adapters/adkgo/events.go:translateCodeExecution` — extend to recognise OpenAI's `ResponseCodeInterpreterToolCall` items and normalise into the existing `CodeExecutionRan` domain event.
- `userrequirements/code_execution_test.go` — new spec `TestAgentWithCodeExecutionPolicyRunsCodeOnOpenAI` using `gpt-4o-mini` + the Responses-API-backed OpenAI provider. Mirror the Gemini assertion shape: expected integer in the reply + at least one `CodeExecutionRan` event.

**UI delta.** None. The designer toggle, the chat code-card, and the Capabilities docs already render provider-agnostically. The existing warning in the designer ("Code execution is currently only wired on Gemini …") flips off automatically when OpenAI is selected once the adapter supports it (`codeExecutionSupportedByProvider` helper covers the provider check).

**Acceptance.**
- An agent with `CodeExecutionPolicy.Enabled=true` + OpenAI model emits `CodeExecutionRan` events with non-empty `Code` + `Output`.
- Regular chat/tool-use flows on OpenAI still work — no regression on function-calling, streaming latency, multi-turn state, or cost.
- `gpt-4o-mini` + `87654321 × 98765432` integration test passes the same way the Gemini one does.
- The designer's "Gemini-only" warning disappears when the selected provider is OpenAI (provider list update in `codeExecutionSupportedByProvider`).

**Blast radius.** Medium. Roughly:
- ~200–400 LOC in `adapters/adkgo/model.go` for the Responses-API streaming rewrite.
- Multi-turn conversation state: decision point on `PreviousResponseID` chaining vs re-sending the full message array. Former is lighter on tokens but changes our history-replay model; latter is a one-liner.
- Regression risk: every OpenAI run in the app uses this code path; needs careful diffing of event translation + error paths.

**Why now / why wait.** OpenAI is investing in the Responses API and treating Chat Completions as legacy for new features (every built-in tool they've shipped in the last year is Responses-only). Migrating gains code interpreter immediately + positions us to pick up file search, web search, image generation, computer use, and remote MCP without further OpenAI-side work. Waiting means writing the adapter a second time once more built-ins land.

**Does-not-touch.** The foundation `AgentCodeExecutionPolicy` type (provider-agnostic), the domain event (provider-agnostic), the designer UI (provider-agnostic), the Anthropic adapter path (separate M if/when we add Claude's code_execution). Also: no legacy Chat Completions fallback — Responses API is stable enough to be the sole path once we move.

---

## 4. Custom patterns we should replace with ADK natives

These aren't user-facing features — they're internal simplifications. Treat each as a **refactor** item.

| # | Today (custom) | Replace with (ADK-native) | File(s) | Risk |
|---|---|---|---|---|
| C1 | Our own `foundation/agent.Middleware` (before/after/BeforeTool/AfterTool) paralleled with ADK callbacks | Drop the foundation middleware layer; register directly as `BeforeToolCallback`/`AfterToolCallback` etc. Keep the shape of our plugin middlewares but remove the duplicate abstraction. | `foundation/agent/middleware.go`, `adapters/adkgo/callbacks.go` | Medium — many callers |
| C2 | Manual session reconstruction by replaying thread events (`adapters/adkgo/session.go`) | **Keep.** Documented as intentional (thread aggregate is source of truth). But stop double-writing to `InMemoryService` state where ADK's own state would suffice. Move temp/app/user state keys onto ADK's prefixed state. | `adapters/adkgo/session.go` | Low — additive |
| C3 | `buildBeforeRunHooks` synthesises a pseudo-user message for requirement guidance | Replace with `llmagent.Config.GlobalInstruction` on root + per-turn `InstructionProvider` that reads `CallbackContext.State()` | `adapters/adkgo/requirements.go`, `adapters/adkgo/agent_builder.go` | Medium |
| C4 | Tool exposure via a custom `foundation/toolregistry` + translation in `tools.go` | Register directly as `tool.Toolset`s keyed by agent and let ADK's toolset filtering do the rest. Collapses two layers. | `foundation/toolregistry/`, `adapters/adkgo/tools.go` | High — touches every tool |
| C5 | `RetryAndReflect` wired unconditionally in `adapters/adkgo/runner.go` | Surface as toggle (see M16) and attach only when the toggle is on. Remove the implicit default. | `adapters/adkgo/runner.go` | Low |
| C6 | `SessionService = session.InMemoryService()` hardcoded at line 27 of `runner.go` | Plumb the service as a DI dependency; use `database/` backend when `SessionScope` ≥ user (M14). | `adapters/adkgo/runner.go`, `main.go` | Medium |
| C7 | Plugin injections: `preload_memory`, `load_memory`, `load_artifacts` appended in `runner.go` | Register them in `toolregistry` like any other tool so the designer (M17) controls them | `adapters/adkgo/runner.go`, `foundation/toolregistry` | Low |
| C8 | Two parallel agent dialogs: AgentsDialog + AgentDesignerDialog | Merge into one; AgentsDialog's inline-edit "form" view is a subset of the designer. | `frontend/ui/agents/agents.templ`, `frontend/ui/agenteditor/agenteditor.templ` | Medium — UI surgery |
| C9 | Composition tree rendering via `x-html` in `workflow-tree-builder.js` | Render as templ partials; pass the composition graph through a typed handler. No string HTML. | `assets/js/components/workflow-tree-builder.js`, `frontend/ui/agenteditor/` | Medium — also fixes XSS exposure |

None of C1–C9 blocks the user-facing M-items; they're grouped here so developers know the *refactor* target when they touch adjacent code during an M-item.

---

## 5. Cross-cutting UI cleanup

These show up while working on M-items — flagging once so nobody re-discovers them:

1. **Dead `inputSchema` field** (`assets/js/components/agent-designer-dialog.js`, `agenteditor.templ:519`) — either M4 ships the builder or the field is deleted. Don't leave it half-wired.
2. **`AgentCapabilities` nested-scope workaround** (`frontend/ui/agentcapabilities/agentcapabilities.templ:11-15`) — the portaling comment is a smell; if it breaks on the first popover redesign, fix it with Alpine `@teleport` rather than piling on more inline workarounds.
3. **Duplicate `openToolSetRequirements` / `openToolRequirements`** in `agent-designer-dialog.js` — M20 merges them.
4. **Provider selector fetches models in one place, renders them in another** — when doing M3 or M12, consolidate the model-override card.

---

## 6. Scope guards

To keep the non-programmer audience front of mind, deliberately **do not build**:

- Raw code editors for callbacks (M16 covers this via presets).
- A "Custom Agent" node type that requires a Go function (ADK supports it, but it has no place in a GUI builder).
- A generic "safe eval" DSL for predicates beyond the minimal arg-pattern check in M20.
- Local code executors or container code executors (stick to the built-in code-exec in M10).
- Credential UIs per tool — tool auth stays in the respective module (aisettings, mcp).
- Telemetry dashboards — telemetry is wired via OTEL; surfacing in UI is a separate track.

---

## 7. Suggested dependency graph for scheduling

Most items are independent. These few chains reduce churn:

```
M4 (input schema) ─┐
                   ├──► M1 (agent-as-tool) ──► M19 (agent-card export)
M5 (output schema) ┘

M6 (template vars) ──► M7 (global instruction UX)

M8 (memory toggle) + M9 (artifacts toggle) share the "Memory & files" card — ship together

M17 (built-in tool catalog) unlocks M10/M8/M9 to look coherent in the Tools list

M13 (MCP picker) + M12 (A2A config) share "external integrations" — ship together

C1 (drop foundation middleware) should land after M16 (callback presets) wires
the remaining user-facing needs, so C1 has nothing left to remove in one shot.
```

Everything else can be picked up independently.

---

### Appendix — key file pointers

| What | Where |
|---|---|
| Agent `Config` (framework-agnostic) | `foundation/agent/config.go` |
| Agent persistence + CRUD | `modules/agentdefinition/` |
| ADK translation | `adapters/adkgo/{runner,agent_builder,workflow,tools,callbacks,events,session}.go` |
| Designer dialog (templ) | `frontend/ui/agenteditor/agenteditor.templ` |
| Designer store (Alpine) | `assets/js/components/agent-designer-dialog.js` |
| Composition tree renderer | `assets/js/components/workflow-tree-builder.js` |
| Tool registry | `foundation/toolregistry/registry.go` |
| Chat run orchestrator | `modules/chat/agentorchestrator/service.go` |
| Message HTTP handler | `modules/chat/handlers/messagehandler/handler.go` |
| AG-UI event projection | `modules/chat/projection/agui/` |
| ADK-Go source (read-only) | `.READ/adk-go/` |
