# Code Signals — Data Report

**Generated:** 2026-04-19T03:35:51.322708+00:00  **Window:** 6 months ago

This document **collects measurements**. It does not rank, prioritize, or recommend. Files are grouped by how many analyses mentioned them (an objective count), and within each group listed alphabetically.

**Markdown scope.** To keep this document readable, files shown here must appear in ≥ 2 analyses AND have ≥ 3 commits in the window. The JSON report emitted alongside this file contains the full, unfiltered dataset.

## What each column means

| Column | Source | What it is |
|---|---|---|
| hotspot-score | `go_hotspots.py` | Revisions × effective LOC for Go packages/functions |
| trend-verdict | `complexity_trend.py` | Classification by threshold rules: DETERIORATING (Δmean > 0.08 or Δtotal > ΔLOC+15%), REFACTORED (peak-dip > 12%), STABLE (|Δtotal| < 5% & |Δmean| < 0.05), GROWING (otherwise) |
| Δmean | `complexity_trend.py` | Last sample mean indent minus first sample mean indent (whole window) |
| max-indent | `complexity_trend.py` | Deepest whitespace-indent in the last sampled revision |
| fix-pct | `bugfix_density.py` | Fraction of this file's commits whose subject line matched fix keywords (`fix`, `bug`, `hotfix`, `regression`, `crash`, `revert`, `typo`, etc., word-bounded) |
| coupling count | `file_coupling.py` | Number of partner files that co-change with this one at cooccurrence ≥ 6 |

## Top package hotspots (orientation)

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

## File overview (66 files)

| File | Signals | Commits | Fix count | Fix % | Trend verdict | Δmean | Max indent | Hotspot score | Coupling partners |
|---|:-:|---:|---:|---:|:-:|---:|---:|---:|---:|
| `adapters/adkgo/runner.go` | 4 | 45 | 3 | 7 | REFACTORED | +0.16 | 6 | 14,133 | 1 |
| `adapters/adkgo/workflow.go` | 4 | 26 | 0 | 0 | DETERIORATING | +0.75 | 6 | 2,736 | 4 |
| `modules/agentdefinition/handler.go` | 4 | 10 | 0 | 0 | GROWING | -0.02 | 4 | 2,173 | 3 |
| `modules/agentdefinition/service.go` | 4 | 8 | 0 | 0 | GROWING | +0.03 | 5 | 1,028 | 1 |
| `adapters/adkgo/events.go` | 3 | 14 | 1 | 7 | DETERIORATING | +0.37 | 6 | 544 |  |
| `adapters/adkgo/model.go` | 3 | 10 | 2 | 20 | GROWING | -0.03 | 7 | 212 |  |
| `adapters/adkgo/ports.go` | 3 | 17 | 0 | 0 | GROWING | -0.03 | 2 |  | 1 |
| `foundation/agent/config.go` | 3 | 27 | 1 | 4 | GROWING | -0.18 | 3 |  | 2 |
| `modules/agentdefinition/agent_definition_repo.go` | 3 | 7 | 0 | 0 |  |  |  | 2,117 | 2 |
| `modules/agentdefinition/catalog_yaml.go` | 3 | 8 | 0 | 0 | GROWING | +0.02 | 4 | 675 |  |
| `modules/agentdefinition/types.go` | 3 | 10 | 0 | 0 | GROWING | +0.03 | 3 |  | 3 |
| `modules/chat/chat.go` | 3 | 23 | 1 | 4 | REFACTORED | -0.07 | 2 |  | 2 |
| `modules/chat/projection/agui/postgres_projection.go` | 3 | 9 | 0 | 0 | GROWING | -0.03 | 9 |  | 2 |
| `modules/chat/thread/events.go` | 3 | 12 | 1 | 8 | GROWING | +0.00 | 1 |  | 2 |
| `MISALIGNMENTS.md` | 2 | 9 | 0 | 0 |  |  |  |  | 1 |
| `adapters.go` | 2 | 33 | 0 | 0 |  |  |  |  | 4 |
| `adapters/adkgo/agent_builder.go` | 2 | 9 | 1 | 11 |  |  |  | 210 |  |
| `adapters/adkgo/session.go` | 2 | 8 | 2 | 25 | GROWING | -0.12 | 5 |  |  |
| `adapters/adkgo/tools.go` | 2 | 8 | 0 | 0 | DETERIORATING | +0.17 | 4 |  |  |
| `assets/js/chat/chat-event-handlers.js` | 2 | 19 | 2 | 10 | DETERIORATING | +0.29 | 6 |  |  |
| `assets/js/chat/chat-messaging.js` | 2 | 35 | 7 | 20 | GROWING | -0.80 | 7 |  |  |
| `assets/js/chat/chat-sessions.js` | 2 | 18 | 1 | 6 | DETERIORATING | +0.38 | 9 |  |  |
| `assets/js/chat/chat-state.js` | 2 | 48 | 6 | 12 | DETERIORATING | +0.43 | 7 |  |  |
| `assets/js/chat/chat-streaming.js` | 2 | 43 | 3 | 7 | REFACTORED | +0.43 | 10 |  |  |
| `assets/js/chat/chat-utils.js` | 2 | 14 | 2 | 14 | GROWING | +0.01 | 5 |  |  |
| `assets/js/chat/index.js` | 2 | 47 | 4 | 8 | DETERIORATING | +1.04 | 9 |  |  |
| `assets/js/chat/markdown-renderer.js` | 2 | 12 | 1 | 8 | REFACTORED | -1.06 | 7 |  |  |
| `assets/js/components/agent-designer-dialog.js` | 2 | 47 | 3 | 6 | GROWING | +0.02 | 7 |  |  |
| `assets/js/components/agents-dialog.js` | 2 | 18 | 1 | 6 | REFACTORED | +0.02 | 5 |  |  |
| `assets/js/components/alpineflow.js` | 2 | 10 | 0 | 0 | GROWING | +0.06 | 7 |  |  |
| `assets/js/components/editable-workflow-tree.js` | 2 | 16 | 1 | 6 | DETERIORATING | +0.25 | 11 |  |  |
| `assets/js/components/workflow-tree-visualizer.js` | 2 | 8 | 0 | 0 | REFACTORED | -0.15 | 8 |  |  |
| `foundation/agent/agent.go` | 2 | 25 | 0 | 0 | GROWING | -0.04 | 3 |  |  |
| `foundation/agent/message.go` | 2 | 8 | 0 | 0 | DETERIORATING | +0.16 | 3 |  |  |
| `foundation/agent/tool.go` | 2 | 10 | 0 | 0 | REFACTORED | -0.24 | 2 |  |  |
| `foundation/toolregistry/plugin.go` | 2 | 4 | 0 | 0 |  |  |  |  | 3 |
| `frontend/ui/agenteditor/agenteditor.templ` | 2 | 11 | 1 | 9 | INSUFFICIENT |  |  |  |  |
| `frontend/ui/docs/agents.templ` | 2 | 6 | 1 | 17 |  |  |  |  | 2 |
| `frontend/ui/docs/tools.templ` | 2 | 3 | 0 | 0 |  |  |  |  | 2 |
| `infra/projection/subscriber/event_deserializer.go` | 2 | 19 | 1 | 5 |  |  |  |  | 2 |
| `infra/projection/subscriber/projection_updater_new.go` | 2 | 9 | 0 | 0 | GROWING | -0.29 | 7 |  |  |
| `infra/sqldb/migrate/sql/migrate.sql` | 2 | 61 | 2 | 3 |  |  |  |  | 1 |
| `infra/sqldb/sqldb.go` | 2 | 10 | 1 | 10 | GROWING | +0.01 | 4 |  |  |
| `main.go` | 2 | 287 | 9 | 3 | DETERIORATING | +1.07 | 6 |  |  |
| `modules/a2a/service.go` | 2 | 9 | 0 | 0 | DETERIORATING | +0.14 | 5 |  |  |
| `modules/agentdefinition/module.go` | 2 | 3 | 0 | 0 |  |  |  |  | 1 |
| `modules/agentdefinition/provider.go` | 2 | 5 | 0 | 0 |  |  |  | 270 |  |
| `modules/analytics/analytics.go` | 2 | 8 | 0 | 0 | DETERIORATING | +0.10 | 3 |  |  |
| `modules/chat/agentorchestrator/service.go` | 2 | 17 | 1 | 6 | INSUFFICIENT |  |  |  |  |
| `modules/chat/agui/handlers.go` | 2 | 8 | 0 | 0 |  |  |  |  | 1 |
| `modules/chat/handlers/common/converter.go` | 2 | 3 | 0 | 0 |  |  |  |  | 2 |
| `modules/chat/projection/agui/agui_projection.go` | 2 | 8 | 0 | 0 |  |  |  |  | 1 |
| `modules/chat/projection/llmcontext/postgres_projection.go` | 2 | 4 | 0 | 0 |  |  |  |  | 1 |
| `modules/chat/thread/thread_aggregate.go` | 2 | 7 | 1 | 14 |  |  |  |  | 1 |
| `modules/chat/threadmgmt/manager.go` | 2 | 4 | 0 | 0 |  |  |  |  | 2 |
| `modules/evaluation/handler.go` | 2 | 12 | 0 | 0 | GROWING | -0.02 | 5 |  |  |
| `modules/evaluation/service.go` | 2 | 9 | 2 | 22 | DETERIORATING | +0.21 | 6 |  |  |
| `modules/evaluation/types.go` | 2 | 8 | 1 | 12 | GROWING | -0.00 | 1 |  |  |
| `modules/knowledge/handler.go` | 2 | 9 | 0 | 0 | STABLE | +0.01 | 4 |  |  |
| `modules/mcp/handler.go` | 2 | 8 | 0 | 0 | DETERIORATING | +1.19 | 4 |  |  |
| `modules/mcp/service.go` | 2 | 9 | 0 | 0 | GROWING | -0.04 | 8 |  |  |
| `modules/templates/handler.go` | 2 | 4 | 1 | 25 |  |  |  |  | 2 |
| `modules/toolreqs/service.go` | 2 | 10 | 2 | 20 | DETERIORATING | +0.13 | 3 |  |  |
| `plugins/document/provider.go` | 2 | 11 | 0 | 0 |  |  |  |  | 3 |
| `plugins/knowledgebase/provider.go` | 2 | 11 | 0 | 0 |  |  |  |  | 3 |
| `plugins/tts/provider.go` | 2 | 13 | 0 | 0 |  |  |  |  | 4 |

