# System Concept & Ubiquitous Language

> This document defines the core ideas behind SchoolWithAI and establishes the
> ubiquitous language used throughout the codebase. Each concept is annotated
> with its implementation status: ✅ Implemented, ⚠️ Partial, or 🔲 Planned.

---

## 1 — Agents

An **Agent** is the fundamental unit of intelligence in the system. It has a name,
a description, a system prompt that governs its behavior, and an optional icon.

Agents exist at two levels:

| Level | Purpose |
|-------|---------|
| **Agent Catalog** | A shared, read-only store of agent templates available to all users. Entries are organized by category (Education, Programming, Writing, etc.) and can be featured or tagged for discovery. ✅ |
| **Agent Definition** | A user-owned agent. Created either by **forking** from the catalog (preserving provenance via `SourceCatalogID`) or from scratch as a **custom agent**. ✅ |

On first access, every user automatically receives a default "Assistant" agent. ✅

At runtime, an `AgentDefinition` is resolved into an executable `Agent` that the
framework can run. From the framework's perspective, an Agent is anything that is
**Identifiable** (has an ID, name, description) and **Runnable** (produces a
stream of events given an invocation context). ✅

---

## 2 — Tools & Toolsets

A **Tool** is a discrete capability an agent can invoke — a function with a name,
a description, and a JSON Schema for its parameters.

Tools come in two types:

| Type | Description |
|------|-------------|
| **Built-in Tools** | Compiled into the platform as **Plugins**. Each plugin registers its tools and toolsets on startup. Examples: calculator, web search, memory, knowledge base query, cognitive "think" tool, TTS, document management, plan mode, todos, task tracker, skills. ✅ |
| **External Tools (MCP)** | Discovered at runtime from user-connected **MCP Servers**. The system probes the server, discovers available tools via the MCP handshake, and registers them dynamically. Execution uses ephemeral sessions for horizontal scalability. ✅ |

> **Key principle:** From the agent's perspective, it does not matter whether a
> tool is built-in or comes from an external MCP server. Both surface as the
> same unified `ExecutableTool` interface. ✅

A **Toolset** is a cohesive group of related tools. Every tool belongs to exactly
one toolset. Built-in plugins define their toolsets at compile time; MCP
connections each become a toolset at runtime. Toolsets are the primary unit for
granting and revoking access. ✅

---

## 3 — Permissions

Agents are **decoupled** from tools and resources. Access is never implicit — it
must be explicitly granted through permission rules. The system enforces a
two-tier permission model:

### Tier 1: Agent → Tool Permissions

Controls which tools an agent is allowed to use.

| Scope | Granularity |
|-------|-------------|
| **Toolset Permission** | Enables or disables an entire toolset for an agent. ✅ |
| **Tool Permission** | Overrides at the individual tool level within a toolset, allowing a user to enable only a subset of tools. ✅ |

Resolution: individual tool permission is checked first; if absent, the toolset-level
permission is used as the fallback. ✅

### Tier 2: Tool → Resource Permissions

Controls which resources a tool is allowed to access. This applies to tools that
operate on user-owned resources:

| Resource | Permission Model | Status |
|----------|-----------------|--------|
| **Knowledge Base Workspaces** | `WorkspacePermission` — grants an agent access to specific knowledge base workspaces. Multiple workspaces can be enabled per agent. Enabled workspaces are injected into the agent's system prompt so the LLM knows which namespaces it can query. | ✅ |
| **File Systems** | `FSPermission` — grants an agent access to specific virtual file systems. Multiple file systems can be enabled per agent. | ⚠️ Permission model exists, but agent-facing filesystem tools have not been built yet. |

> **Summary:** Agents must have permissions to use tools. Tools must have
> permissions to access resources.

---

## 4 — Knowledge Bases

A **Knowledge Base** is a workspace managed by the Edgequake RAG engine. Each
workspace contains documents, a knowledge graph (entities and relationships),
embeddings, and a processing pipeline. ✅

The built-in `query_knowledge_base` tool allows an agent to search a knowledge
base by query and namespace. Internally it retrieves context from Edgequake
(chunks, entities, relationships) and synthesizes an answer via an LLM — an
agent-as-tool pattern. Results include the answer, sources, an optional
knowledge graph subgraph, and performance metrics. ✅

An agent can access **multiple knowledge bases** simultaneously — each enabled
workspace appears as an available namespace in the agent's prompt. ✅

Access is governed by workspace permissions (see §3, Tier 2). ✅

---

## 5 — Virtual File Systems

A **File System** is a user-owned storage abstraction modeled after the Linux VFS
layer. It provides standard operations: mkdir, readdir, create, read, remove,
rename, stat. ✅

Two backend implementations exist:

| Backend | Capabilities |
|---------|-------------|
| **S3/MinIO** | Full read-write. Uses MinIO for blob storage and PostgreSQL for directory metadata. ✅ |
| **GitHub** | Read-only. Uses the GitHub Contents API with OAuth token resolution. ✅ |

An agent can be granted access to **multiple file systems**, each with
independent permissions. ⚠️

> **Gap:** The VFS layer, filesystem domain model, and `FSPermission` model are
> all implemented. However, there are **no agent-facing filesystem tools yet**
> (e.g. `fs_read_file`, `fs_list_dir`). The filesystem is currently accessible
> only through the HTTP API and UI. Building the filesystem plugin to expose
> these operations as tools — following the same pattern as the knowledge base
> plugin — is the remaining work. 🔲

