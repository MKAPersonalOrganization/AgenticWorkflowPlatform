# Missing Workflow Features — Gap Analysis Report

**Date:** April 12, 2026
**Source:** ADK Samples (`adk-samples`) — changes from last 3 days
**Status:** Our app already supports **Sequential**, **Parallel**, **Loop**, **Conditional**, **Remote/A2A**, and **Nested Composition** workflows with full UI visualization. This report covers the **7 remaining gaps** found in the newest ADK samples.

---

## 1. MCP Server Visualization

### What Is It?

**MCP (Model Context Protocol)** is a standard way to expose tools to AI agents over HTTP (SSE) or stdio. Think of it like a plugin system: an MCP server hosts a set of tools, and an agent connects to it as a client to discover and invoke those tools at runtime.

In the `genmedia-for-commerce` sample, there's a dedicated MCP server with 7 tools (image VTO, video VTO, product spinning, background changer, catalog search, etc.) exposed over both **SSE** and **stdio** transports.

### How It Works

```
Agent ──connects──→ MCP Server
                    ├── Tool: image_vto
                    ├── Tool: video_vto
                    ├── Tool: product_spinning
                    ├── Tool: background_changer
                    ├── Tool: catalog_search
                    └── Tool: product_fitting
```

The agent discovers available tools at runtime, and the MCP server handles the actual execution. This decouples the agent logic from the tool implementation.

### What's Missing in Our UI

We already support MCP servers in our **Tools/Plugins dialog** — users can see registered MCP servers and their tools. But we don't have:

- **MCP-specific workflow visualization**: When a tool call routes through an MCP server, the UI shows it as a regular tool call. There's no visual indicator that it came from an MCP server vs. a built-in plugin.
- **MCP server status in the pipeline tree**: In the workflow graph panel, MCP-based tools appear as flat leaf nodes. We don't show "this agent connects to an MCP server with N tools."
- **MCP transport indicators**: No badge showing whether the connection is SSE, stdio, or streamable HTTP.

### Value Assessment

| Dimension | Rating | Notes |
|---|---|---|
| **User-facing value** | Medium | Most end users don't care *how* a tool is implemented. They just see it works. |
| **Developer value** | High | Knowing whether a tool is MCP-based helps debug connection issues, latency, and tool discovery problems. |
| **Effort to add** | Low–Medium | We already have the MCP infrastructure. Adding badges and tree annotations is mostly frontend work. |

**Recommendation:** Worth adding **MCP badges on tool calls** and **server annotations in the pipeline tree**. Low effort, improves developer experience.

---

## 2. Callback Lifecycle Visualization

### What Is It?

ADK agents have **four callback hooks** that fire at specific points during execution:

| Callback | When It Fires | What It's Used For |
|---|---|---|
| `before_model_callback` | Right before the LLM is called | Inject session context, prefix messages, modify the prompt |
| `after_model_callback` | Right after the LLM responds | Append grounding metadata, transform the response, store results |
| `before_tool_callback` | Right before a tool executes | Inject arguments, add uploaded files, validate inputs |
| `after_tool_callback` | Right after a tool finishes | Extract results, save artifacts, clean up the response sent back to the LLM |

In `genmedia-for-commerce`, all four callbacks are used: before_model injects session IDs, before_tool auto-injects uploaded images into tool arguments, after_tool saves generated media as artifacts, and after_model stores the final response.

### How It Works

```
User message
  → before_model_callback (inject session ID, append to history)
    → LLM generates response
  → after_model_callback (store response, add grounding refs)
    → before_tool_callback (inject uploaded images into args)
      → Tool executes
    → after_tool_callback (save base64 output as artifact, return clean summary)
      → LLM continues or finalizes
```

### What's Missing in Our UI

We have `PromptInjector` (a before_model-like callback) and the runner handles tool execution. But the UI doesn't show:

- **Which callbacks are active** on a given agent
- **What each callback did** during a specific run (e.g., "before_tool injected 2 images")
- **Callback execution order** in the event log

### Value Assessment

| Dimension | Rating | Notes |
|---|---|---|
| **User-facing value** | Low | End users don't need to see callback internals. |
| **Developer value** | High | Critical for debugging — when a tool gets wrong inputs or the LLM gets unexpected outputs, callbacks are often the culprit. |
| **Effort to add** | Medium | Requires adding callback events to the SSE stream and rendering them in the debugger/event log. |