## Detail — files in 4 analyses

### `adapters/adkgo/runner.go`  (signals: hotspot, trend, coupling, bugfix)

**go_hotspots** — file-aggregate score 14,133 across 3 hot function(s):
- `RunnerAdapter.Run` L132-614 — score 13,104, 39 revs, 336 LOC · bug-fix 10%
- `RunnerAdapter.resolveTools` L758-886 — score 721, 7 revs, 103 LOC
- `NewRunnerAdapter` L101-128 — score 308, 11 revs, 28 LOC

Hotspot signals: high_churn, knowledge_concentration, large.

**complexity_trend** — classified **REFACTORED** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 45  ·  LOC: 989
- first sample: n=757  total=1548.0  mean=2.04  max=7
- last sample:  n=989  total=2177.0  mean=2.20  max=6
- Δtotal +41%  ·  ΔLOC +31%  ·  Δmean indent +0.16  ·  peak-dip 26%
- recent window (last 2 samples): Δtotal -26%  ·  Δmean -0.53

**bugfix_density** — 3 fix-flavored commits out of 45 touches (7%). Last fix: 2026-04-15.
- `c7d0bdd57d11` (2026-04-15): fix(chat): preserve OriginalToolCallID for stateless confirmation session rebuild
- `8946ffad9eb6` (2026-04-15): feat(chat): implement tool confirmation UI and fix confirmation flow
- `cebf29a316e1` (2026-04-01): fix: Improve session handling and error logging in RunnerAdapter

**file_coupling** — 1 partner(s) co-changing at cooccurrence ≥ 6:
- `adapters.go` — 15 co-changes, J=26 (structural Go import)

### `adapters/adkgo/workflow.go`  (signals: hotspot, trend, coupling, bugfix)

**go_hotspots** — file-aggregate score 2,736 across 3 hot function(s):
- `buildLeafAgent` L445-604 — score 2,185, 19 revs, 115 LOC · bug-fix 5%
- `buildRemoteAgentNode` L614-771 — score 327, 3 revs, 109 LOC
- `buildConditionalNode` L296-436 — score 224, 2 revs, 112 LOC

Hotspot signals: bus_factor_one, high_churn, knowledge_concentration, large.

**complexity_trend** — classified **DETERIORATING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 26  ·  LOC: 695
- first sample: n=235  total=271.0  mean=1.15  max=4
- last sample:  n=695  total=1323.0  mean=1.90  max=6
- Δtotal +388%  ·  ΔLOC +196%  ·  Δmean indent +0.75  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +3%  ·  Δmean -0.00

**bugfix_density** — 0 fix-flavored commits out of 26 touches (0%). Last fix: n/a.

