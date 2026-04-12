# ADK Workflow Patterns: Complete Gap Analysis

> **Date:** April 11, 2026
> **Source:** `/home/mabdelrahman/SchoolWithAI/.READ/adk-samples` (Python, Go, Java, TypeScript)
> **Target:** `/home/mabdelrahman/SchoolWithAI/modules/templates/catalog/`
> **Goal:** Identify every ADK framework functionality and determine which patterns are missing from the user template catalog.

---

## ADK Samples Surveyed

| Language | Agents Found | Key Directories |
|----------|-------------|-----------------|
| **Python** | 30+ agents | `.READ/adk-samples/python/agents/` |
| **Go** | 5 agents | `.READ/adk-samples/go/agents/` |
| **Java** | 2 agents | `.READ/adk-samples/java/agents/` |
| **TypeScript** | 1 agent | `.READ/adk-samples/typescript/agents/` |

### Notable Python Samples
- `workflows-sequential` — Basic sequential state passing
- `workflows-HITL_concierge` — Human-in-the-loop with RequestInput
- `workflow-morning_email_debrief` — Timed/scheduled trigger
- `workflow-concurrent_research_writer` — Conditional routing + ParallelWorker
- `parallel_task_decomposition_execution` — Parallel with nested sequential
- `story_teller` — Loop + parallel + nested sequential
- `podcast_transcript_agent` — 3-stage sequential pipeline
- `blog-writer` — Two allow_early_exit loops
- `deep-search` — Iterative refinement with escalation
- `image-scoring` — Loop with threshold-based exit
- `on-brand-genmedia` — Loop with brand checker
- `claim-adjudication-agent` — Sequential containing parallel
- `nexshift-agent` — Sequential with parallel validation
- `global-kyc-agent` — Parallel data retrieval
- `marketing-agency` — 5-agent coordinator pipeline
- `academic-research` — Coordinator + web search
- `short-movie-agents` — Creative transformation pipeline
- `travel-concierge` — Multi-phase travel planning
- `small-business-loan-agent` — Loan decision pipeline
- `invoice_processing` — Multi-stage with gated stopping
- `camel` — Code generation loop (max 10 iterations)
- `machine-learning-engineering` — Deeply nested debug/run loops
- `retail-ai-location-strategy` — 7-step location intelligence
- `RAG` — Simple RAG agent
- `personalized-shopping` — Shopping assistant
- `safety-plugins` — Safety enforcement
- `fomc-research` — FOMC research agent
- `policy-as-code` — Policy-as-code with MCP

### Notable Go Samples
- `financial-advisor` — Coordinator with AgentTool for 4 analysts
- `llm-auditor` — SequentialAgent (critic + reviser)
- `sail-researcher` — MultiLoader (3 independent agents as REST API)
- `boat-agent` — Multi-agent REST server pattern
- `navallist` — Navigation agent

### Notable Java Samples
- `software-bug-assistant` — MCP integration via SSE
- `time-series-forecasting` — MCP toolbox for forecasting

### Notable TypeScript Samples
- `customer_service` — before_model_callback for rate limiting

---

## Complete Pattern Inventory (30 Patterns)

### 1. Sequential Workflow
**Description:** Agents execute in strict linear order. Output of each step flows to the next.

| ADK Sample | File |
|------------|------|
| workflows-sequential | `python/agents/workflows-sequential/agent.py` |
| podcast_transcript_agent | `python/agents/podcast_transcript_agent/podcast_transcript_agent/agent.py` |
| workflow-morning_email_debrief | `python/agents/workflow-morning_email_debrief/agent.py` |
| retail-ai-location-strategy | `python/agents/retail-ai-location-strategy/app/agent.py` |
| llm-auditor (Go) | `go/agents/llm-auditor/auditor/auditor.go` |

**In catalog:** ✅ city-time-report, podcast-transcript, llm-auditor

---

### 2. Sequential with `output_key` (Named State Injection)
**Description:** Agent output is stored in a named session state key for downstream agents to reference via `{variable}` injection.

| ADK Sample | output_key Examples |
|------------|-------------------|
| workflows-sequential | `"city"` |
| deep-search | `"report_sections"`, `"section_research_findings"`, `"research_evaluation"`, `"final_cited_report"`, `"research_plan"` |
| story_teller | `KEY_ENHANCED_PROMPT`, `KEY_CURRENT_STORY`, `KEY_FINAL_STORY` |
| travel-concierge/planning | `"itinerary"`, `"room"`, `"hotel"`, `"seat"`, `"flight"` |
| parallel_task_decomposition | `"drafted_email"`, `"email_publication_result"`, `"drafted_slack_message"`, `"event_details"` |
| financial-advisor (Go) | `"financial_coordinator_output"`, `"market_data_analysis_output"`, `"proposed_trading_strategies_output"` |

