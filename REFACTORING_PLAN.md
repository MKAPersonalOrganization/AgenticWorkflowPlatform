# Refactoring Plan — Evidence-Based

**Generated:** 2026-04-19T03:05:49.453332+00:00  **Window:** 6 months ago

Three independent signals cross-referenced to pick targets:

| Tool | What it measures |
|---|---|
| `go_hotspots.py` | Change frequency × effective LOC |
| `complexity_trend.py` | Whitespace-indent complexity per revision |
| `file_coupling.py` | Temporal co-change (Jaccard of commit sets) |

Files appearing in **multiple** signals are prioritized — one signal is opinion, two is a hypothesis, three is a diagnosis.

## Top package hotspots

| Score | Revs | LOC | Package |
|---:|---:|---:|---|
| 275,700 | 300 | 919 | `aicompass` |
| 225,736 | 58 | 3,892 | `aicompass/adapters/adkgo` |
| 74,694 | 59 | 1,266 | `aicompass/foundation/agent` |
| 47,656 | 14 | 3,404 | `aicompass/modules/evaluation` |
| 44,640 | 18 | 2,480 | `aicompass/modules/agentdefinition` |
| 34,644 | 12 | 2,887 | `aicompass/modules/mcp` |
| 20,853 | 9 | 2,317 | `aicompass/modules/billing` |
| 18,540 | 10 | 1,854 | `aicompass/modules/knowledge` |
| 16,890 | 10 | 1,689 | `aicompass/modules/a2a` |
| 15,912 | 9 | 1,768 | `aicompass/modules/filesystem` |

## Tier 1 — cross-signal targets

_Appear in ≥ 2 independent signals. Highest leverage._

### `adapters/adkgo/workflow.go`

**Evidence**

- `go_hotspots`: file-aggregate score **2,736** (3 hot functions).
  - `buildLeafAgent` L445-604 — score 2185, 19 revs, 115 LOC
  - `buildRemoteAgentNode` L614-771 — score 327, 3 revs, 109 LOC
  - `buildConditionalNode` L296-436 — score 224, 2 revs, 112 LOC
  - Signals: bus_factor_one, high_churn, knowledge_concentration, large.
- `complexity_trend`: **DETERIORATING** — score 442 (full 442, recent 0), Δmean +0.75, Δtotal +388% vs ΔLOC +196%, max indent 6, 26 commits.
- `file_coupling`: high-coupling partners —
  - `modules/agentdefinition/types.go` — 14 co-changes, J=35
  - `adapters.go` — 13 co-changes, J=31 (structural)
  - `modules/agentdefinition/agent_definition_repo.go` — 9 co-changes, J=27

**Suggested actions (from hotspot analyzer)**

- Knowledge silo — pair with the primary author before refactoring; undocumented invariants likely live only in their head.
- Large and hot function — extract sub-functions along the axes that vary together vs independently. Hot functions that keep growing usually glue 2–3 orthogonal concerns.
- Large but stable — leave alone unless a feature requires touching it; big-and-quiet is working code.

### `adapters/adkgo/runner.go`

**Evidence**

- `go_hotspots`: file-aggregate score **14,133** (3 hot functions).
  - `RunnerAdapter.Run` L132-614 — score 13104, 39 revs, 336 LOC
  - `RunnerAdapter.resolveTools` L758-886 — score 721, 7 revs, 103 LOC
  - `NewRunnerAdapter` L101-128 — score 308, 11 revs, 28 LOC
  - Signals: high_churn, knowledge_concentration, large.
- `file_coupling`: high-coupling partners —
  - `adapters.go` — 15 co-changes, J=26 (structural)

**Suggested actions (from hotspot analyzer)**

- Hot function — extract a strategy / helper; high per-commit surface suggests it accumulates new cases over time.
- Knowledge silo — pair with the primary author before refactoring; undocumented invariants likely live only in their head.
- Large and hot function — extract sub-functions along the axes that vary together vs independently. Hot functions that keep growing usually glue 2–3 orthogonal concerns.
- Large but stable — leave alone unless a feature requires touching it; big-and-quiet is working code.