**file_coupling** — 4 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/agentdefinition/types.go` — 14 co-changes, J=35
- `adapters.go` — 13 co-changes, J=31 (structural Go import)
- `modules/agentdefinition/agent_definition_repo.go` — 9 co-changes, J=27
- `modules/agentdefinition/handler.go` — 9 co-changes, J=26

### `modules/agentdefinition/handler.go`  (signals: hotspot, trend, coupling, bugfix)

**go_hotspots** — file-aggregate score 2,173 across 3 hot function(s):
- `storeHandler.update` L310-387 — score 1,020, 15 revs, 68 LOC
- `storeHandler.create` L201-252 — score 705, 15 revs, 47 LOC
- `toAgentResponse` L165-196 — score 448, 14 revs, 32 LOC

Hotspot signals: high_churn, knowledge_concentration, large.

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 3  ·  commits observed: 10  ·  LOC: 645
- first sample: n=575  total=710.0  mean=1.24  max=4
- last sample:  n=645  total=786.0  mean=1.22  max=4
- Δtotal +11%  ·  ΔLOC +12%  ·  Δmean indent -0.02  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +4%  ·  Δmean +0.01

**bugfix_density** — 0 fix-flavored commits out of 10 touches (0%). Last fix: n/a.

**file_coupling** — 3 partner(s) co-changing at cooccurrence ≥ 6:
- `adapters/adkgo/workflow.go` — 9 co-changes, J=26
- `foundation/agent/config.go` — 9 co-changes, J=24 (structural Go import)
- `MISALIGNMENTS.md` — 6 co-changes, J=27 (structural Go import)

### `modules/agentdefinition/service.go`  (signals: hotspot, trend, coupling, bugfix)

**go_hotspots** — file-aggregate score 1,028 across 3 hot function(s):
- `Service.update` L194-249 — score 352, 8 revs, 44 LOC
- `Service.ImportComposedFromCatalogEntry` L120-191 — score 348, 6 revs, 58 LOC
- `Service.create` L31-84 — score 328, 8 revs, 41 LOC

Hotspot signals: bus_factor_one, knowledge_concentration.

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 3  ·  commits observed: 8  ·  LOC: 760
- first sample: n=444  total=638.0  mean=1.44  max=4
- last sample:  n=760  total=1112.0  mean=1.46  max=5
- Δtotal +74%  ·  ΔLOC +71%  ·  Δmean indent +0.03  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +42%  ·  Δmean -0.01

**bugfix_density** — 0 fix-flavored commits out of 8 touches (0%). Last fix: n/a.

**file_coupling** — 1 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/templates/handler.go` — 6 co-changes, J=25 (structural Go import)

## Detail — files in 3 analyses

### `adapters/adkgo/events.go`  (signals: hotspot, trend, bugfix)

**go_hotspots** — file-aggregate score 544 across 2 hot function(s):
- `translateModelEvent` L112-210 — score 296, 4 revs, 74 LOC · bug-fix 25%
- `translateStateDelta` L324-397 — score 248, 4 revs, 62 LOC

Hotspot signals: large.

**complexity_trend** — classified **DETERIORATING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 14  ·  LOC: 598
- first sample: n=223  total=288.0  mean=1.29  max=4
- last sample:  n=598  total=995.0  mean=1.66  max=6
- Δtotal +246%  ·  ΔLOC +168%  ·  Δmean indent +0.37  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +15%  ·  Δmean -0.03

**bugfix_density** — 1 fix-flavored commits out of 14 touches (7%). Last fix: 2026-04-15.
- `8946ffad9eb6` (2026-04-15): feat(chat): implement tool confirmation UI and fix confirmation flow

### `adapters/adkgo/model.go`  (signals: hotspot, trend, bugfix)

**go_hotspots** — file-aggregate score 212 across 1 hot function(s):
- `openAIModel.GenerateContent` L80-145 — score 212, 4 revs, 53 LOC · bug-fix 50%

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 10  ·  LOC: 680
- first sample: n=399  total=828.0  mean=2.08  max=7
- last sample:  n=680  total=1392.0  mean=2.05  max=7
- Δtotal +68%  ·  ΔLOC +70%  ·  Δmean indent -0.03  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +1%  ·  Δmean -0.00

**bugfix_density** — 2 fix-flavored commits out of 10 touches (20%). Last fix: 2026-04-17.
- `30774a21d4dc` (2026-04-17): fix(adk): make OutputSchema actually respected on every provider
- `4bbaca709c11` (2026-04-12): fix: skip temperature for GPT-5 and o1 models

### `adapters/adkgo/ports.go`  (signals: trend, coupling, bugfix)

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 17  ·  LOC: 159
- first sample: n=86  total=45.0  mean=0.52  max=2
- last sample:  n=159  total=79.0  mean=0.50  max=2
- Δtotal +76%  ·  ΔLOC +85%  ·  Δmean indent -0.03  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +0%  ·  Δmean +0.00

**bugfix_density** — 0 fix-flavored commits out of 17 touches (0%). Last fix: n/a.

**file_coupling** — 1 partner(s) co-changing at cooccurrence ≥ 6:
- `adapters.go` — 9 co-changes, J=26 (structural Go import)

### `foundation/agent/config.go`  (signals: trend, coupling, bugfix)

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 27  ·  LOC: 322
- first sample: n=36  total=25.0  mean=0.69  max=1
- last sample:  n=322  total=166.0  mean=0.52  max=3
- Δtotal +564%  ·  ΔLOC +794%  ·  Δmean indent -0.18  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +24%  ·  Δmean -0.10

**bugfix_density** — 1 fix-flavored commits out of 27 touches (4%). Last fix: 2026-04-17.
- `27c1a0dc7a98` (2026-04-17): docs: fix output_schema description to match ADK code behavior

**file_coupling** — 2 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/agentdefinition/agent_definition_repo.go` — 9 co-changes, J=26 (structural Go import)
- `modules/agentdefinition/handler.go` — 9 co-changes, J=24 (structural Go import)

### `modules/agentdefinition/agent_definition_repo.go`  (signals: hotspot, coupling, bugfix)

**go_hotspots** — file-aggregate score 2,117 across 2 hot function(s):
- `postgresAgentDefinitionRepository.Create` L61-139 — score 1,095, 15 revs, 73 LOC
- `postgresAgentDefinitionRepository.Update` L141-219 — score 1,022, 14 revs, 73 LOC

Hotspot signals: high_churn, knowledge_concentration, large.

**bugfix_density** — 0 fix-flavored commits out of 7 touches (0%). Last fix: n/a.

**file_coupling** — 2 partner(s) co-changing at cooccurrence ≥ 6:
- `adapters/adkgo/workflow.go` — 9 co-changes, J=27
- `foundation/agent/config.go` — 9 co-changes, J=26 (structural Go import)

### `modules/agentdefinition/catalog_yaml.go`  (signals: hotspot, trend, bugfix)

**go_hotspots** — file-aggregate score 675 across 1 hot function(s):
- `yamlCatalogSpec.toEntry` L86-168 — score 675, 9 revs, 75 LOC

Hotspot signals: bus_factor_one, knowledge_concentration, large.

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 5  ·  commits observed: 8  ·  LOC: 178
- first sample: n=117  total=157.0  mean=1.34  max=4
- last sample:  n=178  total=243.0  mean=1.36  max=4
- Δtotal +55%  ·  ΔLOC +52%  ·  Δmean indent +0.02  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +5%  ·  Δmean -0.03

**bugfix_density** — 0 fix-flavored commits out of 8 touches (0%). Last fix: n/a.

### `modules/agentdefinition/types.go`  (signals: trend, coupling, bugfix)

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 3  ·  commits observed: 10  ·  LOC: 594
- first sample: n=361  total=275.0  mean=0.76  max=3
- last sample:  n=594  total=468.0  mean=0.79  max=3
- Δtotal +70%  ·  ΔLOC +64%  ·  Δmean indent +0.03  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +34%  ·  Δmean +0.03

**bugfix_density** — 0 fix-flavored commits out of 10 touches (0%). Last fix: n/a.

**file_coupling** — 3 partner(s) co-changing at cooccurrence ≥ 6:
- `infra/sqldb/migrate/sql/migrate.sql` — 19 co-changes, J=27
- `adapters/adkgo/workflow.go` — 14 co-changes, J=35
- `adapters.go` — 12 co-changes, J=26 (structural Go import)

### `modules/chat/chat.go`  (signals: trend, coupling, bugfix)

**complexity_trend** — classified **REFACTORED** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 23  ·  LOC: 207
- first sample: n=222  total=182.0  mean=0.82  max=2
- last sample:  n=207  total=155.0  mean=0.75  max=2
- Δtotal -15%  ·  ΔLOC -7%  ·  Δmean indent -0.07  ·  peak-dip 18%
- recent window (last 2 samples): Δtotal -7%  ·  Δmean -0.07

**bugfix_density** — 1 fix-flavored commits out of 23 touches (4%). Last fix: 2026-04-15.
- `c7d0bdd57d11` (2026-04-15): fix(chat): preserve OriginalToolCallID for stateless confirmation session rebuild

**file_coupling** — 2 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/chat/handlers/messagehandler/handler_test.go` — 6 co-changes, J=29 (structural Go import)
- `modules/chat/handlers/messagehandler/handler.go` — 6 co-changes, J=26 (structural Go import)

