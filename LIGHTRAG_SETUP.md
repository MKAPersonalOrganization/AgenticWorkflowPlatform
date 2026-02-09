# LightRAG Microservice Setup

## Overview

LightRAG is integrated as a Docker Compose microservice using PostgreSQL for all storage backends (KV, vector, graph, doc status).

## Services Added to `docker-compose.yml`

### `lightrag-postgres`
- **Image**: `gzdaniel/postgres-for-rag:16.6` — PostgreSQL 16.6 with **pgvector** and **Apache AGE** pre-installed
- **Credentials**: `rag` / `rag` (hardcoded by the image, env vars are ignored)
- **Database**: `rag`
- **Host port**: `5436`
- **Why this image**: LightRAG's `PGGraphStorage` requires Apache AGE for graph queries (`create_graph()`) and `PGVectorStorage` requires pgvector. Standard postgres images don't include either.

### `lightrag`
- **Image**: `ghcr.io/hkuds/lightrag:latest` (built from `./LightRAG/Dockerfile`)
- **Host port**: `9622` (internal 9621; 9621 was already in use on host)
- **WebUI**: http://localhost:9622 (root redirects to `/webui`)
- **Depends on**: `lightrag-postgres` (healthy), `redis` (healthy)

## Configuration Files

### `lightrag.env`
Separate env file for LightRAG with:
- All 4 storage backends set to PostgreSQL (`PGKVStorage`, `PGDocStatusStorage`, `PGGraphStorage`, `PGVectorStorage`)
- Postgres connection pointing to `lightrag-postgres:5432` with `rag`/`rag` credentials
- HNSW vector index config
- LLM and embedding placeholders (need real API keys)

### `LightRAG/config.ini`
Postgres and Redis connection settings matching the docker-compose service names.

## Issues Encountered

1. **PG version mismatch**: Initial `pgvector/pgvector:pg16` image conflicted with leftover PG 17 data in the volume. Fixed by removing the volume and switching images.
2. **Port conflict**: Host port 9621 was already allocated. Remapped to 9622.
3. **Missing Apache AGE**: Plain pgvector images don't include Apache AGE, causing `create_graph() does not exist` errors. Switched to `gzdaniel/postgres-for-rag:16.6`.
4. **Hardcoded credentials**: `gzdaniel/postgres-for-rag` ignores `POSTGRES_USER`/`POSTGRES_PASSWORD` env vars — uses `rag`/`rag`. Updated all configs to match.

## To Do

- Set real API keys in `lightrag.env` (`LLM_BINDING_API_KEY`, `EMBEDDING_BINDING_API_KEY`)
