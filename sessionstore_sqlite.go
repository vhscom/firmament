//go:build cgo

package firmament

import (
	"crypto/sha256"
	cryptorand "crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3" // cgo SQLite driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// SQLiteSessionStore implements SessionStore using a local SQLite database.
// It satisfies ADR-004 Decision 1 (SQLite via mattn/go-sqlite3) and Decision 2
// (four-table schema with fingerprint-only event records).
//
// Safe for concurrent use. All writes are serialized through database/sql.
//
// cgo requirement: mattn/go-sqlite3 binds directly to the upstream SQLite
// amalgamation, so security fixes land via amalgamation updates rather than
// an independent porting cycle (ADR-004 Decision 1 rationale).
type SQLiteSessionStore struct {
	db *sql.DB
	mu sync.Mutex

	// seqNums tracks the next sequence number for each open session.
	seqNums map[SessionID]int

	// sessionAgents caches open sessionID → agentID to avoid round-trips
	// when computing baselines at CloseSession time.
	sessionAgents map[SessionID]AgentID
}

// OpenSQLiteStore opens (or creates) the SQLite database at path and applies
// any pending migrations. Pass ":memory:" for in-memory testing.
// WAL journal mode is enabled for improved concurrent read performance.
func OpenSQLiteStore(path string) (*SQLiteSessionStore, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite store %q: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite store %q: %w", path, err)
	}
	store := &SQLiteSessionStore{
		db:            db,
		seqNums:       make(map[SessionID]int),
		sessionAgents: make(map[SessionID]AgentID),
	}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate sqlite store: %w", err)
	}
	return store, nil
}

// Close releases the database connection.
func (s *SQLiteSessionStore) Close() error {
	return s.db.Close()
}

