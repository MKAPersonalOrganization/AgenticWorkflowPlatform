# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# First-time setup (installs templ and templui CLIs)
task setup-ubuntu

# Start all infrastructure (Redis, Postgres, Kratos, KurrentDB, Langfuse)
task up

# Development with hot reload (runs Tailwind watcher + Templ generator with proxy)
task dev
# Frontend: http://localhost:7331 (proxy), Backend: http://localhost:8090 (direct)

# Stop all infrastructure and remove volumes
task down

# Generate templ files (required before build/test)
templ generate

# Build
go build -v ./...

# Run all CI checks (format, lint, test, arch tests, build, cleanup)
task ci

# Run tests (requires gnuplot for plot tool tests)
go test -v -race ./...

# Run tests for specific package
go test -v ./services/taskmanagement
go test -v ./services/agentorchestrator
go test -v ./foundation/agentsdk/core

# Lint
golangci-lint run ./... --timeout=5m

# View Kratos logs
task logs

# Kill processes on app ports (8090, 4015, 7331)
task kill-ports
```

## Architecture Overview

This is an AI-powered educational chat application implementing a multi-agent system with event sourcing and streaming responses via AG-UI protocol.

### Core Architecture: Event Sourcing + CQRS + Multi-Agent Collaboration

**Directory Structure**:
- `services/`: Business logic services (agentorchestrator, taskmanagement, threadmgmt, auth, chatread)
- `foundation/agentsdk/`: Reusable agent SDK with core abstractions (EventStore, Thread, Tool, Config)
- `infra/`: Infrastructure adapters (projection, eventstore, kratos, sqldb, rag)
- `components/`: Templ server-side components (UI templates)
- `assets/`: Frontend assets (CSS, JavaScript, charts, animations)

**Agent System** (`services/agentorchestrator/agents/`):
- **student-facing-agent**: Router/coordinator that routes ALL user questions to specialist instructors. Never answers directly.
- **math-instructor** (Prof. Ada): Algebra, calculus, equations. Has plot/graphing tools via gnuplot.
- **physics-instructor** (Dr. Newton): Mechanics, thermodynamics, unit conversions.
- **chemistry-instructor** (Prof. Curie): Elements, reactions, molecular structures.
- **statistics-instructor**: Statistical analysis, probability distributions, regression (using montanaflynn/stats).
- **diagramming-instructor**: Mermaid diagram generation for visual explanations.
- **shared/**: Shared tools (calculator, grep, resource lookups) and agent registry

**Agent SDK** (`foundation/agentsdk/`):
- **core/**: EventStore interface, Thread abstraction, Tool interface, AgentConfig, middleware (elicitation)
- **llm/**: LLM client wrapper for streaming chat completions
- **tools/**: Tool execution and parameter validation

**Event Sourcing + CQRS**:
1. **Write Side (Commands)**: All state changes are persisted as events in KurrentDB (event store)
2. **Read Side (Queries)**: PostgreSQL materialized projections updated in real-time via event subscription
3. **Projection Updater** (`infra/projection/subscriber/`): Subscribes to KurrentDB `$all` stream and updates PostgreSQL tables
4. **Projections**:
   - **AGUI Projection** (`infra/projection/agui/`): Fast thread state for SSE streaming (3-8ms vs 50-500ms)
   - **LLM Context Projection** (`infra/projection/llmcontext/`): Fast conversation history for LLM prompts
   - **Statistics Projection** (`infra/projection/statistics/`): Usage metrics and analytics

**Task Processing** (`services/taskmanagement/`):
- **Asynq Worker Pool**: Background job processing with Redis queue (default: 5 concurrent workers)
- **Task Types**: `chat:process` (TypeProcessChat)
- **Langfuse Integration**: Optional LLM observability tracing for all agent executions

**Multi-Pod Scalability**:
- **Redis Pub/Sub** (`infra/projection/subscriber/redis_broadcaster.go`): Broadcasts events across pods
- User on Pod A receives real-time updates even if their job runs on Pod B

**Infrastructure** (`infra/`):
- **eventstore/**: KurrentDB client adapter implementing EventStore interface
- **projection/**: Real-time projection updaters and PostgreSQL repositories
- **kratos/**: Ory Kratos auth client wrapper
- **sqldb/**: PostgreSQL connection and student profile repository
- **rag/**: Typesense vector search (optional)

**Data Flow**:
```
POST /send → Asynq TaskManager → Redis Queue → Worker Pool → AgentOrchestrator.Run()
→ LLM streaming → Domain events → KurrentDB → Projection Updater → PostgreSQL
→ Redis Pub/Sub → SSE to frontend (all pods)
```

### Key Patterns

1. **Event Sourcing**: All state changes persisted as events, enabling replay and state reconstruction
2. **Tool Interface** (`services/agent/tool.go`): Tools are self-describing (Name, Description, Parameters JSON Schema)
3. **Instructor Registry**: Factory pattern for creating instructors with their assigned tools
4. **Streaming**: Token-by-token content streaming + incremental tool call accumulation

### HTTP Routes

```
POST   /send                              → Enqueue user message as job (CQRS Command)
GET    /api/jobs/{jobId}/stream           → SSE stream of job events (CQRS Query)
GET    /api/threads/{threadId}/messages   → Get thread messages from projection
DELETE /api/threads/{threadId}            → Delete thread (emits ThreadDeleted event)
```

### Educational Tools

Tools are defined in agent directories and auto-registered:
- **Math tools** (`agents/math/tools/`): equation_solver, plot_tool (gnuplot), animate_tool (GIF generation)
- **Statistics tools** (`agents/statistics/tools/`): statistical analysis (mean, median, regression, distributions)
- **Diagramming tools** (`agents/diagramming/tools/`): mermaid_diagram_tool
- **Shared tools** (`agents/shared/tools/`): calculator, grep (text search), resource definitions
- **Chemistry tools**: periodic_table (element lookups)
- **Physics tools**: unit_conversion

All tools implement `core.Tool` interface (Name, Description, Parameters JSON Schema, Execute).

### Frontend Stack

- **Templ**: Server-side Go templates (`components/`) - generate with `templ generate` before build
- **Alpine.js**: Reactive chat UI (`assets/js/chat.js`)
- **Tailwind CSS v4**: Styling with CSS variables (`assets/css/`)
- **templui**: Component library (shadcn/ui equivalent for Go/Templ)

## Key Development Patterns

### Adding a New Agent

1. Create agent directory: `services/agentorchestrator/agents/yourname/`
2. Implement agent with personality and tools: `yourname.go`
3. Optional: Add agent-specific tools in `agents/yourname/tools/`
4. Register in agent registry: `agents/shared/agentregistry.go`
5. Tools are auto-discovered from agent's `Tools()` method

### Adding a New Tool

1. Create tool file in appropriate agent directory (e.g., `agents/math/tools/yourtool.go`)
2. Implement `core.Tool` interface:
   - `Name() string` - unique tool identifier
   - `Description() string` - LLM-friendly description
   - `Parameters() any` - JSON schema struct
   - `Execute(ctx, params) (string, error)` - tool logic
3. Add to agent's tool list in constructor
4. Test with `go test -v ./services/agentorchestrator/agents/.../tools`

### Working with Events

1. **Event Sourcing**: All domain events are persisted in KurrentDB (event store)
2. **Event Types**: Defined in domain packages, serialized as JSON with `__typename` metadata
3. **Projections**: Updated asynchronously via `infra/projection/subscriber/projection_updater.go`
4. **Testing**: Use in-memory event store (`core.NewInMemoryThread()`) for unit tests

### Projection Updates

1. All projections subscribe to KurrentDB `$all` stream
2. Events are deserialized by `event_deserializer.go` based on `__typename`
3. Each projection handles relevant events and updates PostgreSQL tables
4. Redis pub/sub broadcasts updates to all application pods for SSE streaming

### Testing Strategy

- **Unit tests**: Use `core.NewInMemoryThread()` for isolated agent testing
- **Integration tests**: Mock LLM client with canned responses
- **Tool tests**: Test tools independently with fixtures
- **Architecture tests**: `arch/` directory validates dependency rules using archunit

## Architectural Constraints

1. **Agent Communication**: Agents CANNOT call each other directly - they must use event emission
2. **Event Store as Source of Truth**: Never update projections directly - emit events
3. **Projection Eventually Consistent**: Reads from PostgreSQL may lag slightly behind KurrentDB
4. **Tool Registration**: Tools are capability-based - agents only see tools registered to them
5. **Templ Generation**: Always run `templ generate` before building/testing (components are code-generated)
6. **Artifact Cleanup**: Charts/animations/diagrams created during tests are cleaned by `task ci`

## Environment Variables

See `.env.example` for all configuration options. Key variables:

```bash
# Required
OPENAI_API_KEY=sk-your-key                           # OpenAI API key
SCHOOL_WEB_API_HOST=localhost:8090                   # HTTP server address
SCHOOL_AI_MODEL=gpt-4o-mini                          # LLM model to use

