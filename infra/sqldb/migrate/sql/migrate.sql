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