### `modules/chat/projection/agui/postgres_projection.go`  (signals: trend, coupling, bugfix)

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 2  ·  commits observed: 9  ·  LOC: 897
- first sample: n=811  total=1550.0  mean=1.91  max=9
- last sample:  n=897  total=1690.0  mean=1.88  max=9
- Δtotal +9%  ·  ΔLOC +11%  ·  Δmean indent -0.03  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +9%  ·  Δmean -0.03

**bugfix_density** — 0 fix-flavored commits out of 9 touches (0%). Last fix: n/a.

**file_coupling** — 2 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/chat/projection/llmcontext/postgres_projection.go` — 13 co-changes, J=35
- `modules/chat/threadmgmt/manager.go` — 13 co-changes, J=25

### `modules/chat/thread/events.go`  (signals: trend, coupling, bugfix)

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 2  ·  commits observed: 12  ·  LOC: 404
- first sample: n=302  total=146.0  mean=0.48  max=1
- last sample:  n=404  total=196.0  mean=0.48  max=1
- Δtotal +34%  ·  ΔLOC +34%  ·  Δmean indent +0.00  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +34%  ·  Δmean +0.00

**bugfix_density** — 1 fix-flavored commits out of 12 touches (8%). Last fix: 2026-04-15.
- `8946ffad9eb6` (2026-04-15): feat(chat): implement tool confirmation UI and fix confirmation flow

**file_coupling** — 2 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/chat/handlers/common/converter.go` — 22 co-changes, J=44 (structural Go import)
- `infra/projection/subscriber/event_deserializer.go` — 12 co-changes, J=25

## Detail — files in 2 analyses

### `MISALIGNMENTS.md`  (signals: coupling, bugfix)

**bugfix_density** — 0 fix-flavored commits out of 9 touches (0%). Last fix: n/a.

