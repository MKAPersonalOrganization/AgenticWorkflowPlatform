# ADK-Go UI Cohesion Enhancement Report

> **Date:** April 11, 2026  
> **Analysis:** ADK-Go source code architecture vs. current UI/UX  
> **Goal:** Achieve 1:1 cohesion between ADK-Go concepts and UI presentation  

---

## Executive Summary

The current UI has **7 major cohesion violations** where ADK-Go concepts are split, mixed, or hidden. This report provides **12 prioritized enhancements** to achieve a simple, direct, intuitive UI that mirrors ADK-Go's architecture.

**Core Principle:** *What is cohesive in ADK-Go should be cohesive in the UI. What is unrelated in ADK-Go should be visually separated in the UI.*

---

## Cohesion Violations (Current State)

### 🔴 CRITICAL: False Dichotomies

#### 1. "Single Agent" vs "Multi-Agent Workflow" Type Selector
**Violation:** The creation dialog presents these as mutually exclusive paths, but ADK-Go treats all agents uniformly through the `Agent` interface. A "Single Agent" can have sub-agents (coordinator pattern), making it functionally a multi-agent structure.

**Impact:** Users don't understand that adding sub-agents to a "Single Agent" creates a coordinator pattern. The conceptual model is broken.

**Root Cause:** UI presents creation paths based on *form structure* rather than ADK-Go's *composition model*.

---

#### 2. Coordinator Pattern Disconnected from Workflow Visualization
**Violation:** An agent with `SubAgents []*SubAgentRef` (coordinator pattern) is architecturally a multi-agent structure, but it's only editable through the single agent form's sub-agents section—never visualized in the workflow canvas.

**Impact:** Users creating coordinators can't see the delegation structure visually. Two parallel ways to build multi-agent structures (form vs canvas) create confusion.

**Root Cause:** The workflow canvas only supports `CompositionNode` trees, not the coordinator pattern's flat `SubAgents` list.

---

### 🟡 HIGH: Unrelated Concepts Grouped Together

#### 3. "Advanced Settings" Grab-Bag
**Violation:** This section lumps together 5 unrelated ADK-Go concepts:
- Delegation Control → `TransferPolicy` (agent isolation)
- Output Variable → `OutputKey` (state injection)
- Conversation History → `IncludeContents` (context control)
- Shared Instructions → `GlobalInstruction` (pipeline-level config)
- Input/Output Schema → `InputSchema/OutputSchema` (data contracts)

**Impact:** Users can't find what they need. The section name "Advanced" provides no semantic guidance.

**Root Cause:** UI grouped these because they're "less frequently used" rather than organizing by ADK-Go concept.

---

#### 4. Model Configuration Split Across Two Panels
**Violation:** ADK-Go has a unified model configuration pipeline (`AgentModelConfig` + `AgentModelOverride`), but the UI splits this into:
- "Model Override" (violet panel) → Provider/Model selection
- "Generation Settings" (rose panel) → Temperature, TopP, TopK, MaxTokens, StopSequences

**Impact:** Users don't understand these are parts of the same model configuration. The split suggests they're unrelated.

**Root Cause:** UI design prioritized visual variety over conceptual cohesion.

---

### 🟠 MEDIUM: Missing ADK-Go Concepts

#### 5. Conditional Routing Undeveloped
**Violation:** ADK-Go has full `conditionalagent.New()` support with evaluator + branch routing, but the UI:
- Has conditional nodes in the CAD canvas but no creation card
- Missing from tree builder type selector
- No UI for defining routing conditions or evaluation logic

**Impact:** A complete ADK-Go feature is inaccessible through the UI.

---

#### 6. Pipeline-Level Instructions Hidden
**Violation:** `globalInstruction` (pipeline shared instructions) and `systemPrompt` are critical ADK-Go concepts that set context for all agents, but they're only visible when NO node is selected in the property panel.

**Impact:** Most users never see these important settings. They're invisible in the workflow editing flow.

---

#### 7. Agent-Level vs Workflow-Level Configuration Mixed
**Violation:** The property panel mixes:
- **Agent-level:** Model config, tools, KBs, filesystems (properties of the `AgentDefinition`)
- **Workflow-level:** `output_key`, transfer policy, `include_contents` (properties of the `WorkflowNodeInfo`)

**Impact:** Users can't distinguish "what I'm configuring"—the agent itself or its role in this workflow?

---

## Enhancement Proposals (12 Prioritized)

### Phase 1: Quick Wins (Fix Cohesion Violations)

#### ✅ Enhancement 1: Unify Model Configuration Section
**Priority:** 🔴 CRITICAL | **Effort:** Low | **Impact:** High

