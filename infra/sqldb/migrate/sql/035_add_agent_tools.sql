-- Version: 1.80
-- Description: Add agent_tools JSONB column to support the agent-as-tool pattern
-- (wrap another agent via ADK-Go's tool/agenttool so the parent can call it).
-- Type: ddl
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS agent_tools JSONB;
