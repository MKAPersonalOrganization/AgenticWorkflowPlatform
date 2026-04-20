# Evaluation Subsystem — Go-Native Port (Option A)

> **Scope:** port the Langfuse *evaluation subsystem* (scores, score configs, eval templates, evaluators, evaluator executions, datasets, dataset runs, annotation queues, default eval model, score analytics) into this Go app as a native feature. Nothing else — no Langfuse observability platform, no public Langfuse-compatible API, no external trace ingestion.
>
> **Architecture:** matches Langfuse's own — homogeneous single-language. Judge execution, ragas-style metrics, and analytics all live inline in Go, the same way Langfuse's TypeScript `worker` package runs everything inline. **No Python microservice.** A future Python sidecar is an escape hatch (§12), not part of v1.
>
> **Sources of truth (reference only, never imported):**
> - `/home/mabdelrahman/SchoolWithAI/.READ/langfuse/` — TypeScript monorepo. Authoritative on architecture, DB shape, lifecycle, queue semantics, and the exact prompts + output schemas Langfuse uses (including its "RAGAS" partner templates, which are just native TS templates with a `__ragas__` name prefix; see [.READ/langfuse/web/src/features/evals/types.ts](.READ/langfuse/web/src/features/evals/types.ts)).
> - `/home/mabdelrahman/SchoolWithAI/.READ/langfuse-docs/` — user-facing docs. Authoritative on behaviour (sampling, time scope, delay, output schema, block reasons, analytics metrics, TEXT-score restrictions, observation-over-trace preference).
> - `/home/mabdelrahman/SchoolWithAI/.READ/ragas/` — used only as the source for porting partner prompts into native Go eval templates. Never executed.
>
> **Previous legacy system removed:** the seven old tables (`eval_sets`, `eval_cases`, `evaluation_runs`, `evaluation_results`, `eval_metric_profiles`, `eval_audit_log`, `rubrics`) were dropped in migration `1.86`, and all of `/modules/evaluation/` was deleted. `go build ./... && go vet ./...` is clean.

---

## 0. Executive summary

Seven concerns, ported 1:1 to Go, using the repo's existing idioms (event sourcing, Postgres-advisory-lock projection updater, sqlx, chi, templ, typed IDs). LLM calls use the SDKs already in [go.mod](go.mod):

```
github.com/openai/openai-go/v3
google.golang.org/adk                         // Anthropic
google.golang.org/genai                       // Gemini
```

