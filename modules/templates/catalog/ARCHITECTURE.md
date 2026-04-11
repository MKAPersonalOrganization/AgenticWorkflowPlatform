# Catalog Agent Architecture Reference

Each entry in the catalog demonstrates a specific set of ADK-Go workflow features. This document describes the architecture of every example and maps it to the system capabilities it exercises.

---

## Feature Matrix

| Agent | Sequential | Parallel | Loop | allow\_early\_exit | State Injection | Nested Workflows |
|---|:---:|:---:|:---:|:---:|:---:|:---:|
| Sequential Workflow | ✓ | | | | ✓ | |
| LLM Auditor | ✓ | | | | | |
| Podcast Transcript | ✓ | | | | ✓ | |
| Parallel Workflow | ✓ | ✓ | | | ✓ | |
| Story Teller | ✓ | ✓ | ✓ | | ✓ | ✓ |
| Blog Writer | ✓ | | ✓ | ✓ | ✓ | |
| Iterative Deep Research | ✓ | | ✓ | ✓ | ✓ | |
| Academic Research | ✓ | | | | ✓ | |
| Marketing Agency | ✓ | | | | ✓ | |
| Short Movie Agent | ✓ | | | | ✓ | |
| Customer Support Triage | ✓ | | | | | |

---

## 1. Sequential Workflow

**Source:** ADK Python — `workflows-sequential`

**What it demonstrates:** The simplest possible sequential workflow. One agent writes a value to a named state key (`output_key`), and the next agent reads it back via `{variable}` injection in its system prompt.

```mermaid
flowchart TD
    User([User Input]) --> A[CityGeneratorAgent]
    A -->|"output_key: city"| S[(state: city)]
    S --> B["CityReportAgent\nreads {city}"]
    B --> Out([Single sentence report])
```

**Key state variables**

| Key | Written by | Read by |
|---|---|---|
| `city` | CityGeneratorAgent | CityReportAgent |

**ADK features used**
- `type: sequential` workflow node
- `output_key` on an agent node — stores the agent's final response text into session state
- `{variable}` injection — substitutes the state value into the next agent's system prompt at runtime

---

## 2. LLM Auditor

**Source:** ADK Python + Go — `llm-auditor`

**What it demonstrates:** A two-agent sequential pipeline where the second agent reads the first agent's output purely from **conversation history** (no explicit `output_key`). The critic's response — including all verdicts — is automatically part of the context the reviser receives.

```mermaid
flowchart TD
    User([Question + LLM Answer]) --> A["CriticAgent\nIdentifies every claim\nAssigns Accurate / Inaccurate /\nDisputed / Unsupported verdicts"]
    A -->|findings appended to conversation| B["ReviserAgent\nMinimally corrects inaccuracies\nPreserves original style"]
    B --> Out([Verified and corrected answer])
```

**ADK features used**
- `type: sequential` workflow node
- Implicit state passing via **conversation history** — no `output_key` needed; each agent sees the full prior exchange
- Sequential agents share one continuous message thread

---

## 3. Podcast Transcript Generator

**Source:** ADK Python — `podcast_transcript_agent`

**What it demonstrates:** A three-stage sequential pipeline with explicit `output_key` handoffs. Each agent reads the previous agent's output via named state injection, building up structured data across three steps.

```mermaid
flowchart TD
    User([Content or Research Paper]) --> A["PodcastTopicsAgent\nExtracts main topic +\n5 subtopics as JSON"]
    A -->|"output_key: podcast_topics"| B["PodcastEpisodePlannerAgent\nCreates 5-segment episode outline\nreads {podcast_topics}"]
    B -->|"output_key: podcast_episode_plan"| C["PodcastTranscriptWriterAgent\nWrites full Host / Expert script\nreads {podcast_episode_plan}\nand {podcast_topics}"]
    C --> Out([Complete podcast script])
```

**Key state variables**

| Key | Written by | Read by |
|---|---|---|
| `podcast_topics` | PodcastTopicsAgent | PodcastEpisodePlannerAgent, PodcastTranscriptWriterAgent |
| `podcast_episode_plan` | PodcastEpisodePlannerAgent | PodcastTranscriptWriterAgent |

**ADK features used**
- `type: sequential` workflow node
- Multiple `output_key` declarations — each agent publishes a named result
- `{variable}` injection referencing keys from **earlier, non-adjacent** agents (transcript reads both `podcast_topics` and `podcast_episode_plan`)

---

## 4. Parallel Workflow

**Source:** ADK Python — `parallel_task_decomposition_execution`