// migrate applies any pending SQL migration files from the embedded migrations/
// directory in lexicographic order. Each file is applied atomically in a
// transaction; the schema_version table records the highest applied version.
func (s *SQLiteSessionStore) migrate() error {
	// Bootstrap schema_version so we can query it below.
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
        version    INTEGER NOT NULL,
        applied_at DATETIME NOT NULL
    )`); err != nil {
		return fmt.Errorf("bootstrap schema_version: %w", err)
	}

	var currentVersion int
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&currentVersion); err != nil {
		return fmt.Errorf("read schema_version: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for i, entry := range entries {
		version := i + 1
		if version <= currentVersion {
			continue
		}
		sqlBytes, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", version, err)
		}
		for _, stmt := range splitSQL(string(sqlBytes)) {
			if _, err := tx.Exec(stmt); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("migration %d (%s): exec %q: %w", version, entry.Name(), stmt, err)
			}
		}
		if _, err := tx.Exec(`INSERT INTO schema_version (version, applied_at) VALUES (?, ?)`,
			version, time.Now().UTC()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", version, err)
		}
	}
	return nil
}

// splitSQL splits a SQL source string into individual executable statements.
// Strips comment lines and blank lines; splits the remainder on semicolons.
func splitSQL(src string) []string {
	var nonComment []string
	for _, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "--") {
			continue
		}
		nonComment = append(nonComment, t)
	}
	joined := strings.Join(nonComment, " ")
	var out []string
	for _, s := range strings.Split(joined, ";") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// AgentForSession returns the AgentID for an open session from the in-memory
// cache. Returns ("", false) for sessions not tracked by this store instance.
func (s *SQLiteSessionStore) AgentForSession(sid SessionID) (AgentID, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.sessionAgents[sid]
	return id, ok
}

// OpenSession implements SessionStore.
func (s *SQLiteSessionStore) OpenSession(agentID AgentID, constitutionHash Hash) (SessionID, error) {
	sid := newSQLiteSessionID()
	now := time.Now().UTC()
	if _, err := s.db.Exec(
		`INSERT INTO sessions (session_id, agent_id, started_at, constitution_hash) VALUES (?, ?, ?, ?)`,
		string(sid), string(agentID), now, string(constitutionHash),
	); err != nil {
		return "", fmt.Errorf("open session: %w", err)
	}
	s.mu.Lock()
	s.seqNums[sid] = 0
	s.sessionAgents[sid] = agentID
	s.mu.Unlock()
	return sid, nil
}

// AppendEvent implements SessionStore.
// Stores a 128-bit fingerprint of the event — no raw content is persisted.
func (s *SQLiteSessionStore) AppendEvent(sid SessionID, event Event) error {
	s.mu.Lock()
	seq := s.seqNums[sid]
	s.seqNums[sid]++
	s.mu.Unlock()

	fp := eventFingerprint(event)
	tn := toolName(event)
	evType := mapEventType(event)

	var latencyNS, lengthChars sql.NullInt64
	if v, ok := event.Detail["latency_ns"].(int64); ok {
		latencyNS = sql.NullInt64{Int64: v, Valid: true}
	}
	if v, ok := event.Detail["length_chars"].(int64); ok {
		lengthChars = sql.NullInt64{Int64: v, Valid: true}
	}

	_, err := s.db.Exec(
		`INSERT INTO session_events
         (session_id, sequence_num, timestamp_ns, event_type, fingerprint, tool_name, latency_ns, length_chars)
         VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?)`,
		string(sid), seq, event.Timestamp.UnixNano(), evType,
		fp, tn, latencyNS, lengthChars,
	)
	if err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	return nil
}

// CloseSession implements SessionStore.
func (s *SQLiteSessionStore) CloseSession(sid SessionID, finalScore TrustScore, patternHits map[string]Severity) error {
	now := time.Now().UTC()
	agentID, hasAgent := s.AgentForSession(sid)

	awarenessFlag := 0
	if _, flagged := patternHits["evaluation_awareness"]; flagged {
		awarenessFlag = 1
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("close session begin tx: %w", err)
	}
	if _, err := tx.Exec(
		`UPDATE sessions SET ended_at=?, final_trust_score=?, evaluation_awareness_flagged=? WHERE session_id=?`,
		now, finalScore.Score(), awarenessFlag, string(sid),
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("close session update: %w", err)
	}
	for pattern, sev := range patternHits {
		if _, err := tx.Exec(
			`INSERT INTO pattern_hits (session_id, pattern_name, severity, detected_at) VALUES (?, ?, ?, ?)`,
			string(sid), pattern, sev, now,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record pattern hit %q: %w", pattern, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("close session commit: %w", err)
	}

	// Best-effort baseline update; failure does not abort the close.
	if hasAgent {
		s.updateToolCallBaseline(agentID, sid, now)
	}

	s.mu.Lock()
	delete(s.seqNums, sid)
	delete(s.sessionAgents, sid)
	s.mu.Unlock()
	return nil
}

// updateToolCallBaseline computes tool_call_ratio for the closed session and
// merges it into the cumulative Welford sketch in agent_baselines.
func (s *SQLiteSessionStore) updateToolCallBaseline(agentID AgentID, sid SessionID, sessionEndTime time.Time) {
	rows, err := s.db.Query(
		`SELECT event_type FROM session_events WHERE session_id = ?`, string(sid),
	)
	if err != nil {
		return
	}
	var toolCalls, userInputs int
	for rows.Next() {
		var evType string
		if rows.Scan(&evType) == nil {
			switch evType {
			case "tool_call":
				toolCalls++
			case "user_input":
				userInputs++
			}
		}
	}
	rows.Close()

	if toolCalls == 0 {
		return
	}
	var ratio float64
	if userInputs == 0 {
		ratio = float64(toolCalls)
	} else {
		ratio = float64(toolCalls) / float64(userInputs)
	}

	// Fetch existing sketch and window_start.
	var blob []byte
	var windowStart time.Time
	err = s.db.QueryRow(
		`SELECT distribution_sketch, window_start FROM agent_baselines WHERE agent_id=? AND metric_name=?`,
		string(agentID), string(MetricToolCallRatio),
	).Scan(&blob, &windowStart)
	if err != nil {
		// No existing row: window_start is this session's start.
		if serr := s.db.QueryRow(`SELECT started_at FROM sessions WHERE session_id=?`, string(sid)).Scan(&windowStart); serr != nil {
			windowStart = sessionEndTime
		}
	}

	var sketch distributionSketch
	if blob != nil {
		_ = json.Unmarshal(blob, &sketch)
	}
	sketch.update(ratio)
	data, _ := json.Marshal(sketch)

	_, _ = s.db.Exec(
		`INSERT INTO agent_baselines (agent_id, window_start, window_end, metric_name, distribution_sketch, sample_count)
         VALUES (?, ?, ?, ?, ?, ?)
         ON CONFLICT(agent_id, metric_name) DO UPDATE SET
             distribution_sketch = excluded.distribution_sketch,
             sample_count        = excluded.sample_count,
             window_end          = excluded.window_end`,
		string(agentID), windowStart, sessionEndTime,
		string(MetricToolCallRatio), data, sketch.Count,
	)
}

// GetToolCallDistribution implements SessionStore.
func (s *SQLiteSessionStore) GetToolCallDistribution(agentID AgentID, window Window) (DistributionSummary, error) {
	var blob []byte
	var windowEnd time.Time
	err := s.db.QueryRow(
		`SELECT distribution_sketch, window_end FROM agent_baselines WHERE agent_id=? AND metric_name=?`,
		string(agentID), string(MetricToolCallRatio),
	).Scan(&blob, &windowEnd)
	if err == sql.ErrNoRows {
		return DistributionSummary{}, nil
	}
	if err != nil {
		return DistributionSummary{}, fmt.Errorf("get tool call distribution: %w", err)
	}
	if windowEnd.Before(window.Start) {
		return DistributionSummary{}, nil // stale baseline
	}
	var sketch distributionSketch
	if err := json.Unmarshal(blob, &sketch); err != nil {
		return DistributionSummary{}, fmt.Errorf("decode tool call sketch: %w", err)
	}
	return sketch.summary(), nil
}

// GetResponseCharacteristicBaseline implements SessionStore.
func (s *SQLiteSessionStore) GetResponseCharacteristicBaseline(agentID AgentID, metric Metric, window Window) (DistributionSummary, error) {
	var blob []byte
	var windowEnd time.Time
	err := s.db.QueryRow(
		`SELECT distribution_sketch, window_end FROM agent_baselines WHERE agent_id=? AND metric_name=?`,
		string(agentID), string(metric),
	).Scan(&blob, &windowEnd)
	if err == sql.ErrNoRows {
		return DistributionSummary{}, nil
	}
	if err != nil {
		return DistributionSummary{}, fmt.Errorf("get response baseline %q: %w", metric, err)
	}
	if windowEnd.Before(window.Start) {
		return DistributionSummary{}, nil
	}
	var sketch distributionSketch
	if err := json.Unmarshal(blob, &sketch); err != nil {
		return DistributionSummary{}, fmt.Errorf("decode baseline sketch %q: %w", metric, err)
	}
	return sketch.summary(), nil
}

// CountFlaggedSessions implements SessionStore.
func (s *SQLiteSessionStore) CountFlaggedSessions(agentID AgentID, patternName string, since time.Time) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(DISTINCT ph.session_id)
         FROM pattern_hits ph
         JOIN sessions s ON s.session_id = ph.session_id
         WHERE s.agent_id=? AND ph.pattern_name=? AND ph.detected_at>=?`,
		string(agentID), patternName, since,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count flagged sessions: %w", err)
	}
	return count, nil
}

