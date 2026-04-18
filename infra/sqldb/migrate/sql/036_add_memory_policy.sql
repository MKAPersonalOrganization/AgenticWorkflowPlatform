-- Version: 1.81
-- Description: Add memory_policy JSONB column to user_agents. Replaces the
-- legacy memory-set toolset-permission gate (which reused the plugin
-- permission system for a service-level concern). Nullable; when null the
-- runner adapter falls back to the legacy permission check so existing
-- agents keep working during the migration window.
-- Type: ddl
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS memory_policy JSONB;
