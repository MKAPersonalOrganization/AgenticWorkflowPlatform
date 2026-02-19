# Local Development → Production Gap Analysis

> What in this codebase **works for local dev but will break, bottleneck, or become a security risk** at millions-of-users scale.

---




## 3 · MinIO Masquerading as S3

**File:** `docker-compose.yml` (minio service) · `main.go:347-357`

- Local MinIO runs with default credentials (`minio` / `miniosecret`).
- Bucket is set to **public** anonymous access (`mc anonymous set public`).
- SSL is disabled (`S3UseSSL=false`).

**Why it breaks:** Public anonymous buckets expose all student artifacts to the internet. MinIO is single-node with no replication.

**Fix:**
- Use **AWS S3** (or GCS / Azure Blob) with IAM roles — no static keys.
- Enable SSL (`S3UseSSL=true`).
- Remove the `mc anonymous set public` step; serve files through **pre-signed URLs** from the app.

---

## 4 · Kratos Running in Dev Mode with Insecure Secrets

**File:** `docker-compose.yml:95` · `kratos/kratos.yml:95-99`

```yaml
# docker-compose.yml
command: serve -c /etc/config/kratos/kratos.yml --dev --watch-courier

# kratos.yml
secrets:
  cookie:
    - PLEASE-CHANGE-ME-I-AM-VERY-INSECURE
  cipher:
    - 32-LONG-SECRET-NOT-SECURE-AT-ALL
```

- `--dev` disables security checks and enables unsafe behaviours.
- Hardcoded cookie/cipher secrets mean **any attacker can forge sessions**.

**Fix:**
- Remove `--dev` flag.
- Inject secrets via environment variables or a secrets manager (Vault, AWS Secrets Manager).

---

## 5 · Hardcoded Localhost URLs Everywhere

All Kratos flow URLs in `kratos.yml` and the app config point to `http://localhost:8090`:

```yaml
default_browser_return_url: http://localhost:8090/
ui_url: http://localhost:8090/auth/login
# ... 10+ more occurrences
```

**Fix:** Templatise with environment variables or separate `kratos.production.yml`.

---

## 6 · Database TLS Disabled

**File:** `main.go:132` · `.env.example:84`

```
SCHOOL_DB_DISABLE_TLS=true
```

All traffic between the app and Postgres is in **cleartext**, including credentials and PII.

**Fix:** Enable TLS (`DisableTLS=false`) and configure certificate verification in production.

---

## 7 · Single-Instance Redis with No Authentication

**File:** `docker-compose.yml:3-15`

```yaml
redis:
  image: redis:7-alpine
  ports:
    - "6379:6379"     # exposed to host
```

- No `requirepass` — anyone on the network can read/write.
- No persistence configuration tuned for production (AOF/RDB).
- Single instance — no Sentinel/Cluster for HA.

**Fix:**
- Set a strong password.
- Use **Redis Sentinel** or **Redis Cluster** (or a managed service like ElastiCache).
- Remove the host port binding.

---

## 8 · Low Database Connection Pool Limits

**File:** `main.go:130-131`

```go
MaxIdleConns int `conf:"default:2"`
MaxOpenConns int `conf:"default:25"`
```

With millions of users and concurrent AI agent tasks, **25 open connections** will saturate instantly, causing request queuing and timeouts.

**Fix:** Tune to hundreds of connections per pod, or use **PgBouncer** as a connection pooler in front of Postgres.

---

## 9 · ~~No HTTPS / TLS Termination~~ ✅ DONE

`ListenAndServeTLS` is now used with TLS config restricted to assembly-optimised curves (X25519, P256). Self-signed cert for local dev; replace with a real cert or terminate at a reverse proxy in production.

---

## 10 · Debug Server Exposed on 0.0.0.0

**File:** `main.go:112`

```go
DebugHost string `conf:"default:0.0.0.0:4010"`
```

The debug server (pprof, expvar) listens on **all interfaces**. In production it leaks heap dumps, goroutine profiling data, and internal metrics to anyone.

**Fix:** Bind to `127.0.0.1` or firewall the port — or disable the debug server entirely in production builds.

---

## 11 · Static Assets Served from Local Filesystem

**File:** `main.go:564`

```go
r.Mount("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))
```

Serving static files from the Go binary is fine for dev but adds unnecessary load to the app server in production.

**Fix:** Serve assets from a **CDN** (CloudFront, Cloudflare) or an object store bucket with a CDN in front.

---

## 12 · MailSlurper for Email

**File:** `docker-compose.yml:64-70` · `kratos/kratos.yml:117`

```yaml
connection_uri: smtps://test:test@mailslurper:1025/?skip_ssl_verify=true
from_address: noreply@schoolwithai.local
```

MailSlurper is a dev-only email sink — emails are never actually delivered.

**Fix:** Configure a production SMTP relay (SES, SendGrid, Postmark) with proper DNS (`SPF`, `DKIM`, `DMARC`).

---