**What it demonstrates:** A sequential root wrapping a `type: parallel` node whose three child agents run concurrently — each independently drafting content for Email, Slack, and Calendar from the same upstream state — and a summary agent collects the results. This is the purest fan-out/fan-in pattern with no nested sequential branches.

```mermaid
flowchart TD
    User([Announcement Topic]) --> E["MessageEnhancerAgent\nResearches and enriches the topic"]
    E -->|"output_key: enhanced_message"| PAR

    subgraph PAR [Parallel — all three agents run concurrently]
        direction TB
        D1["EmailDrafterAgent\nreads {enhanced_message}\noutput_key: drafted_email"]
        D2["SlackDrafterAgent\nreads {enhanced_message}\noutput_key: drafted_slack_message"]
        D3["EventDetailsExtractorAgent\nreads {enhanced_message}\noutput_key: event_details"]
    end

    D1 --> SUM[SummaryAgent\nreads all three results]
    D2 --> SUM
    D3 --> SUM
    SUM --> Out([Broadcast summary])
```

**Key state variables**

| Key | Written by | Read by |
|---|---|---|
| `enhanced_message` | MessageEnhancerAgent | All three parallel drafter/extractor agents |
| `drafted_email` | EmailDrafterAgent | SummaryAgent |
| `drafted_slack_message` | SlackDrafterAgent | SummaryAgent |
| `event_details` | EventDetailsExtractorAgent | SummaryAgent |

**ADK features used**
- `type: parallel` node — all child agents start simultaneously and share the same session state
- Each parallel child is a single `type: agent` node (no nesting)
- `{variable?}` optional injection in SummaryAgent — results may not exist if a branch fails

---

## 5. Story Teller

**Source:** ADK Python — `story_teller`

**What it demonstrates:** The most deeply nested workflow in the catalog. A sequential root contains a loop whose body is itself a sequential containing a parallel. Shows how state written by the outermost agents (`enhanced_prompt`) is readable deep inside a nested loop-parallel structure, and how `{variable?}` handles the first-iteration case before state is initialised.

```mermaid
flowchart TD
    User([Story Idea]) --> PE["PromptEnhancerAgent\nExpands idea into rich premise\noutput_key: enhanced_prompt\ninit current_story = Chapter 1"]

    PE --> LOOP

    subgraph LOOP [Loop — 3 iterations, fixed]
        direction TB
        subgraph SEQ [Sequential per iteration]
            direction TB
            subgraph PAR [Parallel — both writers run concurrently]
                CW["CreativeStoryTellerAgent\nreads {enhanced_prompt?} {current_story?}\noutput_key: creative_chapter_candidate"]
                FW["FocusedStoryTellerAgent\nreads {enhanced_prompt?} {current_story?}\noutput_key: focused_chapter_candidate"]
            end
            PAR --> CR["CritiqueAgent\npicks best chapter, appends to story\noutput_key: current_story"]
        end
    end

    LOOP -->|after 3 chapters| ED["EditorAgent\npolishes final draft\nreads {current_story}\noutput_key: final_story"]
    ED --> Out([Polished multi-chapter story])
```

**Key state variables**

| Key | Written by | Read by |
|---|---|---|
| `enhanced_prompt` | PromptEnhancerAgent | CreativeWriter, FocusedWriter, CritiqueAgent |
| `current_story` | PromptEnhancerAgent (init) then CritiqueAgent (each iteration) | CreativeWriter, FocusedWriter, CritiqueAgent, EditorAgent |
| `creative_chapter_candidate` | CreativeStoryTellerAgent | CritiqueAgent |
| `focused_chapter_candidate` | FocusedStoryTellerAgent | CritiqueAgent |
| `final_story` | EditorAgent | — |

**ADK features used**
- `type: loop` with fixed `max_iterations: 3` and `allow_early_exit: false`
- `type: parallel` nested inside `type: sequential` nested inside `type: loop`
- `{variable?}` optional injection — iteration 1 runs before `current_story` is populated by `CritiqueAgent`; the `?` suffix returns an empty string instead of erroring
- State **accumulation** across loop iterations — `current_story` grows with each chapter

---

## 6. Blog Writer

**Source:** ADK Python — `blog-writer`

**What it demonstrates:** Two back-to-back `allow_early_exit` loops, each containing a worker agent followed by a validation agent. The validator checks whether the expected state key was populated and calls `exit_loop` if it was, skipping remaining iterations. Shows the "retry-until-success" pattern without hard-coding a fixed iteration count.