**In catalog:** ✅ Most templates use output_key extensively

---

### 3. Optional State Injection `{variable?}`
**Description:** Template variable substitution that returns empty string if key is absent instead of erroring.

| ADK Sample | File |
|------------|------|
| deep-search | Uses `{research_plan}`, `{section_research_findings}`, `{sources}` in instruction strings |
| story_teller | `{enhanced_prompt?}`, `{current_story?}` in nested writers |
| marketing-agency | `{domain_create_output?}` read by 3 downstream agents |
| short-movie-agents | `{storyboard?}`, `{story?}` in VideoAgent |

**In catalog:** ✅ Most templates use `{var?}` syntax

---

### 4. Parallel Execution (Fan-out/Fan-in)
**Description:** Multiple sub-agents execute concurrently; results are combined.

| ADK Sample | File |
|------------|------|
| parallel_task_decomposition | `parallel_task_decomposition_agent/agent.py` — 3 channels |
| story_teller | `story_teller_agent/agent.py` — creative + focused writers |
| claim-adjudication | `claim_adjudication_agent/agent.py` — admissibility + amount |
| nexshift-agent | `nexshift_agent/sub_agents/coordinator.py` — compliance + empathy |
| global-kyc-agent | `global_kyc_agent/sub_agents/uk_kyc/agent.py` — multi-source retrieval |
| machine-learning-engineering | `sub_agents/initialization/agent.py` and `sub_agents/refinement/agent.py` |

**In catalog:** ✅ parallel-broadcast, story-teller

---

### 5. Nested: Sequential Inside Parallel
**Description:** Each parallel branch is itself a sequential pipeline.

| ADK Sample | Structure |
|------------|-----------|
| parallel_task_decomposition | Parallel( Sequential(drafter→publisher) × 3 ) |
| global-kyc-agent | Parallel + sequential sub-agents |

**In catalog:** ✅ parallel-broadcast

---

### 6. Nested: Parallel Inside Sequential
**Description:** A sequential workflow contains a parallel node as one of its steps.

| ADK Sample | Structure |
|------------|-----------|
| claim-adjudication | Sequential( Parallel(admissibility, amount) → summary ) |
| nexshift | Sequential( context → solver → Parallel(validation) → presenter ) |
| parallel_task_decomposition | Sequential( enhancer → Parallel(...) → summary ) |

**In catalog:** ✅ parallel-broadcast

---

### 7. Nested: Parallel Inside Loop Inside Sequential
**Description:** The deepest nesting pattern — sequential root → loop → sequential body → parallel writers.

| ADK Sample | Structure |
|------------|-----------|
| story_teller | Sequential → Loop → Sequential → Parallel(creative, focused) → Critique |

**In catalog:** ✅ story-teller

---

### 8. Loop (Fixed Iterations, No Early Exit)
**Description:** Loop runs exactly `max_iterations` times.

| ADK Sample | File | max_iterations |
|------------|------|----------------|
| story_teller | `story_teller_agent/agent.py` | 3 (chapters) |

**In catalog:** ✅ story-teller (max_iterations=3, allow_early_exit=false)

---

### 9. Loop with `allow_early_exit`
**Description:** Last agent in loop body receives `exit_loop` tool; calling it stops the loop before max_iterations.

| ADK Sample | File | Purpose |
|------------|------|---------|
| blog-writer | `sub_agents/blog_planner.py` | Retry outline until valid |
| blog-writer | `sub_agents/blog_writer.py` | Retry post until valid |
| deep-search | `app/agent.py` | Iterative research refinement |
| image-scoring | `image_scoring/agent.py` | Generate/score until threshold |
| on-brand-genmedia | `on_brand_genmedia/agent.py` | Generate/score brand media |
| camel | `camel/camel_agent/camel_agent.py` | Code gen with security check |

**In catalog:** ✅ blog-writer (2 loops), iterative-deep-research

---

### 10. Validation/Checker Agent Pattern
**Description:** Custom BaseAgent that inspects state and calls `exit_loop` when ready.