## Tier 2 — single-signal, high magnitude

_One strong signal. Refactor when you're already in the area._

### `assets/js/chat/chat-sessions.js` 🔥 actively decaying

**Evidence**

- `complexity_trend`: **DETERIORATING** — score 465 (full 282, recent 122), Δmean +0.38, Δtotal +166% vs ΔLOC +140%, max indent 9, 18 commits.

### `assets/js/chat/index.js`

**Evidence**

- `complexity_trend`: **DETERIORATING** — score 375 (full 375, recent 0), Δmean +1.04, Δtotal +288% vs ΔLOC +190%, max indent 9, 47 commits.

### `assets/js/components/editable-workflow-tree.js` 🔥 actively decaying

**Evidence**

- `complexity_trend`: **DETERIORATING** — score 112 (full 88, recent 16), Δmean +0.25, Δtotal +74% vs ΔLOC +63%, max indent 11, 16 commits.

### `modules/evaluation/service.go`

**Evidence**

- `complexity_trend`: **DETERIORATING** — score 338 (full 338, recent 0), Δmean +0.21, Δtotal +288% vs ΔLOC +236%, max indent 6, 9 commits.

### `adapters/adkgo/events.go`

**Evidence**

- `go_hotspots`: file-aggregate score **544** (2 hot functions).
  - `translateModelEvent` L112-210 — score 296, 4 revs, 74 LOC
  - `translateStateDelta` L324-397 — score 248, 4 revs, 62 LOC
  - Signals: large.
- `complexity_trend`: **DETERIORATING** — score 310 (full 310, recent 0), Δmean +0.37, Δtotal +246% vs ΔLOC +168%, max indent 6, 14 commits.

**Suggested actions (from hotspot analyzer)**

- Large but stable — leave alone unless a feature requires touching it; big-and-quiet is working code.

### `adapters/adkgo/tools.go` 🔥 actively decaying

**Evidence**

- `complexity_trend`: **DETERIORATING** — score 119 (full 64, recent 37), Δmean +0.17, Δtotal +74% vs ΔLOC +49%, max indent 4, 8 commits.

### `adapters.go`

**Evidence**

- `file_coupling`: high-coupling partners —
  - `adapters/adkgo/runner.go` — 15 co-changes, J=26 (structural)
  - `adapters/adkgo/workflow.go` — 13 co-changes, J=31 (structural)
  - `modules/agentdefinition/types.go` — 12 co-changes, J=26 (structural)

## Tier 3 — monitor

_Scored but not urgent. Re-run quarterly._

### `modules/agentdefinition/handler.go`

**Evidence**

- `go_hotspots`: file-aggregate score **2,173** (3 hot functions).
  - `storeHandler.update` L310-387 — score 1020, 15 revs, 68 LOC
  - `storeHandler.create` L201-252 — score 705, 15 revs, 47 LOC
  - `toAgentResponse` L165-196 — score 448, 14 revs, 32 LOC
  - Signals: high_churn, knowledge_concentration, large.
- `file_coupling`: high-coupling partners —
  - `adapters/adkgo/workflow.go` — 9 co-changes, J=26
  - `foundation/agent/config.go` — 9 co-changes, J=24 (structural)

**Suggested actions (from hotspot analyzer)**

- Hot function — extract a strategy / helper; high per-commit surface suggests it accumulates new cases over time.
- Knowledge silo — pair with the primary author before refactoring; undocumented invariants likely live only in their head.
- Large and hot function — extract sub-functions along the axes that vary together vs independently. Hot functions that keep growing usually glue 2–3 orthogonal concerns.

### `modules/agentdefinition/agent_definition_repo.go`

**Evidence**