```mermaid
flowchart TD
    User([Blog Topic]) --> PL

    subgraph PL [Planning Loop — max 3, allow_early_exit]
        BP["BlogPlannerAgent\nWrites outline\noutput_key: blog_outline"] --> OC["OutlineValidationChecker\nchecks blog_outline key\ncalls exit_loop if present"]
        OC -->|blog_outline missing — retry| BP
    end

    PL -->|blog_outline exists — exit_loop called| WL

    subgraph WL [Writing Loop — max 3, allow_early_exit]
        BW["BlogWriterAgent\nWrites full article\noutput_key: blog_post"] --> PC["BlogPostValidationChecker\nchecks blog_post key\ncalls exit_loop if present"]
        PC -->|blog_post missing — retry| BW
    end

    WL -->|"blog_post exists — exit_loop called"| BE["BlogEditorAgent\nPolishes the article\nreads {blog_post}"]
    BE --> SM["SocialMediaWriterAgent\nTwitter + LinkedIn posts\nreads {blog_post}"]
    SM --> Out([Blog post + social media posts])
```

**Key state variables**

| Key | Written by | Read by |
|---|---|---|
| `blog_outline` | BlogPlannerAgent | BlogWriterAgent, OutlineValidationChecker |
| `blog_post` | BlogWriterAgent | BlogEditorAgent, SocialMediaWriterAgent, BlogPostValidationChecker |
| `social_media_posts` | SocialMediaWriterAgent | — |

**ADK features used**
- `allow_early_exit: true` — the last agent in the loop body receives the `exit_loop` tool
- Validation agent pattern — a lightweight agent that inspects state and calls `exit_loop` when ready
- Two independent loops in the same sequential workflow

---

## 7. Iterative Deep Research

**Source:** ADK samples — `iterative-deep-research`

**What it demonstrates:** A single `allow_early_exit` loop where the last agent in the loop body grades research quality. If it passes, it calls `exit_loop` and the workflow advances to the report composer. If it fails, it provides gap-filling feedback and the loop re-runs. Shows iterative refinement driven by LLM judgement rather than a fixed counter.

```mermaid
flowchart TD
    User([Research Topic]) --> PG["PlanGenerator\n5 action-oriented research goals\noutput_key: research_plan"]
    PG --> SP["SectionPlanner\n4-6 section report outline\noutput_key: research_outline"]
    SP --> SR["SectionResearcher\nInitial comprehensive pass\noutput_key: research_findings"]

    SR --> LOOP

    subgraph LOOP [Refinement Loop — max 3, allow_early_exit]
        ES["EnhancedSearchExecutor\nFills identified gaps\nreads {research_evaluation?}\noutput_key: research_findings"] --> RE["ResearchEvaluator\nGrades: pass or fail\npass → calls exit_loop\nfail → provides gap list\noutput_key: research_evaluation"]
        RE -->|fail — loop continues| ES
    end

    LOOP -->|pass — exit_loop called| RC["ReportComposer\nWrites polished report\nreads {research_findings}\noutput_key: final_report"]
    RC --> Out([Final research report])
```

**Key state variables**

| Key | Written by | Read by |
|---|---|---|
| `research_plan` | PlanGenerator | SectionPlanner, SectionResearcher |
| `research_outline` | SectionPlanner | SectionResearcher |
| `research_findings` | SectionResearcher, EnhancedSearchExecutor | EnhancedSearchExecutor, ResearchEvaluator, ReportComposer |
| `research_evaluation` | ResearchEvaluator | EnhancedSearchExecutor |
| `final_report` | ReportComposer | — |

**ADK features used**
- `allow_early_exit: true` — `ResearchEvaluator` (last agent in loop body) receives the `exit_loop` tool
- **LLM-driven loop termination** — the exit decision is based on qualitative assessment, not a fixed counter
- State **overwrite** across iterations — `research_findings` is replaced each iteration with the improved version
- `{research_evaluation?}` — optional on the first iteration (no prior evaluation exists yet)

---

## 8. Academic Research Assistant

**Source:** ADK Python — `academic-research`

**What it demonstrates:** A coordinator-style sequential pipeline where each agent adds a new layer of analysis on top of the previous agent's output. The final agent reads two upstream state keys (`seminal_paper` and `recent_citing_papers`) that were produced by non-adjacent agents.

```mermaid
flowchart TD
    User([Seminal Paper]) --> CO["AcademicCoordinatorAgent\nExtracts title, authors, abstract,\nsummary, innovations, references\noutput_key: seminal_paper"]
    CO --> WS["AcademicWebSearchAgent\nFinds citing papers from\ncurrent and previous year\nreads {seminal_paper?}\noutput_key: recent_citing_papers"]
    WS --> NR["AcademicNewResearchAgent\nSynthesises 10+ future research directions\nreads {seminal_paper?}\nand {recent_citing_papers?}"]
    NR --> Out([Future research directions])
```

**Key state variables**