Analytics math (Pearson, Spearman, MAE, RMSE, Cohen's Kappa, macro F1, Overall Agreement) is ~40 lines per metric in Go; where the stdlib is inadequate we pull from `gonum.org/v1/gonum/stat`.

Everything runs in-process. No new deployables. No new external services.

| # | Concern | DB tables | Go module |
|---|---|---|---|
| 1 | Scores + ScoreConfigs | `scores`, `score_configs` | `modules/score`, `modules/scoreconfig` |
| 2 | Eval Templates | `eval_templates` | `modules/evaltemplate` (+ `defaults/` with ported partner prompts) |
| 3 | Evaluators (Langfuse: JobConfigurations) | `evaluators` | `modules/evaluator` |
| 4 | Evaluator Executions (Langfuse: JobExecutions) | `evaluator_executions` | `modules/evaluatorexec` |
| 5 | Datasets + Runs + Run Items | `datasets`, `dataset_items`, `dataset_runs`, `dataset_run_items` | `modules/dataset` |
| 6 | Annotation queues | `annotation_queues`, `annotation_queue_items`, `annotation_queue_assignments` | `modules/annotationqueue` |
| 7 | Default Eval Model | `default_eval_models` | `modules/defaultevalmodel` |

Plus one adapter module that lets the eval system read native chat runs/steps from the existing event store (no parallel trace tables) and one job-runner module that drives async execution.

---

## 1. Requirements catalogue

Each subsection is a testable acceptance criterion for the port. Cross-references use `.READ/langfuse/packages/shared/prisma/schema.prisma` for Langfuse's field names and `.READ/langfuse-docs/content/docs/evaluation/` for behavioural rules.

### 1.1 Data model

#### R-DM-1 `score_configs`
`(id, user_id, name, data_type, is_archived, min_value?, max_value?, categories?, description?, created_by?, updated_by?, created_at, updated_at)`

- `data_type ∈ {NUMERIC, CATEGORICAL, BOOLEAN, TEXT}`.
- `min_value` / `max_value` independent; defaults `±∞` per docs.
- `categories` JSONB array of `{label, value}`; `value` int (ordinal) or string.
- `is_archived` default false.
- Unique `(user_id, name)`.

#### R-DM-2 `scores`
`(id, user_id, timestamp, name, value?, string_value?, data_type, source, author_user_id?, comment?, target_kind, target_thread_id?, target_run_id?, target_step_id?, dataset_run_id?, config_id?, queue_id?, environment, metadata?, created_at, updated_at)`

- `source ∈ {ANNOTATION, API, EVAL}`.
- `target_kind ∈ {THREAD, RUN, STEP, DATASET_RUN_ITEM}` with exactly one valid combination:

  | target_kind | required fields |
  |---|---|
  | `THREAD` | `target_thread_id` |
  | `RUN` | `target_thread_id, target_run_id` |
  | `STEP` | `target_thread_id, target_run_id, target_step_id` (a domain-event id in the existing event store) |
  | `DATASET_RUN_ITEM` | `dataset_run_id` |

  Replaces Langfuse's `trace_id / observation_id / session_id / dataset_run_id`. Enforced by a check constraint.
- Value storage by `data_type`:

  | data_type | `value` | `string_value` |
  |---|---|---|
  | NUMERIC | required float | null |
  | BOOLEAN | required `0` or `1` | auto-set `"True"/"False"` when `config_id` present |
  | CATEGORICAL | ordinal number if config has ordinal values else 0 | required string, must be in config categories when `config_id` present |
  | TEXT | null | required, ≤500 chars |
- **Idempotency:** re-POST with same `id` replaces. Caller supplies `id`; server generates if missing.
- `environment` defaults `"default"`; evaluator self-traces use `"langfuse-llm-as-a-judge"` (R-EXEC-4).
- Custom `timestamp` accepted (back-dating allowed, ±1 s tolerance on round-trip).

#### R-DM-3 `eval_templates`
`(id, user_id?, name, version, prompt, partner?, provider?, model?, model_params?, vars[], output_schema, created_at, updated_at)`

- `user_id IS NULL` ⇒ partner / org-default (shipped in `/modules/evaltemplate/defaults/`).
- Version monotonic per `(user_id, name)`; edits create a new row.
- `prompt` uses `{{variable}}`.
- `output_schema` v2:
  ```json
  {
    "version": 2,
    "dataType": "NUMERIC" | "BOOLEAN" | "CATEGORICAL",
    "reasoning": { "description": "..." },
    "score": {
      "description": "...",
      "categories": ["yes","no","maybe"],
      "shouldAllowMultipleMatches": false
    }
  }
  ```
  v1 legacy readable via compat shim.
- `partner` values: `null`, `"ragas"`, future additions.
- Chosen model must support JSON / structured output.

#### R-DM-4 `evaluators` (Langfuse: `job_configurations`)
`(id, user_id, status, blocked_at?, block_reason?, block_message?, eval_template_id?, score_name, filter, target_kind, variable_mapping, sampling, delay_ms, time_scope[], created_at, updated_at)`

- `status ∈ {ACTIVE, INACTIVE}`.
- `target_kind ∈ {RUN, STEP, DATASET_ITEM}`. RUN = full chat-run eval; STEP = per-generation / per-tool-call eval (preferred for production per Langfuse docs); DATASET_ITEM for offline experiments.
- `sampling ∈ [0, 1]`.
- `delay_ms` — delay after the target becomes eligible before the judge fires. Default 10 000 ms.
- `time_scope ⊆ {NEW, EXISTING}` non-empty.
- `filter` = JSON array of filter conditions; fields depend on `target_kind` (see R-ADAPT-1/2 below for available fields).
- `variable_mapping` = array of `{templateVariable, source, field, jsonPath?}` where `source ∈ {run, step, dataset_item}` consistent with `target_kind`.
- `block_reason ∈ {LLM_CONNECTION_AUTH_INVALID, LLM_CONNECTION_MISSING, DEFAULT_EVAL_MODEL_MISSING, EVAL_MODEL_CONFIG_INVALID, EVAL_MODEL_UNAVAILABLE, PROVIDER_ACCOUNT_NOT_READY}`.

#### R-DM-5 `evaluator_executions` (Langfuse: `job_executions`)
`(id, user_id, evaluator_id, eval_template_id?, status, start_time?, end_time?, error?, input_target_kind, input_thread_id?, input_run_id?, input_step_id?, input_dataset_item_id?, input_dataset_item_valid_from?, output_score_id?, execution_thread_id?, execution_run_id?, attempts, created_at, updated_at)`

- `status ∈ {PENDING, DELAYED, COMPLETED, ERROR, CANCELLED}`. First four per docs; `CANCELLED` is ours (valid only from PENDING | DELAYED).
- `eval_template_id` snapshots template version at dispatch.
- `execution_thread_id` / `execution_run_id` — pointer to the judge's own chat run (R-EXEC-4).

#### R-DM-6 `datasets`
`(id, user_id, name, description?, metadata?, input_schema?, expected_output_schema?, created_at, updated_at)`

- `name` may contain `/` for virtual folders. Unique `(user_id, name)`.

#### R-DM-7 `dataset_items`
`(id, user_id, dataset_id, input?, expected_output?, metadata?, source_thread_id?, source_run_id?, source_step_id?, status, valid_from, valid_to?, is_deleted, created_at, updated_at)`

- `status ∈ {ACTIVE, ARCHIVED}`.
- Versioned: edits write a new row with `valid_from = now`; prior row gets `valid_to = now`. Soft delete via `is_deleted`. PK `(user_id, dataset_id, id, valid_from)`.
- API: `GET .../items?version=<ts>` → rows where `valid_from <= ts AND (valid_to IS NULL OR ts < valid_to) AND NOT is_deleted`.

#### R-DM-8 `dataset_runs`
`(id, user_id, dataset_id, name, description?, metadata?, created_at)`

- Unique `(user_id, dataset_id, name)`.

#### R-DM-9 `dataset_run_items`
`(id, user_id, dataset_run_id, dataset_item_id, dataset_item_valid_from, thread_id, run_id, step_id?, created_at)`

- Pins exact item version via `dataset_item_valid_from`.
- Links a dataset-item execution to the native chat run that produced the output.

#### R-DM-10 `annotation_queues`
`(id, user_id, name, description?, score_config_ids[], created_at, updated_at)`

- Unique `(user_id, name)`.

#### R-DM-11 `annotation_queue_items`
`(id, user_id, queue_id, target_kind, target_thread_id?, target_run_id?, target_step_id?, status, locked_at?, locked_by_user_id?, annotator_user_id?, completed_at?, created_at, updated_at)`

- `target_kind ∈ {THREAD, RUN, STEP}`.
- `status ∈ {PENDING, COMPLETED}`.
- Lock TTL 10 min (configurable); reaper goroutine clears expired locks.

#### R-DM-12 `annotation_queue_assignments`
`(id, user_id, queue_id, reviewer_user_id, created_at)`

#### R-DM-13 `default_eval_models`
`(id, user_id UNIQUE, provider, model, model_params?, ai_settings_ref?, created_at, updated_at)`

- `ai_settings_ref` points at an existing row in [modules/aisettings/](modules/aisettings/) for the API key. We do **not** re-implement key storage.
- Model must support structured output.

### 1.2 Adapter layer — how evaluators read native chat runs (R-ADAPT-*)

Instead of a Langfuse-shaped `traces` table, the evaluator reads from the existing chat event store through a thin adapter in `modules/chatrunindex/`:

#### R-ADAPT-1 `ListRuns(filter, after) → []RunSummary`

```go
type RunSummary struct {
    ThreadID     string
    RunID        string
    UserID       string
    AgentID      string
    StartedAt    time.Time
    EndedAt      *time.Time
    EndedWith    string            // FINISHED | ERROR | RUNNING
    InputMessage string
    OutputMessage string
    Metadata     map[string]any
    Tags         []string
    TokenCount   int
    DurationMS   int64
    Environment  string
}
```

Backed by a lightweight projection that writes one row to `chat_run_summaries` per `RunStarted … RunFinished|RunError` bracket. ≈11 fields per row, no payloads — full detail stays in the event store.

#### R-ADAPT-2 `ListSteps(runID, filter) → []StepSummary`

```go
type StepSummary struct {
    ThreadID  string
    RunID     string
    StepID    string   // event id in the store
    StepType  string   // generation | tool | code_execution | agent_transfer | workflow_node
    Name      string
    StartedAt time.Time
    EndedAt   *time.Time
    Input     any
    Output    any
    Metadata  map[string]any
    Model     string
    Tokens    int
    ErrorText string
}
```

Produced by replaying the thread aggregate and interpreting typed events on demand.

#### R-ADAPT-3 Event subscription

The dispatcher subscribes to `RunFinished` / `RunError` via the existing [infra/projection/subscriber/projection_updater_new.go](infra/projection/subscriber/projection_updater_new.go) (leader-elected, Postgres advisory lock) — the same hook AGUI + LLM-context projections already use. No new events.

### 1.3 Evaluation kinds (R-KIND-*)

All write to `scores`; distinguished by `source`:

- **R-KIND-1** LLM-as-judge on live chat — `evaluators.target_kind = RUN | STEP`. `source=EVAL`.
- **R-KIND-2** LLM-as-judge on datasets — `target_kind=DATASET_ITEM`. `source=EVAL`.
- **R-KIND-3** Human annotation via queue UI — `source=ANNOTATION`, `author_user_id`, `queue_id`.
- **R-KIND-4** Programmatic API POST — `source=API`.

### 1.4 Execution pipeline (R-EXEC-*) — all Go

Lives in `/modules/jobrunner/executor.go`. Direct LLM calls via the SDKs in [go.mod](go.mod), no IPC.

#### R-EXEC-1 Variable resolution
For each `variable_mapping` entry:
1. Fetch the source object per `source` (`run` → `RunSummary`, `step` → `StepSummary` + full event payload, `dataset_item` → the row at the pinned version).
2. Extract `field`; apply `jsonPath` if present (via `github.com/tidwall/gjson` already in `go.sum` or equivalent).
3. Unresolvable → execution `ERROR`, `error = "variable '<name>' not resolvable"`.

#### R-EXEC-2 Prompt compilation + judge call
1. Substitute `{{variable}}` tokens against resolved vars.
2. Build a JSON schema from `output_schema` (reasoning + score).
3. Call the judge LLM via the provider-appropriate SDK:
    - `provider=openai`   → `github.com/openai/openai-go/v3`, `chat.completions.create` with `response_format: { type: "json_schema" }`.
    - `provider=anthropic` → `google.golang.org/adk` (Anthropic via ADK adapter) with tool-use-for-structured-output.
    - `provider=google`    → `google.golang.org/genai`, `responseMimeType = "application/json"` + `responseJsonSchema`.
4. Parse response; validate against `output_schema`. Schema mismatch is an `ERROR` (not retried).

#### R-EXEC-3 Score write
- Upsert `scores`: `source=EVAL`, `config_id` auto-linked when a `score_configs.name` matches, `comment = reasoning`.
- CATEGORICAL with `shouldAllowMultipleMatches=true` → one score per matched category.
- Update `evaluator_executions`: `status=COMPLETED`, `output_score_id`, `end_time`.

#### R-EXEC-4 Self-trace of the judge call
Each judge invocation runs inside a **dedicated chat thread** owned by a synthetic `evaluator-<evaluatorId>` agent. The existing chat machinery records the run — `RunStarted`, the judge's `AssistantMessageStarted/Completed`, `TokenUsageReported`, `RunFinished` — so the UI can deep-link to the full judge transcript. `execution_thread_id` + `execution_run_id` on `evaluator_executions` hold the pointer. The self-trace row in `chat_run_summaries` carries `environment = "langfuse-llm-as-a-judge"` so the main chat UI can filter it out.

### 1.5 Auto-block state machine (R-BLOCK-*)

```
ACTIVE ──(auth/config error from provider)──► blocked_at=now, block_reason, block_message   (status stays ACTIVE, dispatcher skips)
ACTIVE ──(user Deactivate)───────────────► INACTIVE
BLOCKED ──(user Reactivate)──────────────► blocked_at=null, block_reason=null
BLOCKED ──(LLM connection fixed globally)─► auto-clear sibling blocks sharing the connection
```

Transient errors (`HTTP 429`, `5xx`, timeout, connection reset) never block — they retry with exponential backoff up to 3 attempts, then the execution is `ERROR` and the evaluator stays healthy.

### 1.6 Partner-template catalogue (R-PARTNER-*) — RAGAS-style prompts ported

Langfuse's approach: ragas-compatible metrics are **regular eval templates with name prefix `__ragas__` and `partner="ragas"`** ([.READ/langfuse/web/src/features/evals/types.ts:25](.READ/langfuse/web/src/features/evals/types.ts)). We do the same.

Seeded at boot from `/modules/evaltemplate/defaults/*.json` — one JSON file per template, each containing `{name, version, prompt, output_schema, vars, partner, model_params}`. Sources for each file:

| Template name | Origin | Notes |
|---|---|---|
| `__ragas__faithfulness`       | Port of `.READ/ragas/src/ragas/metrics/collections/faithfulness/prompts.py` | NLI-style 2-step prompt, output NUMERIC 0–1 |
| `__ragas__answer_relevancy`   | Port of ragas `answer_relevancy/prompts.py` + embeddings similarity done in Go | Needs embedding call; see R-EXEC-5 |
| `__ragas__context_recall`     | Port of ragas `context_recall/prompts.py` | |
| `__ragas__context_precision`  | Port of ragas `context_precision/metric.py` prompts | |
| `__ragas__context_relevance`  | Port of ragas `context_relevance/prompts.py` | |
| `__ragas__answer_correctness` | Port of ragas `answer_correctness/prompts.py` + embeddings | |
| `__ragas__response_groundedness` | Port of ragas `response_groundedness/prompts.py` | |
| `__ragas__factual_correctness` | Port of ragas `factual_correctness/prompts.py` | |
| `hallucination`               | Port of Langfuse's built-in, [.READ/langfuse/packages/shared/src/features/evals/](.READ/langfuse/packages/shared/src/features/evals/) / web templates where present | |
| `toxicity`                    | Same | |
| `helpfulness`                 | Same | |
| `correctness`                 | Same | |

Deterministic metrics (no LLM) are a separate small catalogue also seeded as partner templates but flagged `llm_based = false` — the executor skips the LLM step for these:

| Template | Implementation | Vars |
|---|---|---|
| `deterministic/exact_match`       | `output == expected_output` | output, expected_output |
| `deterministic/string_presence`   | substring check | output, expected_output |
| `deterministic/string_similarity` | Levenshtein / Jaro-Winkler via `github.com/hbollon/go-edlib` | output, expected_output |
| `deterministic/bleu`              | BLEU-4 — port of standard algorithm | reference, candidate |
| `deterministic/rouge_l`           | ROUGE-L — port of standard algorithm | reference, candidate |
| `deterministic/chrf`              | character F-score | reference, candidate |

Execution path for deterministic templates is `/modules/jobrunner/executor.go#runDeterministic` — pure Go, no LLM, still follows the same variable-mapping + score-write flow.

### R-EXEC-5 Embeddings path

Templates that include `embeddings: true` in their manifest (notably `answer_relevancy` and `answer_correctness`) call the embeddings API after the LLM judge step, to compute cosine similarity between generated questions and the original question (ragas's approach). Embeddings go through the same model-config system (default model's embedding counterpart, configured per provider). Default embedding models:

- `openai`    → `text-embedding-3-small`
- `google`    → `text-embedding-004`
- `anthropic` → not supported (Anthropic has no embeddings endpoint) — template flagged unavailable if provider=anthropic.

Cosine similarity computed in Go (`math` stdlib, ≤15 lines). No external deps.

### 1.7 Score analytics (R-ANALYTICS) — native Go

By `data_type`:

- **NUMERIC** — histogram (configurable bins), time-series trend, 10×10 heatmap for two-score comparison, **Pearson** + **Spearman** correlations, **MAE**, **RMSE**.
- **CATEGORICAL** — bar chart, N×M confusion matrix, **Cohen's Kappa**, **macro F1**, **Overall Agreement %**.
- **BOOLEAN** — same as CATEGORICAL, fixed 2×2.
- **TEXT** — excluded.

Implementations in `/modules/scoreanalytics/metrics.go`:

```go
func Pearson(xs, ys []float64) (r, p float64)          // ≈25 LOC
func Spearman(xs, ys []float64) (rho, p float64)       // ≈35 LOC (rank then Pearson)
func MAE(xs, ys []float64) float64                     // 5 LOC
func RMSE(xs, ys []float64) float64                    // 7 LOC
func CohenKappa(a, b []string, labels []string) float64 // ≈40 LOC
func MacroF1(a, b []string, labels []string) float64   // ≈35 LOC
func OverallAgreement(a, b []string) float64           // 5 LOC
func Histogram(xs []float64, bins int) HistogramResult // ≈20 LOC
```

Reach for `gonum.org/v1/gonum/stat` only if p-values need a proper t/z distribution; stdlib + small helpers cover the rest.

### 1.8 Non-functional

- Idempotent score writes on `id`.
- Per-user (`user_id`) row isolation on every query.
- Timestamps UTC `timestamptz`; wire RFC3339.
- Rate-limit score ingestion per user at the handler.
- Output-schema v1 templates remain executable (compat shim on read).
- JSON wire: camelCase top-level; nested freeform JSON (metadata, filter, model_params) unchanged (pitfall #10).
- TEXT scores rejected by LLM-as-judge + experiments + analytics; allowed only for annotation + direct API.

---

## 2. Current state of this repo

### 2.1 The existing event store is the trace store

[modules/chat/thread/events.go](modules/chat/thread/events.go) already emits every event an evaluator needs:
- `RunStarted` / `RunFinished` / `RunError` bracket a **Run** (Langfuse "trace").
- `AssistantMessageStarted/Streamed/Completed` + `TokenUsageReported` → generation **step**.
- `ToolStart` → `ToolResultAdded` / `ToolCancelled` → tool step.
- `CodeExecutionRan` → code-execution step.
- `AgentTransferStarted` → agent step.
- `WorkflowNodeCompleted` → workflow-node step.

[modules/chat/thread/thread_aggregate.go](modules/chat/thread/thread_aggregate.go) applies events into Thread state; `R-ADAPT-2 ListSteps` uses this.

[infra/projection/subscriber/projection_updater_new.go](infra/projection/subscriber/projection_updater_new.go) is our leader-elected projection loop; `R-ADAPT-3` plugs in here alongside the existing AGUI and LLM-context projections.

### 2.2 Langfuse ↔ ours mapping

| Langfuse | Ours |
|---|---|
| Session | Thread |
| Trace | Run within a thread |
| Observation (generation) | AssistantMessage* + TokenUsageReported |
| Observation (tool) | ToolStart → ToolResultAdded / ToolCancelled |
| Observation (code execution) | CodeExecutionRan |
| Observation (agent transfer) | AgentTransferStarted |
| Observation (workflow node) | WorkflowNodeCompleted |
| Score with `trace_id` | Score with `target_kind=RUN, target_thread_id, target_run_id` |
| Score with `observation_id` | Score with `target_kind=STEP, … + target_step_id` |
| Score with `session_id` | Score with `target_kind=THREAD, target_thread_id` |

### 2.3 LLM SDKs already available
- `github.com/openai/openai-go/v3` — OpenAI, structured output via JSON schema.
- `google.golang.org/genai` — Gemini, `responseJsonSchema`.
- `google.golang.org/adk` — Anthropic via ADK.

No new LLM SDK dependencies required.

### 2.4 What is net-new
1. Thirteen DB tables (§1.1).
2. Eleven Go modules (§3.3).
3. One thin projection: `chat_run_summaries` header-only index.
4. Internal REST under `/api/evaluation/*`.
5. Partner-template catalogue seeded from `/modules/evaltemplate/defaults/*.json`.
6. templ UI for every concern.
7. Postgres-backed job queue + executor goroutine.

---

## 3. Translation plan

### 3.1 Naming conventions
- Go packages: lowercase, no underscores — `score`, `scoreconfig`, `evaltemplate`, `evaluator`, `evaluatorexec`, `jobrunner`, `dataset`, `annotationqueue`, `defaultevalmodel`, `scoreanalytics`, `chatrunindex`.
- DB columns: `snake_case` (repo convention).
- Wire JSON: **camelCase top-level**; nested freeform JSON unchanged.
- Typed IDs: `ScoreID`, `ScoreConfigID`, `EvalTemplateID`, `EvaluatorID`, `EvaluatorExecutionID`, `DatasetID`, `DatasetItemID`, `DatasetRunID`, `DatasetRunItemID`, `AnnotationQueueID`, `AnnotationQueueItemID`. Reuse `ThreadID`, `RunID`, `StepID` from chat.
- User scoping via `user_id VARCHAR(255)` per existing repo convention.

### 3.2 Phased rollout

**Phase 1 — Scores primitive + chat-run adapter**
- R-DM-1 / R-DM-2 tables.
- `chat_run_summaries` projection + `R-ADAPT-1 ListRuns` + `R-ADAPT-2 ListSteps`.
- Internal REST: `POST /api/evaluation/scores`, `GET /api/evaluation/scores`, `GET|DELETE /api/evaluation/scores/{id}`, full CRUD on `/api/evaluation/score-configs`.
- UI: Score Configs page; Scores tab on run-detail (read-only + manual annotation form, `source=ANNOTATION`).
- TEXT-score block enforced at handler.

**Phase 2 — Eval templates + partner catalogue + sync manual run**
- R-DM-3 with versioning; internal REST + template editor UI.
- `/modules/evaltemplate/defaults/` seeded with the partner templates listed in R-PARTNER (hallucination, toxicity, helpfulness, correctness, the `__ragas__*` prompts, and the deterministic catalogue).
- `POST /api/evaluation/templates/{id}/run` — synchronous one-shot execution against a chosen `RunID` / `StepID` / `DatasetItemID`. Drives the executor inline so users can smoke-test a template before wiring it into an evaluator.
- Template editor UI: prompt, `{{vars}}`, output schema picker, model override, partner selector.

**Phase 3 — Continuous evaluators**
- R-DM-4 / R-DM-5 tables.
- New domain events: `EvaluatorExecutionStatusChanged`, `EvaluatorBlocked`, `EvaluatorUnblocked`, `ScoreWritten`. Register in [modules/chat/thread/events_registry.go](modules/chat/thread/events_registry.go) (pitfall #1).
- Postgres-backed queue + consumer (§3.5).
- Dispatcher subscribes to `RunFinished` / `RunError` + dataset-run-item inserts.
- Backfill driver for `time_scope ⊇ {EXISTING}`.
- R-BLOCK auto-block + reactivation flow.
- R-EXEC-4 self-trace in a dedicated evaluator-owned thread.
- UI: evaluator list with block badge, form (filter, mapping, sampling, delay, scope; target defaults to RUN), executions list + detail with self-trace deep-link.

**Phase 4 — Datasets + experiments**
- R-DM-6..9.
- Server-side item versioning; public query via `?version=<ts>`.
- Experiment: UI creates a `dataset_run`, iterates items, runs each through the agent (a native chat run), links via `dataset_run_items`. Evaluators with `target_kind=DATASET_ITEM` fire automatically.
- UI: datasets list + folders, item detail with version filter, add-from-run, experiment launcher, run-compare view (R-ANALYTICS metrics).

**Phase 5 — Annotation queues**
- R-DM-10..12.
- 10-min lock TTL, reaper goroutine.
- Reviewer UI: next-PENDING → lock → submit → COMPLETED → next.
- Admin: assignments, reopen.

**Phase 6 — Default eval model + analytics**
- R-DM-13 + auto-block integration.
- Analytics page with the full R-ANALYTICS suite.

### 3.3 Module layout

```
/modules/
  score/                      # Phase 1
    types.go                  # Score, ScoreSource, ScoreDataType, TargetKind
    repo.go                   # upsert-on-id, query with filters
    service.go                # validation, TEXT block on eval flows
    handler.go                # /api/evaluation/scores

  scoreconfig/                # Phase 1
    types.go | repo.go | service.go | handler.go   # /api/evaluation/score-configs

  chatrunindex/               # Phase 1 — the ONE new projection
    projection.go             # subscribes to Run*/TokenUsageReported
    repo.go                   # ListRuns queries
    stepadapter.go            # ListSteps via thread aggregate replay

  evaltemplate/               # Phase 2
    types.go                  # EvalTemplate, OutputSchema (v1+v2)
    repo.go                   # version bump
    service.go
    handler.go                # /api/evaluation/templates
    compile.go                # {{var}} expansion + JSON schema builder
    defaults/                 # seeded partner + deterministic templates
      __ragas__faithfulness.json
      __ragas__answer_relevancy.json
      …
      hallucination.json
      toxicity.json
      …
      deterministic/exact_match.json
      …

  evaluator/                  # Phase 3
    types.go                  # Evaluator, BlockReason, TimeScope, TargetKind
    repo.go
    service.go                # pause / resume / block / unblock
    handler.go                # /api/evaluation/evaluators
    filter.go                 # filter DSL against RunSummary / StepSummary / DatasetItem
    mapping.go                # variable resolution

  evaluatorexec/              # Phase 3
    types.go | repo.go | service.go | handler.go   # /api/evaluation/executions

  jobrunner/                  # Phase 3 — the core executor
    dispatcher.go             # event subscription + filter + sampling + enqueue
    backfill.go               # EXISTING-scope driver
    queue.go                  # Postgres FOR UPDATE SKIP LOCKED (§3.5)
    executor.go               # resolves vars, dispatches to judge or deterministic, writes score
    judge_llm.go              # provider-specific structured-output calls (openai / genai / adk)
    judge_deterministic.go    # exact_match, bleu, rouge, chrf, etc.
    embeddings.go             # answer_relevancy / answer_correctness embedding step
    errors.go                 # classify transient vs auth/config vs schema
    selftrace.go              # spin up an evaluator-owned thread for the judge call

  dataset/                    # Phase 4
    types.go                  # Dataset, DatasetItem (versioned), DatasetRun, DatasetRunItem
    repo.go | service.go | handler.go   # /api/evaluation/datasets[…]

  annotationqueue/            # Phase 5
    types.go | repo.go | service.go | handler.go   # /api/evaluation/annotation-queues[…]
    lock.go                   # 10-min TTL, reaper

  defaultevalmodel/           # Phase 6
    types.go | repo.go | service.go | handler.go   # /api/evaluation/default-model

  scoreanalytics/             # Phase 6
    metrics.go                # pearson, spearman, mae, rmse, kappa, f1, agreement
    handler.go                # /api/evaluation/analytics/scores
```

### 3.4 Domain events to register

In [modules/chat/thread/events.go](modules/chat/thread/events.go) + register each in [events_registry.go](modules/chat/thread/events_registry.go) (pitfall #1):

**Phase 1 — score lifecycle**
- `ScoreWritten{scoreId, userId, targetKind, targetThreadId?, targetRunId?, targetStepId?, datasetRunId?, source, name, value?, stringValue?, dataType, configId?, environment}`

**Phase 3 — evaluator lifecycle**
- `EvaluatorExecutionStatusChanged{executionId, evaluatorId, userId, oldStatus, newStatus, error?}`
- `EvaluatorBlocked{evaluatorId, userId, reason, message}` / `EvaluatorUnblocked{evaluatorId, userId}`

**NOT added** — no `TraceCreated / TraceFinalised / ObservationCreated / ObservationFinalised`. The existing events carry that data; a parallel eventset would double-write and invite drift.

### 3.5 Postgres job queue — `FOR UPDATE SKIP LOCKED`

```sql
CREATE TABLE job_execution_queue (
    evaluator_execution_id VARCHAR(255) PRIMARY KEY
        REFERENCES evaluator_executions(id) ON DELETE CASCADE,
    user_id                VARCHAR(255) NOT NULL,
    not_before             TIMESTAMPTZ  NOT NULL DEFAULT now(),
    attempts               INT          NOT NULL DEFAULT 0,
    locked_by              VARCHAR(255),
    locked_until           TIMESTAMPTZ,
    last_error             TEXT,
    created_at             TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_jobq_claimable ON job_execution_queue(not_before)
  WHERE locked_until IS NULL OR locked_until < now();
```

Claim loop:
```sql
WITH claim AS (
  SELECT evaluator_execution_id FROM job_execution_queue
  WHERE (locked_until IS NULL OR locked_until < now()) AND not_before <= now()
  ORDER BY not_before FOR UPDATE SKIP LOCKED LIMIT $1
)
UPDATE job_execution_queue q
SET locked_by = $2, locked_until = now() + interval '5 minutes', attempts = q.attempts + 1
FROM claim WHERE q.evaluator_execution_id = claim.evaluator_execution_id
RETURNING q.evaluator_execution_id;
```

- Transient error → `not_before = now() + backoff(attempts)`, `locked_until = NULL`.
- Success → `DELETE` + mark execution `COMPLETED`.
- Auth/config error → `DELETE` + block the evaluator.
- 3 failed attempts → `DELETE` + mark execution `ERROR` (evaluator stays healthy).

### 3.6 UI

| Concern | templ path |
|---|---|
| Score configs | `frontend/ui/evaluation/score_configs.templ` |
| Scores tab on run detail | extend `frontend/ui/messagelist/*` + dedicated panel |
| Templates list / editor | `frontend/ui/evaluation/templates_list.templ`, `templates_editor.templ` |
| Evaluators list / form | `frontend/ui/evaluation/evaluators_list.templ`, `evaluator_form.templ` |
| Executions list / detail | `frontend/ui/evaluation/executions.templ` |
| Datasets list / detail | `frontend/ui/evaluation/datasets.templ`, `dataset_detail.templ` |
| Runs + compare | `frontend/ui/evaluation/runs.templ`, `compare.templ` |
| Annotation queues list / detail / reviewer | `frontend/ui/evaluation/queues_list.templ`, `queue_detail.templ`, `annotate.templ` |
| Default model settings | `frontend/ui/evaluation/default_model.templ` |
| Score analytics | `frontend/ui/evaluation/analytics.templ` |

### 3.7 Integration tests (under `/userrequirements/`)

- `score_ingest_test.go` — NUMERIC/CATEGORICAL/BOOLEAN round-trip, idempotent upsert.
- `score_boolean_string_value_test.go` — `string_value` auto-set when `config_id` provided.
- `score_text_block_test.go` — TEXT rejected on eval/dataset paths.
- `score_config_test.go` — CRUD + archive + unique constraint.
- `eval_template_version_test.go` — edit → new version; v1 schema still executable.
- `eval_template_sync_run_test.go` — `POST /templates/{id}/run` on a historical run produces a score.
- `partner_template_seed_test.go` — all partner templates loadable; deterministic ones produce correct scores on fixtures; LLM ones produce `NUMERIC [0,1]` values against canned fixtures (mocked LLM).
- `evaluator_new_run_test.go` — `time_scope=[NEW]`, target=RUN; send chat message; score appears within 30 s.
- `evaluator_backfill_test.go` — `time_scope=[EXISTING]` backfills matching historical runs.
- `evaluator_block_test.go` — break credentials → `blocked_at` set; fix → reactivate resumes.
- `sampling_test.go` — sampling=0 → 0 scores, sampling=1 → N.
- `delay_test.go` — `delay_ms=5000` → `start_time ≥ run.ended_at + 5 s`.
- `dataset_versioning_test.go` — edit item → fetch at past version → old payload.
- `dataset_run_compare_test.go` — two runs; compare endpoint aggregates correctly.
- `annotation_queue_lock_test.go` — lock TTL expiry; concurrent reviewers isolated.
- `default_eval_model_test.go` — auth error broadcasts block to every ACTIVE evaluator using the default.
- `analytics_metrics_test.go` — Pearson/Spearman/MAE/RMSE/Kappa/F1 against hand-computed fixtures.

### 3.8 Observability
- Each judge call opens its own chat run (R-EXEC-4), inspectable in the existing chat detail UI.
- Prometheus counters: `evaluator_executions_total{status}`, `evaluator_execution_duration_seconds`, `scores_written_total{source}`, `evaluators_blocked_total{reason}`.

---

## 4. Acceptance checklist

- [ ] All internal endpoints under `/api/evaluation/*` with camelCase JSON.
- [ ] Every new event registered in `events_registry.go` (pitfall #1).
- [ ] Every row gated on `user_id`.
- [ ] Score writes idempotent on `id`.
- [ ] TEXT scores rejected from LLM-as-judge, experiments, analytics.
- [ ] Score `target_kind ∈ {THREAD, RUN, STEP, DATASET_RUN_ITEM}` enforced by check constraint.
- [ ] Evaluator `target_kind ∈ {RUN, STEP, DATASET_ITEM}` end-to-end.
- [ ] `chat_run_summaries` catches up from `global_position = 0` on first boot.
- [ ] Partner templates (`__ragas__*`, `hallucination`, `toxicity`, …) produce valid NUMERIC [0,1] scores on canned fixtures.
- [ ] Deterministic templates (`exact_match`, `bleu`, `rouge_l`, `chrf`, …) run without calling an LLM.
- [ ] Self-trace threads recorded with `environment = "langfuse-llm-as-a-judge"` and linked from the execution row.
- [ ] Auto-block only fires on auth/config errors, never on 429/5xx/timeout.
- [ ] Dataset items queryable at past `version=<ts>`; `dataset_run_items.dataset_item_valid_from` pins version.
- [ ] Annotation-queue lock TTL 10 min; expired locks reaped; concurrent reviewers isolated.
- [ ] Phase 6 analytics compute Pearson, Spearman, MAE, RMSE, Cohen's Kappa, macro F1, Overall Agreement; TEXT excluded.
- [ ] Prometheus counters per §3.8 exposed.
- [ ] Integration tests of §3.7 green.
- [ ] No Python, no subprocess, no shell-outs, no new services. `docker-compose` unchanged.

---

## 5. File index

**Reference (read-only):**
- TS: [.READ/langfuse/packages/shared/prisma/schema.prisma](.READ/langfuse/packages/shared/prisma/schema.prisma), [.READ/langfuse/web/src/features/evals/](.READ/langfuse/web/src/features/evals/), [.READ/langfuse/worker/src/features/evaluation/](.READ/langfuse/worker/src/features/evaluation/).
- Docs: [.READ/langfuse-docs/content/docs/evaluation/](.READ/langfuse-docs/content/docs/evaluation/).
- Partner prompt source: [.READ/ragas/src/ragas/metrics/collections/](.READ/ragas/src/ragas/metrics/collections/) — ported to JSON, never executed.

**Local targets:** [main.go](main.go), [modules/chat/thread/events.go](modules/chat/thread/events.go), [modules/chat/thread/events_registry.go](modules/chat/thread/events_registry.go), [modules/chat/thread/thread_aggregate.go](modules/chat/thread/thread_aggregate.go), [infra/eventstore/postgres/](infra/eventstore/postgres/), [infra/projection/subscriber/projection_updater_new.go](infra/projection/subscriber/projection_updater_new.go), [adapters/adkgo/](adapters/adkgo/), [modules/aisettings/](modules/aisettings/), [infra/sqldb/migrate/sql/migrate.sql](infra/sqldb/migrate/sql/migrate.sql), [arch/guidelines/workflows.md](arch/guidelines/workflows.md), [arch/guidelines/pitfalls.md](arch/guidelines/pitfalls.md), [userrequirements/](userrequirements/), [go.mod](go.mod).

---

## 12. Escape hatch — if we ever need Python

Not part of v1. Documented here so the door isn't nailed shut.

Should the port later hit a metric we genuinely cannot implement natively — e.g. something requiring `sentence-transformers` or `datacompy` DataFrame diffing — the fallback is a narrow FastAPI sidecar exposing only the missing metrics:

```
POST /v1/metrics/{metricSlug}    # sample → score
GET  /v1/metrics                 # discovery
GET  /healthz
```

Called from `/modules/jobrunner/judge_python.go` (new file at that point). All other execution paths stay in Go. State remains in Postgres; the sidecar is stateless, HMAC-authed.

Decision trigger: a specific metric the product needs and we've tried to port for ≥2 days without cleanly replicating. Not "it would be nice." Until then, Go does all the work.
