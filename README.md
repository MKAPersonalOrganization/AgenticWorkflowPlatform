# School with AI

A chat application built with Go, Templ, Tailwind CSS, and Alpine.js, implementing the AG-UI protocol for AI agent interactions.

## Repository Statistics

![Repository Statistics](.github/images/repo_statistics.png)

## Containers Diagram

```mermaid
graph TB
    Browser[Web Browser<br/>Alpine.js + Tailwind]

    GoApp[Go Application<br/>Event-Sourced Multi-Agent System]

    KurrentDB[(KurrentDB<br/>Event Store)]
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
    GoApp -->|Query-Time<br/>Projections| KurrentDB
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
- **Go Application**: Event-sourced multi-agent system with query-time projections
- **KurrentDB**: PostgreSQL-backed event store (single source of truth)
- **Redis**: Task queue for async job processing (Asynq)
- **Ory Kratos**: Identity and session management
- **PostgreSQL**: Database for Kratos user data
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
        ProjS["<b>UI Projection Service</b><br/>Coordinates the reconstruction of UI state from events."]
        
        OrchS["<b>Agent Orchestrator</b><br/>Manages multi-agent execution, tool-calling, and event persistence."]
        
        LLMProj["<b>LLM Context Projection</b><br/>Provides historical conversation context for LLM prompts."]
        WhiteProj["<b>Whiteboard Projection</b><br/>Maintains shared agent workspace state (artifacts, variables)."]
        
        AGUIProj["<b>AGUI Projection</b><br/>[Technology: Event-sourced Projection]<br/>Replays events from KurrentDB to rebuild current thread state."]
    end

    User["<b>User / Browser</b><br/>[Person/Client]"]
    Kratos["<b>Ory Kratos</b><br/>[Software System]<br/>Identity Management"]
    Redis["<b>Redis</b><br/>[Container]<br/>Task Queue Store"]
    KurrentDB["<b>KurrentDB</b><br/>[Container]<br/>Event Store"]
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
    
    ProjS -->|"Retrieves state [In-process]"| AGUIProj
    
    LLMProj -->|"Queries events [GRPC]"| KurrentDB
    WhiteProj -->|"Queries events [GRPC]"| KurrentDB
    AGUIProj -->|"Queries events [GRPC]"| KurrentDB
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
| **UI Projection Service** | - | Domain logic for mapping lower-level event streams into high-level AGUI UI state. |
| **Agent Orchestrator** | - | Manages LLM interactions, tool-calling loops, and agent handoffs. (Package: `services/agentorchestrator`) |
| **LLM Context Projection** | - | Re-constructs conversation history for agent prompts from the event store. |
| **Whiteboard Projection** | - | Manages the collaborative state and shared variables used by agents during a run. |
| **AGUI Projection** | Event-sourced Projection | Logic for aggregating historical events to produce a point-in-time view of a conversation thread. |

### Key Patterns

- **Event Sourcing**: All state changes are persisted as a sequence of events in KurrentDB.
- **Query-Time Projections**: State is reconstructed by replaying events on every read request, ensuring strong consistency (or horizontal scalability if using read-models).
- **CQRS**: Clear separation between command side (Task Handlers) and query side (AGUI Handlers).
- **Multi-Agent Orchestration**: Collaborative agent execution with shared whiteboard state and explicit handoff protocols.

## Tech Stack

- **Backend**: Go with Templ templating
- **Frontend**: Alpine.js for reactivity
- **Styling**: Tailwind CSS v4
- **Components**: templui (shadcn/ui equivalent for Go/Templ)
- **Protocol**: AG-UI for server-sent events
- **Linear Algebra**: gosl library (LAPACK/OpenBLAS integration)