**Recommendation:** Add callback indicators to the **debugger panel only** (not the main chat UI). Show "before_tool injected X" as a subtle log entry. Useful for development/debugging, invisible to end users.

---

## 3. Constraint Satisfaction Visualization

### What Is It?

Some agents don't just call tools — they run **constraint optimization solvers** to find the best answer among millions of possibilities. The `nexshift-agent` uses **OR-Tools CP-SAT** (Constraint Programming — Satisfiability) to generate nurse rosters that satisfy dozens of hard rules (certifications, seniority, max hours) and soft preferences (fairness, burnout prevention, shift preferences).

The solver doesn't just produce an answer — it produces a **feasibility report**:

- **Hard constraints**: All met? ✓/✗ (if any fail, the solution is invalid)
- **Soft constraints**: What percentage satisfied? (e.g., "85% of fairness preferences met")
- **Optimality gap**: How close to the theoretical best solution? (e.g., "within 2% of optimal")
- **Solve time**: How long did it take? (e.g., "3.2 seconds")

### How It Works

```
Agent receives: "Create next week's nurse roster"
  → Constraint solver runs (OR-Tools CP-SAT)
  → Finds solution satisfying 100% of hard constraints, 85% of soft constraints
  → Compliance validator checks programmatically: ✓ All certified nurses assigned correctly
  → Empathy validator checks: ⚠ 2 nurses got 3 night shifts in a row (burnout risk)
  → Agent presents: "Here's the roster + compliance report + fairness notes"
```

### What's Missing in Our UI

Our parallel branch rendering shows tool calls and text responses, but we have **no dedicated UI for constraint solver results**. If an agent returns a compliance report as text, it renders as a markdown block — not as a structured visualization.

We don't show:
- Constraint satisfaction scorecards (hard vs. soft constraints)
- Optimality gap indicators
- Solver performance metrics (solve time, iterations)
- Visual "constraints met / constraints violated" breakdowns

### Value Assessment

| Dimension | Rating | Notes |
|---|---|---|
| **User-facing value** | High | Users love seeing "100% compliant" badges and understanding trade-offs. This makes AI decisions transparent and trustworthy. |
| **Developer value** | High | Seeing which constraints failed and by how much is essential for tuning the solver. |
| **Effort to add** | Medium–High | Requires detecting constraint solver output (either via a specific tool type or by parsing structured output) and rendering a dedicated component. |

**Recommendation:** If your users will build agents that use constraint solvers (scheduling, resource allocation, optimization), this is **very valuable**. If not, skip it. Consider a generic "structured report" component that can render constraint results when the agent outputs them in a known format.

---

## 4. Evaluation Dashboard

### What Is It?

ADK samples now include **automated evaluation suites** — collections of test cases that score an agent's performance. The `nexshift-agent` has **82 test cases** across 7 evaluation sets (coordinator, context_gatherer, solver, compliance, empathy, presenter, workflow).

Each test case has:
- **Input**: What you feed the agent
- **Expected output**: What the agent should produce
- **Scoring method**: How to measure correctness (exact match, tool trajectory, semantic similarity)
- **Score**: Pass/fail or a numeric score

### How It Works

```
Evaluation Suite: nexshift-agent
├── Coordinator (12 tests) → 92% pass rate
├── Context Gatherer (8 tests) → 100% pass rate
├── Roster Solver (20 tests) → 85% pass rate
├── Compliance Officer (15 tests) → 100% pass rate
├── Empathy Advocate (10 tests) → 80% pass rate
├── Roster Presenter (7 tests) → 86% pass rate
└── Full Workflow (10 tests) → 70% pass rate
```

### What's Missing in Our UI

We have no evaluation results display. There's no place in the app where you can:
- See how well an agent performs on its test suite
- Compare agent versions by evaluation score
- Drill into individual test failures
- Track evaluation scores over time

### Value Assessment

| Dimension | Rating | Notes |
|---|---|---|
| **User-facing value** | Medium | Some users (team leads, QA) would love seeing "this agent scores 92% on tests." Most end users won't care. |
| **Developer value** | Very High | Essential for agent development. Without it, you're flying blind on whether changes improve or break agent behavior. |
| **Effort to add** | High | Requires an evaluation framework, test storage, scoring infrastructure, and a full dashboard UI. |