## 13 · Fire-and-Forget User Sync Goroutine

**File:** `main.go:785-809`

```go
go func(u *domain.User) {
    // ... sync user to RAG
}(user)
```

This goroutine runs with no tracking, no retry, and no back-pressure. At scale, thousands of goroutines can pile up; if they all fail, there is no visibility.

**Fix:** Enqueue user sync as a **durable Asynq task** with retry and dead-letter support, instead of a raw goroutine.

---

## 14 · `exec.Command("dot")` for Graph Rendering

**File:** `main.go:891-901`

The `renderGraph` function shells out to the `dot` binary (Graphviz). This:
- Assumes Graphviz is installed on the production image.
- Is a potential **command-injection** vector if input is not sanitised.
- Creates a child process per request, which does not scale.

**Fix:** Use a Go-native graph layout library, or pre-render graphs and cache them.

---

## 15 · Dev-Only Monitoring Tools in docker-compose

| Service | Purpose | Production Issue |
|---------|---------|-----------------|
| `asynqmon` | Asynq dashboard | Exposed with no auth on `:8080` |
| `redis-insight` | Redis GUI | Exposed on `:5540`, no auth |
| `edgequake-web` | Edgequake admin UI | Exposed on `:3002` |
| `ollama-socat` | Local LLM proxy | `network_mode: host`, only for local Ollama |

**Fix:** Remove or move to a separate `docker-compose.override.yml` for dev. In production, use managed monitoring (Grafana, Datadog) with proper auth.

---

## 16 · Single-Node Postgres (No Replication / Backups)

All three Postgres instances (`postgres`, `app-postgres`, `edgequake-db`) run as single Docker containers with local volumes.

**Fix:**
- Use a managed Postgres service (RDS, Cloud SQL) with automated backups, read replicas, and failover.
- Or deploy a Postgres cluster (Patroni, Citus).

---

## 17 · Kratos DB Credentials Hardcoded

**File:** `docker-compose.yml:49-50, 75, 93`

```yaml
POSTGRES_PASSWORD=secret
DSN=postgres://kratos:secret@postgres:5432/kratos?sslmode=disable
```

Plaintext, committed to the repo.

**Fix:** Inject all secrets via environment variables or a secrets manager at deploy time. Never commit secrets.

---

## 18 · Dashboard Admin Protected by Basic Auth Only

**File:** `main.go:122-124`

```go
DashboardAdmin struct {
    User string `conf:"default:admin"`
    Pass string `conf:"default:admin,mask"`
}
```

Basic Auth with default `admin/admin` credentials over HTTP. No MFA, no session management.

**Fix:** Put the analytics dashboard behind the same Kratos auth (or an internal SSO), and enforce MFA for admin accounts.

---

## Summary Priority Matrix

| Priority | Issue | Risk | Status |
|----------|-------|------|--------|
| 🔴 Critical | Kratos dev mode + insecure secrets (#4) | Session forgery | ✅ Fixed (`--dev` removed, secrets via env) |
| 🔴 Critical | No HTTPS (#9) | Data interception | ✅ Fixed (`ListenAndServeTLS`) |
| 🔴 Critical | MinIO public bucket + no SSL (#3) | Data exposure | ✅ Fixed (private bucket) |
| 🔴 Critical | Hardcoded DB credentials (#17) | Credential leak | ✅ Fixed (all via env vars) |
| 🟠 High | In-memory repositories (#1) | Data loss + inconsistency | ✅ Fixed (removed) |
| 🟠 High | Database TLS disabled (#6) | PII in cleartext | ⏳ Set `SCHOOL_DB_DISABLE_TLS=false` in prod |
| 🟠 High | Redis no auth (#7) | Unauthorised access | ✅ Fixed (`requirepass`, no host port) |
| 🟠 High | Debug server on 0.0.0.0 (#10) | Internal data leak | ✅ Fixed (default `127.0.0.1`) |
| 🟡 Medium | In-process rate limiter (#2) | Rate limit bypass | ✅ Fixed (Redis-backed distributed) |
| 🟡 Medium | Low DB pool limits (#8) | Request timeouts | ✅ Fixed (tuned defaults) |
| 🟡 Medium | Static assets from app (#11) | Wasted compute | ⏳ Use CDN in production |
| 🟡 Medium | MailSlurper (#12) | No emails delivered | ⏳ Use SES/SendGrid in production |
| 🟡 Medium | Fire-and-forget sync (#13) | Silent failures | ⏳ Enqueue as Asynq task |
| 🟡 Medium | exec.Command for graphs (#14) | Security + scalability | ⏳ Use native Go library |
| 🟡 Medium | Dev monitoring tools (#15) | Unauthenticated dashboards | ✅ Fixed (moved to override file) |
| 🟢 Low | Single-node Postgres (#16) | No HA / disaster recovery | ⏳ Use managed Postgres in production |
| 🟢 Low | Basic Auth dashboard (#18) | Weak admin access | ⏳ Enforce SSO + MFA in production |