**What:** Merge "Model Override" and "Generation Settings" into a single "Model Configuration" section.

**ADK-Go Alignment:** Both map to the same model config pipeline (`AgentModelConfig` + `AgentModelOverride`).

**UI Changes:**
```
┌─ Model Configuration ──────────────────┐
│                                        │
│ [Provider Dropdown]  [Model Dropdown] │
│                                        │
│ Temperature:   [0.7    ]               │
│ Top P:         [0.9    ]               │
│ Top K:         [40     ]               │
│ Max Tokens:    [2048   ]               │
│ Stop Sequences: [___add___]            │
│                                        │
│ [Use Global Defaults] (toggle)        │
└────────────────────────────────────────┘
```

**Key Design:** "Use Global Defaults" toggle disables all fields when user wants to inherit from user global config. This makes the override nature explicit.

---

#### ✅ Enhancement 2: Split "Advanced Settings" into Semantic Sections
**Priority:** 🔴 CRITICAL | **Effort:** Low | **Impact:** High

**What:** Replace the grab-bag "Advanced Settings" with purpose-named sections that mirror ADK-Go concepts:

**New Sections:**
1. **State & Output** → `OutputKey` (output variable)
2. **Context Control** → `IncludeContents` (conversation history)
3. **Agent Isolation** → `TransferPolicy` (delegation control)
4. **Data Contracts** → `InputSchema/OutputSchema`
5. **Pipeline Instructions** → `GlobalInstruction`

**UI Changes:**
```
Before:                            After:
┌─ Advanced Settings ──┐           ┌─ State & Output ───────┐
│ (5 unrelated things) │           │ Output Key: [______]   │
└──────────────────────┘           └────────────────────────┘

                                      ┌─ Context Control ──────┐
                                      │ History: [Dropdown]    │
                                      └────────────────────────┘

                                      ┌─ Agent Isolation ──────┐
                                      │ ☐ Prevent escalation   │
                                      │ ☐ Prevent delegation   │
                                      └────────────────────────┘

                                      ┌─ Data Contracts ───────┐
                                      │ [Input Schema]         │
                                      │ [Output Schema]        │
                                      └────────────────────────┘

                                      ┌─ Pipeline Instructions ──┐
                                      │ Shared Instructions:     │
                                      │ [textarea]               │
                                      └──────────────────────────┘
```

---

#### ✅ Enhancement 3: Clarify Agent Creation Type Selector
**Priority:** 🟡 HIGH | **Effort:** Low | **Impact:** High

**What:** Replace the false dichotomy with ADK-Go-aligned terminology:

**New Cards:**
1. **LLM Agent** → `llmagent.New()` (single agent, may have sub-agents via coordinator pattern)
2. **Workflow Orchestrator** → Sequential/Parallel/Loop/Conditional composition
3. **From Template** → Pre-built workflow patterns
4. **Import YAML** → ADK-Go workflow import

**Card Descriptions:**
```
┌─────────────────────────┐
│  LLM Agent              │
│                         │
│  A single AI agent      │
│  that can use tools,    │
│  knowledge bases, and   │
│  delegate to sub-agents │
│  (coordinator pattern)  │
│                         │
│  [Create Agent]         │
└─────────────────────────┘

┌─────────────────────────┐
│  Workflow Orchestrator  │
│                         │
│  A multi-agent pipeline │
│  with sequential,       │
│  parallel, or loop      │
│  execution patterns     │
│                         │
│  [Create Workflow]      │
└─────────────────────────┘
```

**Key Change:** The "LLM Agent" card explicitly mentions sub-agents/coordinator pattern to make it clear this is a multi-agent capability.

---

#### ✅ Enhancement 4: Expose Pipeline Instructions Prominently
**Priority:** 🟡 HIGH | **Effort:** Low | **Impact:** Medium

**What:** Move `globalInstruction` (shared instructions) from the hidden "empty selection" state to a visible location:

**Options:**
- **Option A:** Add a "Pipeline Settings" button in the workflow canvas toolbar
- **Option B:** Show pipeline instructions in a collapsible header above the canvas
- **Option C:** Add a "⚙️ Pipeline Config" node as the root of the workflow tree

**Recommendation:** Option B (collapsible header)—keeps it visible but not intrusive.

```
┌─ Workflow: Marketing Campaign ──────────────────────────┐
│ 📝 Shared Instructions: [Edit] (shows preview if set)   │
│ "All agents should maintain brand voice consistency..." │
└─────────────────────────────────────────────────────────┘
[Canvas Area...]
```

---

### Phase 2: Structural Improvements

#### 🔧 Enhancement 5: Separate Agent-Level vs Workflow-Level Config in Property Panel
**Priority:** 🟡 HIGH | **Effort:** Medium | **Impact:** High