**file_coupling** — 1 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/agentdefinition/handler.go` — 6 co-changes, J=27 (structural Go import)

### `adapters.go`  (signals: coupling, bugfix)

**bugfix_density** — 0 fix-flavored commits out of 33 touches (0%). Last fix: n/a.

**file_coupling** — 4 partner(s) co-changing at cooccurrence ≥ 6:
- `adapters/adkgo/runner.go` — 15 co-changes, J=26 (structural Go import)
- `adapters/adkgo/workflow.go` — 13 co-changes, J=31 (structural Go import)
- `modules/agentdefinition/types.go` — 12 co-changes, J=26 (structural Go import)
- `adapters/adkgo/ports.go` — 9 co-changes, J=26 (structural Go import)

### `adapters/adkgo/agent_builder.go`  (signals: hotspot, bugfix)

**go_hotspots** — file-aggregate score 210 across 1 hot function(s):
- `buildLLMAgent` L117-150 — score 210, 7 revs, 30 LOC

Hotspot signals: knowledge_concentration.

**bugfix_density** — 1 fix-flavored commits out of 9 touches (11%). Last fix: 2026-04-17.
- `30774a21d4dc` (2026-04-17): fix(adk): make OutputSchema actually respected on every provider

### `adapters/adkgo/session.go`  (signals: trend, bugfix)

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 8  ·  LOC: 279
- first sample: n=147  total=225.0  mean=1.53  max=5
- last sample:  n=279  total=395.0  mean=1.42  max=5
- Δtotal +76%  ·  ΔLOC +90%  ·  Δmean indent -0.12  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +5%  ·  Δmean +0.02

**bugfix_density** — 2 fix-flavored commits out of 8 touches (25%). Last fix: 2026-04-15.
- `c7d0bdd57d11` (2026-04-15): fix(chat): preserve OriginalToolCallID for stateless confirmation session rebuild
- `8946ffad9eb6` (2026-04-15): feat(chat): implement tool confirmation UI and fix confirmation flow

### `adapters/adkgo/tools.go`  (signals: trend, bugfix)

**complexity_trend** — classified **DETERIORATING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 8  ·  LOC: 281
- first sample: n=189  total=193.0  mean=1.02  max=4
- last sample:  n=281  total=336.0  mean=1.20  max=4
- Δtotal +74%  ·  ΔLOC +49%  ·  Δmean indent +0.17  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +24%  ·  Δmean +0.12

**bugfix_density** — 0 fix-flavored commits out of 8 touches (0%). Last fix: n/a.

### `assets/js/chat/chat-event-handlers.js`  (signals: trend, bugfix)

**complexity_trend** — classified **DETERIORATING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 19  ·  LOC: 589
- first sample: n=418  total=1155.0  mean=2.76  max=5
- last sample:  n=589  total=1798.75  mean=3.05  max=6
- Δtotal +56%  ·  ΔLOC +41%  ·  Δmean indent +0.29  ·  peak-dip 10%
- recent window (last 2 samples): Δtotal -2%  ·  Δmean +0.01

**bugfix_density** — 2 fix-flavored commits out of 19 touches (10%). Last fix: 2026-04-15.
- `8946ffad9eb6` (2026-04-15): feat(chat): implement tool confirmation UI and fix confirmation flow
- `8b86abd49f04` (2026-04-02): fix: SSE tokens arriving in bursts instead of streaming smoothly

### `assets/js/chat/chat-messaging.js`  (signals: trend, bugfix)

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 35  ·  LOC: 540
- first sample: n=271  total=1164.75  mean=4.30  max=9
- last sample:  n=540  total=1887.75  mean=3.50  max=7
- Δtotal +62%  ·  ΔLOC +99%  ·  Δmean indent -0.80  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +46%  ·  Δmean +0.09

**bugfix_density** — 7 fix-flavored commits out of 35 touches (20%). Last fix: 2026-04-15.
- `c7d0bdd57d11` (2026-04-15): fix(chat): preserve OriginalToolCallID for stateless confirmation session rebuild
- `ff2d046037fd` (2026-04-01): fix stop streaming
- `afb14a70c461` (2026-01-25): fixes for message ordering and tool data binding in the frontend. Includes fixes for undefined ca...
- `dd4bb4b0d69e` (2026-01-24): fix rendering after refresh
- `3fb6dde17fc0` (2026-01-23): fix:  token usage component will now persist   across browser refreshes

### `assets/js/chat/chat-sessions.js`  (signals: trend, bugfix)

**complexity_trend** — classified **DETERIORATING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 18  ·  LOC: 451
- first sample: n=188  total=666.5  mean=3.54  max=7
- last sample:  n=451  total=1769.5  mean=3.92  max=9
- Δtotal +166%  ·  ΔLOC +140%  ·  Δmean indent +0.38  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +62%  ·  Δmean +0.51

**bugfix_density** — 1 fix-flavored commits out of 18 touches (6%). Last fix: 2026-01-19.
- `14101ea69dd3` (2026-01-19): Fixing Dark Mode Font Color

### `assets/js/chat/chat-state.js`  (signals: trend, bugfix)

**complexity_trend** — classified **DETERIORATING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 48  ·  LOC: 340
- first sample: n=152  total=308.25  mean=2.03  max=6
- last sample:  n=340  total=837.5  mean=2.46  max=7
- Δtotal +172%  ·  ΔLOC +124%  ·  Δmean indent +0.43  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +7%  ·  Δmean +0.02

**bugfix_density** — 6 fix-flavored commits out of 48 touches (12%). Last fix: 2026-04-15.
- `8946ffad9eb6` (2026-04-15): feat(chat): implement tool confirmation UI and fix confirmation flow
- `dd4bb4b0d69e` (2026-01-24): fix rendering after refresh
- `741a87b045ec` (2026-01-21): fix": theme switching
- `5d0a917ff24c` (2026-01-20): Fix: Added a hidden file input and connected the paperclip button to it.
- `6031b1528f14` (2026-01-19): fix theme switching

### `assets/js/chat/chat-streaming.js`  (signals: trend, bugfix)

**complexity_trend** — classified **REFACTORED** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 43  ·  LOC: 300
- first sample: n=546  total=1928.75  mean=3.53  max=7
- last sample:  n=300  total=1187.75  mean=3.96  max=10
- Δtotal -38%  ·  ΔLOC -45%  ·  Δmean indent +0.43  ·  peak-dip 43%
- recent window (last 2 samples): Δtotal -9%  ·  Δmean +0.19

**bugfix_density** — 3 fix-flavored commits out of 43 touches (7%). Last fix: 2026-04-01.
- `ff2d046037fd` (2026-04-01): fix stop streaming
- `dd4bb4b0d69e` (2026-01-24): fix rendering after refresh
- `8bd54882eece` (2026-01-19): Refactor elicitation state to Single Source of Truth and fix UI bugs

### `assets/js/chat/chat-utils.js`  (signals: trend, bugfix)

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 14  ·  LOC: 316
- first sample: n=248  total=322.0  mean=1.30  max=5
- last sample:  n=316  total=411.75  mean=1.30  max=5
- Δtotal +28%  ·  ΔLOC +27%  ·  Δmean indent +0.01  ·  peak-dip 5%
- recent window (last 2 samples): Δtotal -5%  ·  Δmean +0.01

**bugfix_density** — 2 fix-flavored commits out of 14 touches (14%). Last fix: 2026-04-15.
- `8946ffad9eb6` (2026-04-15): feat(chat): implement tool confirmation UI and fix confirmation flow
- `dd4bb4b0d69e` (2026-01-24): fix rendering after refresh

### `assets/js/chat/index.js`  (signals: trend, bugfix)

**complexity_trend** — classified **DETERIORATING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 47  ·  LOC: 639
- first sample: n=220  total=680.75  mean=3.09  max=8
- last sample:  n=639  total=2642.75  mean=4.14  max=9
- Δtotal +288%  ·  ΔLOC +190%  ·  Δmean indent +1.04  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +3%  ·  Δmean -0.05

**bugfix_density** — 4 fix-flavored commits out of 47 touches (8%). Last fix: 2026-04-11.
- `f29cffd22a6b` (2026-04-11): fix: refresh AVAILABLE_AGENTS after adding from Agent Store
- `e06894edcac9` (2026-01-23): Fix elicitation input Alpine.js scope error
- `741a87b045ec` (2026-01-21): fix": theme switching
- `8bd54882eece` (2026-01-19): Refactor elicitation state to Single Source of Truth and fix UI bugs

### `assets/js/chat/markdown-renderer.js`  (signals: trend, bugfix)

**complexity_trend** — classified **REFACTORED** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 12  ·  LOC: 400
- first sample: n=116  total=223.0  mean=1.92  max=6
- last sample:  n=400  total=344.0  mean=0.86  max=7
- Δtotal +54%  ·  ΔLOC +245%  ·  Δmean indent -1.06  ·  peak-dip 21%
- recent window (last 2 samples): Δtotal +2%  ·  Δmean -0.03

**bugfix_density** — 1 fix-flavored commits out of 12 touches (8%). Last fix: 2026-01-23.
- `4b55239fbf2a` (2026-01-23): Fixing LaTeX Inline Spacing

### `assets/js/components/agent-designer-dialog.js`  (signals: trend, bugfix)

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 5  ·  commits observed: 47  ·  LOC: 2244
- first sample: n=1049  total=2448.25  mean=2.33  max=7
- last sample:  n=2244  total=5282.5  mean=2.35  max=7
- Δtotal +116%  ·  ΔLOC +114%  ·  Δmean indent +0.02  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +27%  ·  Δmean -0.11

**bugfix_density** — 3 fix-flavored commits out of 47 touches (6%). Last fix: 2026-04-12.
- `381e0610bff0` (2026-04-12): fix: tools disabled by default for new agents
- `cee1cf9814e0` (2026-04-12): fix(ui): reorganize agent form for true cohesion
- `2fa355d7e63f` (2026-04-12): fix: persist subagents correctly on agent save

### `assets/js/components/agents-dialog.js`  (signals: trend, bugfix)

**complexity_trend** — classified **REFACTORED** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 18  ·  LOC: 639
- first sample: n=115  total=266.75  mean=2.32  max=5
- last sample:  n=639  total=1495.25  mean=2.34  max=5
- Δtotal +460%  ·  ΔLOC +456%  ·  Δmean indent +0.02  ·  peak-dip 19%
- recent window (last 2 samples): Δtotal +1%  ·  Δmean -0.02

**bugfix_density** — 1 fix-flavored commits out of 18 touches (6%). Last fix: 2026-04-11.
- `f29cffd22a6b` (2026-04-11): fix: refresh AVAILABLE_AGENTS after adding from Agent Store

### `assets/js/components/alpineflow.js`  (signals: trend, bugfix)

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 10  ·  LOC: 1041
- first sample: n=716  total=1504.25  mean=2.10  max=7
- last sample:  n=1041  total=2246.5  mean=2.16  max=7
- Δtotal +49%  ·  ΔLOC +45%  ·  Δmean indent +0.06  ·  peak-dip 10%
- recent window (last 2 samples): Δtotal +2%  ·  Δmean -0.01

**bugfix_density** — 0 fix-flavored commits out of 10 touches (0%). Last fix: n/a.

### `assets/js/components/editable-workflow-tree.js`  (signals: trend, bugfix)

**complexity_trend** — classified **DETERIORATING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 16  ·  LOC: 1579
- first sample: n=969  total=3574.5  mean=3.69  max=13
- last sample:  n=1579  total=6223.5  mean=3.94  max=11
- Δtotal +74%  ·  ΔLOC +63%  ·  Δmean indent +0.25  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +10%  ·  Δmean +0.07

**bugfix_density** — 1 fix-flavored commits out of 16 touches (6%). Last fix: 2026-04-11.
- `86720c560825` (2026-04-11): fix: parse max_iterations as int in workflow tree input handler

### `assets/js/components/workflow-tree-visualizer.js`  (signals: trend, bugfix)

**complexity_trend** — classified **REFACTORED** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 8  ·  LOC: 464
- first sample: n=472  total=1593.0  mean=3.38  max=7
- last sample:  n=464  total=1497.0  mean=3.23  max=8
- Δtotal -6%  ·  ΔLOC -2%  ·  Δmean indent -0.15  ·  peak-dip 36%
- recent window (last 2 samples): Δtotal -15%  ·  Δmean -0.06

**bugfix_density** — 0 fix-flavored commits out of 8 touches (0%). Last fix: n/a.

### `foundation/agent/agent.go`  (signals: trend, bugfix)

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 25  ·  LOC: 181
- first sample: n=63  total=48.0  mean=0.76  max=2
- last sample:  n=181  total=130.0  mean=0.72  max=3
- Δtotal +171%  ·  ΔLOC +187%  ·  Δmean indent -0.04  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +10%  ·  Δmean -0.01

**bugfix_density** — 0 fix-flavored commits out of 25 touches (0%). Last fix: n/a.

### `foundation/agent/message.go`  (signals: trend, bugfix)

**complexity_trend** — classified **DETERIORATING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 8  ·  LOC: 162
- first sample: n=64  total=41.0  mean=0.64  max=1
- last sample:  n=162  total=130.0  mean=0.80  max=3
- Δtotal +217%  ·  ΔLOC +153%  ·  Δmean indent +0.16  ·  peak-dip 6%
- recent window (last 2 samples): Δtotal -3%  ·  Δmean -0.01

**bugfix_density** — 0 fix-flavored commits out of 8 touches (0%). Last fix: n/a.

### `foundation/agent/tool.go`  (signals: trend, bugfix)

**complexity_trend** — classified **REFACTORED** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 10  ·  LOC: 167
- first sample: n=36  total=24.0  mean=0.67  max=1
- last sample:  n=167  total=71.0  mean=0.42  max=2
- Δtotal +196%  ·  ΔLOC +364%  ·  Δmean indent -0.24  ·  peak-dip 35%
- recent window (last 2 samples): Δtotal +9%  ·  Δmean +0.02

**bugfix_density** — 0 fix-flavored commits out of 10 touches (0%). Last fix: n/a.

### `foundation/toolregistry/plugin.go`  (signals: coupling, bugfix)

**bugfix_density** — 0 fix-flavored commits out of 4 touches (0%). Last fix: n/a.

**file_coupling** — 3 partner(s) co-changing at cooccurrence ≥ 6:
- `plugins/tts/provider.go` — 7 co-changes, J=41 (structural Go import)
- `plugins/document/provider.go` — 6 co-changes, J=40 (structural Go import)
- `plugins/knowledgebase/provider.go` — 6 co-changes, J=40 (structural Go import)

### `frontend/ui/agenteditor/agenteditor.templ`  (signals: trend, bugfix)

**complexity_trend** — classified **INSUFFICIENT** by threshold rules (see column legend above).
- samples: 1  ·  commits observed: 11  ·  LOC: 3253
- first sample: n=3253  total=27562.0  mean=8.47  max=42
- last sample:  n=3253  total=27562.0  mean=8.47  max=42

**bugfix_density** — 1 fix-flavored commits out of 11 touches (9%). Last fix: 2026-04-17.
- `ff62cd5a37d0` (2026-04-17): refactor(dialog): premium surface + cmdk-style search + fix threads dialog

### `frontend/ui/docs/agents.templ`  (signals: coupling, bugfix)

**bugfix_density** — 1 fix-flavored commits out of 6 touches (17%). Last fix: 2026-04-17.
- `27c1a0dc7a98` (2026-04-17): docs: fix output_schema description to match ADK code behavior

**file_coupling** — 2 partner(s) co-changing at cooccurrence ≥ 6:
- `frontend/ui/landing/agents.templ` — 9 co-changes, J=43 (structural Go import)
- `frontend/ui/landing/features.templ` — 8 co-changes, J=31 (structural Go import)

### `frontend/ui/docs/tools.templ`  (signals: coupling, bugfix)

**bugfix_density** — 0 fix-flavored commits out of 3 touches (0%). Last fix: n/a.

**file_coupling** — 2 partner(s) co-changing at cooccurrence ≥ 6:
- `frontend/ui/landing/features.templ` — 8 co-changes, J=42 (structural Go import)
- `frontend/ui/landing/page.templ` — 6 co-changes, J=35 (structural Go import)

### `infra/projection/subscriber/event_deserializer.go`  (signals: coupling, bugfix)

**bugfix_density** — 1 fix-flavored commits out of 19 touches (5%). Last fix: 2026-01-19.
- `8bd54882eece` (2026-01-19): Refactor elicitation state to Single Source of Truth and fix UI bugs

**file_coupling** — 2 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/chat/thread/events.go` — 12 co-changes, J=25
- `modules/chat/handlers/common/converter.go` — 9 co-changes, J=26