| ADK Sample | File |
|------------|------|
| blog-writer | `blogger_agent/validation_checkers.py` — OutlineValidationChecker, BlogPostValidationChecker |
| deep-search | `app/agent.py` — EscalationChecker |
| image-scoring | `image_scoring/checker_agent.py` |
| on-brand-genmedia | `on_brand_genmedia/checker_agent.py` |
| invoice_processing | `invoice_processing/agent.py` — multi-validator (LLMRules, DataSource, BypassDetector, ToleranceExtractor, PerGroupValidator) |

**In catalog:** ✅ blog-writer, iterative-deep-research

---

### 11. Coordinator/Orchestrator (AgentTool)
**Description:** Coordinator LLM agent decides which sub-agents to invoke. Sub-agents wrapped as AgentTool.

| ADK Sample | File | Sub-agents |
|------------|------|------------|
| marketing-agency | `marketing_agency/agent.py` | domain_create, website_create, marketing_create, logo_create |
| academic-research | `academic_research/agent.py` | websearch, newresearch |
| travel-concierge | `travel_concierge/agent.py` | inspiration, planning, booking, pre_trip, in_trip, post_trip |
| small-business-loan-agent | `small_business_loan_agent/agent.py` | document_extraction, underwriting, pricing, loan_decision |
| short-movie-agents | `app/agent.py` | story, screenplay, storyboard, video |
| blog-writer | `blogger_agent/agent.py` | planner, writer, editor, social_media_writer |
| financial-advisor (Go) | `main.go` | data_analyst, trading_analyst, execution_analyst, risk_analyst |
| nexshift coordinator | `nexshift_agent/sub_agents/coordinator.py` | RosteringWorkflow + tools |
| invoice_processing | `invoice_processing/agent.py` | Acting, Investigation, ALF stages |

**In catalog:** ✅ marketing-agency, short-movie-agent, academic-research

---

### 12. Human-in-the-Loop (HITL)
**Description:** Workflow pauses and requests user input via `RequestInput` event, then resumes.

| ADK Sample | File |
|------------|------|
| workflows-HITL_concierge | `python/agents/workflows-HITL_concierge/agent.py` |

**In catalog:** ❌ **MISSING**

**Implementation pattern:**
```python
async def get_user_feedback(node_input: ActivitiesList):
    yield RequestInput(message=f"Here is your recommended itinerary:\n{node_input}", ...)

root_agent = WorkflowAgent(
    rerun_on_resume=True,
    edges=[
        ("START", initial_prompt, concierge_agent, get_user_feedback, process_feedback),
        (process_feedback, concierge_agent),  # Edge back creates loop
    ],
)
```

---

### 13. Conditional Routing / DAG-Style Edges
**Description:** Function nodes evaluate conditions and route execution to different paths.

| ADK Sample | File |
|------------|------|
| workflow-concurrent_research_writer | `python/agents/workflow-concurrent_research_writer/agent.py` |

**In catalog:** ✅ customer-support-triage

**Implementation pattern:**
```python
blog_workflow = WorkflowAgent(
    edges=[
        (START, start_blog, generate_blog_post_agent, route_changer),
        (route_changer, post_to_x, "X"),           # Short → X
        (route_changer, post_to_linkedin, "LINKEDIN"),  # Medium → LinkedIn
        (route_changer, post_to_medium, "MEDIUM"),       # Long → Medium
    ],
)
```

---

### 14. ParallelWorker (Fan-out Across Inputs)
**Description:** In WorkflowAgent, a single worker agent is fanned out across multiple inputs/platforms.

| ADK Sample | File |
|------------|------|
| workflow-concurrent_research_writer | `agent_nodes/research.py` |

**In catalog:** ❌ **MISSING**

**Implementation pattern:**
```python
research_workflow = WorkflowAgent(
    edges=[(START, start_node, ParallelWorker(research_worker_agent), distill_agent, save_node)],
)
# research_worker_agent runs for X, LinkedIn, Reddit, Medium independently
```

---

### 15. Function Nodes in WorkflowAgent
**Description:** Custom Python functions as workflow nodes that yield `Event` objects for state manipulation and routing.

| ADK Sample | File |
|------------|------|
| workflow-concurrent_research_writer | `function_nodes/research.py`, `function_nodes/publishing.py` |
| workflow-morning_email_debrief | `agent.py` |
| workflows-sequential | `agent.py` |

**In catalog:** ❌ **MISSING**

**Implementation pattern:**
```python
async def route_changer(blog_post):
    word_count = len(blog_post.split())
    if word_count < 500:
        yield Event(route="X")
    elif word_count < 1500:
        yield Event(route="LINKEDIN")
    else:
        yield Event(route="MEDIUM")
```

