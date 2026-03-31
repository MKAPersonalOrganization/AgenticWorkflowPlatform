# ADR 021: Artifact Service (Versioned File Management)

**Status:** Accepted

## Context

Agents that produce deliverables — reports, CSVs, images, code files — have no
way to persist, version, or reference those outputs across turns or sessions.
The only artifact-related capability is `A2AArtifactReceived`, a transient event
for displaying text artifacts streamed from remote A2A agents. There is no
agent-generated artifact creation, no versioning, and no user-facing file
management for native agents.

ADK-Go ships an `artifact.Service` interface with full lifecycle support:

| Method | Purpose |
|--------|---------|
| `Save(ctx, req)` | Store a versioned artifact (auto-increments version) |
| `Load(ctx, req)` | Retrieve latest or specific version |
| `Delete(ctx, req)` | Remove one version or all versions |
| `List(ctx, req)` | List all artifact filenames in a session |
| `Versions(ctx, req)` | List all version numbers for an artifact |
| `GetArtifactVersion(ctx, req)` | Get metadata for a specific version |

The runner accepts an `ArtifactService` field. When set, it wraps the service in
a scoped `Artifacts` adapter (`internal/artifact.Artifacts`) and injects it into
the `InvocationContext`. Tools access artifacts via `ctx.Artifacts()`.
ADK-Go tracks every save in `EventActions.ArtifactDelta` (map of filename →
version), giving the event translation layer visibility into what was created.

ADK-Go also provides `loadartifactstool` — a tool that lists available artifacts
in the system prompt and loads them on demand when the LLM calls
`load_artifacts`.

## Decision

Implement `artifact.Service` backed by S3-compatible storage and wire it into
the ADK runner. Delegate all artifact orchestration to ADK-Go — no custom
artifact management code.

### Backend: `adkartifact.S3Service`

ADK-Go ships an in-memory implementation (testing only) and a GCS implementation.
Neither fits our infrastructure (S3/MinIO). We implement `artifact.Service` in
`platform/artifact/adkartifact/s3.go` using the same MinIO client as the
existing platform artifact store.

**S3 key layout** (matches ADK-Go GCS conventions):

```
{appName}/{userID}/{sessionID}/{fileName}/{version}
```

User-scoped artifacts (`"user:"` prefix) use a well-known session ID:

```
{appName}/{userID}/user/{fileName}/{version}
```

**Versioning strategy:** List objects under the artifact prefix, find max
version, increment. This is the same approach ADK-Go's GCS implementation uses.
The race window on concurrent saves to the same artifact is acceptable — it
matches upstream behaviour, and per-user artifact contention is negligible.

**Scalability for millions of users, hundreds of stateless pods:**

| Concern | Approach |
|---------|----------|
| Zero in-memory state | All data in S3; any pod serves any request |
| S3 partitioning | Keys prefix-partitioned by `appName/userID` — S3 auto-shards |
| Connection pooling | 200 idle connections, HTTP/2, 30s keep-alive |
| Pod statelesness | No local caches, no version counters in memory |
| Bucket isolation | Dedicated bucket (`{bucket}-agent-artifacts`) separates agent artifacts from user uploads |

### Wiring

1. **`RunnerAdapter`** accepts `artifact.Service` as an optional dependency.
2. **`adkrunner.Config.ArtifactService`** receives the service — ADK runner
   handles wrapping, context injection, and delta tracking automatically.
3. **`loadartifactstool`** is injected into both regular agents and workflow
   leaf agents when the artifact service is non-nil.
4. **`translateArtifactDelta()`** in `events.go` converts
   `EventActions.ArtifactDelta` entries into `ArtifactVersionSaved` domain
   events, which the AGUI converter maps to `ARTIFACT_VERSION_SAVED` SSE
   custom events for the frontend.

### Relationship to Existing Artifact Code

Two other artifact-related systems remain and are **not replaced**:

| System | Purpose | Why it stays |
|--------|---------|-------------|
| `platform/artifact.Service` (S3) | User-facing file uploads (resources, attachments, avatars) | Different interface (`io.Reader`-based, no versioning), used by resources module, task handler, health checks |
| A2A artifact markers (`__a2a_artifact__`, `__a2a_file__`) | Compatibility shim for remote A2A agents (Python ADK) | External agents return artifacts through StateDelta, not through the native artifact service |

The new ADK artifact service handles a distinct concern: **agents within the
local ADK runtime creating, versioning, and loading artifacts during execution.**

## Consequences

### Positive

- Agents can produce versioned files (reports, CSVs, images) that persist across
  sessions and are accessible via `loadartifactstool`.
- Zero custom orchestration code — ADK handles context injection, version
  tracking, and tool integration.
- User-scoped artifacts (`"user:"` prefix) are shared across all sessions for a
  user, enabling persistent agent-generated knowledge bases.
- Frontend receives `ARTIFACT_VERSION_SAVED` events for real-time artifact card
  rendering with version history.
- Fully stateless — scales horizontally with no coordination between pods.

### Negative

- Version increment has a theoretical race window on concurrent saves to the
  same artifact (list-then-write). Acceptable: matches ADK-Go upstream, and
  per-user contention is negligible.
- S3 `ListObjects` for version discovery adds latency compared to a database
  counter. Acceptable: artifact saves are infrequent relative to LLM calls, and
  S3 list operations on small key ranges are fast.
- Dedicated bucket (`{bucket}-agent-artifacts`) requires provisioning in
  deployment infrastructure.