### `infra/projection/subscriber/projection_updater_new.go`  (signals: trend, bugfix)

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 9  ·  LOC: 316
- first sample: n=166  total=319.0  mean=1.92  max=6
- last sample:  n=316  total=514.0  mean=1.63  max=7
- Δtotal +61%  ·  ΔLOC +90%  ·  Δmean indent -0.29  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +0%  ·  Δmean +0.00

**bugfix_density** — 0 fix-flavored commits out of 9 touches (0%). Last fix: n/a.

### `infra/sqldb/migrate/sql/migrate.sql`  (signals: coupling, bugfix)

**bugfix_density** — 2 fix-flavored commits out of 61 touches (3%). Last fix: 2026-02-12.
- `ad6c316c8f34` (2026-02-12): Revert "feat: Implement RAG services and PostgreSQL storage for lightrag-go, and enhance artifact...
- `537f634b4065` (2026-02-12): Revert "feat: Implement document trash, hierarchical structure, and new management operations lik...

**file_coupling** — 1 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/agentdefinition/types.go` — 19 co-changes, J=27

### `infra/sqldb/sqldb.go`  (signals: trend, bugfix)

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 10  ·  LOC: 305
- first sample: n=273  total=357.0  mean=1.31  max=4
- last sample:  n=305  total=401.0  mean=1.31  max=4
- Δtotal +12%  ·  ΔLOC +12%  ·  Δmean indent +0.01  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +0%  ·  Δmean +0.00

**bugfix_density** — 1 fix-flavored commits out of 10 touches (10%). Last fix: 2026-01-13.
- `e3747108d65a` (2026-01-13): fix: replace blank identifier workarounds with proper error handling

### `main.go`  (signals: trend, bugfix)

**complexity_trend** — classified **DETERIORATING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 287  ·  LOC: 927
- first sample: n=10  total=5.0  mean=0.50  max=1
- last sample:  n=927  total=1457.0  mean=1.57  max=6
- Δtotal +29040%  ·  ΔLOC +9170%  ·  Δmean indent +1.07  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +9%  ·  Δmean +0.00

**bugfix_density** — 9 fix-flavored commits out of 287 touches (3%). Last fix: 2026-04-15.
- `8946ffad9eb6` (2026-04-15): feat(chat): implement tool confirmation UI and fix confirmation flow
- `80f0182fd4f7` (2026-02-23): Revert "feat: Add settings tabs for account, AI configuration, data management, general settings,...
- `a2ad46fbf3bf` (2026-02-12): Revert "feat: add LightRAG"
- `ad6c316c8f34` (2026-02-12): Revert "feat: Implement RAG services and PostgreSQL storage for lightrag-go, and enhance artifact...
- `537f634b4065` (2026-02-12): Revert "feat: Implement document trash, hierarchical structure, and new management operations lik...

### `modules/a2a/service.go`  (signals: trend, bugfix)

**complexity_trend** — classified **DETERIORATING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 9  ·  LOC: 828
- first sample: n=610  total=794.0  mean=1.30  max=5
- last sample:  n=828  total=1190.0  mean=1.44  max=5
- Δtotal +50%  ·  ΔLOC +36%  ·  Δmean indent +0.14  ·  peak-dip 3%
- recent window (last 2 samples): Δtotal +0%  ·  Δmean +0.00

**bugfix_density** — 0 fix-flavored commits out of 9 touches (0%). Last fix: n/a.

### `modules/agentdefinition/module.go`  (signals: coupling, bugfix)

**bugfix_density** — 0 fix-flavored commits out of 3 touches (0%). Last fix: n/a.

**file_coupling** — 1 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/templates/handler.go` — 7 co-changes, J=54 (structural Go import)