**What:** Visually distinguish what level the user is configuring:

**Property Panel Structure:**
```
┌─ Agent Node: "Content Writer" ──────────────┐
│                                             │
│ ── Agent Definition (edit in designer) ─── │
│ 📊 Tools: 3 | Subagents: 2 | KBs: 1        │
│ [Open Agent Designer] ← Full editor        │
│                                             │
│ ── Workflow Role Configuration ─────────── │
│ Output Key: [content_draft]                │
│                                             │
│ Context:                                   │
│ ○ Include history  ○ Fresh context only    │
│                                             │
│ Transfer Policy:                           │
│ ☐ Prevent escalation to parent             │
│ ☐ Prevent delegation to siblings           │
│                                             │
│ [Advanced]                                 │
│   └─ Data Contracts (schemas)              │
└─────────────────────────────────────────────┘
```

**Key Principle:** The top section is a *summary* linking to the full agent designer. The bottom section configures *this agent's role in this workflow*.

---

#### 🔧 Enhancement 6: Add Conditional Agent Creation & Configuration
**Priority:** 🟡 HIGH | **Effort:** Medium | **Impact:** High

**What:** Fully expose ADK-Go's `conditionalagent.New()` pattern:

**Creation:** Add "Conditional Router" card to type selector
```
┌─────────────────────────┐
│  Conditional Router     │
│                         │
│  Route to different     │
│  agents based on        │
│  evaluator output       │
│                         │
│  [Create Router]        │
└─────────────────────────┘
```

**Configuration UI:**
```
┌─ Conditional Router ──────────────────────┐
│                                           │
│ Evaluator Agent: [Sentiment Analyzer ▼]  │
│ Output Key: [sentiment]                   │
│                                           │
│ Routing Rules:                            │
│ ┌─────────────────────────────────────┐  │
│ │ If sentiment = "positive" → Happy  │  │
│ │ If sentiment = "negative" → Empath │  │
│ │ If sentiment = "*" → Default       │  │
│ └─────────────────────────────────────┘  │
│                                           │
│ [+ Add Route]                             │
└───────────────────────────────────────────┘
```

**Also:** Add conditional type to tree builder dropdown.

---

#### 🔧 Enhancement 7: Visualize Coordinator Pattern in Workflow Canvas
**Priority:** 🟡 HIGH | **Effort:** High | **Impact:** High

**What:** When a node is an agent with sub-agents (coordinator pattern), visualize the delegation structure:

**Approach:** Click coordinator node → expand to show sub-agent graph
```
┌─────────────────────────┐
│  Coordinator:           │
│  "Research Manager"     │
│                         │
│  ┌───────────────────┐ │
│  │ Research Manager  │ │
│  │      ↓ ↓ ↓        │ │
│  │  A   B   C        │ │
│  └───────────────────┘ │
│                         │
│  [Expand to Canvas]     │
└─────────────────────────┘
```

**Alternative:** Show coordinator badge on agent nodes, click opens a "mini-canvas" modal showing the sub-agent delegation graph.

---

### Phase 3: Advanced Features

#### 🚀 Enhancement 8: Visualize State Flow (Output Keys)
**Priority:** 🟠 MEDIUM | **Effort:** Medium | **Impact:** Medium

**What:** Show how data flows between agents via `output_key`:

**UI Changes:**
- In workflow canvas, draw labeled arrows between nodes showing state reads/writes
- Node property panel shows "Reads: {var1}, Writes: var2"
- Test panel shows session state key-value pairs after execution

**Example:**
```
[Research Agent] ───(findings)──→ [Writer Agent]
      ↓ writes: findings              ↓ reads: findings
```

---

#### 🚀 Enhancement 9: Expose Model Configuration Precedence
**Priority:** 🟠 MEDIUM | **Effort:** Low | **Impact:** Medium

**What:** Make the 3-layer model config precedence explicit:

**UI Indicator:**
```
┌─ Model Configuration ──────────────────┐
│                                        │
│ ℹ️ Configuration precedence:          │
│ 1. Request override (run-time)        │
│ 2. Agent config (this panel)          │
│ 3. User global defaults               │
│                                        │
│ Current: Using agent config ✓         │
│                                        │
│ [Provider] [Model]                    │
│ [Temperature] [TopP] [TopK] ...       │
│                                        │
│ [Inherit from Global] (toggle)        │
└────────────────────────────────────────┘
```

---

#### 🚀 Enhancement 10: Add Session State Inspector to Test Panel
**Priority:** 🟠 MEDIUM | **Effort:** Medium | **Impact:** Medium

**What:** After workflow execution, show accumulated session state:

