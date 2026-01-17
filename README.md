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
    RedisTQ[(Redis<br/>Task Queue)]
    RedisPS[(Redis<br/>Pub/Sub)]
    Kratos[Ory Kratos<br/>Identity & Auth]
    Postgres[(PostgreSQL<br/>Kratos DB)]

    EmailService[Email Service]

    Browser -->|HTTP/SSE| GoApp
    GoApp -->|Events| KurrentDB
    GoApp -->|Subscribe<br/>to Events| KurrentDB
    GoApp -->|Fast Reads<br/>Materialized Views| AppPostgres
    GoApp -->|Update<br/>Projections| AppPostgres
    GoApp -->|Tasks| RedisTQ
    GoApp -->|Pub/Sub| RedisPS
    GoApp -->|Auth API| Kratos

    Kratos -->|SQL| Postgres
    Kratos -->|SMTP| EmailService

    note["<i>Note: Solid lines denote synchronous interactions; dashed lines denote asynchronous interactions. The system also supports optional LLM observability via Langfuse (not shown here for clarity).</i>"]
```

### Containers Overview

- **Web Browser**: Frontend with Alpine.js for reactivity, Tailwind CSS for styling
- **Go Application**: Event-sourced multi-agent system with PostgreSQL materialized projections
- **KurrentDB**: PostgreSQL-backed event store (single source of truth for events)
- **PostgreSQL (App)**: Materialized projection database for fast reads (updated in real-time via event subscription)
- **Redis (Task Queue)**: Distributed task queue (Asynq) for background agent execution
- **Redis (Pub/Sub)**: Real-time event broadcaster for multi-pod scalability
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
        
        TaskS["<b>Asynq Task Manager</b><br/>[Technology: Asynq/Redis]<br/>Orchestrates background workers and handles job concurrency."]
        ThreadMgmt["<b>Thread Manager</b><br/>[Module]<br/>Handles thread lifecycle events directly via event store."]
        ProjS["<b>UI Projection Service</b><br/>Coordinates fast reads from PostgreSQL materialized projections."]
        
        OrchS["<b>Agent Orchestrator</b><br/>Manages multi-agent execution, tool-calling, and event persistence."]
        
        ProjUpdater["<b>Projection Updater</b><br/>[Technology: KurrentDB Subscriptions]<br/>Subscribes to event stream and updates PostgreSQL projections in real-time."]
        
        RedisB["<b>Redis Broadcaster</b><br/>[Technology: Redis Pub/Sub]<br/>Broadcasts events across multiple pods for real-time UI updates."]

        LLMProj["<b>LLM Context Projection (PostgreSQL)</b><br/>[Technology: Materialized Views]<br/>Fast conversation history from PostgreSQL for LLM prompts."]
        
        AGUIProj["<b>AGUI Projection (PostgreSQL)</b><br/>[Technology: Materialized Views]<br/>Fast reads from PostgreSQL for current thread state."]
    end

    User["<b>User / Browser</b><br/>[Person/Client]"]
    Kratos["<b>Ory Kratos</b><br/>[Software System]<br/>Identity Management"]
    RedisTQ["<b>Redis (Task Queue)</b><br/>[Container]<br/>Asynq Backend"]
    RedisPS["<b>Redis (Pub/Sub)</b><br/>[Container]<br/>Real-time Broadcaster"]
    KurrentDB["<b>KurrentDB</b><br/>[Container]<br/>Event Store"]
    AppPostgres["<b>PostgreSQL</b><br/>[Container]<br/>Projections DB"]

    User -->|"Uses [HTTPS/SSE]"| Router
    Router -->|"Routes to"| HealthH
    Router -->|"Passes to"| AuthM
    AuthM -->|"Routes to"| TaskH
    AuthM -->|"Routes to"| AGUIH
    
    AuthM -->|"Validates session [HTTPS]"| Kratos
    
    TaskH -->|"Command to enqueue [In-process]"| TaskS
    TaskH -->|"Manages threads [In-process]"| ThreadMgmt
    AGUIH -->|"Requests projection [In-process]"| ProjS
    
    TaskS -.->|"Enqueues job"| RedisTQ
    RedisTQ -.->|"Pulls job for processing"| TaskS
    
    TaskS -->|"Executes business logic [In-process]"| OrchS
    
    OrchS -->|"Requests context [In-process]"| LLMProj
    
    ProjS -->|"Fast reads"| AGUIProj
    AGUIProj -->|"Query"| AppPostgres
    
    AGUIH -->|"Subscribes (Live)"| RedisB
    RedisB -.->|"Subscribe/Publish"| RedisPS
    ProjUpdater -.->|"Notify (Live)"| RedisB
    
    LLMProj -->|"Query"| AppPostgres
    
    ProjUpdater -->|"Subscribe to $all"| KurrentDB
    ProjUpdater -->|"Update all projections"| AppPostgres
    
    OrchS -->|"Appends events"| KurrentDB
    ThreadMgmt -->|"Appends events"| KurrentDB
    
    note["<i>Note: Solid lines denote synchronous interactions; dashed lines denote asynchronous interactions. LLM interactions and detailed tracing (via Langfuse) are omitted for clarity as they are considered implicit.</i>"]
```