---

## 6 — Skills

A **Skill** is a specialized instruction set that an agent can activate at
runtime. Skills extend an agent's behavior without modifying its core system
prompt. ✅

Skills follow the same catalog → user library → permission pattern as agents:

| Layer | Purpose |
|-------|---------|
| **Skill Catalog** | Shared, read-only skill templates organized by category. ✅ |
| **User Skills** | Per-user library — imported from the catalog or custom-created. ✅ |
| **Skill–Agent Permissions** | Grant table scoped by (user, agent, skill). ✅ |

At runtime, the skills plugin exposes `list_skills` and `activate_skill` tools.
When a skill is activated, its instructions are injected into the agent's context
and guide the LLM's subsequent responses. ✅

---

## 7 — Requirements

A **Requirement** is a declarative constraint evaluated before each LLM turn. It
inspects the current invocation state (conversation history, available tools,
config) and produces **Rules** that shape the agent's behavior for that turn. ✅

A **Rule** targets a specific tool and can:

| Action | Effect |
|--------|--------|
| **Block** | Hard veto — the tool is removed from the agent's available tools. Any single block wins. ✅ |
| **Force** | The agent *must* use this tool. Conflicts resolved by priority. ✅ |
| **Hide** | Silently omitted from the LLM's view (no explanation injected). ✅ |
| **PreventStop** | The agent cannot finish its turn yet — it must continue using tools. ✅ |

Rules carry a `Reason` (shown to the LLM) and a `Priority` for conflict
resolution. ✅

### Built-in Requirement Types

| Type | Purpose |
|------|---------|
| **ConditionalRequirement** | Declarative, history-based rules: force at step N, limit max invocations, require prerequisites, force after triggers, enforce ReAct patterns (think→act→think), block consecutive calls. ✅ |
| **AskPermissionRequirement** | Human-in-the-loop — pauses execution and requests user approval before specific (or all) tool calls. ✅ |
| **Inline Requirement** | Ad-hoc requirement defined as a function for one-off constraints. ✅ |

Requirements can also provide additional tools (`ToolProvider`), append to the
system prompt (`PromptProvider`), toggle dynamically (`Enableable`), and inject
function-level middleware (`FunctionMiddlewareProvider`). ✅

> **This is how the system enforces specific tool-call trajectories and generic
> constraints** — the original concept of "forcing specific agent tool call
> trajectory or generic requirements that must be enforced."

---

## 8 — Threads & Sessions

A **Thread** is the unit of conversation. It is an **event-sourced aggregate**
that holds messages, tracks in-flight tool call states, and enforces business
rules through domain events. ✅

Each thread records a stream of events: `ThreadCreated`, `UserMessageAdded`,
`AssistantMessageAdded`, `ToolResultAdded`, `ThreadTitleUpdated`, and tool
lifecycle events (`ToolStart`, `ToolNeedsInput`, `ToolNeedsAuth`, `AuthGranted`,
`AuthDenied`, `ToolCancelled`). ✅

At the framework level, a `Runner` receives a `RunRequest` with a `SessionID`
that maps to a thread. Agent execution produces a stream of events classified as
either **Persistent** (messages, tool results, usage, title) or **Ephemeral**
(streaming deltas, tool call starts, errors, completion signals). ✅

---

## 9 — Middleware

The agent execution pipeline supports **three levels of middleware**, each
wrapping a different scope:

| Level | Scope |
|-------|-------|
| **Agent Middleware** | Wraps the entire agent run. ✅ |
| **Chat Middleware** | Wraps each individual LLM call. Requirements are evaluated here. ✅ |
| **Function Middleware** | Wraps each tool execution. Used by ask-permission and similar interceptors. ✅ |

---

## Alignment Summary

| Concept | Status | Notes |
|---------|--------|-------|
| Agent Catalog & Forking | ✅ | Fully implemented with provenance tracking |
| Custom Agents | ✅ | Full CRUD with per-user scoping |
| Built-in Tools | ✅ | 14 plugins registered at startup |
| MCP External Tools | ✅ | Dynamic discovery, ephemeral execution, OAuth 2.1 |
| Unified Tool Interface | ✅ | Agent sees no difference between built-in and MCP |
| Toolsets | ✅ | Tools grouped, registered in PostgreSQL |
| Agent → Tool Permissions | ✅ | Toolset-level and individual tool-level |
| Knowledge Bases | ✅ | Edgequake RAG with query tool |
| KB Workspace Permissions | ✅ | Multi-workspace, prompt-injected |
| Virtual File System (VFS) | ✅ | S3/MinIO + GitHub backends |
| Filesystem Permissions | ⚠️ | `FSPermission` model exists |
| **Filesystem Agent Tools** | **🔲** | **No `fs_read_file` / `fs_list_dir` plugin yet** |
| Requirements | ✅ | Full rule engine with conditional, permission, and inline types |
| Skills | ✅ | Catalog → user library → agent permissions → runtime activation |
| Event-Sourced Threads | ✅ | 22+ event types, optimistic concurrency |
| Middleware Pipeline | ✅ | Agent / Chat / Function levels |