---

### 16. Output Schema (Structured Output)
**Description:** `output_schema` enforces Pydantic model-validated structured output.

| ADK Sample | File | Schema Types |
|------------|------|-------------|
| deep-search | `app/agent.py` | `Feedback` (grade: pass/fail, comment, follow_up_queries), `SearchQuery` |
| travel-concierge/planning | `sub_agents/planning/agent.py` | `types.Itinerary`, `types.RoomsSelection`, `types.HotelsSelection`, `types.SeatsSelection`, `types.FlightsSelection` |
| workflows-HITL_concierge | `agent.py` | `ActivitiesList` (list of dicts) |
| small-business-loan-agent | Multiple sub-agents | `UnderwritingReport`, `LoanDecisionResult`, `LoanApplicationData`, `PricingResult` |

**In catalog:** ⚠️ Not explicitly demonstrated in template YAMLs

---

### 17. Callbacks (before/after agent/model/tool)
**Description:** Lifecycle hooks for pre/post processing at various stages.

| Callback Type | ADK Sample | File | Purpose |
|--------------|------------|------|---------|
| `before_agent_callback` | deep-search | `app/agent.py` | Collect research sources |
| `after_agent_callback` | deep-search | `app/agent.py` | Citation replacement |
| `before_model_callback` | customer_service (TS) | `customer_service/agent.ts` | Rate limiting |
| `after_model_callback` | nexshift | `nexshift_agent/sub_agents/coordinator.py` | Format model output |
| `before_tool_callback` | small-business-loan-agent | `small_business_loan_agent/agent.py` | Check process status |
| `after_tool_callback` | claim-adjudication | `claim_adjudication_agent/agent.py` | Custom post-processing |
| `before_agent_callback` | image-scoring | `image_scoring/agent.py` | Set unique_id and timestamp |
| `before_agent_callback` | travel-concierge | `travel_concierge/agent.py` | Load precreated itinerary |
| `after_agent_callback` | blog-writer/editor | `blogger_agent/sub_agents/blog_editor.py` | Suppress output |
| AfterModelCallback (Go) | llm-auditor/critic | `critic/critic.go` | renderReference |
| AfterModelCallback (Go) | llm-auditor/reviser | `reviser/reviser.go` | removeEndOfEditMark |
| BeforeTool/AfterTool (Go) | sail-researcher | `agent_setup.go` | Monitor tool usage |

**In catalog:** ❌ **MISSING**

---

### 18. MCP Tool Integration
**Description:** External Model Context Protocol server integration for tools.

| ADK Sample | Language | File |
|------------|----------|------|
| software-bug-assistant | Java | `SoftwareBugAssistant.java` — MCP via SSE |
| time-series-forecasting | Java | `ForecastingAgent.java` — MCP toolbox |
| policy-as-code | Python | `policy_as_code_agent/mcp.py` — Dataplex MCP |
| travel-concierge | Python | `tests/mcp_abnb.py` — Airbnb MCP |

**In catalog:** ❌ **MISSING**

---

### 19. Multi-Agent Server (Go MultiLoader)
**Description:** Multiple independent agents loaded as REST endpoints in a single server.

| ADK Sample | File |
|------------|------|
| sail-researcher | `go/agents/sail-researcher/main.go` — 3 agents: stopAgent, voyageAgent, discoveryAgent |
| boat-agent | `go/agents/boat-agent/main.go` |

**In catalog:** ❌ **MISSING**

---

### 20. Event-Based Data Passing in WorkflowAgent
**Description:** Nodes yield `Event` objects with state or routing data.

| ADK Sample | File |
|------------|------|
| workflows-sequential | `agent.py` — `yield Event(data={"time_info": ..., "city": ...})` |
| workflow-morning_email_debrief | `agent.py` — `yield Event(data={"emails": result})` |
| workflow-concurrent_research_writer | `function_nodes/research.py` — `yield Event(state={"topic": topic})` |

**In catalog:** ⚠️ Implicit via output_key (not explicitly shown)

---

### 21. rerun_on_resume (Stateful Resume)
**Description:** Workflow can resume and re-execute from saved state.

| ADK Sample | File |
|------------|------|
| workflow-concurrent_research_writer | `agent.py` |
| workflows-HITL_concierge | `agent.py` |

**In catalog:** ❌ **MISSING**

---

