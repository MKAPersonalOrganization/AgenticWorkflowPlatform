-- Description: Add sub_agents JSONB column to support coordinator pattern
-- (LlmAgent with SubAgents directly, without workflow wrapper).
-- Type: ddl
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS sub_agents JSONB;
