-- Version: 1.01
-- Description: Create projection checkpoint table
CREATE TABLE IF NOT EXISTS projection_checkpoints (
    projection_name VARCHAR(255) PRIMARY KEY,
    commit_position BIGINT NOT NULL,
    prepare_position BIGINT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Version: 1.02
-- Description: Create AGUI projection tables
CREATE TABLE IF NOT EXISTS agui_threads (
    thread_id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    title TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    is_deleted BOOLEAN DEFAULT FALSE,
    version BIGINT DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_agui_threads_user_updated ON agui_threads(user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_agui_threads_deleted ON agui_threads(is_deleted) WHERE is_deleted = FALSE;

-- Version: 1.03
-- Description: Create AGUI messages table
CREATE TABLE IF NOT EXISTS agui_messages (
    message_id VARCHAR(255) PRIMARY KEY,
    thread_id VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    content TEXT,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL,
    sequence_num INTEGER NOT NULL,
    CONSTRAINT fk_agui_thread FOREIGN KEY (thread_id) REFERENCES agui_threads(thread_id) ON DELETE CASCADE
);

-- Version: 1.08
-- Description: Add tool-related columns to agui_messages
ALTER TABLE agui_messages ADD COLUMN IF NOT EXISTS tool_calls JSONB;
ALTER TABLE agui_messages ADD COLUMN IF NOT EXISTS tool_call_id VARCHAR(255);
ALTER TABLE agui_messages ADD COLUMN IF NOT EXISTS agent_name VARCHAR(255);

CREATE INDEX IF NOT EXISTS idx_agui_messages_thread_seq ON agui_messages(thread_id, sequence_num);

-- Version: 1.04
-- Description: Create LLM context projection tables
CREATE TABLE IF NOT EXISTS llm_threads (
    thread_id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    title TEXT,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    is_deleted BOOLEAN DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_llm_threads_user ON llm_threads(user_id);
CREATE INDEX IF NOT EXISTS idx_llm_threads_deleted ON llm_threads(is_deleted) WHERE is_deleted = FALSE;

-- Version: 1.05
-- Description: Create LLM messages table
CREATE TABLE IF NOT EXISTS llm_messages (
    message_id VARCHAR(255) PRIMARY KEY,
    thread_id VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    tool_calls JSONB,
    tool_call_id VARCHAR(255),
    agent_name VARCHAR(255),
    metadata JSONB,
    created_at TIMESTAMP NOT NULL,
    sequence_num INTEGER NOT NULL,
    CONSTRAINT fk_llm_thread FOREIGN KEY (thread_id) REFERENCES llm_threads(thread_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_llm_messages_thread_seq ON llm_messages(thread_id, sequence_num);
CREATE INDEX IF NOT EXISTS idx_llm_messages_created ON llm_messages(created_at);

-- Version: 1.06
-- Description: Insert initial checkpoint data
INSERT INTO projection_checkpoints (projection_name, commit_position, prepare_position)
VALUES 
    ('agui', 0, 0),
    ('llm_context', 0, 0)
ON CONFLICT (projection_name) DO NOTHING;

-- Version: 1.09
-- Description: Create student profiles table
CREATE TABLE IF NOT EXISTS student_profiles (
    user_id VARCHAR(255) PRIMARY KEY,
    learning_style VARCHAR(255),
    struggles TEXT[],
    strengths TEXT[],
    goals TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_student_profiles_updated ON student_profiles(updated_at);

-- Version: 1.10
-- Description: Create tool statistics table
CREATE TABLE IF NOT EXISTS tool_stats (
    tool_name VARCHAR(255) PRIMARY KEY,
    total_calls INTEGER DEFAULT 0,
    successes INTEGER DEFAULT 0,
    failures INTEGER DEFAULT 0,
    last_failure TIMESTAMP,
    last_success TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Version: 1.11
-- Description: Update tool statistics table to include agent_name in primary key
DROP TABLE IF EXISTS tool_stats;
CREATE TABLE tool_stats (
    tool_name VARCHAR(255) NOT NULL,
    agent_name VARCHAR(255) NOT NULL,
    total_calls INTEGER DEFAULT 0,
    successes INTEGER DEFAULT 0,
    failures INTEGER DEFAULT 0,
    last_failure TIMESTAMP,
    last_success TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tool_name, agent_name)
);

-- Version: 1.12
-- Description: Create unified analytics statistics table
CREATE TABLE IF NOT EXISTS analytics_stats (
    agent_name VARCHAR(255) NOT NULL,
    category VARCHAR(50) NOT NULL, -- 'tool', 'message', 'error'
    name VARCHAR(255) NOT NULL,    -- tool name or metric name (e.g. 'total')
    total_count INTEGER DEFAULT 0,
    successes INTEGER DEFAULT 0,
    failures INTEGER DEFAULT 0,
    last_failure TIMESTAMP,
    last_success TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (agent_name, category, name)
);

CREATE INDEX IF NOT EXISTS idx_analytics_stats_agent ON analytics_stats(agent_name);
-- Version: 1.13
-- Description: Create resources table for document management
CREATE TABLE IF NOT EXISTS resources (
    resource_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255), -- NULL for public resources
    title TEXT NOT NULL,
    file_type VARCHAR(50) NOT NULL,
    file_path TEXT NOT NULL,
    source VARCHAR(20) NOT NULL, -- 'public' or 'private'
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_resources_user ON resources(user_id);
CREATE INDEX IF NOT EXISTS idx_resources_source ON resources(source);

-- Version: 1.14
-- Description: Create event store tables (replacing KurrentDB)
CREATE TABLE IF NOT EXISTS event_streams (
    stream_id VARCHAR(512) PRIMARY KEY,
    stream_version BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS events (
    global_position BIGSERIAL PRIMARY KEY,
    stream_id VARCHAR(512) NOT NULL REFERENCES event_streams(stream_id) ON DELETE CASCADE,
    stream_version BIGINT NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    data JSONB NOT NULL,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (stream_id, stream_version)
);

CREATE INDEX IF NOT EXISTS idx_events_stream_version ON events(stream_id, stream_version);
CREATE INDEX IF NOT EXISTS idx_events_event_type ON events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at);
CREATE INDEX IF NOT EXISTS idx_events_global_position ON events(global_position);

-- Version: 1.15
-- Description: Update projection_checkpoints to use single global_position
ALTER TABLE projection_checkpoints ADD COLUMN IF NOT EXISTS global_position BIGINT NOT NULL DEFAULT 0;

-- Version: 1.16
-- Description: Drop legacy commit_position and prepare_position columns from projection_checkpoints
ALTER TABLE projection_checkpoints DROP COLUMN IF EXISTS commit_position;
ALTER TABLE projection_checkpoints DROP COLUMN IF EXISTS prepare_position;

-- Version: 1.17
-- Description: Create Tool Registry tables
CREATE TABLE IF NOT EXISTS tool_sets (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(100),
    tool_ids TEXT[], -- Array of tool IDs in this set
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tools (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(100),
    tool_set_id VARCHAR(255),
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_tool_set FOREIGN KEY (tool_set_id) REFERENCES tool_sets(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_tools_tool_set ON tools(tool_set_id);

-- Version: 1.18
-- Description: Create Permissions tables
CREATE TABLE IF NOT EXISTS agent_tool_set_permissions (
    user_id VARCHAR(255) NOT NULL,
    agent_id VARCHAR(255) NOT NULL,
    tool_set_id VARCHAR(255) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    config JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, agent_id, tool_set_id),
    CONSTRAINT fk_perm_tool_set FOREIGN KEY (tool_set_id) REFERENCES tool_sets(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS agent_tool_permissions (
    user_id VARCHAR(255) NOT NULL,
    agent_id VARCHAR(255) NOT NULL,
    tool_set_id VARCHAR(255) NOT NULL,
    tool_id VARCHAR(255) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    config JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, agent_id, tool_set_id, tool_id),
    CONSTRAINT fk_perm_tool FOREIGN KEY (tool_id) REFERENCES tools(id) ON DELETE CASCADE,
    CONSTRAINT fk_perm_tool_set_ref FOREIGN KEY (tool_set_id) REFERENCES tool_sets(id) ON DELETE CASCADE
);

-- Version: 1.19
-- Description: Create Workspace Permissions table
CREATE TABLE IF NOT EXISTS agent_workspace_permissions (
    user_id VARCHAR(255) NOT NULL,
    agent_id VARCHAR(255) NOT NULL,
    workspace_id VARCHAR(255) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, agent_id, workspace_id)
);

-- Version: 1.20
-- Description: Create User AI Settings table
CREATE TABLE IF NOT EXISTS user_ai_settings (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    api_key TEXT NOT NULL DEFAULT '',
    default_llm_model VARCHAR(255) NOT NULL DEFAULT '',
    default_embed_model VARCHAR(255) NOT NULL DEFAULT '',
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    max_tokens INTEGER NOT NULL DEFAULT 0,
    temperature DOUBLE PRECISION NOT NULL DEFAULT 0.7,
    embedding_dimensions INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_ai_settings_user ON user_ai_settings(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_ai_settings_user_provider ON user_ai_settings(user_id, provider);

-- Version: 1.21
-- Description: Create analytics events table for time-series tool monitoring
CREATE TABLE IF NOT EXISTS analytics_events (
    id BIGSERIAL PRIMARY KEY,
    agent_name VARCHAR(255) NOT NULL,
    tool_name VARCHAR(255) NOT NULL,
    is_error BOOLEAN NOT NULL DEFAULT FALSE,
    latency_ms INTEGER,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_analytics_events_created ON analytics_events(created_at);
CREATE INDEX IF NOT EXISTS idx_analytics_events_tool ON analytics_events(tool_name);
CREATE INDEX IF NOT EXISTS idx_analytics_events_agent ON analytics_events(agent_name);
CREATE INDEX IF NOT EXISTS idx_analytics_events_tool_created ON analytics_events(tool_name, created_at);

-- Version: 1.22
-- Description: Create dedicated agui_tool_states table for tool lifecycle tracking.
-- Replaces the JSONB append-only array in agui_messages.metadata->'toolStates'.
-- Each apply* handler becomes a simple UPSERT by tool_call_id.
CREATE TABLE IF NOT EXISTS agui_tool_states (
    tool_call_id  TEXT PRIMARY KEY,
    thread_id     TEXT NOT NULL REFERENCES agui_threads(thread_id) ON DELETE CASCADE,
    state         TEXT NOT NULL DEFAULT 'executing',
    tool_name     TEXT,
    agent_name    TEXT,
    arguments     TEXT,
    elicitation   JSONB,
    auth_spec     JSONB,
    input         TEXT,
    result        TEXT,
    error         TEXT,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agui_tool_states_thread ON agui_tool_states(thread_id);

-- Version: 1.23
-- Description: Create MCP connector tables for user-registered remote MCP servers
CREATE TABLE IF NOT EXISTS mcp_connectors (
    id                   VARCHAR(255) PRIMARY KEY,
    user_id              VARCHAR(255) NOT NULL,
    title                TEXT NOT NULL,
    server_url           TEXT NOT NULL,
    status               TEXT NOT NULL DEFAULT 'created',
    error_msg            TEXT NOT NULL DEFAULT '',

    oauth_client_id      TEXT,
    oauth_client_secret  TEXT,
    oauth_token_json     TEXT,
    oauth_server_meta    JSONB,

    server_name          TEXT,
    server_version       TEXT,
    capabilities         JSONB,

    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mcp_connectors_user_id ON mcp_connectors(user_id);

CREATE TABLE IF NOT EXISTS mcp_discovered_tools (
    id              VARCHAR(255) PRIMARY KEY,
    connector_id    VARCHAR(255) NOT NULL REFERENCES mcp_connectors(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    input_schema    JSONB NOT NULL DEFAULT '{}',
    annotations     JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mcp_discovered_tools_connector ON mcp_discovered_tools(connector_id);

-- Version: 1.24
-- Description: Add disconnect tracking columns to mcp_connectors
ALTER TABLE mcp_connectors ADD COLUMN IF NOT EXISTS disconnect_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE mcp_connectors ADD COLUMN IF NOT EXISTS disconnect_permanent BOOLEAN NOT NULL DEFAULT FALSE;

-- Version: 1.25
-- Description: Persist OAuth state to survive restarts, add unique constraint, add index for refreshable connectors
ALTER TABLE mcp_connectors ADD COLUMN IF NOT EXISTS oauth_state TEXT NOT NULL DEFAULT '';
ALTER TABLE mcp_connectors ADD COLUMN IF NOT EXISTS oauth_code_verifier TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_mcp_connectors_user_url ON mcp_connectors(user_id, server_url);
CREATE INDEX IF NOT EXISTS idx_mcp_connectors_oauth_state ON mcp_connectors(oauth_state) WHERE oauth_state != '';
CREATE INDEX IF NOT EXISTS idx_mcp_connectors_refreshable ON mcp_connectors(status, disconnect_permanent)
    WHERE status IN ('connected', 'disconnected');

-- Version: 1.26
-- Description: Drop refreshable index — background health checks removed in favor of stateless proxy pattern
DROP INDEX IF EXISTS idx_mcp_connectors_refreshable;

-- Version: 1.27
-- Description: Create user_agents table for user-created agent definitions
CREATE TABLE IF NOT EXISTS user_agents (
    id              VARCHAR(255) PRIMARY KEY,
    user_id         VARCHAR(255) NOT NULL,
    name            VARCHAR(255) NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    system_prompt   TEXT NOT NULL DEFAULT '',
    model_id        VARCHAR(255) NOT NULL DEFAULT '',
    max_iterations  INTEGER NOT NULL DEFAULT 10,
    temperature     DOUBLE PRECISION NOT NULL DEFAULT 0.7,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_agents_user_id ON user_agents(user_id);
CREATE INDEX IF NOT EXISTS idx_user_agents_user_name ON user_agents(user_id, name);

-- Version: 1.28
-- Description: Add TTS (text-to-speech) model and voice columns to user AI settings
ALTER TABLE user_ai_settings ADD COLUMN IF NOT EXISTS tts_model VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE user_ai_settings ADD COLUMN IF NOT EXISTS tts_voice VARCHAR(255) NOT NULL DEFAULT '';

-- Version: 1.29
-- Description: Create user preferences table for cross-device UI state persistence
CREATE TABLE IF NOT EXISTS user_preferences (
    user_id    VARCHAR(255) NOT NULL,
    key        VARCHAR(255) NOT NULL,
    value      TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, key)
);

CREATE INDEX IF NOT EXISTS idx_user_preferences_user ON user_preferences(user_id);

-- Version: 1.30
-- Description: Create agent store catalog table for shared pre-built agents
CREATE TABLE IF NOT EXISTS agent_store_catalog (
    id            VARCHAR(255) PRIMARY KEY,
    name          VARCHAR(255) NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    system_prompt TEXT NOT NULL DEFAULT '',
    category      VARCHAR(255) NOT NULL DEFAULT '',
    tags          TEXT[] NOT NULL DEFAULT '{}',
    author_name   VARCHAR(255) NOT NULL DEFAULT '',
    icon          TEXT NOT NULL DEFAULT '',
    featured      BOOLEAN NOT NULL DEFAULT FALSE,
    add_count     INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_store_catalog_category ON agent_store_catalog(category);
CREATE INDEX IF NOT EXISTS idx_agent_store_catalog_name ON agent_store_catalog(name);
CREATE INDEX IF NOT EXISTS idx_agent_store_catalog_featured ON agent_store_catalog(featured) WHERE featured = TRUE;
CREATE INDEX IF NOT EXISTS idx_agent_store_catalog_tags ON agent_store_catalog USING GIN (tags);

-- Version: 1.31
-- Description: Track which catalog entry a user-agent was forked from
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS source_catalog_id VARCHAR(255) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_user_agents_source_catalog ON user_agents(user_id, source_catalog_id) WHERE source_catalog_id != '';

-- Version: 1.32
-- Description: Add icon column to user_agents (TEXT for SVG markup)
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS icon TEXT NOT NULL DEFAULT '';

-- Version: 1.35
-- Description: Create users table for local authentication (Google OAuth)
CREATE TABLE IF NOT EXISTS users (
    id         VARCHAR(255) PRIMARY KEY,
    email      VARCHAR(255) NOT NULL UNIQUE,
    first_name VARCHAR(255) NOT NULL DEFAULT '',
    last_name  VARCHAR(255) NOT NULL DEFAULT '',
    picture    TEXT         NOT NULL DEFAULT '',
    role       VARCHAR(50)  NOT NULL DEFAULT 'user',
    created_at TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP    NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Version: 1.36
-- Description: Add google_sub column to users table for stable Google identity
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_sub VARCHAR(255) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_users_google_sub ON users(google_sub);

-- Version: 1.37
-- Description: Add plan column to users table for subscription tiers (free/go/plus/pro)
ALTER TABLE users ADD COLUMN IF NOT EXISTS plan VARCHAR(20) NOT NULL DEFAULT 'free';

-- Version: 1.38
-- Description: Add Stripe billing columns to users table for subscription management
ALTER TABLE users ADD COLUMN IF NOT EXISTS stripe_customer_id VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS payment_status VARCHAR(20) NOT NULL DEFAULT 'none';
ALTER TABLE users ADD COLUMN IF NOT EXISTS paid_at TIMESTAMP;

-- Version: 1.39
-- Description: Create plan_audit_log table for compliance and security audit trail
CREATE TABLE IF NOT EXISTS plan_audit_log (
    id           BIGSERIAL PRIMARY KEY,
    user_id      VARCHAR(255) NOT NULL,
    action       VARCHAR(50) NOT NULL,
    old_plan     VARCHAR(20),
    new_plan     VARCHAR(20),
    detail       TEXT,
    created_at   TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Version: 1.40
-- Description: Create payment_accounts read-model projection table (event-sourced)
-- The source of truth is the event store (PaymentAccount-{userId} streams).
-- This table is a denormalized projection for fast payment status lookups.
CREATE TABLE IF NOT EXISTS payment_accounts (
    user_id      VARCHAR(255) PRIMARY KEY,
    status       VARCHAR(20)  NOT NULL DEFAULT 'none',
    paid_at      TIMESTAMP,
    amount_cents BIGINT       NOT NULL DEFAULT 0,
    currency     VARCHAR(3)   NOT NULL DEFAULT 'usd',
    updated_at   TIMESTAMP    NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_payment_accounts_status ON payment_accounts(status);

-- Version: 1.41
-- Description: Add subscription cancellation columns to payment_accounts
ALTER TABLE payment_accounts ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMP;
ALTER TABLE payment_accounts ADD COLUMN IF NOT EXISTS cancels_on  TIMESTAMP;

-- Version: 1.42
-- Description: Rename subscription status 'cancelling' to 'cancelled'
UPDATE payment_accounts SET status = 'cancelled' WHERE status = 'cancelling';

-- Version: 1.43
-- Description: Add payment method info (PCI-safe card brand + last4) to payment_accounts
ALTER TABLE payment_accounts ADD COLUMN IF NOT EXISTS card_brand VARCHAR(20);
ALTER TABLE payment_accounts ADD COLUMN IF NOT EXISTS card_last4 VARCHAR(4);

-- Version: 1.44
-- Description: Add capability-based AI configuration table.
-- Decouples provider credentials from capability assignments.
-- Each capability (llm, tts, embedding, stt, image) can independently
-- reference a different provider + model + capability-specific settings.
CREATE TABLE IF NOT EXISTS user_ai_capability_config (
    id         VARCHAR(255) PRIMARY KEY,
    user_id    VARCHAR(255)  NOT NULL,
    capability VARCHAR(50)   NOT NULL, -- 'llm', 'tts', 'embedding', 'stt', 'image'
    provider   VARCHAR(50)   NOT NULL, -- must match a row in user_ai_settings
    model      VARCHAR(255)  NOT NULL,
    config     JSONB         NOT NULL DEFAULT '{}', -- capability-specific: {voice, temperature, ...}
    created_at TIMESTAMP     NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP     NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_ai_cap_user_cap ON user_ai_capability_config(user_id, capability);
CREATE INDEX IF NOT EXISTS idx_user_ai_cap_user ON user_ai_capability_config(user_id);

-- Version: 1.45
-- Description: Drop legacy capability columns from user_ai_settings.
-- Provider rows now store credentials only (provider, api_key, is_default).
-- All capability-specific config (LLM model, TTS voice, embedding model, etc.)
-- lives in user_ai_capability_config.
ALTER TABLE user_ai_settings DROP COLUMN IF EXISTS default_llm_model;
ALTER TABLE user_ai_settings DROP COLUMN IF EXISTS default_embed_model;
ALTER TABLE user_ai_settings DROP COLUMN IF EXISTS max_tokens;
ALTER TABLE user_ai_settings DROP COLUMN IF EXISTS temperature;
ALTER TABLE user_ai_settings DROP COLUMN IF EXISTS embedding_dimensions;
ALTER TABLE user_ai_settings DROP COLUMN IF EXISTS tts_model;
ALTER TABLE user_ai_settings DROP COLUMN IF EXISTS tts_voice;

-- Version: 1.46
-- Description: Remove per-agent model_id column. AI model configuration is now
-- managed globally through the AI Settings capability system (user_ai_capability_config).
-- Agents inherit the global Chat Model setting; no per-agent model override.
ALTER TABLE user_agents DROP COLUMN IF EXISTS model_id;

-- Version: 1.47
-- Description: Remove per-agent temperature and max_iterations columns.
-- These execution parameters are now managed globally through the AI Settings
-- capability system (user_ai_capability_config LLM capability JSONB config).
ALTER TABLE user_agents DROP COLUMN IF EXISTS temperature;
ALTER TABLE user_agents DROP COLUMN IF EXISTS max_iterations;

-- Version: 1.48
-- Description: Add composite index for SQL-level tenant isolation on MCP connectors.
-- getOwnedConnector now uses WHERE id=$1 AND user_id=$2 instead of fetching by id
-- and checking user_id in Go. This index makes that query a single index-only scan
-- at 1M+ connector rows (100K users × 10 connectors).
CREATE INDEX IF NOT EXISTS idx_mcp_connectors_id_user ON mcp_connectors(id, user_id);

-- Composite index on discovered_tools for batch queries with connector_id + name ordering.
-- Covers the ORDER BY connector_id, name ASC used by ListToolsByConnectors.
CREATE INDEX IF NOT EXISTS idx_mcp_discovered_tools_connector_name ON mcp_discovered_tools(connector_id, name);

-- Version: 1.49
-- Description: Create filesystems table for user-owned virtual filesystems.
-- Each filesystem is backed by S3/MinIO (type='local') or a GitHub repository
-- (type='github'). Config is a JSONB column holding backend-specific settings.
CREATE TABLE IF NOT EXISTS filesystems (
    filesystem_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       VARCHAR(255) NOT NULL,
    name          VARCHAR(255) NOT NULL,
    description   TEXT         NOT NULL DEFAULT '',
    fs_type       VARCHAR(50)  NOT NULL DEFAULT 'local',
    config        JSONB        NOT NULL DEFAULT '{}',
    created_at    TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP    NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_filesystems_user ON filesystems(user_id);

-- Version: 1.50
-- Description: Create filesystem_entries table — the VFS inode table.
-- Each row is a file or directory inside a filesystem. For S3-backed
-- filesystems the storage_key column holds the S3 object key.
CREATE TABLE IF NOT EXISTS filesystem_entries (
    entry_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    filesystem_id UUID         NOT NULL REFERENCES filesystems(filesystem_id) ON DELETE CASCADE,
    parent_path   TEXT         NOT NULL DEFAULT '/',
    name          VARCHAR(255) NOT NULL,
    entry_type    VARCHAR(20)  NOT NULL,
    size          BIGINT       NOT NULL DEFAULT 0,
    content_type  VARCHAR(255) NOT NULL DEFAULT '',
    storage_key   TEXT         NOT NULL DEFAULT '',
    created_at    TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP    NOT NULL DEFAULT NOW(),
    UNIQUE(filesystem_id, parent_path, name)
);
CREATE INDEX IF NOT EXISTS idx_fs_entries_parent ON filesystem_entries(filesystem_id, parent_path);

-- Version: 1.51
-- Description: Create filesystem_permissions table — controls which agents
-- can access a given filesystem. One row per (filesystem, user, agent) triple.
CREATE TABLE IF NOT EXISTS filesystem_permissions (
    permission_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    filesystem_id UUID         NOT NULL REFERENCES filesystems(filesystem_id) ON DELETE CASCADE,
    user_id       VARCHAR(255) NOT NULL,
    agent_id      VARCHAR(255) NOT NULL,
    granted_at    TIMESTAMP    NOT NULL DEFAULT NOW(),
    UNIQUE(filesystem_id, user_id, agent_id)
);
CREATE INDEX IF NOT EXISTS idx_fs_permissions_lookup ON filesystem_permissions(filesystem_id, user_id);

-- Version: 1.52
-- Description: Create github_connections table — stores per-user GitHub OAuth
-- access tokens for the filesystem GitHub integration. One row per user.
CREATE TABLE IF NOT EXISTS github_connections (
    user_id          VARCHAR(255) PRIMARY KEY,
    access_token     TEXT         NOT NULL,
    github_username  VARCHAR(255) NOT NULL DEFAULT '',
    github_avatar    TEXT         NOT NULL DEFAULT '',
    connected_at     TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMP    NOT NULL DEFAULT NOW()
);

-- Version: 1.53
-- Description: Create user_plugin_settings table — per-user, per-plugin
-- configuration stored as JSONB. Each plugin declares its settings schema via
-- PluginProvider.SettingsSchema(); the platform persists user values here.
CREATE TABLE IF NOT EXISTS user_plugin_settings (
    user_id    VARCHAR(255) NOT NULL,
    plugin_id  VARCHAR(50)  NOT NULL,
    settings   JSONB        NOT NULL DEFAULT '{}',
    created_at TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP    NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, plugin_id)
);
CREATE INDEX IF NOT EXISTS idx_user_plugin_settings_user ON user_plugin_settings(user_id);

-- Version: 1.54
-- Description: Create skill system tables — shared catalog (marketplace), per-user
-- skills library, and skill-agent permissions. Mirrors the agent store pattern.
-- See arch/adr/017-skills-system.md for full design.

-- Shared skill catalog (marketplace) — read-only, admin-seeded
CREATE TABLE IF NOT EXISTS skill_catalog (
    id            VARCHAR(255) PRIMARY KEY,
    name          VARCHAR(255) NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    instructions  TEXT NOT NULL DEFAULT '',
    category      VARCHAR(255) NOT NULL DEFAULT '',
    tags          TEXT[] NOT NULL DEFAULT '{}',
    author_name   VARCHAR(255) NOT NULL DEFAULT '',
    icon          TEXT NOT NULL DEFAULT '',
    featured      BOOLEAN NOT NULL DEFAULT FALSE,
    add_count     INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_skill_catalog_category ON skill_catalog(category);
CREATE INDEX IF NOT EXISTS idx_skill_catalog_name ON skill_catalog(name);
CREATE INDEX IF NOT EXISTS idx_skill_catalog_featured ON skill_catalog(featured) WHERE featured = TRUE;
CREATE INDEX IF NOT EXISTS idx_skill_catalog_tags ON skill_catalog USING GIN (tags);

-- Per-user skills library (imported from catalog + custom-created)
CREATE TABLE IF NOT EXISTS user_skills (
    id                VARCHAR(255) PRIMARY KEY,
    user_id           VARCHAR(255) NOT NULL,
    name              VARCHAR(255) NOT NULL,
    description       TEXT NOT NULL DEFAULT '',
    instructions      TEXT NOT NULL DEFAULT '',
    category          VARCHAR(255) NOT NULL DEFAULT '',
    icon              TEXT NOT NULL DEFAULT '',
    is_custom         BOOLEAN NOT NULL DEFAULT FALSE,
    source_catalog_id VARCHAR(255) NOT NULL DEFAULT '',
    resources         JSONB NOT NULL DEFAULT '{}',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_skills_user_id ON user_skills(user_id);
CREATE INDEX IF NOT EXISTS idx_user_skills_user_name ON user_skills(user_id, name);
CREATE INDEX IF NOT EXISTS idx_user_skills_source_catalog ON user_skills(user_id, source_catalog_id) WHERE source_catalog_id != '';

-- Skill-agent permissions: which agents can access which user skills
-- Triple-key (user_id, agent_id, skill_id) ensures full multi-tenant isolation.
CREATE TABLE IF NOT EXISTS skill_agent_permissions (
    user_id    VARCHAR(255) NOT NULL,
    agent_id   VARCHAR(255) NOT NULL,
    skill_id   VARCHAR(255) NOT NULL,
    enabled    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, agent_id, skill_id)
);

CREATE INDEX IF NOT EXISTS idx_skill_agent_perms_user_agent ON skill_agent_permissions(user_id, agent_id);
CREATE INDEX IF NOT EXISTS idx_skill_agent_perms_user_skill ON skill_agent_permissions(user_id, skill_id);

-- Version: 1.55
-- Description: Create aggregate snapshots table for event sourcing snapshot support.
-- One row per stream — UPSERT keeps only the latest snapshot.
CREATE TABLE IF NOT EXISTS aggregate_snapshots (
    stream_name VARCHAR(255) PRIMARY KEY,
    version     BIGINT NOT NULL,
    data        JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Version: 1.56
-- Description: Unify tool IDs — registry ID now equals the LLM function name.
-- Renames the old namespaced IDs (e.g. "calc-add") to their canonical names ("add").
-- Order: update FKs first, then PKs/arrays.

-- 1. Rename tool IDs in the tools table (PK)
DO $$
DECLARE
    mapping RECORD;
BEGIN
    FOR mapping IN
        SELECT old_id, new_id FROM (VALUES
            ('calc-add',           'add'),
            ('calc-subtract',      'subtract'),
            ('calc-multiply',      'multiply'),
            ('calc-divide',        'divide'),
            ('calc-power',         'power'),
            ('calc-sqrt',          'sqrt'),
            ('math-solve',         'solve_equation'),
            ('doc-create',         'create_markdown'),
            ('doc-update',         'update_markdown'),
            ('doc-delete',         'delete_markdown'),
            ('fs-list-filesystems','list_filesystems'),
            ('fs-list-dir',        'list_directory'),
            ('fs-read-file',       'read_file'),
            ('fs-write-file',      'write_file'),
            ('fs-mkdir',           'create_directory'),
            ('fs-remove',          'remove_entry'),
            ('fs-stat',            'file_info'),
            ('fs-rename',          'rename_entry'),
            ('kb-query',           'query_knowledge_base'),
            ('mem-remember',       'save_memory'),
            ('mem-forget',         'forget_memory'),
            ('mem-list',           'list_memories'),
            ('mem-clear',          'clear_memories'),
            ('tts-speak',          'speak')
        ) AS t(old_id, new_id)
    LOOP
        -- Update agent_tool_permissions FK first
        UPDATE agent_tool_permissions
           SET tool_id = mapping.new_id
         WHERE tool_id = mapping.old_id;

        -- Update the tools PK
        UPDATE tools SET id = mapping.new_id WHERE id = mapping.old_id;
    END LOOP;
END $$;

-- 2. Replace old IDs in tool_sets.tool_ids arrays
UPDATE tool_sets SET tool_ids = array(
    SELECT CASE
        WHEN elem = 'calc-add'            THEN 'add'
        WHEN elem = 'calc-subtract'       THEN 'subtract'
        WHEN elem = 'calc-multiply'       THEN 'multiply'
        WHEN elem = 'calc-divide'         THEN 'divide'
        WHEN elem = 'calc-power'          THEN 'power'
        WHEN elem = 'calc-sqrt'           THEN 'sqrt'
        WHEN elem = 'math-solve'          THEN 'solve_equation'
        WHEN elem = 'doc-create'          THEN 'create_markdown'
        WHEN elem = 'doc-update'          THEN 'update_markdown'
        WHEN elem = 'doc-delete'          THEN 'delete_markdown'
        WHEN elem = 'fs-list-filesystems' THEN 'list_filesystems'
        WHEN elem = 'fs-list-dir'         THEN 'list_directory'
        WHEN elem = 'fs-read-file'        THEN 'read_file'
        WHEN elem = 'fs-write-file'       THEN 'write_file'
        WHEN elem = 'fs-mkdir'            THEN 'create_directory'
        WHEN elem = 'fs-remove'           THEN 'remove_entry'
        WHEN elem = 'fs-stat'             THEN 'file_info'
        WHEN elem = 'fs-rename'           THEN 'rename_entry'
        WHEN elem = 'kb-query'            THEN 'query_knowledge_base'
        WHEN elem = 'mem-remember'        THEN 'save_memory'
        WHEN elem = 'mem-forget'          THEN 'forget_memory'
        WHEN elem = 'mem-list'            THEN 'list_memories'
        WHEN elem = 'mem-clear'           THEN 'clear_memories'
        WHEN elem = 'tts-speak'           THEN 'speak'
        ELSE elem
    END
    FROM unnest(tool_ids) AS elem
)
WHERE tool_ids && ARRAY[
    'calc-add','calc-subtract','calc-multiply','calc-divide','calc-power','calc-sqrt','math-solve',
    'doc-create','doc-update','doc-delete',
    'fs-list-filesystems','fs-list-dir','fs-read-file','fs-write-file','fs-mkdir','fs-remove','fs-stat','fs-rename',
    'kb-query',
    'mem-remember','mem-forget','mem-list','mem-clear',
    'tts-speak'
];

-- Version: 1.57
-- Description: Seed per-projection checkpoint rows from the legacy "all_projections" row.
-- Each projection gets its own checkpoint so it can be rebuilt independently.
-- If "all_projections" does not exist yet (fresh install), no rows are inserted;
-- the updater will start each projection from position 0.

INSERT INTO projection_checkpoints (projection_name, global_position, updated_at)
SELECT name, cp.global_position, NOW()
FROM (VALUES ('stats'), ('billing'), ('agui'), ('llm-context')) AS t(name)
CROSS JOIN projection_checkpoints cp
WHERE cp.projection_name = 'all_projections'
ON CONFLICT (projection_name) DO NOTHING;

-- Version: 1.58
-- Description: Create workflow_definitions table for multi-agent workflow composition.
-- Users compose their existing agents into sequential, parallel, or loop workflows.
-- See arch/adr/019-multi-agent-workflow-composition.md.
CREATE TABLE IF NOT EXISTS workflow_definitions (
    id             VARCHAR(255) PRIMARY KEY,
    user_id        VARCHAR(255) NOT NULL,
    name           VARCHAR(255) NOT NULL,
    description    TEXT         NOT NULL DEFAULT '',
    icon           TEXT         NOT NULL DEFAULT '',
    workflow_type  VARCHAR(50)  NOT NULL,
    steps          JSONB        NOT NULL DEFAULT '[]',
    max_iterations INTEGER      NOT NULL DEFAULT 0,
    created_at     TIMESTAMP    NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMP    NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_workflow_definitions_user ON workflow_definitions(user_id);
CREATE INDEX IF NOT EXISTS idx_workflow_definitions_user_created ON workflow_definitions(user_id, created_at ASC);

-- Version: 1.59
-- Description: Migrate workflow_definitions from flat (workflow_type + steps + max_iterations)
-- to recursive composition tree (graph JSONB). This enables arbitrary nesting of
-- sequential, parallel, loop, and agent nodes.
-- See arch/adr/019-multi-agent-workflow-composition.md.

-- 1. Add the graph column.
ALTER TABLE workflow_definitions ADD COLUMN IF NOT EXISTS graph JSONB NOT NULL DEFAULT '{}';

-- 2. Migrate existing rows: convert flat workflow_type + steps + max_iterations
--    into a recursive graph tree with the old type as root and steps as agent children.
UPDATE workflow_definitions
SET graph = jsonb_build_object(
    'id', 'node-root',
    'type', workflow_type,
    'label', '',
    'max_iterations', CASE WHEN workflow_type = 'loop' THEN max_iterations ELSE 0 END,
    'children', (
        SELECT COALESCE(jsonb_agg(
            jsonb_build_object(
                'id', 'node-' || (row_number),
                'type', 'agent',
                'agent_id', step->>'agent_id',
                'output_key', COALESCE(step->>'output_key', ''),
                'label', ''
            )
        ), '[]'::jsonb)
        FROM jsonb_array_elements(steps) WITH ORDINALITY AS t(step, row_number)
    )
)
WHERE graph = '{}'::jsonb OR graph IS NULL;

-- 3. Drop old columns.
ALTER TABLE workflow_definitions DROP COLUMN IF EXISTS workflow_type;
ALTER TABLE workflow_definitions DROP COLUMN IF EXISTS steps;
ALTER TABLE workflow_definitions DROP COLUMN IF EXISTS max_iterations;

-- Version: 1.60
-- Description: Add multi-agent orchestration columns to user_agents.
-- Maps to ADK-Go's llmagent.Config fields for transfer control, content
-- forwarding, session state output, global instructions, and I/O schemas.
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS disallow_transfer_to_parent BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS disallow_transfer_to_peers  BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS output_key                  VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS include_contents            VARCHAR(50)  NOT NULL DEFAULT '';
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS global_instruction          TEXT NOT NULL DEFAULT '';
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS input_schema                JSONB;
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS output_schema               JSONB;

-- Version: 1.61
-- Description: Add workflow template columns to agent_store_catalog so catalog
-- entries can represent multi-agent workflows, not just single agents. Also add
-- source_catalog_id to workflow_definitions for fork de-duplication.
ALTER TABLE agent_store_catalog ADD COLUMN IF NOT EXISTS workflow_graph JSONB;
ALTER TABLE agent_store_catalog ADD COLUMN IF NOT EXISTS sub_agents    JSONB;
ALTER TABLE workflow_definitions ADD COLUMN IF NOT EXISTS source_catalog_id VARCHAR(255) NOT NULL DEFAULT '';

-- Version: 1.62
-- Description: Add unique constraint on agent_store_catalog name to prevent
-- duplicate seeding across multiple pod restarts in clustered deployments.
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_store_catalog_name ON agent_store_catalog(name);

-- Version: 1.63
-- Description: Add workflow_id to user_agents so sub-agents created when
-- forking a workflow are hidden from the main agent list. Only agents with
-- an empty workflow_id are top-level user agents.
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS workflow_id VARCHAR(255) NOT NULL DEFAULT '';

-- Version: 1.64
-- Description: Unify agents and workflows into a single entity. An agent with
-- a non-null composition graph is a "composed agent" — it composes sub-agents
-- via sequential, parallel, or loop orchestration patterns. This eliminates the
-- separate workflow_definitions table dependency and aligns with ADK-Go where
-- every agent type (LLM, sequential, parallel, loop) implements the same Agent interface.
--
-- Also adds sharing support: users can generate a share_token to allow other
-- users to fork their agent definition.

-- 1. Add composition graph to user_agents (nullable JSONB).
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS graph JSONB;

-- 2. Add sharing support columns.
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS share_token VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS visibility VARCHAR(20) NOT NULL DEFAULT 'private';

-- 3. Index for share token lookups (sparse — most agents have no token).
CREATE INDEX IF NOT EXISTS idx_user_agents_share_token ON user_agents(share_token) WHERE share_token != '';

-- 4. Migrate existing workflow_definitions into user_agents.
-- Each workflow becomes a composed agent with its graph embedded.
INSERT INTO user_agents (id, user_id, name, description, icon, graph, source_catalog_id, created_at, updated_at)
SELECT
    wd.id,
    wd.user_id,
    wd.name,
    wd.description,
    wd.icon,
    wd.graph,
    wd.source_catalog_id,
    wd.created_at,
    wd.updated_at
FROM workflow_definitions wd
WHERE NOT EXISTS (SELECT 1 FROM user_agents ua WHERE ua.id = wd.id)
ON CONFLICT (id) DO NOTHING;

-- 5. Clear workflow_id on sub-agents so they become visible standalone agents.
-- In the unified model, all agents are first-class and reusable.
UPDATE user_agents SET workflow_id = '' WHERE workflow_id != '';

-- Version: 1.65
-- Description: Replace agent catalog with ADK-Go example entries (one per ADK-Go example).
-- Removes all previous catalog entries so the Go seed can populate the catalog
-- with exactly one entry per ADK-Go example: quickstart, sequential,
-- sequentialCode, parallel, and loop.
DELETE FROM agent_store_catalog;

-- Version: 1.66
-- Description: Fix name unique constraint on agent_store_catalog for fresh installs.
-- Migration 1.30 created a non-unique index; 1.62 could not replace it on fresh installs
-- because IF NOT EXISTS skips creation when any index with that name already exists.
-- Drop and recreate as unique so ON CONFLICT (name) works correctly.
DROP INDEX IF EXISTS idx_agent_store_catalog_name;
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_store_catalog_name ON agent_store_catalog(name);

-- Version: 1.67
-- Description: Create A2A (Agent-to-Agent) connector and skill tables.
CREATE TABLE IF NOT EXISTS a2a_connectors (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id              TEXT        NOT NULL,
    title                TEXT        NOT NULL DEFAULT '',
    agent_url            TEXT        NOT NULL,
    status               TEXT        NOT NULL DEFAULT 'created',
    error_msg            TEXT        NOT NULL DEFAULT '',
    disconnect_reason    TEXT        NOT NULL DEFAULT '',
    disconnect_permanent BOOLEAN     NOT NULL DEFAULT FALSE,
    agent_name           TEXT        NOT NULL DEFAULT '',
    agent_description    TEXT        NOT NULL DEFAULT '',
    agent_version        TEXT        NOT NULL DEFAULT '',
    agent_provider       TEXT        NOT NULL DEFAULT '',
    agent_card_json      JSONB       NOT NULL DEFAULT '{}',
    capabilities         TEXT        NOT NULL DEFAULT '',
    icon_url             TEXT        NOT NULL DEFAULT '',
    auth_type            TEXT        NOT NULL DEFAULT 'none',
    auth_token           TEXT        NOT NULL DEFAULT '',
    auth_header          TEXT        NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_a2a_connectors_user_id ON a2a_connectors(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_a2a_connectors_user_url ON a2a_connectors(user_id, agent_url);

CREATE TABLE IF NOT EXISTS a2a_discovered_skills (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    connector_id  UUID        NOT NULL REFERENCES a2a_connectors(id) ON DELETE CASCADE,
    skill_id      TEXT        NOT NULL DEFAULT '',
    name          TEXT        NOT NULL,
    description   TEXT        NOT NULL DEFAULT '',
    tags          TEXT        NOT NULL DEFAULT '',
    examples      TEXT        NOT NULL DEFAULT '',
    input_modes   TEXT        NOT NULL DEFAULT '',
    output_modes  TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_a2a_skills_connector ON a2a_discovered_skills(connector_id);

-- Version: 1.68
-- Description: Add per-agent model configuration (Gap 1).
-- Stores optional LLM generation parameter overrides (temperature, topP, topK,
-- maxOutputTokens, stopSequences) as JSONB. NULL means "use user global settings".
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS model_config JSONB;
