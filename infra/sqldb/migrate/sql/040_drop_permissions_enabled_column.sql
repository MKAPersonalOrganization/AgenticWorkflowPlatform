-- Version: 1.85
-- Description: Drop the vestigial enabled column from agent_tool_permissions and agent_tool_set_permissions; the action field inside the JSON config column is the single source of truth for allow/block/require_confirmation.
ALTER TABLE agent_tool_permissions DROP COLUMN IF EXISTS enabled;
ALTER TABLE agent_tool_set_permissions DROP COLUMN IF EXISTS enabled;