### `modules/agentdefinition/provider.go`  (signals: hotspot, bugfix)

**go_hotspots** — file-aggregate score 270 across 1 hot function(s):
- `UserAwareProvider.agentFromDefinition` L63-119 — score 270, 5 revs, 54 LOC

Hotspot signals: knowledge_concentration.

**bugfix_density** — 0 fix-flavored commits out of 5 touches (0%). Last fix: n/a.

### `modules/analytics/analytics.go`  (signals: trend, bugfix)

**complexity_trend** — classified **DETERIORATING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 8  ·  LOC: 157
- first sample: n=134  total=148.0  mean=1.10  max=3
- last sample:  n=157  total=189.0  mean=1.20  max=3
- Δtotal +28%  ·  ΔLOC +17%  ·  Δmean indent +0.10  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +0%  ·  Δmean +0.00

**bugfix_density** — 0 fix-flavored commits out of 8 touches (0%). Last fix: n/a.

### `modules/chat/agentorchestrator/service.go`  (signals: trend, bugfix)

**complexity_trend** — classified **INSUFFICIENT** by threshold rules (see column legend above).
- samples: 1  ·  commits observed: 17  ·  LOC: 622
- first sample: n=622  total=1152.0  mean=1.85  max=5
- last sample:  n=622  total=1152.0  mean=1.85  max=5

**bugfix_density** — 1 fix-flavored commits out of 17 touches (6%). Last fix: 2026-04-15.
- `8946ffad9eb6` (2026-04-15): feat(chat): implement tool confirmation UI and fix confirmation flow

### `modules/chat/agui/handlers.go`  (signals: coupling, bugfix)

**bugfix_density** — 0 fix-flavored commits out of 8 touches (0%). Last fix: n/a.

**file_coupling** — 1 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/chat/projection/agui/agui_projection.go` — 17 co-changes, J=33 (structural Go import)

### `modules/chat/handlers/common/converter.go`  (signals: coupling, bugfix)

**bugfix_density** — 0 fix-flavored commits out of 3 touches (0%). Last fix: n/a.

**file_coupling** — 2 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/chat/thread/events.go` — 22 co-changes, J=44 (structural Go import)
- `infra/projection/subscriber/event_deserializer.go` — 9 co-changes, J=26

### `modules/chat/projection/agui/agui_projection.go`  (signals: coupling, bugfix)

**bugfix_density** — 0 fix-flavored commits out of 8 touches (0%). Last fix: n/a.

**file_coupling** — 1 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/chat/agui/handlers.go` — 17 co-changes, J=33 (structural Go import)

### `modules/chat/projection/llmcontext/postgres_projection.go`  (signals: coupling, bugfix)

**bugfix_density** — 0 fix-flavored commits out of 4 touches (0%). Last fix: n/a.

**file_coupling** — 1 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/chat/projection/agui/postgres_projection.go` — 13 co-changes, J=35

### `modules/chat/thread/thread_aggregate.go`  (signals: coupling, bugfix)

**bugfix_density** — 1 fix-flavored commits out of 7 touches (14%). Last fix: 2026-04-15.
- `c7d0bdd57d11` (2026-04-15): fix(chat): preserve OriginalToolCallID for stateless confirmation session rebuild

**file_coupling** — 1 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/chat/threadmgmt/manager.go` — 14 co-changes, J=29 (structural Go import)

### `modules/chat/threadmgmt/manager.go`  (signals: coupling, bugfix)

**bugfix_density** — 0 fix-flavored commits out of 4 touches (0%). Last fix: n/a.

**file_coupling** — 2 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/chat/thread/thread_aggregate.go` — 14 co-changes, J=29 (structural Go import)
- `modules/chat/projection/agui/postgres_projection.go` — 13 co-changes, J=25