### 22. Pipeline Gated Stopping
**Description:** A stage returns STOPPED status on violation, preventing downstream stages from running.

| ADK Sample | File |
|------------|------|
| invoice_processing | `invoice_processing/agent.py` — MAJOR_VIOLATION stops ALF stage |

**In catalog:** ❌ **MISSING**

**Implementation pattern:**
```python
# Investigation stage returns STOPPED on MAJOR_VIOLATION
# Prevents ALF (Accounts Loss Forecast) stage from running
```

---

### 23. Timed/Scheduled Trigger
**Description:** Workflow designed to be deployed as an endpoint triggered by an external scheduler.

| ADK Sample | File |
|------------|------|
| workflow-morning_email_debrief | `agent.py` — Cloud Scheduler endpoint |

**In catalog:** ❌ **MISSING**

---

### 24. EscalationChecker Custom Agent
**Description:** Custom BaseAgent that yields `EventActions(escalate=True)` to control loop exit.

| ADK Sample | File |
|------------|------|
| deep-search | `app/agent.py` |
| blog-writer | `blogger_agent/validation_checkers.py` |
| camel | `camel/camel_agent/camel_agent.py` |
| image-scoring/loop_condition_tool | `image_scoring/tools/loop_condition_tool.py` |

**In catalog:** ⚠️ Partially in iterative-deep-research (evaluator calls exit_loop)

---

### 25. State Accumulation Across Iterations
**Description:** State grows with each loop iteration (appending, not overwriting).

| ADK Sample | File | State |
|------------|------|-------|
| story_teller | `story_teller_agent/agent.py` | `current_story` grows with each chapter |
| deep-search | `app/agent.py` | `collect_research_sources_callback` accumulates |

**In catalog:** ✅ story-teller

---

### 26. State Fan-out (One Key Read by Many)
**Description:** Single upstream state key consumed by multiple downstream agents.

| ADK Sample | File | Key | Readers |
|------------|------|-----|---------|
| marketing-agency | `marketing_agency/agent.py` | `domain_create_output` | website_create, marketing_create, logo_create |
| deep-search | `app/agent.py` | `research_plan` | section_planner, section_researcher |

**In catalog:** ✅ marketing-agency

---

### 27. Non-Adjacent State Reading
**Description:** Downstream agent reads state from two or more non-adjacent upstream agents.

| ADK Sample | File | Pattern |
|------------|------|---------|
| academic-research | `academic_research/agent.py` | newresearch reads seminal_paper + citing_papers |
| short-movie-agents | `app/agent.py` | VideoAgent reads screenplay + storyboard + story |
| podcast_transcript | `podcast_transcript_agent/agent.py` | transcript_writer reads podcast_topics + podcast_episode_plan |

**In catalog:** ✅ academic-research, podcast-transcript, short-movie-agent

---

### 28. Implicit State via Conversation History
**Description:** No explicit output_key — agents see prior output via shared conversation context.

| ADK Sample | File |
|------------|------|
| llm-auditor | `python/agents/llm-auditor/auditor/auditor.go` |
| llm-auditor | `python/agents/llm-auditor/critic/critic.py` |

**In catalog:** ✅ llm-auditor

---

### 29. Agent Isolation: `disallow_transfer_to_parent` / `disallow_transfer_to_peers`
**Description:** Controls agent communication boundaries.

| ADK Sample | File |
|------------|------|
| deep-search | `app/agent.py` — evaluator uses both flags |

**In catalog:** ❌ **MISSING**

---

### 30. Context Control: `include_contents="none"`
**Description:** Controls what context an agent receives — `none` means isolated execution with no conversation history.

| ADK Sample | File |
|------------|------|
| deep-search | `app/agent.py` — report composer uses this |

**In catalog:** ❌ **MISSING**

---

## Summary

### Patterns Covered in Catalog (14)
1. ✅ Sequential workflow
2. ✅ Sequential with output_key
3. ✅ Optional state injection `{var?}`
4. ✅ Parallel execution (fan-out/fan-in)
5. ✅ Nested: sequential inside parallel
6. ✅ Nested: parallel inside sequential
7. ✅ Nested: parallel inside loop inside sequential
8. ✅ Loop (fixed iterations)
9. ✅ Loop with allow_early_exit
10. ✅ Validation/checker agent pattern
11. ✅ Coordinator/orchestrator (AgentTool)
12. ✅ Conditional routing
13. ✅ State accumulation across iterations
14. ✅ State fan-out (one key, many readers)
15. ✅ Non-adjacent state reading
16. ✅ Implicit state via conversation history

