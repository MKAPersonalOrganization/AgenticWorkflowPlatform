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