**Test Panel Addition:**
```
┌─ Test Results ──────────────────────────┐
│ [Conversation] [Session State ▼]       │
│                                         │
│ Session State:                          │
│ ┌───────────────────────────────────┐  │
│ │ city: "Paris"                     │  │
│ │ weather_report: "Sunny, 22°C"    │  │
│ │ attractions: ["Eiffel Tower",...]│  │
│ │ final_report: "<markdown...>"    │  │
│ └───────────────────────────────────┘  │
│                                         │
│ Output Keys Written:                    │
│ ✅ city (by "Get City" agent)          │
│ ✅ weather_report (by "Weather" agent) │
│ ⏳ attractions (not yet written)       │
└─────────────────────────────────────────┘
```

**Value:** Users can debug state flow issues, see which agents wrote which keys.

---

#### 🚀 Enhancement 11: Expose Advanced Agent Features
**Priority:** 🔵 LOW | **Effort:** Medium | **Impact:** Low-Medium

**What:** Add UI for less common but important ADK-Go features:

**Features to Expose:**
1. **`rerun_on_resume`** — Checkbox in agent form: "Re-execute on workflow resume"
2. **Callback hooks** — Expandable "Callbacks" section:
   - Before Model: [Edit] (custom prompt injection)
   - After Model: [Edit] (output validation/transformation)
   - After Tool: [Edit] (post-processing)
   - On Tool Error: [Edit] (fallback/retry logic)
3. **Optional state injection (`{var?}`)** — Tooltip/guidance distinguishing required `{var}` from optional `{var?}`

**UI Placement:** These go in a new "Advanced Behavior" section (separate from the current grab-bag "Advanced Settings").

---

#### 🚀 Enhancement 12: Data Contract Validation Visualization
**Priority:** 🔵 LOW | **Effort:** High | **Impact:** Medium

**What:** Visualize schema compatibility between connected agents:

**UI Features:**
- When two agents are connected (sequential/parallel), validate output schema of source matches input schema of target
- Show green checkmark or red warning on edges
- Property panel shows "Compatible with downstream agents ✓" or "Schema mismatch with [Agent Name]"

**Example:**
```
[Agent A: Output {report: string}] ───✅──→ [Agent B: Input {report: string}]
[Agent X: Output {data: object}] ───❌──→ [Agent Y: Input {data: string}]
                                             ↑ Type mismatch: object vs string
```

---

## Implementation Roadmap

### Phase 1: Quick Wins (1-2 weeks)
- ✅ Enhancement 1: Unify Model Configuration
- ✅ Enhancement 2: Split "Advanced Settings"
- ✅ Enhancement 3: Clarify Type Selector
- ✅ Enhancement 4: Expose Pipeline Instructions

**Impact:** Immediate cohesion improvements with minimal effort. Fixes the worst violations.

### Phase 2: Structural Improvements (2-4 weeks)
- 🔧 Enhancement 5: Separate Agent vs Workflow Config
- 🔧 Enhancement 6: Add Conditional Agent Support
- 🔧 Enhancement 7: Visualize Coordinator Pattern

**Impact:** Major UX improvements. Users can now work with all ADK-Go patterns visually.

### Phase 3: Advanced Features (4-6 weeks)
- 🚀 Enhancement 8: State Flow Visualization
- 🚀 Enhancement 9: Model Config Precedence
- 🚀 Enhancement 10: Session State Inspector
- 🚀 Enhancement 11: Expose Advanced Features
- 🚀 Enhancement 12: Data Contract Validation

**Impact:** Power user features. Complete ADK-Go feature coverage in UI.

---

## Design Principles Going Forward

1. **1:1 Concept Mapping** — Each ADK-Go concept has exactly one UI representation
2. **Progressive Disclosure** — Common features visible, advanced features accessible but not overwhelming
3. **Semantic Grouping** — Sections named after ADK-Go concepts, not "Advanced"/"Basic"
4. **Visual Consistency** — Same concept looks similar whether in agent form or workflow canvas
5. **Explicit Over Implicit** — Show configuration precedence, state flow, and relationships explicitly

---

## Conclusion

The current UI has grown organically, resulting in cohesion violations that confuse users about ADK-Go's architecture. By realigning the UI to mirror ADK-Go's conceptual structure, we can achieve:

- **Reduced cognitive load** — Users understand the model faster
- **Fewer mistakes** — Clear separation of agent vs workflow config prevents errors
- **Feature discovery** — Hidden ADK-Go features (conditional, callbacks) become accessible
- **Maintainability** — UI structure maps to code structure, making both easier to evolve

**Recommendation:** Start with Phase 1 (quick wins) to fix the worst violations, then incrementally implement Phases 2-3 for complete ADK-Go coverage.
