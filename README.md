# School with AI

A chat application built with Go, Templ, Tailwind CSS, and Alpine.js, implementing the AG-UI protocol for AI agent interactions.

[Changelog](CHANGELOG.md)

## Repository Statistics

![Repository Statistics](.github/images/repo_statistics.png)

## Containers Diagram

```mermaid
graph TB
    Browser[Web Browser<br/>Alpine.js + Tailwind]

    GoApp[Go Application<br/>Event-Sourced Multi-Agent System]

    KurrentDB[(KurrentDB<br/>Event Store)]
    AppPostgres[(PostgreSQL<br/>Projections DB)]
    Redis[(Redis<br/>Task Queue)]
    Kratos[Ory Kratos<br/>Identity & Auth]
    Postgres[(PostgreSQL<br/>Kratos DB)]

    Langfuse[Langfuse<br/>LLM Observability]
    LangfuseDB[(PostgreSQL<br/>Langfuse DB)]
    ClickHouse[(ClickHouse<br/>Analytics)]
    MinIO[(MinIO<br/>Object Storage)]

    EmailService[Email Service]

    Browser -->|HTTP/SSE| GoApp
    GoApp -->|Events| KurrentDB
    GoApp -->|Subscribe<br/>to Events| KurrentDB
    GoApp -->|Fast Reads<br/>Materialized Views| AppPostgres
    GoApp -->|Update<br/>Projections| AppPostgres
    GoApp -->|Tasks| Redis
    GoApp -->|Auth API| Kratos
    GoApp -->|Traces & Metrics| Langfuse

    Kratos -->|SQL| Postgres
    Kratos -->|SMTP| EmailService

    Langfuse -->|SQL| LangfuseDB
    Langfuse -->|Analytics| ClickHouse
    Langfuse -->|Media Storage| MinIO
```

### Containers Overview

- **Web Browser**: Frontend with Alpine.js for reactivity, Tailwind CSS for styling
- **Go Application**: Event-sourced multi-agent system with PostgreSQL materialized projections
- **KurrentDB**: PostgreSQL-backed event store (single source of truth for events)
- **PostgreSQL (App)**: Materialized projection database for fast reads (updated in real-time via event subscription)
- **Redis**: Task queue for async job processing (Asynq)
- **Ory Kratos**: Identity and session management
- **PostgreSQL (Kratos)**: Database for Kratos user data
- **Email Service**: Notification service for authentication emails
- **Langfuse**: LLM observability platform for tracing agent executions, handoffs, and responses
- **PostgreSQL (Langfuse)**: Database for Langfuse traces and metadata
- **ClickHouse**: Analytics database for Langfuse metrics and aggregations
- **MinIO**: S3-compatible object storage for Langfuse media and event logs

## Components Diagram (Level 3)

The following diagram zooms into the **Go Application** container to show its internal components, their responsibilities, technologies, and interactions.

