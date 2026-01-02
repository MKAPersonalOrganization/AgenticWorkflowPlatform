# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# First-time setup (installs templ and templui CLIs)
task setup-ubuntu

# Development with hot reload (runs Tailwind watcher + Templ generator with proxy)
task dev
# Frontend: http://localhost:7331 (proxy), Backend: http://localhost:8090 (direct)

# Generate templ files (required before build/test)
templ generate

# Build
go build -v ./...

# Run tests (requires gnuplot for plot tool tests)
go test -v -race ./...

# Run tests for specific package
go test -v ./services/agent
go test -v ./services/tools

# Lint
golangci-lint run ./... --timeout=5m
```

## Architecture Overview

This is an AI-powered educational chat application implementing a multi-agent system with event sourcing and streaming responses via AG-UI protocol.

### Core Architecture: Event Sourcing + Multi-Agent Collaboration

**Agent System** (`services/agent/`):
- **student-facing-agent**: Router/coordinator that routes ALL user questions to specialist instructors. Never answers directly.
- **math-instructor** (Prof. Ada): Algebra, calculus, equations. Has plot/graphing tools.
- **physics-instructor** (Dr. Newton): Mechanics, thermodynamics, unit conversions.
- **chemistry-instructor** (Prof. Curie): Elements, reactions, molecular structures.

Agents communicate exclusively via a shared **whiteboard** - they write answers there and call each other using `call_agent` tool. The student-facing-agent always receives callbacks when instructors complete their work.

**Infrastructure** (`infra/`):
- **WorkerPool**: Manages 5 concurrent workers processing jobs from a queue
- **EventLog**: In-memory event sourcing storage for all domain events
- **ThreadState**: Maintains thread state reconstructed from events
- **AG-UI Protocol** (`infra/agui/`): SSE streaming types (TextMessageStart/Content/End, ToolCall events)

**Data Flow**:
```
POST /send → JobQueue → WorkerPool → Agent.Run() → LLM stream → Domain events → SSE to frontend
```

### Key Patterns

1. **Event Sourcing**: All state changes persisted as events, enabling replay and state reconstruction
2. **Tool Interface** (`services/agent/tool.go`): Tools are self-describing (Name, Description, Parameters JSON Schema)
3. **Instructor Registry**: Factory pattern for creating instructors with their assigned tools
4. **Streaming**: Token-by-token content streaming + incremental tool call accumulation

### HTTP Routes

```
POST   /send                              → Enqueue user message as job
GET    /api/jobs/{jobId}/stream           → SSE stream of job events
GET    /api/threads/{threadId}/messages   → Get thread messages
DELETE /api/threads/{threadId}            → Delete thread
```

### Tools (`services/tools/`)

Educational tools available to instructors:
- `calculator`, `equation_solver`, `graphing`, `plot_tool` (gnuplot), `animate_tool`
- `periodic_table`, `unit_conversion`
- `call_agent`, `write_whiteboard`, `read_whiteboard` (agent collaboration)

### Frontend Stack

- **Templ**: Server-side Go templates (`components/`)
- **Alpine.js**: Reactive chat UI (`assets/js/chat.js`)
- **Tailwind CSS v4**: Styling with CSS variables
- **templui**: Component library (shadcn/ui equivalent for Go/Templ)

## Environment Variables

```
SERVER_ADDR=localhost:8090    # Required - HTTP server address
```

## Test Requirements

- **gnuplot**: Required for plot/chart generation tests. Install with `apt-get install gnuplot`.