- `go_hotspots`: file-aggregate score **2,117** (2 hot functions).
  - `postgresAgentDefinitionRepository.Create` L61-139 — score 1095, 15 revs, 73 LOC
  - `postgresAgentDefinitionRepository.Update` L141-219 — score 1022, 14 revs, 73 LOC
  - Signals: high_churn, knowledge_concentration, large.
- `file_coupling`: high-coupling partners —
  - `adapters/adkgo/workflow.go` — 9 co-changes, J=27
  - `foundation/agent/config.go` — 9 co-changes, J=26 (structural)

**Suggested actions (from hotspot analyzer)**

- Knowledge silo — pair with the primary author before refactoring; undocumented invariants likely live only in their head.
- Large and hot function — extract sub-functions along the axes that vary together vs independently. Hot functions that keep growing usually glue 2–3 orthogonal concerns.

### `modules/agentdefinition/types.go`

**Evidence**

- `file_coupling`: high-coupling partners —
  - `infra/sqldb/migrate/sql/migrate.sql` — 19 co-changes, J=27
  - `adapters/adkgo/workflow.go` — 14 co-changes, J=35
  - `adapters.go` — 12 co-changes, J=26 (structural)

### `assets/js/chat/chat-state.js`

**Evidence**

- `complexity_trend`: **DETERIORATING** — score 227 (full 218, recent 6), Δmean +0.43, Δtotal +172% vs ΔLOC +124%, max indent 7, 48 commits.

### `assets/js/chat/chat-event-handlers.js`

**Evidence**

- `complexity_trend`: **DETERIORATING** — score 172 (full 169, recent 2), Δmean +0.29, Δtotal +56% vs ΔLOC +41%, max indent 6, 19 commits.

### `foundation/agent/config.go`

**Evidence**

- `file_coupling`: high-coupling partners —
  - `modules/agentdefinition/agent_definition_repo.go` — 9 co-changes, J=26 (structural)
  - `modules/agentdefinition/handler.go` — 9 co-changes, J=24 (structural)

### `modules/chat/projection/agui/postgres_projection.go`

**Evidence**

- `file_coupling`: high-coupling partners —
  - `modules/chat/projection/llmcontext/postgres_projection.go` — 13 co-changes, J=35
  - `modules/chat/threadmgmt/manager.go` — 13 co-changes, J=25

### `infra/projection/subscriber/event_deserializer.go`

**Evidence**

- `file_coupling`: high-coupling partners —
  - `modules/chat/thread/events.go` — 12 co-changes, J=25
  - `modules/chat/handlers/common/converter.go` — 9 co-changes, J=26

### `frontend/ui/landing/features.templ`

**Evidence**

- `file_coupling`: high-coupling partners —
  - `frontend/ui/docs/tools.templ` — 8 co-changes, J=42 (structural)
  - `frontend/ui/docs/agents.templ` — 8 co-changes, J=31 (structural)

### `modules/chat/thread/events.go`

**Evidence**

- `file_coupling`: high-coupling partners —
  - `modules/chat/handlers/common/converter.go` — 22 co-changes, J=44 (structural)
  - `infra/projection/subscriber/event_deserializer.go` — 12 co-changes, J=25

### `frontend/ui/docs/agents.templ`

**Evidence**

- `file_coupling`: high-coupling partners —
  - `frontend/ui/landing/agents.templ` — 9 co-changes, J=43 (structural)
  - `frontend/ui/landing/features.templ` — 8 co-changes, J=31 (structural)

### `modules/chat/handlers/common/converter.go`

**Evidence**

- `file_coupling`: high-coupling partners —
  - `modules/chat/thread/events.go` — 22 co-changes, J=44 (structural)
  - `infra/projection/subscriber/event_deserializer.go` — 9 co-changes, J=26

### `modules/chat/threadmgmt/manager.go`

**Evidence**

- `file_coupling`: high-coupling partners —
  - `modules/chat/thread/thread_aggregate.go` — 14 co-changes, J=29 (structural)
  - `modules/chat/projection/agui/postgres_projection.go` — 13 co-changes, J=25

## Reproduce

```bash
task refactor:plan
```