**Recommendation:** **Important but not urgent.** If you're building a platform where teams develop and iterate on agents, an evaluation dashboard is essential. For a single-agent or demo setup, it's not needed. Consider this a **Phase 2** feature.

---

## 5. Ambient/Event-Driven Processing Indicators

### What Is It?

Most agents are **interactive** — a user sends a message, the agent responds. But some agents are **ambient** — they run autonomously in response to events, with no human in the loop.

The `ambient-support-agent` sample processes support tickets that arrive via **Pub/Sub** or on a **Cloud Scheduler** schedule. When a ticket arrives, the agent:
1. Parses the ticket
2. Classifies priority (P0–P3)
3. Categorizes the issue
4. Recommends next steps
5. For critical tickets (P0/P1), emits structured log alerts that trigger email notifications

The user never "chats" with this agent. It runs silently and only surfaces results when something needs attention.

### How It Works

```
Event Source (Pub/Sub or Cloud Scheduler)
  → Trigger endpoint (/trigger/pubsub)
    → Agent creates ephemeral session
    → Processes ticket autonomously
    → Produces structured triage report
    → If P0/P1: emits CRITICAL log entry
      → Cloud Monitoring detects pattern
      → Sends email alert to on-call team
```

### What's Missing in Our UI

Our entire chat UI assumes **synchronous, user-initiated conversations**. We have no UI for:

- **Ambient agent status**: Is the ambient agent listening? Did it process an event recently?
- **Event trigger indicators**: A visual cue that "an event triggered this agent to run"
- **Autonomous execution log**: A timeline of events processed, with triage results
- **Alert configuration**: Where users can set up what counts as "critical" and who gets notified
- **Scheduled runs indicator**: A badge showing "next scheduled run at 8:00 AM UTC"

### Value Assessment

| Dimension | Rating | Notes |
|---|---|---|
| **User-facing value** | Medium–High | For support teams, operations teams, and monitoring use cases, seeing "3 tickets processed in the last hour, 1 critical" is very useful. |
| **Developer value** | High | Knowing that events are arriving and being processed is essential for debugging ambient agents. |
| **Effort to add** | Medium | Requires adding ambient agent state to the agent model and creating a monitoring panel. |

**Recommendation:** **Worth supporting if you expect ambient agent use cases.** Even a simple "last processed: 5 min ago, events today: 12" badge on the agent card would add significant visibility. A full event timeline would be a nice addition to the debugger.

---

## 6. Grounding Metadata Rendering

### What Is It?

When an LLM uses **grounding** (Google Search, Vertex AI Search, or custom retrieval), it receives **grounding metadata** alongside its response. This metadata includes:

- **Grounding Chunks**: The text snippets the model used as source material
- **Support Scores**: How well each chunk supports the response
- **Web Search Results**: If Google Search was used, the search queries and result snippets

In the `financial-advisor` Go sample, the `after_model_callback` uses `renderReference` to transform grounding chunks into **markdown footnote references** appended to the response:

```markdown
Based on current market conditions, tech stocks show moderate growth potential.[1][2]

---
**References:**
[1] Bloomberg — "Tech Sector Q2 Outlook" (Score: 0.92)
[2] Reuters — "Market Analysis Report" (Score: 0.87)
```

### How It Works

```
LLM call with grounding enabled
  → LLM responds + receives grounding metadata
    → GroundingChunks[] = [
        {web: {title: "Q2 Report", uri: "...", content: "..."}},
        {retrieved_context: {title: "...", text: "..."}}
      ]
    → Callback renders them as formatted references
    → References appended to response
```

### What's Missing in Our UI

Our assistant message rendering shows text, tool calls, and thinking blocks, but we **don't render grounding metadata as structured citations**. If grounding chunks are present, they're either:
- Embedded in the response text by the LLM (unstructured)
- Not displayed at all

We don't show:
- Numbered footnote-style citations linking source to response claims
- A "References" section below the message with source titles, URLs, and relevance scores
- Distinguishing between web search results and retrieved context

### Value Assessment

| Dimension | Rating | Notes |
|---|---|---|
| **User-facing value** | Very High | This is a **trust and transparency** feature. Users need to know *why* the agent said what it said. Showing sources dramatically increases trust. |
| **Developer value** | Medium | Helpful for debugging grounding quality, but less critical than for end users. |
| **Effort to add** | Low–Medium | The grounding metadata is already in the event stream. We just need to detect it and render a references section below assistant messages. |