### Components Overview

| Name | Technology | Description |
| :--- | :--- | :--- |
| **Chi Router** | Go Chi | High-performance routing with middleware for panic recovery, logging, and metrics. |
| **Auth Middleware** | - | Security layer that enforces session requirements using external Kratos identity service. |
| **Health Handlers** | - | Public system status endpoints used for infrastructure health checks. |
| **Task Handlers** | - | The "Command" side of CQRS, transforming HTTP requests into background tasks. |
| **AGUI Handlers** | - | The "Query" side of CQRS, serving structured JSON/SSE for the AGUI frontend. |
| **Asynq Task Manager** | Asynq/Redis | Reliable task queueing and processing with retry logic and priority management. |
| **Thread Manager** | - | Handles thread lifecycle events (creation, deletion) directly via event store adapter. |
| **UI Projection Service** | - | Domain logic for fast reads from PostgreSQL materialized projections. |
| **Agent Orchestrator** | - | Manages LLM interactions, tool-calling loops, and agent handoffs. (Package: `services/agentorchestrator`) |
| **Projection Updater** | KurrentDB Subscriptions | Subscribes to KurrentDB event stream and updates PostgreSQL projections in real-time. |
| **Redis Broadcaster** | Redis Pub/Sub | Distributed event broadcaster that ensures real-time updates are delivered across all application pods. |
| **LLM Context Projection** | PostgreSQL Materialized Views | Fast conversation history from PostgreSQL for LLM prompts (3-8ms reads). |
| **AGUI Projection** | PostgreSQL Materialized Views | Fast reads from PostgreSQL tables that are kept up-to-date via event subscription (3-8ms vs 50-500ms). |

### Key Patterns

- **Event Sourcing**: All state changes are persisted as a sequence of events in KurrentDB.
- **Materialized Projections**: PostgreSQL tables updated in real-time via event subscription for fast reads (~100x faster than query-time).
- **Event Subscription**: Projection updater subscribes to KurrentDB `$all` stream and incrementally updates both projections (AGUI, LLM Context).
- **CQRS**: Clear separation between command side (Task Handlers writing events) and query side (AGUI Handlers reading projections).
- **Multi-Pod Scalability**: Real-time events are broadcasted via **Redis Pub/Sub**. This ensures that even if a user is connected to Pod A but their background task is running on Pod B, they still receive real-time UI updates seamlessly.
- **Multi-Agent Orchestration**: Collaborative agent execution with explicit handoff protocols.

## Tech Stack

- **Backend**: Go with Templ templating
- **Frontend**: Alpine.js for reactivity
- **Styling**: Tailwind CSS v4
- **Components**: templui (shadcn/ui equivalent for Go/Templ)
- **Protocol**: AG-UI for server-sent events
- **Linear Algebra**: gosl library (LAPACK/OpenBLAS integration)