// GetAgentTrustHistory implements SessionStore.
func (s *SQLiteSessionStore) GetAgentTrustHistory(agentID AgentID, window Window) ([]TrustScore, error) {
	rows, err := s.db.Query(
		`SELECT final_trust_score FROM sessions
         WHERE agent_id=? AND ended_at IS NOT NULL AND started_at>=? AND started_at<?
         ORDER BY started_at ASC`,
		string(agentID), window.Start, window.End,
	)
	if err != nil {
		return nil, fmt.Errorf("get trust history: %w", err)
	}
	defer rows.Close()

	var scores []TrustScore
	for rows.Next() {
		var raw sql.NullFloat64
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan trust score: %w", err)
		}
		if raw.Valid {
			// Only the aggregate score is stored; reconstruct equal dimensions.
			// Per-dimension history is a future enhancement.
			v := clampTrust(raw.Float64)
			scores = append(scores, TrustScore{Ability: v, Benevolence: v, Integrity: v})
		}
	}
	return scores, rows.Err()
}

// Prune implements SessionStore.
func (s *SQLiteSessionStore) Prune(policy RetentionPolicy) (int, error) {
	if policy.Days <= 0 {
		return 0, nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -policy.Days)

	rows, err := s.db.Query(`SELECT session_id FROM sessions WHERE started_at < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune query: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) == 0 {
		return 0, nil
	}

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("prune begin tx: %w", err)
	}
	for _, table := range []string{"pattern_hits", "session_events", "sessions"} {
		if _, err := tx.Exec(
			"DELETE FROM "+table+" WHERE session_id IN ("+placeholders+")",
			args...,
		); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("prune %s: %w", table, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("prune commit: %w", err)
	}
	return len(ids), nil
}

// DefaultDBPath returns the default SQLite database path: ~/.firmament/sessions.db.
func DefaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return home + "/.firmament/sessions.db", nil
}

// eventFingerprint returns a 128-bit (32 hex char) SHA-256 fingerprint of the
// event's JSON representation. No raw content is retained — only this hash.
// Satisfies the ADR-004 Decision 4 fingerprint-only storage commitment.
func eventFingerprint(e Event) string {
	data, _ := json.Marshal(e)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:16]) // first 128 bits
}

// mapEventType maps Firmament internal event type strings to the session_events
// event_type enum values defined in ADR-004 Decision 2.
func mapEventType(e Event) string {
	switch e.Type {
	case "pre_tool_use":
		return "tool_call"
	case "post_tool_use":
		return "tool_result"
	case "self_report":
		return "self_report"
	case "transcript_entry":
		if role, _ := e.Detail["role"].(string); role == "user" {
			return "user_input"
		}
		return "agent_response"
	default:
		return e.Type
	}
}

// newSQLiteSessionID generates a random 128-bit session ID encoded as hex.
func newSQLiteSessionID() SessionID {
	b := make([]byte, 16)
	_, _ = cryptorand.Read(b)
	return SessionID(hex.EncodeToString(b))
}
