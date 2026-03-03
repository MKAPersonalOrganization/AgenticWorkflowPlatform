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
    icon_label    VARCHAR(50) NOT NULL DEFAULT '',
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
-- Description: Add icon column to user_agents
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS icon VARCHAR(50) NOT NULL DEFAULT '';

-- Version: 1.33
-- Description: Widen icon columns to TEXT for SVG markup; re-seed catalog with professional icons
ALTER TABLE agent_store_catalog ALTER COLUMN icon_label TYPE TEXT;
ALTER TABLE user_agents ALTER COLUMN icon TYPE TEXT;
TRUNCATE TABLE agent_store_catalog;

-- Version: 1.34
-- Description: Backfill user agent icons with SVGs from re-seeded catalog (match by name)
UPDATE user_agents ua
   SET icon = cat.icon_label
  FROM agent_store_catalog cat
 WHERE LOWER(TRIM(ua.name)) = LOWER(TRIM(cat.name))
   AND ua.icon NOT LIKE '<svg%'
   AND cat.icon_label LIKE '<svg%';

-- Remaining agents (custom-created or renamed) get the default bot icon
UPDATE user_agents
   SET icon = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8V4H8"/><rect width="16" height="12" x="4" y="8" rx="2"/><path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/></svg>'
 WHERE icon NOT LIKE '<svg%';

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
