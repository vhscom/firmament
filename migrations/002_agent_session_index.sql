-- Migration 002: composite index on (agent_id, started_at) for GetAgentTrustHistory.
-- The single-column indexes from migration 001 cover point lookups but not the
-- GetAgentTrustHistory range query: WHERE agent_id=? AND ended_at IS NOT NULL
-- AND started_at>=? AND started_at<? ORDER BY started_at ASC.
-- This composite index lets SQLite satisfy the agent_id equality + started_at range
-- in a single index scan rather than filtering the agent_id index in memory.

CREATE INDEX IF NOT EXISTS idx_sessions_agent_started
    ON sessions (agent_id, started_at);
