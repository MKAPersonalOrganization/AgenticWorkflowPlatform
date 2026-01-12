# School with AI

A chat application built with Go, Templ, Tailwind CSS, and Alpine.js, implementing the AG-UI protocol for AI agent interactions.

## Repository Statistics

![Repository Statistics](.github/images/repo_statistics.png)

## Container Diagram

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

### Component Overview

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

### Key Patterns

- **Event Sourcing**: All state changes are stored as events in KurrentDB
- **Query-Time Projections**: State is rebuilt from events on every query (no in-memory cache)
- **CQRS**: Write commands append events, read queries rebuild from events
- **Multi-Agent**: Tool-based agent collaboration with transfer_to_agent and shared whiteboard

## Tech Stack

- **Backend**: Go with Templ templating
- **Frontend**: Alpine.js for reactivity
- **Styling**: Tailwind CSS v4
- **Components**: templui (shadcn/ui equivalent for Go/Templ)
- **Protocol**: AG-UI for server-sent events
- **Linear Algebra**: gosl library (LAPACK/OpenBLAS integration)
