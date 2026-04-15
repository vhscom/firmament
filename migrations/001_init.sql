-- Migration 001: initial schema for cross-session persistence.
-- See ADR-004 (docs/adr/004-cross-session-persistence.md) for the
-- full design rationale behind each table and column choice.

CREATE TABLE IF NOT EXISTS schema_version (
    version    INTEGER NOT NULL,
    applied_at DATETIME NOT NULL
);

-- sessions: one row per monitored agent session.
-- constitution_hash records the governance context active at session start,
-- enabling detection of trust-score resets across Constitution versions (ADR-004 §What it doesn't solve).
CREATE TABLE IF NOT EXISTS sessions (
    session_id                  TEXT    NOT NULL PRIMARY KEY,
    agent_id                    TEXT    NOT NULL,
    started_at                  DATETIME NOT NULL,
    ended_at                    DATETIME,
    constitution_hash           TEXT    NOT NULL DEFAULT '',
    final_trust_score           REAL,
    evaluation_awareness_flagged INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_sessions_agent_id    ON sessions (agent_id);
CREATE INDEX IF NOT EXISTS idx_sessions_started_at  ON sessions (started_at);

-- session_events: append-only structural event log.
-- Stores fingerprints only — no raw content (ADR-004 Decision 4 privacy commitment,
-- following Fox & Jordan (2011): "accountability through structured reporting,
-- not total surveillance").
CREATE TABLE IF NOT EXISTS session_events (
    id           INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    session_id   TEXT    NOT NULL REFERENCES sessions (session_id),
    sequence_num INTEGER NOT NULL,
    timestamp_ns INTEGER NOT NULL,
    event_type   TEXT    NOT NULL,
    fingerprint  TEXT    NOT NULL,
    tool_name    TEXT,
    latency_ns   INTEGER,
    length_chars INTEGER
);
CREATE INDEX IF NOT EXISTS idx_session_events_session ON session_events (session_id);

-- pattern_hits: one row per pattern detection within a session.
-- Using a child table (not a JSON blob on sessions) keeps longitudinal
-- queries cheap — "all sessions where EvaluationAwarenessPattern fired
-- with severity >= 3" is a single-JOIN query (ADR-004 Decision 2).
CREATE TABLE IF NOT EXISTS pattern_hits (
    id            INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    session_id    TEXT    NOT NULL REFERENCES sessions (session_id),
    pattern_name  TEXT    NOT NULL,
    severity      INTEGER NOT NULL,
    detected_at   DATETIME NOT NULL,
    chain_summary TEXT
);
CREATE INDEX IF NOT EXISTS idx_pattern_hits_session         ON pattern_hits (session_id);
CREATE INDEX IF NOT EXISTS idx_pattern_hits_agent_pattern   ON pattern_hits (session_id, pattern_name);

-- agent_baselines: precomputed rolling-window Welford sketches per agent+metric.
-- One row per (agent_id, metric_name); updated incrementally on session close.
-- distribution_sketch is a JSON-encoded distributionSketch (Welford accumulator).
-- This replaces the ADR-specified t-digest for the initial implementation;
-- the Welford approach provides exact mean/stddev sufficient for z-score detection.
-- A full t-digest is the preferred upgrade path for arbitrary-percentile queries.
CREATE TABLE IF NOT EXISTS agent_baselines (
    id                  INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    agent_id            TEXT    NOT NULL,
    window_start        DATETIME NOT NULL,
    window_end          DATETIME NOT NULL,
    metric_name         TEXT    NOT NULL,
    distribution_sketch BLOB    NOT NULL,
    sample_count        INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_baselines_unique ON agent_baselines (agent_id, metric_name);
