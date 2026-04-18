-- Version: 1.83
-- Description: Add code_execution_policy JSONB column to user_agents. Lets
-- the designer toggle the LLM provider's built-in code executor per agent —
-- Gemini today, OpenAI / Anthropic when their provider adapters gain native
-- code-interpreter support. Nullable — null means code execution is off.
-- Type: ddl
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS code_execution_policy JSONB;
