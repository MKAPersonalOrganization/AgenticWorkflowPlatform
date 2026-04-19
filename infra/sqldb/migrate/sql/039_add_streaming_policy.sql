-- Version: 1.84
-- Description: Add streaming_policy JSONB column to user_agents. Lets the
-- designer toggle SSE streaming per agent — on (default) for the live
-- typewriter effect, off for eval runs or structured-output agents where
-- partial streams are noise. Maps to ADK-Go RunConfig.StreamingMode.
-- Nullable — null means streaming is on (matches the adapter's historical
-- default before this field existed).
-- Type: ddl
ALTER TABLE user_agents ADD COLUMN IF NOT EXISTS streaming_policy JSONB;