### Patterns Missing from Catalog (8 key ones)
1. ❌ **Human-in-the-Loop (HITL)** — RequestInput + rerun_on_resume
2. ❌ **ParallelWorker** — Fan-out single worker across multiple inputs
3. ❌ **Function nodes** — Custom functions as workflow nodes yielding Events
4. ❌ **Callbacks** — before/after agent/model/tool lifecycle hooks
5. ❌ **Structured output schemas** — output_schema with Pydantic models
6. ❌ **Pipeline gated stopping** — Stop downstream stages on violation
7. ❌ **Agent isolation** — disallow_transfer_to_parent/peers, include_contents="none"
8. ❌ **MCP integration** — External tool server integration
9. ❌ **Multi-agent server** — Go MultiLoader pattern
10. ❌ **Timed/scheduled trigger** — Cloud Scheduler endpoint pattern

---

## Current Template Catalog (10 templates)

| Template | Slug | Featured | Core Patterns |
|----------|------|----------|---------------|
| City Time Report | city-time-report | No | Sequential, output_key |
| LLM Auditor | llm-auditor | Yes | Sequential, conversation history |
| Podcast Transcript | podcast-transcript | Yes | Sequential, multi-state passing |
| Parallel Broadcast | parallel-broadcast | Yes | Parallel, nested sequential |
| Story Teller | story-teller | Yes | Loop, parallel, nested, state accumulation |
| Blog Writer | blog-writer | Yes | Loop (×2), allow_early_exit, validation |
| Iterative Deep Research | iterative-deep-research | Yes | Loop, allow_early_exit, LLM-driven exit |
| Academic Research | academic-research | Yes | Sequential, non-adjacent reading |
| Marketing Agency | marketing-agency | Yes | Coordinator, state fan-out |
| Short Movie Agent | short-movie-agent | Yes | Sequential, multi-state reading |
| Customer Support Triage | customer-support-triage | Yes | Conditional routing |

---

## Recommendations for New Templates

To cover every ADK functionality, the following new templates should be created:

### 1. HITL Concierge (`hitl-concierge`)
**Source:** `workflows-HITL_concierge`
- Demonstrates: `RequestInput`, `rerun_on_resume`, loop with user feedback
- Schema: `ActivitiesList` output_schema
- Workflow: Sequential loop with user pause → process feedback → re-enter loop

### 2. Concurrent Research Writer (`concurrent-research-writer`)
**Source:** `workflow-concurrent_research_writer`
- Demonstrates: ParallelWorker, function nodes, conditional routing, rerun_on_resume
- Workflow: Research → ParallelWorker(X, LinkedIn, Reddit, Medium) → combine → blog → route by length

### 3. Morning Email Debrief (`morning-email-debrief`)
**Source:** `workflow-morning_email_debrief`
- Demonstrates: Function nodes, timed/scheduled trigger, event-based data passing
- Workflow: Function (retrieve emails) → format_news_agent

### 4. Invoice Processing (`invoice-processing`)
**Source:** `invoice_processing`
- Demonstrates: Pipeline gated stopping, multi-stage validation
- Workflow: Acting → Investigation (stops on MAJOR_VIOLATION) → ALF

### 5. Image Scoring (`image-scoring`)
**Source:** `image-scoring`
- Demonstrates: Loop with threshold-based exit, before_agent_callback, loop condition tool
- Workflow: Loop(Sequential(prompt_gen → image_gen → scoring) → checker)

### 6. Callback Showcase (`callback-showcase`)
**Source:** Aggregated from deep-search, travel-concierge, customer_service(TS), llm-auditor(Go)
- Demonstrates: All 4 callback types (before/after × agent/model/tool)
- Workflow: Sequential with agents using different lifecycle hooks

### 7. Deep Research with Isolation (`deep-research-isolated`)
**Source:** `deep-search`
- Demonstrates: disallow_transfer_to_parent/peers, include_contents="none", EscalationChecker, output_schema(Feedback), after_agent_callback
- Workflow: Sequential → Loop(Evaluator[isolated] → EscalationChecker → SearchExecutor) → Composer[no context]

### 8. MCP Tool Integration (`mcp-integration`)
**Source:** `software-bug-assistant`(Java), `time-series-forecasting`(Java), `policy-as-code`(Python)
- Demonstrates: MCP toolset loading, external server connection
- Note: This may be more of a configuration pattern than a YAML template
