-- Version: 1.82
-- Description: Add artifact_policy JSONB column to user_agents. Replaces the
-- legacy artifacts-set toolset-permission gate with a proper per-agent feature
-- flag that also carries the SaveInputBlobsAsArtifacts choice. Nullable — null
-- means artifacts are off for the agent; non-null carries the explicit policy.
-- Type: ddl
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS artifact_policy JSONB;
