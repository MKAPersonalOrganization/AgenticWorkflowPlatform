# Evaluation Framework — Developer Requirements Document

> **Source Material:** Google ADK Python evaluation framework (`src/google/adk/evaluation/`), ADK Go eval stubs, evaluation notebooks, and sample agent eval configurations.
> **Date:** 2026-04-13

---

## Table of Contents

1. [Overview](#overview)
2. [Terminology](#terminology)
3. [Part A — Core Requirements](#part-a--core-requirements) *(Must-have — MVP)*
4. [Part B — Helpful Requirements](#part-b--helpful-requirements) *(Should-have — Phase 2)*
5. [Part C — Advanced Requirements](#part-c--advanced-requirements) *(Nice-to-have — Phase 3+)*
6. [Dependency Graph & Development Order](#dependency-graph--development-order)
7. [Acceptance Criteria Summary](#acceptance-criteria-summary)

---

## Overview

This document extracts the complete evaluation framework capabilities from Google's ADK and maps them to user stories for our web application. The ADK Python evaluation system is production-grade with 12 prebuilt metrics, rubric-based evaluation, LLM-as-a-judge, user simulation, trajectory analysis, hallucination detection, and safety scoring. The Go SDK has skeleton eval API endpoints (unimplemented), confirming that the Python implementation is the reference.

---

## Terminology

| Term | Definition |
|---|---|
| **EvalCase** | A single test case: input prompt(s), expected behavior (tool trajectory + golden response), and optional rubrics. |
| **EvalSet** | A named collection of EvalCases (like a test suite). |
| **Invocation** | One run of the agent on a single test case. Multiple invocations per case enable statistical reliability. |
| **Trajectory** | The sequence of tool calls an agent makes during an invocation. |
| **Criterion** | A scoring rule applied to an invocation — e.g., ROUGE match, LLM-as-judge, hallucination check. |
| **Rubric** | A user-defined scoring guide with score levels (1–5) and descriptions for each level. |
| **User Simulator** | An AI or static system that plays the role of the user during multi-turn evaluation. |
| **EvalResult** | Per-invocation score + overall aggregated score for an EvalCase. |
| **MatchType** | How trajectories are compared: EXACT, IN_ORDER, ANY_ORDER. |

---

# Part A — Core Requirements

---

## EVAL-CORE-01: EvalSet Management — Create, List, Edit, Delete

**Story:** As a developer, I can create, list, view, edit, and delete eval sets so that I can organize my test cases into logical suites.

**Details:**
- An EvalSet has: name, description, and a list of EvalCases.
- EvalSets can be stored locally, in memory, or on GCS.
- EvalSets can be imported/exported as JSON files.

**Acceptance Criteria:**
- [ ] AC-01.1: User can create a new EvalSet with a name and description.
- [ ] AC-01.2: User can view a list of all EvalSets with name, description, and case count.
- [ ] AC-01.3: User can edit an EvalSet's name and description.
- [ ] AC-01.4: User can delete an EvalSet (with confirmation).
- [ ] AC-01.5: User can import an EvalSet from a JSON file.
- [ ] AC-01.6: User can export an EvalSet as a JSON file.
- [ ] AC-01.7: EvalSet name uniqueness is validated.

**Dependencies:** None. Can develop in parallel with EVAL-CORE-02.

---

## EVAL-CORE-02: EvalCase Management — Create, List, Edit, Delete

**Story:** As a developer, I can create and manage individual test cases within an EvalSet so that I can define what to test.

**Details:**
- An EvalCase contains:
  - `input`: The user's initial prompt (or multi-turn conversation plan).
  - `expected_trajectory`: Optional list of expected tool calls (tool name, parameters).
  - `expected_response`: Optional golden reference response for comparison.
  - `rubrics`: Optional list of Rubric objects defining custom scoring criteria.
- EvalCases support both single-turn and multi-turn scenarios.
- Each EvalCase belongs to exactly one EvalSet.

**Acceptance Criteria:**
- [ ] AC-02.1: User can create an EvalCase with input prompt within an EvalSet.
- [ ] AC-02.2: User can define expected tool calls (tool name + parameters) for trajectory matching.
- [ ] AC-02.3: User can provide a golden reference response for response matching.
- [ ] AC-02.4: User can edit and delete EvalCases.
- [ ] AC-02.5: EvalCase list shows input preview, expected tools, and whether rubrics are defined.
- [ ] AC-02.6: User can reorder EvalCases within an EvalSet.
- [ ] AC-02.7: EvalCases can be duplicated for quick variation creation.

**Dependencies:** Depends on EVAL-CORE-01 (EvalSets must exist).

---

## EVAL-CORE-03: Evaluation Configuration — Select Metrics

**Story:** As a developer, I can configure which evaluation metrics to run against my EvalSet so that I score my agent on the dimensions that matter.

**Details:**
- Available metrics are selected per EvalSet evaluation run.
- Each metric has configurable thresholds (e.g., response_match_score >= 0.8).
- Default criteria: `tool_trajectory_avg_score: 1.0`, `response_match_score: 0.8`.

**Acceptance Criteria:**
- [ ] AC-03.1: User can select which prebuilt metrics to enable for an evaluation run.
- [ ] AC-03.2: User can configure the pass/fail threshold for each selected metric.
- [ ] AC-03.3: Default thresholds are pre-populated for each metric.
- [ ] AC-03.4: User can save a metric configuration profile for reuse.
- [ ] AC-03.5: Incompatible metric combinations are prevented with explanations.

**Dependencies:** None (metric catalog must be defined). Can develop in parallel with EVAL-CORE-01 and EVAL-CORE-02.

---

## EVAL-CORE-04: Run Evaluation & View Results

**Story:** As a developer, I can run an evaluation on an EvalSet against a specific agent and view scored results so that I know how well my agent performs.

**Details:**
- Evaluation is a two-phase process: inference (run agent on all test cases) then evaluation (score all invocations).
- `num_runs` parameter (default 2) controls how many times each EvalCase is executed for statistical reliability.
- Results show per-case scores and overall aggregated scores.
- Evaluation can run locally or against a deployed agent.

**Acceptance Criteria:**
- [ ] AC-04.1: User can select an agent and an EvalSet to evaluate.
- [ ] AC-04.2: User can configure `num_runs` (1–10) for the evaluation.
- [ ] AC-04.3: Evaluation progress is shown in real time (cases completed / total).
- [ ] AC-04.4: Results dashboard shows overall score, per-case scores, and pass/fail status per metric.
- [ ] AC-04.5: User can expand any case result to see detailed per-invocation scores.
- [ ] AC-04.6: Results are persisted and can be viewed later.
- [ ] AC-04.7: User can cancel a running evaluation.

**Dependencies:** Depends on EVAL-CORE-01 (EvalSets), EVAL-CORE-02 (EvalCases), EVAL-CORE-03 (metric config). Must be developed after the metric evaluators (EVAL-CORE-05 through EVAL-CORE-08) are implemented.

---

## EVAL-CORE-05: Tool Trajectory Evaluation

**Story:** As a developer, the system can compare the agent's actual tool call sequence against my expected trajectory so that I know if the agent is using tools correctly.

**Details:**
- Three match types:
  - **EXACT**: Tool calls must match in exact order with exact parameters.
  - **IN_ORDER**: Expected tools must be called in order, but other tools may be interleaved.
  - **ANY_ORDER**: All expected tools must be called, order doesn't matter.
- Per-tool-call scoring with average across the trajectory.
- Parameters can be compared exactly or with flexible matching.

**Acceptance Criteria:**
- [ ] AC-05.1: EvalCase can specify expected tool calls with match type (EXACT / IN_ORDER / ANY_ORDER).
- [ ] AC-05.2: Score is 0–1 based on how well the actual trajectory matches expected.
- [ ] AC-05.3: Missing tool calls are identified in the detailed results.
- [ ] AC-05.4: Extra (unexpected) tool calls are identified and flagged.
- [ ] AC-05.5: Out-of-order tool calls are identified when IN_ORDER match type is used.
- [ ] AC-05.6: Per-tool-call parameter comparison is shown in detailed results.

**Dependencies:** None (standalone evaluator). Can develop in parallel with EVAL-CORE-06, 07, 08.

---

## EVAL-CORE-06: Response Match Evaluation

**Story:** As a developer, the system can compare the agent's response against my golden reference response using both ROUGE-1 and LLM-as-judge so that I can measure response quality.

**Details:**
- Two response matching metrics:
  - **response_match_score** (v1): ROUGE-1 F1 score against golden response. Range [0, 1].
  - **final_response_match_v2**: LLM-as-judge semantic equivalence evaluation.
- **response_evaluation_score**: Evaluates response coherence using Vertex AI. Range [1, 5].

**Acceptance Criteria:**
- [ ] AC-06.1: ROUGE-1 score is computed between agent response and golden response.
- [ ] AC-06.2: LLM-as-judge evaluates semantic equivalence (not just lexical overlap).
- [ ] AC-06.3: Response coherence score (1–5) is available.
- [ ] AC-06.4: All three response metrics can be enabled independently.
- [ ] AC-06.5: Detailed results show the agent's actual response alongside the golden response for comparison.

**Dependencies:** None (standalone evaluators). Can develop in parallel with EVAL-CORE-05, 07, 08.

---

## EVAL-CORE-07: Hallucination Detection

**Story:** As a developer, the system can detect hallucinations in the agent's responses by checking each claim against the conversation context so that I can identify unreliable outputs.

**Details:**
- Two-step process:
  1. **Segmenter**: Breaks the response into individual sentences/claims.
  2. **Validator**: Classifies each sentence as: Supported, Unsupported, Contradictory, Disputed, or Not Applicable.
- Optional: also evaluates intermediate NL responses (tool outputs, not just final response).
- Score reflects the proportion of supported claims.

**Acceptance Criteria:**
- [ ] AC-07.1: Each sentence in the agent's response is classified (Supported / Unsupported / Contradictory / Disputed / Not Applicable).
- [ ] AC-07.2: Overall hallucination score reflects the proportion of supported claims.
- [ ] AC-07.3: Detailed results show sentence-by-sentence classification.
- [ ] AC-07.4: Unsupported and Contradictory sentences are highlighted in red.
- [ ] AC-07.5: Optionally evaluates intermediate tool-output responses, not just the final response.

**Dependencies:** None (standalone evaluator). Can develop in parallel with EVAL-CORE-05, 06, 08.

---

## EVAL-CORE-08: Safety Evaluation

**Story:** As a developer, the system can score the agent's responses for safety and harmlessness using Vertex AI's safety models so that I can ensure my agent doesn't produce harmful content.

**Details:**
- Uses Vertex AI's built-in safety evaluation.
- Returns a score in range [0, 1].
- Checks for harmful, biased, toxic, or unsafe content.

**Acceptance Criteria:**
- [ ] AC-08.1: Safety score is computed for each agent response via Vertex AI.
- [ ] AC-08.2: Score range is [0, 1] where higher is safer.
- [ ] AC-08.3: Detailed results show which safety categories were violated (if any).
- [ ] AC-08.4: Responses that fail the safety threshold are flagged.

**Dependencies:** None (standalone evaluator, requires Vertex AI credentials). Can develop in parallel with EVAL-CORE-05, 06, 07.

---

# Part B — Helpful Requirements

---

## EVAL-HELP-01: Rubric-Based Evaluation

**Story:** As a developer, I can define custom rubrics with score levels and descriptions so that I can evaluate my agent against domain-specific quality criteria.

**Details:**
- A Rubric contains:
  - Name and description.
  - Score levels (typically 1–5) with descriptions for each level.
  - Each level has a score value and a description of what qualifies for that score.
- Three rubric-based metrics:
  - **rubric_based_final_response_quality_v1**: Final response quality.
  - **rubric_based_tool_use_quality_v1**: Tool use quality.
  - Custom rubrics at the EvalCase level.
- LLM-as-judge applies the rubric with majority vote aggregation (multiple samples to reduce variance).
- Mean summarization across invocations.

**Acceptance Criteria:**
- [ ] AC-H01.1: User can create a rubric with name, description, and 3–5 score levels.
- [ ] AC-H01.2: Each score level has a numeric score and a text description.
- [ ] AC-H01.3: Rubrics can be assigned to an EvalSet or individual EvalCases.
- [ ] AC-H01.4: LLM-as-judge evaluates responses against rubrics with majority vote (3+ samples).
- [ ] AC-H01.5: Detailed results show the rubric score breakdown and the LLM's reasoning.
- [ ] AC-H01.6: Rubric scores are aggregated across invocations via mean.
- [ ] AC-H01.7: Rubrics can be reused across multiple EvalSets.

**Dependencies:** Depends on EVAL-CORE-03 (metric configuration). Can develop after basic evaluation pipeline is working.

---

## EVAL-HELP-02: Custom Metrics

**Story:** As a developer, I can define my own custom evaluation metric functions so that I can score my agent on domain-specific criteria not covered by prebuilt metrics.

**Details:**
- Custom metrics are Python functions that receive invocation context and return a score.
- Registered via the `custom_metrics` config on EvalConfig.
- Function signature receives: events, final response, tool calls, and returns a numeric score.

**Acceptance Criteria:**
- [ ] AC-H02.1: User can upload or define a custom metric function.
- [ ] AC-H02.2: Custom metric appears alongside prebuilt metrics in the results.
- [ ] AC-H02.3: Custom metric scores are included in overall aggregation.
- [ ] AC-H02.4: Custom metric errors are surfaced with stack traces for debugging.
- [ ] AC-H02.5: Custom metrics can be enabled/disabled per EvalSet.

**Dependencies:** Depends on EVAL-CORE-04 (evaluation pipeline). Develop after prebuilt metrics are working.

---

## EVAL-HELP-03: LLM-as-a-Judge Evaluation

**Story:** As a developer, I can use an LLM as a judge to evaluate agent responses with configurable model, instructions, and number of samples so that I get nuanced quality assessment beyond lexical matching.

**Details:**
- Configurable via `JudgeModelOptions`:
  - `model`: Which LLM to use as judge.
  - `config`: Model parameters (temperature, etc.).
  - `num_samples`: Number of judge evaluations to run (for majority vote).
- Used in: `final_response_match_v2`, rubric-based criteria, multi-turn metrics.
- Majority vote aggregation reduces judge unreliability.

**Acceptance Criteria:**
- [ ] AC-H03.1: User can select which model to use as the judge.
- [ ] AC-H03.2: User can configure the number of judge samples (default 3).
- [ ] AC-H03.3: Judge evaluation includes the LLM's reasoning in detailed results.
- [ ] AC-H03.4: Majority vote aggregation is used when multiple samples are run.
- [ ] AC-H03.5: Judge model and cost are displayed in the evaluation summary.

**Dependencies:** Depends on EVAL-CORE-04 (evaluation pipeline). Can develop in parallel with EVAL-HELP-01.

---

## EVAL-HELP-04: Multi-Turn Evaluation

**Story:** As a developer, I can evaluate my agent's performance across multi-turn conversations so that I know it handles extended interactions well.

**Details:**
- Three multi-turn metrics:
  - **multi_turn_task_success_v1**: Did the agent accomplish the overall task across turns?
  - **multi_turn_trajectory_quality_v1**: Quality of the tool trajectory across all turns.
  - **multi_turn_tool_use_quality_v1**: Quality of individual tool calls across turns.
- EvalCases can define multi-turn conversation scenarios.
- Static multi-turn: pre-defined user messages per turn.
- Dynamic multi-turn: LLM-backed user simulator generates responses.

**Acceptance Criteria:**
- [ ] AC-H04.1: EvalCase can define a multi-turn conversation (list of expected turns).
- [ ] AC-H04.2: Multi-turn metrics produce per-case and overall scores.
- [ ] AC-H04.3: Detailed results show per-turn breakdown of tool calls and responses.
- [ ] AC-H04.4: Task success is evaluated at the conversation level, not per-turn.

**Dependencies:** Depends on EVAL-CORE-04 (evaluation pipeline) and EVAL-HELP-05 (user simulation, for dynamic multi-turn).

---

## EVAL-HELP-05: User Simulation

**Story:** As a developer, I can configure an AI user simulator to have multi-turn conversations with my agent during evaluation so that I can test conversational flows without manual input.

**Details:**
- Two simulator types:
  - **StaticUserSimulator**: Pre-defined list of user messages (from `user_messages` in EvalCase).
  - **LlmBackedUserSimulator**: LLM generates user messages based on a conversation plan.
- LLM-backed simulator configuration:
  - `model`: LLM to use as simulator.
  - `max_invocations`: Maximum number of turns.
  - `custom_instructions`: Custom behavior for the simulated user.
  - `stop_signal`: Condition for ending the conversation.
- `per_turn_user_simulator_quality_v1`: Evaluates how well the simulator performed.
- User personas available for varied simulation behavior.

**Acceptance Criteria:**
- [ ] AC-H05.1: User can select static or LLM-backed simulation mode per EvalCase.
- [ ] AC-H05.2: Static mode: user messages are pre-defined in the EvalCase.
- [ ] AC-H05.3: LLM-backed mode: user can configure model, max turns, custom instructions, and stop signal.
- [ ] AC-H05.4: Simulator's generated messages are visible in the detailed evaluation trace.
- [ ] AC-H05.5: User can select from pre-built personas or define custom personas.
- [ ] AC-H05.6: Simulation quality score is available to assess simulator reliability.
- [ ] AC-H05.7: Simulation cost is tracked and displayed.

**Dependencies:** Depends on EVAL-CORE-04 (evaluation pipeline). LLM-backed simulator requires a judge model configuration.

---

## EVAL-HELP-06: Evaluation Results — Comparison & Trending

**Story:** As a developer, I can compare evaluation results across different agent versions and over time so that I can track quality improvements.

**Details:**
- Evaluation results are stored with: EvalSet ID, agent version, timestamp, and scores.
- Side-by-side comparison of two evaluation runs.
- Trend charts showing score progression over time.
- Per-metric breakdown in comparisons.

**Acceptance Criteria:**
- [ ] AC-H06.1: User can view a history of all evaluation runs for an EvalSet.
- [ ] AC-H06.2: User can select two runs for side-by-side comparison.
- [ ] AC-H06.3: Comparison view highlights score deltas (green = improvement, red = regression).
- [ ] AC-H06.4: Trend chart shows score progression over time per metric.
- [ ] AC-H06.5: User can filter results by agent version or date range.

**Dependencies:** Depends on EVAL-CORE-04 (results must be persisted). Can develop after basic results view is working.

---

## EVAL-HELP-07: Eval Results Detail View — Full Trace Inspection

**Story:** As a developer, I can inspect the full execution trace of any evaluated invocation — including user messages, agent responses, tool calls, and judge reasoning — so that I can understand exactly why a score was assigned.

**Details:**
- For each invocation: full conversation transcript with all turns.
- Tool call timeline with parameters and responses.
- LLM-as-judge reasoning text for each rubric/criterion.
- Hallucination sentence-by-sentence breakdown.
- Safety violation details.

**Acceptance Criteria:**
- [ ] AC-H07.1: User can click into any case result to see the full invocation trace.
- [ ] AC-H07.2: Trace shows user messages, agent responses, and tool calls in chronological order.
- [ ] AC-H07.3: LLM-as-judge reasoning is expandable for each scored criterion.
- [ ] AC-H07.4: Hallucination analysis highlights each sentence with its classification.
- [ ] AC-H07.5: Tool trajectory comparison (expected vs actual) is visualized.
- [ ] AC-H07.6: User can copy the full trace to clipboard or export as JSON.

**Dependencies:** Depends on EVAL-CORE-04 (results) and EVAL-CORE-05 through EVAL-CORE-08 (individual evaluator details).

---

# Part C — Advanced Requirements

---

## EVAL-ADV-01: Conversation Scenario Generation

**Story:** As a developer, I can auto-generate multi-turn conversation scenarios from a high-level description so that I can quickly create comprehensive test suites.

**Details:**
- `conversation_scenarios.py` provides pre-built scenario templates.
- Generates realistic multi-turn conversations based on agent domain.
- Scenarios include expected tool calls and golden responses.

**Acceptance Criteria:**
- [ ] AC-A01.1: User can describe an agent's domain and get suggested test scenarios.
- [ ] AC-A01.2: Generated scenarios include expected tool trajectories.
- [ ] AC-A01.3: User can edit and refine auto-generated scenarios.
- [ ] AC-A01.4: Pre-built scenario templates are available for common domains (customer service, travel, etc.).

**Dependencies:** Depends on EVAL-CORE-02 (EvalCase management). Develop after manual case creation is stable.

---

## EVAL-ADV-02: Evaluation via Web UI (adk web Integration)

**Story:** As a developer, I can run and monitor evaluations directly from the web UI (not just CLI) so that I don't need to switch contexts.

**Details:**
- ADK web UI has evaluation tabs with:
  - EvalSet browser.
  - Evaluation launch form (select agent, EvalSet, metrics, num_runs).
  - Real-time progress monitoring.
  - Results dashboard with charts.
  - Detail view for individual invocations.

**Acceptance Criteria:**
- [ ] AC-A02.1: EvalSets are browsable from the web UI.
- [ ] AC-A02.2: Evaluation can be launched from the web UI.
- [ ] AC-A02.3: Real-time progress bar during evaluation.
- [ ] AC-A02.4: Results dashboard with visual charts.
- [ ] AC-A02.5: Detail view accessible from results dashboard.
- [ ] AC-A02.6: Evaluation can be cancelled from the UI.

**Dependencies:** Depends on EVAL-CORE-04 (evaluation pipeline). Develop after CLI evaluation works.

---

## EVAL-ADV-03: Evaluation API — REST Endpoints

**Story:** As a developer, I can trigger evaluations and retrieve results via REST API so that I can integrate evaluation into CI/CD pipelines.

**Details:**
- REST endpoints (defined in Go ADK as skeleton, implemented in Python):
  - `GET /apps/{app_name}/eval_sets` — list eval sets.
  - `POST /apps/{app_name}/eval_sets/{eval_set_name}` — create/get eval set.
  - `GET /apps/{app_name}/eval_results` — list eval results.
  - Additional endpoints for launching evaluations and getting detailed results.
- SSE streaming for real-time evaluation progress.

**Acceptance Criteria:**
- [ ] AC-A03.1: REST endpoints for EvalSet CRUD are functional.
- [ ] AC-A03.2: REST endpoint for launching evaluations is functional.
- [ ] AC-A03.3: REST endpoint for retrieving results is functional.
- [ ] AC-A03.4: Evaluation progress is streamable via SSE.
- [ ] AC-A03.5: API authentication and authorization are enforced.

**Dependencies:** Depends on EVAL-CORE-04 (evaluation pipeline). Can develop in parallel with EVAL-ADV-02.

---

## EVAL-ADV-04: Distributed / Parallel Evaluation

**Story:** As an operator, evaluation can run invocations in parallel across multiple workers so that large EvalSets complete quickly.

**Details:**
- `LocalEvalService` supports parallel inference and parallel evaluation.
- Each EvalCase invocation is independent and can be parallelized.
- `num_runs` executions per case can run in parallel.
- Thread pool / asyncio-based parallelism.

**Acceptance Criteria:**
- [ ] AC-A04.1: Large EvalSets (50+ cases) complete in a reasonable time through parallelism.
- [ ] AC-A04.2: Parallelism level is configurable (max workers).
- [ ] AC-A04.3: Failures in individual invocations don't block the rest of the evaluation.
- [ ] AC-A04.4: Results are correctly aggregated regardless of parallelism order.

**Dependencies:** Depends on EVAL-CORE-04. Develop when performance becomes a bottleneck for large EvalSets.

---

## EVAL-ADV-05: Evaluation Data Migration & Schema Management

**Story:** As a developer, my old evaluation data can be migrated to new schema versions so that I don't lose historical results when the evaluation system evolves.

**Details:**
- `migrate_eval_data_to_new_schema()` utility for migrating old EvalCase/EvalResult formats.
- EvalResult schema evolution (v1 → v2 for response matching).
- Backward compatibility for older EvalSet JSON files.

**Acceptance Criteria:**
- [ ] AC-A05.1: Old EvalSet JSON files can be imported and auto-migrated.
- [ ] AC-A05.2: Evaluation results from older schema versions are viewable after migration.
- [ ] AC-A05.3: Migration reports any data that couldn't be converted.
- [ ] AC-A05.4: Migration is non-destructive (original data is preserved).

**Dependencies:** Depends on EVAL-CORE-04. Develop when schema changes are introduced.

---

# Dependency Graph & Development Order

## Phase 1 — Core (MVP)

```
EVAL-CORE-01 (EvalSets) ──► EVAL-CORE-02 (EvalCases) ──┐
                                                        │
EVAL-CORE-03 (Metric Config) ───────────────────────────┼──► EVAL-CORE-04 (Run Eval + Results)
                                                        │
EVAL-CORE-05 (Trajectory Eval) ─────────────────────────┤
EVAL-CORE-06 (Response Match Eval) ─────────────────────┤
EVAL-CORE-07 (Hallucination Eval) ──────────────────────┤
EVAL-CORE-08 (Safety Eval) ─────────────────────────────┘
```

**Parallel tracks:**
- **Track 1:** EVAL-CORE-01 → EVAL-CORE-02 (EvalSet/Case management UI)
- **Track 2:** EVAL-CORE-03 (Metric configuration UI)
- **Track 3:** EVAL-CORE-05, 06, 07, 08 (Four metric evaluators — all independent, develop in parallel)
- **Integration:** EVAL-CORE-04 (wires everything together — must wait for Tracks 1, 2, 3)

---

## Phase 2 — Helpful (after Phase 1)

```
Phase 1 Complete
       │
       ├─► EVAL-HELP-01 (Rubrics) ────────────┐
       ├─► EVAL-HELP-02 (Custom Metrics)      │
       ├─► EVAL-HELP-03 (LLM-as-Judge) ───────┼─► EVAL-HELP-04 (Multi-Turn Eval)
       ├─► EVAL-HELP-05 (User Simulation) ────┘
       ├─► EVAL-HELP-06 (Comparison/Trending)
       └─► EVAL-HELP-07 (Trace Detail View)
```

**Parallel tracks:**
- **Track 4:** EVAL-HELP-01 + EVAL-HELP-03 + EVAL-HELP-05 (rubrics, judge, simulation) → EVAL-HELP-04 (multi-turn)
- **Track 5:** EVAL-HELP-02 (custom metrics) — independent
- **Track 6:** EVAL-HELP-06 (comparison/trending) + EVAL-HELP-07 (trace detail) — UI work, independent

---

## Phase 3 — Advanced (after Phase 2)

```
Phase 2 Complete
       │
       ├─► EVAL-ADV-01 (Scenario Generation)
       ├─► EVAL-ADV-02 (Web UI Integration) ── depends on ADV-03 (API) or core eval pipeline
       ├─► EVAL-ADV-03 (REST API)
       ├─► EVAL-ADV-04 (Parallel Evaluation)
       └─► EVAL-ADV-05 (Schema Migration)
```

**Parallel tracks:**
- **Track 7:** EVAL-ADV-02 (Web UI) + EVAL-ADV-03 (REST API) — can develop in parallel, same backend
- **Track 8:** EVAL-ADV-01 (Scenario Generation) + EVAL-ADV-04 (Parallel Eval) + EVAL-ADV-05 (Migration) — all independent

---

# Acceptance Criteria Summary

| ID | Category | # Criteria | Description |
|---|---|---|---|
| EVAL-CORE-01 | EvalSet Management | 7 | CRUD eval sets, import/export JSON |
| EVAL-CORE-02 | EvalCase Management | 7 | Create/edit test cases with expected trajectory & golden response |
| EVAL-CORE-03 | Metric Configuration | 5 | Select metrics, set thresholds, save profiles |
| EVAL-CORE-04 | Run Evaluation | 7 | Launch eval, real-time progress, view results, cancel |
| EVAL-CORE-05 | Trajectory Evaluation | 6 | EXACT/IN_ORDER/ANY_ORDER tool call matching |
| EVAL-CORE-06 | Response Match Evaluation | 5 | ROUGE-1, LLM-as-judge, coherence scoring |
| EVAL-CORE-07 | Hallucination Detection | 5 | Sentence-level claim classification |
| EVAL-CORE-08 | Safety Evaluation | 4 | Vertex AI safety scoring |
| EVAL-HELP-01 | Rubric-Based Evaluation | 7 | Custom rubrics, LLM-as-judge, majority vote |
| EVAL-HELP-02 | Custom Metrics | 5 | User-defined metric functions |
| EVAL-HELP-03 | LLM-as-a-Judge | 5 | Configurable judge model, majority vote |
| EVAL-HELP-04 | Multi-Turn Evaluation | 4 | Task success, trajectory, tool quality across turns |
| EVAL-HELP-05 | User Simulation | 7 | Static and LLM-backed multi-turn simulation |
| EVAL-HELP-06 | Comparison & Trending | 5 | Side-by-side runs, trend charts, filtering |
| EVAL-HELP-07 | Trace Detail View | 6 | Full invocation trace with judge reasoning |
| EVAL-ADV-01 | Scenario Generation | 4 | Auto-generate test cases from domain description |
| EVAL-ADV-02 | Web UI Integration | 6 | Full evaluation experience in browser |
| EVAL-ADV-03 | REST API | 5 | Programmatic eval triggers and results |
| EVAL-ADV-04 | Parallel Evaluation | 4 | Concurrent invocation execution |
| EVAL-ADV-05 | Schema Migration | 4 | Backward-compatible data migration |

**Total: 20 stories · 109 acceptance criteria**

---

## Prebuilt Metrics Reference

The following 12 prebuilt metrics are defined in the ADK evaluation framework and should be supported:

| # | Metric Name | Type | Range | Description |
|---|---|---|---|---|
| 1 | `tool_trajectory_avg_score` | Trajectory | [0, 1] | Tool call sequence matching accuracy |
| 2 | `response_evaluation_score` | Response | [1, 5] | Response coherence (Vertex AI) |
| 3 | `response_match_score` | Response | [0, 1] | ROUGE-1 F1 vs golden response |
| 4 | `safety_v1` | Safety | [0, 1] | Harmlessness via Vertex AI |
| 5 | `final_response_match_v2` | Response | — | LLM-as-judge semantic equivalence |
| 6 | `rubric_based_final_response_quality_v1` | Rubric | — | Response quality via rubrics |
| 7 | `hallucinations_v1` | Hallucination | — | Sentence-level halluc detection |
| 8 | `rubric_based_tool_use_quality_v1` | Rubric | — | Tool use quality via rubrics |
| 9 | `per_turn_user_simulator_quality_v1` | Simulation | — | User simulation quality |
| 10 | `multi_turn_task_success_v1` | Multi-Turn | — | Overall task completion across turns |
| 11 | `multi_turn_trajectory_quality_v1` | Multi-Turn | — | Tool trajectory quality across turns |
| 12 | `multi_turn_tool_use_quality_v1` | Multi-Turn | — | Individual tool call quality across turns |

---

## Test Case JSON Format Reference

EvalCases in the ADK framework use this JSON structure:

```json
{
  "name": "test_case_1",
  "input": "What's the weather in Tokyo?",
  "expected_trajectory": [
    {
      "tool_name": "google_search",
      "args": {"query": "weather Tokyo"},
      "match_type": "EXACT"
    }
  ],
  "expected_response": "The weather in Tokyo is currently...",
  "rubrics": [
    {
      "name": "helpfulness",
      "description": "How helpful is the response?",
      "levels": [
        {"score": 1, "description": "No useful information"},
        {"score": 3, "description": "Partial answer with some detail"},
        {"score": 5, "description": "Complete, accurate, and well-structured"}
      ]
    }
  ],
  "user_messages": [
    "What about tomorrow?",
    "And the weekend?"
  ]
}
```

EvalSet JSON structure:

```json
{
  "eval_set_id": "weather_agent_evals",
  "name": "Weather Agent Tests",
  "description": "Evaluation suite for the weather agent",
  "eval_cases": [ /* array of EvalCases */ ]
}
```

---

## Development Strategy

1. **Start with Phase 1** — Tracks 1, 2, 3 can all begin immediately. The four metric evaluators (Track 3) are fully independent and can be developed by separate developers.
2. **Integration milestone:** Wire EvalSet/Case management + Metric Configuration + all 4 core evaluators into EVAL-CORE-04 (the evaluation runner). This is the first milestone where end-to-end evaluation works.
3. **Phase 2** — Rubrics, LLM-as-Judge, and User Simulation (Track 4) share infrastructure (judge model config, multi-turn runner) so should be developed together. Custom metrics (Track 5) and UI comparison views (Track 6) are independent.
4. **Phase 3** — Web UI integration (EVAL-ADV-02) and REST API (EVAL-ADV-03) should be developed together since they expose the same backend. Scenario generation, parallel evaluation, and schema migration are lower priority and can be developed as needed.
