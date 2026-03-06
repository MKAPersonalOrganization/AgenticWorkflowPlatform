# Developer Onboarding Guide

Welcome to **School with AI** — an event-sourced, multi-agent AI chat platform. This guide will get you productive quickly and help you understand the key architectural decisions that shape every change.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Getting Started](#getting-started)
- [Project Structure](#project-structure)
- [Architecture Overview](#architecture-overview)
- [Key Concepts](#key-concepts)
- [Architecture Decision Records (ADRs)](#architecture-decision-records-adrs)
- [Development Workflow](#development-workflow)
- [Common Tasks](#common-tasks)
- [Coding Conventions](#coding-conventions)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

| Tool | Version | Purpose |
| :--- | :--- | :--- |
| **Go** | 1.24+ | Backend language |
| **Task** | 3.x | Task runner ([taskfile.dev](https://taskfile.dev)) |
| **Docker & Docker Compose** | Latest | Infrastructure services |
| **templ** | Latest | Go HTML templating (`go install github.com/a-h/templ/cmd/templ@latest`) |
| **templui** | Latest | Component library CLI (`go install github.com/templui/templui/cmd/templui@latest`) |
| **Tailwind CSS** | v4 | Utility-first CSS (via `npx tailwindcss@latest` or standalone binary) |
| **Node.js** | 18+ | For Tailwind CLI and some tooling |
| **golangci-lint** | Latest | Linter |
| **gofumpt** | Latest | Stricter gofmt |

Optional:
- **Ollama** — For local LLM inference (runs on host, proxied via `ollama-socat` container).
- **Rust toolchain** — Only needed if modifying the EdgeQuake RAG engine.

---

## Getting Started

### 1. Clone and configure

```bash
git clone <repo-url> && cd SchoolWithAI
cp .env.example .env   # Fill in required values (see below)
```

**Required `.env` variables:**

| Variable | Description |
| :--- | :--- |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Google OAuth credentials |
| `SESSION_SECRET` | Random 32+ char string for cookie sessions |
| `SCHOOL_DB_PASSWORD` | PostgreSQL password |
| `REDIS_PASSWORD` | Redis auth password |
| `MINIO_ROOT_USER` / `MINIO_ROOT_PASSWORD` | MinIO admin credentials |
| `SCHOOL_ARTIFACTS_S_3_ACCESS_KEY` / `SECRET_KEY` | MinIO access keys (can match root user) |

Optional: `OPENAI_API_KEY`, `SCHOOL_STRIPE_*`, `SCHOOL_EDGEQUAKE_*`.

### 2. Start infrastructure

```bash
task up          # Starts all containers (Redis, PostgreSQL, MinIO, EdgeQuake, dev UIs)
```

This uses `docker-compose.yml` + `docker-compose.override.yml` (dev-only services: Asynqmon on `:8080`, RedisInsight on `:5540`, EdgeQuake Web UI on `:3002`).

### 3. Start the application

```bash
task dev         # Starts Go server + Tailwind watcher + Templ hot reload
```

The app starts at **https://localhost:8090** (self-signed TLS). The debug server runs at **http://localhost:4015**.

### 4. Verify everything works

- Open https://localhost:8090 — you should see the login page.
- Open http://localhost:4015/debug/vars — expvar metrics.
- Open http://localhost:8080 — Asynqmon task queue dashboard.

---

## Project Structure

```
SchoolWithAI/
├── main.go                    # Application entry point — wires everything together
├── domain/                    # Domain model — aggregates, events, repositories (interfaces)
├── services/                  # Service layer — business logic orchestration
│   ├── agentorchestrator/     #   Core: event classification, persistence, broadcast
│   ├── taskmanagement/        #   Asynq job queue + AG-UI event conversion
│   ├── threadmgmt/            #   Thread lifecycle (event store adapter)
│   ├── toolregistry/          #   Plugin registration + DB persistence
│   ├── plugindiscovery/       #   MCP connector lifecycle + OAuth
│   ├── billing/               #   Event-sourced Stripe payments
│   ├── edgequake/             #   REST facade to RAG API
│   ├── artifact/              #   S3/MinIO file storage
│   ├── memory/                #   Long-term memory (S3)
│   ├── rag/                   #   Typed Go clients for EdgeQuake API (23 sub-packages)
│   └── ...                    #   AI settings, permissions, resources, profiles, etc.
├── handlers/                  # HTTP handler layer (27 packages)
│   ├── taskmgmt/              #   Command side: send message, agent runs, SSE subscribe
│   ├── agui/                  #   Query side: thread state, message history
│   ├── knowledge/             #   Knowledge base UI + API
│   ├── mcp/                   #   MCP connector management
│   ├── billing/               #   Stripe integration endpoints
│   └── ...                    #   Auth, health, agents, resources, profile, preferences
├── foundation/                # Framework-level abstractions (shared kernel)
│   ├── agent/                 #   Framework-agnostic interfaces (Agent, Runner, Tool, Middleware)
│   ├── agentsdk/              #   Agent SDK: engine, LLM adapters, tools, I/O
│   │   ├── core/              #     Core abstractions (thread, events, tools, config)
│   │   ├── engine/            #     LLM iteration loop + state machine
│   │   ├── internal/llm/      #     LLM provider adapters (OpenAI, Gemini, Ollama)
│   │   ├── io/                #     Human-in-the-loop I/O handlers
│   │   ├── adapters/custom/   #     Framework adapter (core.Event → domain.Event translation)
│   │   └── tools/             #     SDK tool utilities
│   ├── event/                 #   Event router (persistence + transient dispatch)
│   ├── middleware/             #   HTTP middleware (rate limiting, security, logging, etc.)
│   ├── metrics/               #   Application metrics (goroutines, requests, errors, panics)
│   ├── otel/                  #   OpenTelemetry setup
│   ├── errs/                  #   Standardized error types
│   ├── i18n/                  #   Internationalization
│   ├── debug/                 #   Debug server (expvar, pprof, state machine visualization)
│   └── logger/                #   Structured logging
├── infra/                     # Infrastructure adapters
│   ├── eventstore/postgres/   #   PostgreSQL event store implementation
│   ├── projection/            #   Projection updater + Redis broadcaster + 4 projections
│   ├── projections/           #   UI projection service (read model queries)
│   ├── sqldb/                 #   SQL repositories (11 repos) + migrations
│   ├── googleauth/            #   Google OAuth client + session store
│   └── rag/                   #   Legacy RAG infra (mostly unused — delegated to EdgeQuake)
├── plugins/                   # Tool plugins (9 packages, each with PluginProvider)
│   ├── calculator/            #   Math expression + equation solving
│   ├── codebase/              #   Code analysis tools
│   ├── cognitive/             #   "Think" tool for step-by-step reasoning
│   ├── document/              #   Document processing
│   ├── elicitation/           #   Human-in-the-loop input requests
│   ├── knowledgebase/         #   Knowledge base query (EdgeQuake RAG)
│   ├── library/               #   Library / reference tools
│   ├── memory/                #   Long-term memory read/write
│   └── tts/                   #   Text-to-speech synthesis
├── components/                # Templ UI components (templui-based)
├── assets/                    # Static assets (CSS, JS, images, locales)
├── edgequake/                 # EdgeQuake RAG framework (Rust — separate codebase)
├── arch/adr/                  # Architecture Decision Records
├── tls/                       # TLS certificates
├── books/                     # Converted EPUB library
└── data/                      # Static data files
```

---

## Architecture Overview

The system follows a layered architecture with strict dependency rules:

```
┌─────────────────────────────────────────────────┐
│                 handlers/                         │  ← HTTP interface
├─────────────────────────────────────────────────┤
│                 services/                         │  ← Business logic orchestration
├─────────────────────────────────────────────────┤
│                  domain/                          │  ← Domain model (aggregates, events)
├─────────────────────────────────────────────────┤
│         foundation/    plugins/                   │  ← Shared kernel + tool plugins
├─────────────────────────────────────────────────┤
│                   infra/                          │  ← Infrastructure adapters
└─────────────────────────────────────────────────┘
```

**Dependency rules:**
- `handlers/` → `services/`, `domain/`, `foundation/`
- `services/` → `domain/`, `foundation/`
- `plugins/` → `foundation/`, `domain/` (NEVER `services/` — see ADR-008)
- `infra/` → `domain/`, `foundation/`
- `domain/` → `foundation/` only (pure domain model)
- `foundation/` → nothing internal (shared kernel, external libs only)

### Request Flow (Happy Path)

```
Browser → Chi Router → Middleware Stack → Auth → Handler
                                                    │
                                         ┌──────────┴──────────┐
                                         │ Command Side (POST)  │ Query Side (GET)
                                         │                      │
                                    Task Manager           AGUI Projection
                                    (enqueue to Redis)     (fast read from Postgres)
                                         │                      │
                                    Asynq Worker           Redis Broadcaster
                                         │                 (live SSE subscription)
                                    Agent Orchestrator          │
                                         │                      │
                                    ┌────┴────┐                 │
                                    │ Classify │                 │
                                    │  Event   │                 │
                                    └────┬────┘                 │
                              ┌──────────┼──────────┐           │
                         Durable?     Both      Transient?      │
                              │                      │           │
                     Event Store              Redis Pub/Sub ─────┘
                     + Redis Pub/Sub     (broadcast-only)
                              │
                     Projection Updater
                     (update read models)
```

---

## Key Concepts

### Event Sourcing

All state changes are persisted as an ordered sequence of **domain events** in a PostgreSQL-backed event store. The system has two aggregates:

- **`ThreadAggregate`** — Conversation threads (messages, tool calls, agent interactions).
- **`PaymentAccountAggregate`** — Billing state (Stripe subscription lifecycle, ledger entries).

Events are the **single source of truth**. Read models (projections) are derived from events and can be rebuilt at any time.

### Event Classification (ADR-003, ADR-007)

Every event in the system is explicitly classified:

| Classification | Persisted? | Broadcast? | Examples |
| :--- | :---: | :---: | :--- |
| **Durable** | ✅ | ✅ | `UserMessageAdded`, `AssistantMessageAdded`, `ToolResultAdded`, `ThreadCreated` |
| **Transient** | ❌ | ✅ | `RunStarted`, `RunFinished`, `AssistantContentStreamed`, `ToolContentStreamed` |

Durable events are broadcast **synchronously** alongside persistence (outbox bypass — ADR-007) to eliminate latency gaps.

### CQRS (Command Query Responsibility Segregation)

- **Command side**: `handlers/taskmgmt` → `services/taskmanagement` (Asynq) → `services/agentorchestrator` → Event Store.
- **Query side**: `handlers/agui` → `infra/projections` → PostgreSQL read models.

Four materialized projections in PostgreSQL:
1. **AGUI Projection** — Thread state and messages for the UI.
2. **LLM Context Projection** — Conversation history optimized for LLM prompts.
3. **Payment Projection** — Account status for billing queries.
4. **Statistics Projection** — Tool usage and analytics timeseries.

### Agent SDK Boundary (ADR-001, ADR-002, ADR-005)

The `foundation/agent` package defines **framework-agnostic interfaces** (`Agent`, `Runner`, `ExecutableTool`, `AgentProvider`). The application never imports framework-specific packages directly.

```
Application (services/) ──uses──▶ foundation/agent (interfaces)
                                        ▲
                                        │ implements
                         foundation/agentsdk/adapters/custom/ (RunnerAdapter)
                                        │
                                        ▼
                         foundation/agentsdk/engine/ (LLM loop, tools)
                                        │
                                        ▼
                         foundation/agentsdk/internal/llm/ (OpenAI, Gemini, Ollama)
```

The adapter translates `core.Event` (SDK-internal) → `domain.Event` (application-level). This means you can swap the entire agent framework without changing application code.

### Plugin System (ADR-008)

Each tool plugin is a self-contained package under `plugins/` that implements the `PluginProvider` interface:

```go
type PluginProvider interface {
    ID() string
    Icon() string
    DefaultSets() []ToolSet
    DefaultTools() []Tool
    CreateExecutable(tool Tool, deps FactoryDeps) (agent.ExecutableTool, error)
    GetUITemplate(ctx ToolUIContext) templ.Component
    UIHandledTools() []string
    UIHandledElicitations() []string
}
```

Plugins provide both backend execution logic (`backend.go`) and frontend Templ components (`ui.templ`). The tool registry dynamically injects plugin configurations into the browser via `window.SPECIAL_TOOLS`.

**Key rule**: Plugins must NOT import from `services/`. They depend only on `foundation/` and `domain/`.

### MCP Connectors (ADR-010)

Users can register remote MCP (Model Context Protocol) servers through the UI. The `services/plugindiscovery` package manages the full lifecycle:

1. **Create** connector → persisted in DB.
2. **Connect** → probe server, discover tools, register as ephemeral toolset.
3. **Disconnect** → unregister tools, mark as disconnected.
4. **Remove** → clean up DB and revoke OAuth tokens.

Features: OAuth 2.1 + PKCE, per-server-URL circuit breakers, bounded concurrency (100 sessions), 20 connectors per user, SSRF protection.

### Unified Middleware (ADR-006)

A single middleware pipeline intercepts three operation kinds:

| Kind | Scope | Example Concerns |
| :--- | :--- | :--- |
| **Agent** | Entire agent execution | Retry, timeout, tracing, circuit breaker |
| **Chat** | Single LLM request-response | Token counting, rate limiting, caching |
| **Function** | Single tool call | Argument validation, permission checks |

```go
type Middleware func(Handler) Handler

type Handler interface {
    Handle(ctx *Context) error
}
```

### Artifacts (ADR-011)

Files are uploaded to S3/MinIO **immediately on attach** (pre-upload pattern), before the message is sent. Only short URL references (~100 bytes) flow through the event and message pipeline. Audio/video playback uses HTTP Range requests (206 Partial Content).

---

## Architecture Decision Records (ADRs)

All architectural decisions are documented in `arch/adr/`. **Read these before making structural changes.**

| ADR | Title | Key Takeaway |
| :--- | :--- | :--- |
| [001](arch/adr/001-agent-sdk-architectural-boundary.md) | Agent SDK Architectural Boundary | `foundation/agent` is the sole access point to any agent framework. |
| [002](arch/adr/002-run-lifecycle-events-service-orchestration.md) | Run Lifecycle Events | Run lifecycle (start/finish/error) is a service-layer concern, never adapter. |
| [003](arch/adr/003-event-classification-ephemeral-vs-persistent.md) | Event Classification | Every event is either durable or transient — never both. |
| [004](arch/adr/004-standardization-of-pointer-based-domain-events.md) | Pointer-based Domain Events | All domain events must be pointers throughout the entire lifecycle. |
| [005](arch/adr/005-framework-adapter-llm-client-ownership.md) | Framework Adapter LLM Client Ownership | LLM client creation is an adapter implementation detail. |
| [006](arch/adr/006-unified-middleware-interface.md) | Unified Middleware Interface | Single middleware pipeline for Agent, Chat, and Function operations. |
| [007](arch/adr/007-durable-events-subset-of-streamed-events.md) | Durable Events Synchronous Broadcast | Durable events bypass the outbox — broadcast immediately alongside persistence. |
| [008](arch/adr/008-plugin-system-for-tools.md) | Plugin System for Tools | Self-contained plugins under `plugins/` with `PluginProvider` interface. |
| [009](arch/adr/009-streaming-tool-implementation-guide.md) | Streaming Tool Implementation | Two modes: automatic argument streaming + opt-in execution streaming. |
| [010](arch/adr/010-mcp-connectors-system.md) | MCP Connectors System | User-registered remote MCP servers with OAuth, circuit breakers, ephemeral toolsets. |
| [011](arch/adr/011-pre-upload-artifact-streaming.md) | Pre-Upload Artifact Storage | Files uploaded to S3 on attach; only URL references in the event pipeline. |

---

## Development Workflow

### Task commands

| Command | Description |
| :--- | :--- |
| `task dev` | Start dev server with hot reload (Go + Templ + Tailwind watcher). |
| `task up` | Start all Docker Compose services (infrastructure + dev tools). |
| `task up:prod` | Production-parity mode (containerized app, no dev UIs). |
| `task down` | Stop and remove all containers and volumes. |
| `task logs` | Tail Docker Compose logs. |
| `task ci` | Full CI pipeline (format, lint, test, arch tests, build). |
| `task kill-ports` | Kill processes on ports 8090, 4015, 7331. |
| `task statistics` | Generate git repository statistics graph. |

### Making changes

1. **Domain events**: Define in `domain/events.go`. Use pointer receivers. Add to the event deserializer in `infra/eventstore/postgres/`.
2. **Aggregate logic**: Update `Apply()` method in the aggregate and the decider if applicable.
3. **Projections**: Update the relevant projection handler in `infra/projection/` to project new events.
4. **Services**: Add business logic in `services/`. Services orchestrate domain operations and infra calls.
5. **Handlers**: Expose via HTTP in `handlers/`. Handlers are thin — they parse requests, call services, and render responses.
6. **Templ components**: Add/update `.templ` files in `components/`. Run `templ generate` (or let `task dev` handle it).

### Adding a new tool plugin

1. Create a new package under `plugins/` (e.g., `plugins/mytool/`).
2. Implement the `PluginProvider` interface (see existing plugins for examples).
3. Create `backend.go` for execution logic and `ui.templ` for the frontend component.
4. Register the provider in `main.go` where other plugins are registered.
5. The tool registry will automatically persist the tool definitions to the DB.

### Adding a new handler package

1. Create a new package under `handlers/`.
2. Define a `Routes(...)` function that accepts a `*chi.Mux` and mounts your routes.
3. Wire it in `main.go` where other handlers are mounted.
4. Use the auth middleware for protected routes.

---

## Common Tasks

### Run tests

```bash
go test -race -v ./...           # All tests
go test -v ./arch/...            # Architecture boundary tests only
go test -v ./domain/...          # Domain model tests
go test -v ./services/...        # Service layer tests
```

### Database migrations

Migrations are in `infra/sqldb/migrate/` and run automatically on application startup. To add a new migration, create a new numbered SQL file following the existing convention.

### Debug the Agent Orchestrator

The debug server at `http://localhost:4015` exposes:
- `/debug/vars` — expvar metrics.
- `/debug/pprof/` — Go profiling.
- `/api/debug/state-machines/engine` — Engine state machine (SVG).
- `/api/debug/state-machines/tool` — Tool lifecycle state machine (SVG).

### Monitor the task queue

Open Asynqmon at `http://localhost:8080` (dev mode only) to inspect queued, active, and failed tasks.

### Monitor Redis

Open RedisInsight at `http://localhost:5540` (dev mode only) to inspect Pub/Sub channels, keys, and streams.

### View the knowledge graph

Open the EdgeQuake Web UI at `http://localhost:3002` (dev mode only) to explore the knowledge graph, uploaded documents, and pipeline status.

---

## Coding Conventions

### Event naming

- Past tense: `UserMessageAdded`, `ThreadCreated`, `SubscriptionCancelled`.
- Events describe **what happened**, not **what should happen**.

### Pointer-based events (ADR-004)

All domain events must be pointers. In type switches:

```go
// ✅ Correct
case *domain.UserMessageAdded:

// ❌ Wrong — will silently fail
case domain.UserMessageAdded:
```

### Error handling

Use `foundation/errs` for typed errors. Wrap errors with context.

### Package boundaries

- `plugins/` must NOT import `services/`.
- `services/` must NOT import `handlers/`.
- `foundation/` must NOT import any internal package.
- `domain/` is the innermost layer — depends only on `foundation/`.

These rules are enforced by architecture tests in `arch/`.

---

## Troubleshooting

| Problem | Solution |
| :--- | :--- |
| Port already in use | `task kill-ports` |
| TLS certificate errors | The app uses a self-signed cert. Accept the browser warning or add the cert to your trust store. |
| MinIO connection refused | Ensure `task up` has completed and MinIO is healthy: `docker compose ps` |
| Agent not responding | Check Asynqmon (`localhost:8080`) for failed tasks. Check `task logs` for orchestrator errors. |
| Projections stale | The projection updater runs as a background goroutine. Check logs for errors. In extreme cases, projections can be rebuilt from the event store. |
| EdgeQuake API errors | EdgeQuake has a 120s timeout for LLM cold starts. Check `docker compose logs edgequake-api`. |
| OAuth callback fails | Verify `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, and that `https://localhost:8090/auth/google/callback` is in your Google Cloud Console redirect URIs. |