# Event Store (KurrentDB)
SCHOOL_KURRENT_DB_CONNECTION_STRING=esdb://localhost:2113?tls=false

# Redis (Asynq task queue)
SCHOOL_REDIS_ADDR=localhost:6379

# PostgreSQL (Projections)
SCHOOL_DB_HOST=localhost:5435
SCHOOL_DB_USER=schoolai
SCHOOL_DB_PASSWORD=dev-password
SCHOOL_DB_NAME=projections

# Ory Kratos (Authentication)
SCHOOL_AUTH_KRATOS_PUBLIC=http://localhost:4433
SCHOOL_AUTH_KRATOS_ADMIN=http://localhost:4434

# Worker Pool
SCHOOL_ORCHESTRATION_CONCURRENCY=5                   # Number of concurrent workers

# MinIO/S3 (for charts, animations, diagrams)
SCHOOL_ARTIFACTS_S3ENDPOINT=localhost:9100
SCHOOL_ARTIFACTS_S3BUCKET=artifacts
```

## Observability

The application integrates with **Langfuse** for LLM observability (optional):

- **Setup**: `task up` (starts all services including Langfuse)
- **UI**: http://localhost:3000 (login: admin@schoolwithai.local / admin123)
- **Asynqmon**: http://localhost:8080 (monitor task queue, retries, failures)
- **RedisInsight**: http://localhost:5540 (monitor Redis pub/sub, keys, streams)
- **Traces**: Automatically captures all agent executions, handoffs, and responses
- **Integration**: `services/taskmanagement/langfuse_integration.go` wraps agent execution with tracing

When enabled, every user message creates a trace showing:
- Agent routing decisions
- Nested agent executions (e.g., math-instructor routing to statistics-instructor)
- Tool calls and results
- Timing and performance
- Errors and debugging info

## Infrastructure Services (Docker Compose)

```bash
# All services started with: task up
Redis:          localhost:6379          # Asynq task queue + pub/sub
Asynqmon:       localhost:8080          # Task queue monitoring UI
RedisInsight:   localhost:5540          # Redis GUI
Postgres:       localhost:5433          # Kratos auth database
Kratos Public:  localhost:4433          # Auth public API
Kratos Admin:   localhost:4434          # Auth admin API
MailSlurper:    localhost:4436          # Email testing UI
KurrentDB:      localhost:2113          # Event store
App-Postgres:   localhost:5435          # Projections database
Langfuse:       localhost:3000          # LLM observability (optional)
MinIO:          localhost:9100          # S3-compatible artifact storage
```

## Technology Stack

**Backend**:
- **Go 1.24.4+**: Primary language with toolchain 1.24.11
- **KurrentDB**: PostgreSQL-backed event store (EventStoreDB fork)
- **PostgreSQL**: Materialized projections database
- **Redis**: Task queue (Asynq) + pub/sub for multi-pod scalability
- **Ory Kratos**: Identity and session management

**Frontend**:
- **Templ**: Type-safe Go templates compiled to Go code
- **Alpine.js**: Reactive UI without build step
- **Tailwind CSS v4**: Utility-first CSS with CSS variables
- **templui**: shadcn/ui-inspired component library for Go/Templ

**Key Libraries**:
- **github.com/hibiken/asynq**: Distributed task queue with Redis
- **github.com/kurrent-io/KurrentDB-Client-Go**: Event store client
- **github.com/openai/openai-go/v3**: Official OpenAI SDK with streaming
- **github.com/go-chi/chi/v5**: HTTP router with middleware
- **github.com/jackc/pgx/v5**: PostgreSQL driver
- **github.com/montanaflynn/stats**: Statistical computations
- **github.com/ag-ui-protocol/ag-ui**: SSE streaming protocol for agentic UIs

**Observability**:
- **Langfuse**: LLM trace collection and analysis (optional)
- **statsviz**: Real-time Go runtime metrics visualization
- **OpenTelemetry**: Distributed tracing infrastructure

**Development Tools**:
- **Task**: Task runner (Taskfile.yaml)
- **golangci-lint**: Linter aggregator with strict rules
- **templ CLI**: Template code generation
- **archunit**: Architecture validation and dependency rules

## Test Requirements

- **gnuplot**: Required for plot/chart generation tests. Install with `apt-get install gnuplot`.
- Tests may create artifacts in `assets/charts/`, `assets/animations/` - these are cleaned up by `task ci`.
- Run architecture tests separately: `go test -v ./arch/...`