### `modules/evaluation/handler.go`  (signals: trend, bugfix)

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 12  ·  LOC: 1043
- first sample: n=350  total=514.0  mean=1.47  max=4
- last sample:  n=1043  total=1508.0  mean=1.45  max=5
- Δtotal +193%  ·  ΔLOC +198%  ·  Δmean indent -0.02  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +0%  ·  Δmean +0.00

**bugfix_density** — 0 fix-flavored commits out of 12 touches (0%). Last fix: n/a.

### `modules/evaluation/service.go`  (signals: trend, bugfix)

**complexity_trend** — classified **DETERIORATING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 9  ·  LOC: 950
- first sample: n=283  total=377.0  mean=1.33  max=3
- last sample:  n=950  total=1462.0  mean=1.54  max=6
- Δtotal +288%  ·  ΔLOC +236%  ·  Δmean indent +0.21  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +4%  ·  Δmean -0.00

**bugfix_density** — 2 fix-flavored commits out of 9 touches (22%). Last fix: 2026-04-13.
- `66fbc2130f48` (2026-04-13): fix: run evaluation synchronously and fix missing trajectory import
- `022b3cd42582` (2026-04-13): feat: add metric configuration dialog + fix profile JSONB serialization

### `modules/evaluation/types.go`  (signals: trend, bugfix)

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 8  ·  LOC: 217
- first sample: n=67  total=43.0  mean=0.64  max=1
- last sample:  n=217  total=139.0  mean=0.64  max=1
- Δtotal +223%  ·  ΔLOC +224%  ·  Δmean indent -0.00  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +16%  ·  Δmean +0.00

**bugfix_density** — 1 fix-flavored commits out of 8 touches (12%). Last fix: 2026-04-13.
- `66fbc2130f48` (2026-04-13): fix: run evaluation synchronously and fix missing trajectory import

### `modules/knowledge/handler.go`  (signals: trend, bugfix)

**complexity_trend** — classified **STABLE** by threshold rules (see column legend above).
- samples: 3  ·  commits observed: 9  ·  LOC: 1031
- first sample: n=1026  total=1352.0  mean=1.32  max=4
- last sample:  n=1031  total=1364.0  mean=1.32  max=4
- Δtotal +1%  ·  ΔLOC +0%  ·  Δmean indent +0.01  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +0%  ·  Δmean +0.00

**bugfix_density** — 0 fix-flavored commits out of 9 touches (0%). Last fix: n/a.

### `modules/mcp/handler.go`  (signals: trend, bugfix)

**complexity_trend** — classified **DETERIORATING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 8  ·  LOC: 450
- first sample: n=454  total=0.0  mean=0.00  max=0
- last sample:  n=450  total=536.0  mean=1.19  max=4
- Δtotal +0%  ·  ΔLOC -1%  ·  Δmean indent +1.19  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +0%  ·  Δmean +0.00

**bugfix_density** — 0 fix-flavored commits out of 8 touches (0%). Last fix: n/a.

### `modules/mcp/service.go`  (signals: trend, bugfix)

**complexity_trend** — classified **GROWING** by threshold rules (see column legend above).
- samples: 4  ·  commits observed: 9  ·  LOC: 1097
- first sample: n=873  total=1174.0  mean=1.34  max=8
- last sample:  n=1097  total=1427.0  mean=1.30  max=8
- Δtotal +22%  ·  ΔLOC +26%  ·  Δmean indent -0.04  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +0%  ·  Δmean +0.00

**bugfix_density** — 0 fix-flavored commits out of 9 touches (0%). Last fix: n/a.

### `modules/templates/handler.go`  (signals: coupling, bugfix)

**bugfix_density** — 1 fix-flavored commits out of 4 touches (25%). Last fix: 2026-04-12.
- `0fed1d346cab` (2026-04-12): fix: auto-number template forks when name unchanged

**file_coupling** — 2 partner(s) co-changing at cooccurrence ≥ 6:
- `modules/agentdefinition/module.go` — 7 co-changes, J=54 (structural Go import)
- `modules/agentdefinition/service.go` — 6 co-changes, J=25 (structural Go import)

### `modules/toolreqs/service.go`  (signals: trend, bugfix)

**complexity_trend** — classified **DETERIORATING** by threshold rules (see column legend above).
- samples: 7  ·  commits observed: 10  ·  LOC: 454
- first sample: n=271  total=202.0  mean=0.74  max=3
- last sample:  n=454  total=396.0  mean=0.87  max=3
- Δtotal +96%  ·  ΔLOC +68%  ·  Δmean indent +0.13  ·  peak-dip 0%
- recent window (last 2 samples): Δtotal +0%  ·  Δmean +0.00

**bugfix_density** — 2 fix-flavored commits out of 10 touches (20%). Last fix: 2026-04-15.
- `8946ffad9eb6` (2026-04-15): feat(chat): implement tool confirmation UI and fix confirmation flow
- `381e0610bff0` (2026-04-12): fix: tools disabled by default for new agents

### `plugins/document/provider.go`  (signals: coupling, bugfix)

**bugfix_density** — 0 fix-flavored commits out of 11 touches (0%). Last fix: n/a.

**file_coupling** — 3 partner(s) co-changing at cooccurrence ≥ 6:
- `plugins/knowledgebase/provider.go` — 6 co-changes, J=100
- `plugins/tts/provider.go` — 6 co-changes, J=67
- `foundation/toolregistry/plugin.go` — 6 co-changes, J=40 (structural Go import)

### `plugins/knowledgebase/provider.go`  (signals: coupling, bugfix)

**bugfix_density** — 0 fix-flavored commits out of 11 touches (0%). Last fix: n/a.

**file_coupling** — 3 partner(s) co-changing at cooccurrence ≥ 6:
- `plugins/document/provider.go` — 6 co-changes, J=100
- `plugins/tts/provider.go` — 6 co-changes, J=67
- `foundation/toolregistry/plugin.go` — 6 co-changes, J=40 (structural Go import)

### `plugins/tts/provider.go`  (signals: coupling, bugfix)

**bugfix_density** — 0 fix-flavored commits out of 13 touches (0%). Last fix: n/a.

**file_coupling** — 4 partner(s) co-changing at cooccurrence ≥ 6:
- `foundation/toolregistry/plugin.go` — 7 co-changes, J=41 (structural Go import)
- `plugins/document/provider.go` — 6 co-changes, J=67
- `plugins/knowledgebase/provider.go` — 6 co-changes, J=67
- `foundation/toolregistry/factory.go` — 6 co-changes, J=32 (structural Go import)

## Notes on the data

- Whitespace-indent is a proxy for McCabe/Halstead complexity (Hindle–Godfrey–Holt, 2008); it correlates with but does not equal semantic complexity.
- `complexity_trend` only observes files currently on disk. Files deleted or rewritten do not appear — absence from this report is not evidence of health.
- `bugfix_density` classifies commits by keyword matches on the subject line. A commit whose subject lies (e.g. `feat:` for a fix) is not counted.
- `file_coupling` measures temporal co-change — it does not decide whether that coupling is intended or accidental.

## Reproduce

```bash
task refactor:plan
```