**Recommendation:** **Strongly recommend adding.** This is one of the highest-value, lowest-effort features on this list. A simple "References" collapsible below grounded messages with source titles and links would go a long way. Particularly important for research, education, and compliance use cases.

---

## 7. Sub-Agent Hierarchy Visualization (Agent-as-Tool)

### What Is It?

In ADK, you can wrap an entire agent as a **tool** for another agent using `agenttool.New(subAgent, config)`. This creates a nested hierarchy where the parent agent can delegate to sub-agents as needed, and those sub-agents can themselves have sub-agents.

```
Financial Advisor (root agent)
  ├── weather_tool (built-in tool)
  ├── market_data_tool (built-in tool)
  ├── data_analyst_agent (agent-as-tool via agenttool.New)
  │     ├── pandas_query (tool)
  │     └── visualization (tool)
  └── risk_assessor_agent (agent-as-tool via agenttool.New)
        ├── portfolio_analyzer (tool)
        └── risk_score_calculator (tool)
```

The root agent decides when to delegate to `data_analyst_agent` or `risk_assessor_agent`. Those sub-agents then run their own tool-use loops independently.

### What's Missing in Our UI

We show the **agent name** on messages (e.g., "Data Analyst" badge), and we show the **pipeline tree** for sequential/parallel/loop workflows. But we don't show:

- **Agent-as-tool relationships**: When an agent delegates to a sub-agent via `agenttool`, the UI doesn't distinguish this from a regular agent transfer in a sequential workflow.
- **Nested tool hierarchy**: The pipeline tree shows leaf agents, but doesn't show "this agent has 5 tools and 2 sub-agents."
- **Delegation depth**: How many levels deep the agent hierarchy goes.

### Value Assessment

| Dimension | Rating | Notes |
|---|---|---|
| **User-facing value** | Medium | For complex agents, seeing the delegation tree helps users understand the agent's capabilities and why it made certain choices. |
| **Developer value** | High | Understanding the agent hierarchy is essential for debugging delegation issues and optimizing performance. |
| **Effort to add** | Medium | Requires extending the workflow node model to distinguish agent-as-tool nodes from regular workflow nodes, and updating the tree visualizer. |

**Recommendation:** **Worth adding to the pipeline tree and debugger.** A small "tool" icon on agent-as-tool nodes in the tree would clarify the distinction. For the agent editor, showing the sub-agent hierarchy as a nested list would help users understand agent capabilities.

---

## Summary: Priority Matrix

| # | Feature | User Value | Dev Value | Effort | Recommendation |
|---|---|---|---|---|---|
| 1 | **MCP Server Visualization** | Medium | High | Low–Med | ✅ Add MCP badges on tool calls and pipeline tree |
| 2 | **Callback Lifecycle Visualization** | Low | High | Medium | 🔶 Add to debugger panel only (dev tool) |
| 3 | **Constraint Satisfaction Visualization** | High | High | Med–High | 🔶 Build if your users need solvers (scheduling, optimization) |
| 4 | **Evaluation Dashboard** | Medium | Very High | High | 📅 Phase 2 — important for agent development platforms |
| 5 | **Ambient/Event-Driven Processing** | Med–High | High | Medium | ✅ Add if you expect ambient agent use cases |
| 6 | **Grounding Metadata Rendering** | **Very High** | Medium | **Low–Med** | ✅✅ **Do this first** — highest ROI |
| 7 | **Sub-Agent Hierarchy Visualization** | Medium | High | Medium | 🔶 Worth adding to pipeline tree and debugger |

### Quick Decision Guide

| If you want to... | Prioritize |
|---|---|
| **Improve trust & transparency quickly** | #6 Grounding Metadata Rendering |
| **Support MCP-based agents better** | #1 MCP Server Visualization |
| **Build scheduling/optimization agents** | #3 Constraint Satisfaction Visualization |
| **Support autonomous/event-driven agents** | #5 Ambient Processing |
| **Improve developer debugging** | #2 Callback Lifecycle + #7 Sub-Agent Hierarchy |
| **Build an agent development platform** | #4 Evaluation Dashboard (Phase 2) |