```mermaid
graph TB
    subgraph GoApp["Go Application"]
        Router["<b>Chi Router</b><br/>[Technology: Go Chi]<br/>Routes incoming HTTP requests to appropriate handlers and applies global middleware."]
        AuthM["<b>Auth Middleware</b><br/>Intercepts requests to validate user sessions via Ory Kratos."]
        HealthH["<b>Health Handlers</b><br/>Provides public endpoints for liveness and readiness probes."]
        
        TaskH["<b>Task Handlers</b><br/>Handles command-side interactions (POST/DELETE) for task and thread management."]
        AGUIH["<b>AGUI Handlers</b><br/>Handles query-side interactions (GET) for AGUI-compatible thread state."]
        
        TaskS["<b>Task Management Service</b><br/>[Technology: Asynq/Redis]<br/>Orchestrates background workers and handles job concurrency."]
        ProjS["<b>UI Projection Service</b><br/>Coordinates fast reads from PostgreSQL materialized projections."]
        
        OrchS["<b>Agent Orchestrator</b><br/>Manages multi-agent execution, tool-calling, and event persistence."]
        
        ProjUpdater["<b>Projection Updater</b><br/>[Technology: KurrentDB Subscriptions]<br/>Subscribes to event stream and updates PostgreSQL projections in real-time."]
        
        LLMProj["<b>LLM Context Projection (PostgreSQL)</b><br/>[Technology: Materialized Views]<br/>Fast conversation history from PostgreSQL for LLM prompts."]
        
        AGUIProj["<b>AGUI Projection (PostgreSQL)</b><br/>[Technology: Materialized Views]<br/>Fast reads from PostgreSQL for current thread state."]
    end

    User["<b>User / Browser</b><br/>[Person/Client]"]
    Kratos["<b>Ory Kratos</b><br/>[Software System]<br/>Identity Management"]
    Redis["<b>Redis</b><br/>[Container]<br/>Task Queue Store"]
    KurrentDB["<b>KurrentDB</b><br/>[Container]<br/>Event Store"]
    AppPostgres["<b>PostgreSQL</b><br/>[Container]<br/>Projections DB"]
    LLM["<b>OpenAI API</b><br/>[Software System]<br/>LLM Provider"]
    Langfuse["<b>Langfuse</b><br/>[Software System]<br/>LLM Observability"]

    User -->|"Uses [HTTPS/SSE]"| Router
    Router -->|"Routes to"| HealthH
    Router -->|"Passes to"| AuthM
    AuthM -->|"Routes to"| TaskH
    AuthM -->|"Routes to"| AGUIH
    
    AuthM -->|"Validates session [HTTPS]"| Kratos
    
    TaskH -->|"Command to enqueue [In-process]"| TaskS
    AGUIH -->|"Requests projection [In-process]"| ProjS
    
    TaskS -.->|"Enqueues job [Redis]"| Redis
    Redis -.->|"Pulls job for processing [Redis]"| TaskS
    
    TaskS -->|"Executes business logic [In-process]"| OrchS
    
    OrchS -->|"Requests context [In-process]"| LLMProj
    OrchS -->|"Accesses workspace [In-process]"| WhiteProj
    
    ProjS -->|"Fast reads [SQL]"| AGUIProj
    AGUIProj -->|"Query [SQL]"| AppPostgres
    
    AGUIH -->|"Subscribes (Live) [Go Channel]"| AGUIProj
    ProjUpdater -.->|"Notify (Live) [In-Memory]"| AGUIProj
    
    LLMProj -->|"Query [SQL]"| AppPostgres
    
    ProjUpdater -->|"Subscribe to $all [GRPC]"| KurrentDB
    ProjUpdater -->|"Update all projections [SQL]"| AppPostgres
    
    OrchS -->|"Appends events [GRPC]"| KurrentDB
    OrchS -->|"Requests completions [HTTPS]"| LLM
    
    OrchS -.->|"Sends traces [HTTPS]"| Langfuse
    AGUIProj -.->|"Sends traces [HTTPS]"| Langfuse
    
    note["<i>Note: Solid lines denote synchronous interactions; dashed lines denote asynchronous interactions.</i>"]
```

### Components Overview

| Name | Technology | Description |
| :--- | :--- | :--- |
| **Chi Router** | Go Chi | High-performance routing with middleware for panic recovery, logging, and metrics. |
| **Auth Middleware** | - | Security layer that enforces session requirements using external Kratos identity service. |
| **Health Handlers** | - | Public system status endpoints used for infrastructure health checks. |
| **Task Handlers** | - | The "Command" side of CQRS, transforming HTTP requests into background tasks. |
| **AGUI Handlers** | - | The "Query" side of CQRS, serving structured JSON/SSE for the AGUI frontend. |
| **Task Management Service** | Asynq/Redis | Reliable task queueing and processing with retry logic and priority management. |
| **UI Projection Service** | - | Domain logic for fast reads from PostgreSQL materialized projections. |
| **Agent Orchestrator** | - | Manages LLM interactions, tool-calling loops, and agent handoffs. (Package: `services/agentorchestrator`) |
| **Projection Updater** | KurrentDB Subscriptions | Subscribes to KurrentDB event stream and updates PostgreSQL projections in real-time. |
| **LLM Context Projection** | PostgreSQL Materialized Views | Fast conversation history from PostgreSQL for LLM prompts (3-8ms reads). |
| **AGUI Projection** | PostgreSQL Materialized Views | Fast reads from PostgreSQL tables that are kept up-to-date via event subscription (3-8ms vs 50-500ms). |

### Key Patterns

- **Event Sourcing**: All state changes are persisted as a sequence of events in KurrentDB.
- **Materialized Projections**: PostgreSQL tables updated in real-time via event subscription for fast reads (~100x faster than query-time).
- **Event Subscription**: Projection updater subscribes to KurrentDB `$all` stream and incrementally updates all three projections (AGUI, Whiteboard, LLM Context).
- **CQRS**: Clear separation between command side (Task Handlers writing events) and query side (AGUI Handlers reading projections).
- **Multi-Agent Orchestration**: Collaborative agent execution with shared whiteboard state and explicit handoff protocols.

## Tech Stack

- **Backend**: Go with Templ templating
- **Frontend**: Alpine.js for reactivity
- **Styling**: Tailwind CSS v4
- **Components**: templui (shadcn/ui equivalent for Go/Templ)
- **Protocol**: AG-UI for server-sent events
- **Linear Algebra**: gosl library (LAPACK/OpenBLAS integration)