| Key | Written by | Read by |
|---|---|---|
| `seminal_paper` | AcademicCoordinatorAgent | AcademicWebSearchAgent, AcademicNewResearchAgent |
| `recent_citing_papers` | AcademicWebSearchAgent | AcademicNewResearchAgent |

**ADK features used**
- `type: sequential` with three agents
- A downstream agent reading state from **two prior agents** simultaneously
- `{variable?}` optional injection — coordinator output may be partial if the user has not yet provided the paper

---

## 9. Marketing Agency

**Source:** ADK Python — `marketing-agency`

**What it demonstrates:** A five-agent sequential pipeline where each specialist builds on the brand identity established in the first step. The `domain_create_output` key acts as the shared brand anchor read by all downstream agents. Shows how a single upstream state key can be consumed by multiple non-adjacent agents in a long chain.

```mermaid
flowchart TD
    User([Brand Keywords]) --> CO["MarketingCoordinatorAgent\nCollects brand concept and keywords"]
    CO --> DC["DomainCreateAgent\n10 available domain names\noutput_key: domain_create_output"]
    DC --> WC["WebsiteCreateAgent\nFull HTML / CSS / JS site\nreads {domain_create_output?}\noutput_key: website_create_output"]
    WC --> MC["MarketingCreateAgent\nCampaign strategy report\nreads {domain_create_output?}\noutput_key: marketing_create_output"]
    MC --> LC["LogoCreateAgent\nLogo design concept\nreads {domain_create_output?}\noutput_key: logo_create_output"]
    LC --> Out([Complete digital brand identity])
```

**Key state variables**

| Key | Written by | Read by |
|---|---|---|
| `domain_create_output` | DomainCreateAgent | WebsiteCreateAgent, MarketingCreateAgent, LogoCreateAgent |
| `website_create_output` | WebsiteCreateAgent | — |
| `marketing_create_output` | MarketingCreateAgent | — |
| `logo_create_output` | LogoCreateAgent | — |

**ADK features used**
- `type: sequential` with five agents
- **Fan-out state reading** — one state key (`domain_create_output`) is read by three separate downstream agents
- `{variable?}` optional injection — graceful handling if domain step produced no output

---

## 10. Short Movie Agent

**Source:** ADK Python — `short-movie-agents`

**What it demonstrates:** A linear five-agent creative pipeline where each agent transforms the previous output into a new representation — from concept to story to screenplay to storyboard to video production notes. Each transformation step reads the accumulated creative assets from state.

```mermaid
flowchart TD
    User([Story Concept]) --> DIR["DirectorAgent\nGathers concept and guides the pipeline"]
    DIR --> ST["StoryAgent\nScout Leader campfire narrative\nwith character descriptions\noutput_key: story"]
    ST --> SC["ScreenplayAgent\nStandard screenplay format\nslugs + action lines + dialogue\nreads {story}\noutput_key: screenplay"]
    SC --> SB["StoryboardAgent\nScene-by-scene visual panel descriptions\nreads {screenplay} and {story?}\noutput_key: storyboard"]
    SB --> VID["VideoAgent\nShot-by-shot production notes\nreads {screenplay} {storyboard?} {story?}"]
    VID --> Out([Movie production package])
```

**Key state variables**

| Key | Written by | Read by |
|---|---|---|
| `story` | StoryAgent | ScreenplayAgent, StoryboardAgent, VideoAgent |
| `screenplay` | ScreenplayAgent | StoryboardAgent, VideoAgent |
| `storyboard` | StoryboardAgent | VideoAgent |

**ADK features used**
- `type: sequential` with five agents
- **Accumulating context** — VideoAgent reads three separate upstream state keys simultaneously
- `{variable?}` optional injection — later agents degrade gracefully if an earlier step produced no output
- Sequential creative transformation — each agent produces a distinct artifact format from the same underlying narrative

---

## Workflow Type Reference

```mermaid
flowchart LR
    A["type: agent\nLeaf node — runs one sub-agent\nSupports output_key"]
    S["type: sequential\nRuns children in order\nChildren share session state"]
    P["type: parallel\nRuns all children concurrently\nChildren share session state"]
    L["type: loop\nRepeats its child workflow\nmax_iterations + allow_early_exit"]
```

### State injection syntax

| Syntax | Behaviour |
|---|---|
| `{key}` | Required — errors if key is absent from session state |
| `{key?}` | Optional — substitutes empty string if key is absent |

### Loop exit modes

| Setting | Behaviour |
|---|---|
| `allow_early_exit: false` | Loop always runs exactly `max_iterations` times |
| `allow_early_exit: true` | Last agent in the loop body receives the `exit_loop` tool; calling it stops the loop before `max_iterations` is reached |
