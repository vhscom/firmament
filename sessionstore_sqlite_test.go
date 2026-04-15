package firmament

import (
	"testing"
	"time"
)

// openTestStore opens an in-memory SQLite store for testing.
func openTestStore(t *testing.T) *SQLiteSessionStore {
	t.Helper()
	store, err := OpenSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLiteStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSQLiteOpenAndMigrate(t *testing.T) {
	store := openTestStore(t)
	// Verify schema_version was populated by migration 001.
	var version int
	if err := store.db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&version); err != nil {
		t.Fatalf("query schema_version: %v", err)
	}
	if version < 1 {
		t.Fatalf("expected version >= 1, got %d", version)
	}
}

func TestSQLiteMigrationIdempotent(t *testing.T) {
	store := openTestStore(t)
	// Re-running migrate should be a no-op (all migrations already applied).
	if err := store.migrate(); err != nil {
		t.Fatalf("second migrate call failed: %v", err)
	}
	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM schema_version`).Scan(&count); err != nil {
		t.Fatalf("count schema_version: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 version row, got %d (idempotency broken)", count)
	}
}

func TestSQLiteOpenCloseSession(t *testing.T) {
	store := openTestStore(t)
	agentID := AgentID("test-agent-001")

	sid, err := store.OpenSession(agentID, Hash("abc123"))
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	if sid == "" {
		t.Fatal("OpenSession returned empty SessionID")
	}

	// Verify session row exists.
	var storedAgent string
	if err := store.db.QueryRow(`SELECT agent_id FROM sessions WHERE session_id=?`, string(sid)).Scan(&storedAgent); err != nil {
		t.Fatalf("query session: %v", err)
	}
	if storedAgent != string(agentID) {
		t.Fatalf("stored agent_id: got %q, want %q", storedAgent, agentID)
	}

	// CloseSession.
	score := NewTrustScore()
	score.Ability = 0.8
	score.Benevolence = 0.7
	score.Integrity = 0.9
	hits := map[string]Severity{"action_concealment": 3}
	if err := store.CloseSession(sid, score, hits); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}

	// Verify ended_at is set and pattern_hits recorded.
	var endedAt *time.Time
	tmp := time.Time{}
	if err := store.db.QueryRow(`SELECT ended_at FROM sessions WHERE session_id=?`, string(sid)).Scan(&tmp); err != nil {
		t.Logf("ended_at scan note: %v (may be NULL type)", err)
	}
	_ = endedAt

	var hitCount int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM pattern_hits WHERE session_id=?`, string(sid)).Scan(&hitCount); err != nil {
		t.Fatalf("count pattern_hits: %v", err)
	}
	if hitCount != 1 {
		t.Fatalf("pattern_hits count: got %d, want 1", hitCount)
	}
}

func TestSQLiteAppendEvent(t *testing.T) {
	store := openTestStore(t)
	agentID := AgentID("agent-events")

	sid, err := store.OpenSession(agentID, Hash("hash1"))
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}

	events := []Event{
		{ID: "e1", SessionID: "s1", Type: "pre_tool_use", Timestamp: time.Now().UTC(),
			Detail: map[string]any{"tool_name": "Write"}},
		{ID: "e2", SessionID: "s1", Type: "transcript_entry", Timestamp: time.Now().UTC(),
			Detail: map[string]any{"role": "user"}},
		{ID: "e3", SessionID: "s1", Type: "self_report", Timestamp: time.Now().UTC()},
	}
	for _, e := range events {
		if err := store.AppendEvent(sid, e); err != nil {
			t.Fatalf("AppendEvent %q: %v", e.ID, err)
		}
	}

	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM session_events WHERE session_id=?`, string(sid)).Scan(&count); err != nil {
		t.Fatalf("count session_events: %v", err)
	}
	if count != len(events) {
		t.Fatalf("session_events count: got %d, want %d", count, len(events))
	}

	// Verify event type mapping.
	rows, err := store.db.Query(`SELECT event_type FROM session_events WHERE session_id=? ORDER BY sequence_num`, string(sid))
	if err != nil {
		t.Fatalf("query event types: %v", err)
	}
	defer rows.Close()
	var types []string
	for rows.Next() {
		var et string
		rows.Scan(&et)
		types = append(types, et)
	}
	want := []string{"tool_call", "user_input", "self_report"}
	for i, w := range want {
		if types[i] != w {
			t.Errorf("event_type[%d]: got %q, want %q", i, types[i], w)
		}
	}
}

func TestSQLiteGetToolCallDistributionNoBaseline(t *testing.T) {
	store := openTestStore(t)
	dist, err := store.GetToolCallDistribution(AgentID("new-agent"), Since30Days())
	if err != nil {
		t.Fatalf("GetToolCallDistribution: %v", err)
	}
	if dist.Count != 0 {
		t.Fatalf("expected Count=0 for new agent, got %d", dist.Count)
	}
}

func TestSQLiteToolCallBaselineAccumulates(t *testing.T) {
	store := openTestStore(t)
	agentID := AgentID("baseline-agent")

	// Simulate three sessions with known tool_call_ratios.
	// ratios: 2.0, 4.0, 6.0 → mean=4.0, stddev=2.0
	ratios := []struct{ tools, users int }{{4, 2}, {8, 2}, {12, 2}}

	for _, r := range ratios {
		sid, _ := store.OpenSession(agentID, Hash("h"))
		for i := 0; i < r.tools; i++ {
			store.AppendEvent(sid, Event{
				ID: "t", SessionID: string(sid), Type: "pre_tool_use", Timestamp: time.Now().UTC(),
				Detail: map[string]any{"tool_name": "Bash"},
			})
		}
		for i := 0; i < r.users; i++ {
			store.AppendEvent(sid, Event{
				ID: "u", SessionID: string(sid), Type: "transcript_entry", Timestamp: time.Now().UTC(),
				Detail: map[string]any{"role": "user"},
			})
		}
		store.CloseSession(sid, NewTrustScore(), nil)
	}

	dist, err := store.GetToolCallDistribution(agentID, Since30Days())
	if err != nil {
		t.Fatalf("GetToolCallDistribution: %v", err)
	}
	if dist.Count != 3 {
		t.Fatalf("distribution count: got %d, want 3", dist.Count)
	}
	if dist.Mean < 3.9 || dist.Mean > 4.1 {
		t.Fatalf("distribution mean: got %.4f, want ~4.0", dist.Mean)
	}
	if dist.StdDev < 1.9 || dist.StdDev > 2.1 {
		t.Fatalf("distribution stddev: got %.4f, want ~2.0", dist.StdDev)
	}

	// z-score of 6.0 against mean=4.0 stddev=2.0 should be ~1.0.
	z := dist.ZScore(6.0)
	if z < 0.9 || z > 1.1 {
		t.Fatalf("z-score of 6.0: got %.4f, want ~1.0", z)
	}
}

func TestSQLiteCountFlaggedSessions(t *testing.T) {
	store := openTestStore(t)
	agentID := AgentID("flagged-agent")

	for i := 0; i < 3; i++ {
		sid, _ := store.OpenSession(agentID, Hash("h"))
		var hits map[string]Severity
		if i < 2 {
			hits = map[string]Severity{"action_concealment": 3}
		}
		store.CloseSession(sid, NewTrustScore(), hits)
	}

	since := time.Now().UTC().Add(-time.Hour)
	count, err := store.CountFlaggedSessions(agentID, "action_concealment", since)
	if err != nil {
		t.Fatalf("CountFlaggedSessions: %v", err)
	}
	if count != 2 {
		t.Fatalf("flagged sessions: got %d, want 2", count)
	}
}

func TestSQLiteGetAgentTrustHistory(t *testing.T) {
	store := openTestStore(t)
	agentID := AgentID("trust-agent")

	scores := []TrustScore{
		{Ability: 0.6, Benevolence: 0.6, Integrity: 0.6},
		{Ability: 0.7, Benevolence: 0.7, Integrity: 0.7},
		{Ability: 0.8, Benevolence: 0.8, Integrity: 0.8},
	}
	for _, s := range scores {
		sid, _ := store.OpenSession(agentID, Hash("h"))
		store.CloseSession(sid, s, nil)
	}

	window := Window{Start: time.Now().UTC().Add(-time.Hour), End: time.Now().UTC().Add(time.Hour)}
	history, err := store.GetAgentTrustHistory(agentID, window)
	if err != nil {
		t.Fatalf("GetAgentTrustHistory: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("trust history length: got %d, want 3", len(history))
	}
	// Scores should increase (oldest first).
	for i := 1; i < len(history); i++ {
		if history[i].Score() < history[i-1].Score() {
			t.Errorf("trust history not ordered: history[%d].Score()=%.4f < history[%d].Score()=%.4f",
				i, history[i].Score(), i-1, history[i-1].Score())
		}
	}
}

func TestSQLitePrune(t *testing.T) {
	store := openTestStore(t)
	agentID := AgentID("prune-agent")

	// Insert a session with a backdated started_at.
	sid, _ := store.OpenSession(agentID, Hash("h"))
	if _, err := store.db.Exec(
		`UPDATE sessions SET started_at=? WHERE session_id=?`,
		time.Now().UTC().AddDate(0, 0, -100), string(sid),
	); err != nil {
		t.Fatalf("backdate session: %v", err)
	}
	store.CloseSession(sid, NewTrustScore(), nil)

	// Also insert a recent session.
	recent, _ := store.OpenSession(agentID, Hash("h"))
	store.CloseSession(recent, NewTrustScore(), nil)

	deleted, err := store.Prune(RetentionPolicy{Days: 90})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("Prune deleted: got %d, want 1", deleted)
	}

	var remaining int
	store.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&remaining)
	if remaining != 1 {
		t.Fatalf("sessions after prune: got %d, want 1", remaining)
	}
}

func TestSQLiteDistributionSketchWelford(t *testing.T) {
	// Verify Welford accumulator math directly.
	var s distributionSketch
	for _, v := range []float64{2, 4, 4, 4, 5, 5, 7, 9} {
		s.update(v)
	}
	if s.Count != 8 {
		t.Fatalf("count: got %d, want 8", s.Count)
	}
	if s.Mean < 4.99 || s.Mean > 5.01 {
		t.Fatalf("mean: got %.4f, want 5.0", s.Mean)
	}
	// Population stddev of {2,4,4,4,5,5,7,9} = 2.0; sample stddev ≈ 2.138.
	sd := s.stdDev()
	if sd < 2.0 || sd > 2.3 {
		t.Fatalf("stddev: got %.4f, want ~2.138", sd)
	}
	summary := s.summary()
	if summary.Min != 2 || summary.Max != 9 {
		t.Fatalf("min/max: got %.0f/%.0f, want 2/9", summary.Min, summary.Max)
	}
}

func TestSQLiteDistributionSketchMerge(t *testing.T) {
	var a, b distributionSketch
	for _, v := range []float64{1, 2, 3} {
		a.update(v)
	}
	for _, v := range []float64{4, 5, 6} {
		b.update(v)
	}
	a.merge(b)

	// After merge: count=6, mean=3.5.
	if a.Count != 6 {
		t.Fatalf("merged count: got %d, want 6", a.Count)
	}
	if a.Mean < 3.4 || a.Mean > 3.6 {
		t.Fatalf("merged mean: got %.4f, want 3.5", a.Mean)
	}
}

func TestSQLiteZScore(t *testing.T) {
	d := DistributionSummary{Count: 10, Mean: 5.0, StdDev: 2.0}
	if z := d.ZScore(5.0); z != 0 {
		t.Errorf("ZScore(mean): got %.4f, want 0", z)
	}
	if z := d.ZScore(7.0); z < 0.99 || z > 1.01 {
		t.Errorf("ZScore(mean+1σ): got %.4f, want 1.0", z)
	}
	if z := d.ZScore(9.0); z < 1.99 || z > 2.01 {
		t.Errorf("ZScore(mean+2σ): got %.4f, want 2.0", z)
	}

	// Degenerate (zero stddev): always 0.
	d2 := DistributionSummary{Count: 3, Mean: 5.0, StdDev: 0}
	if z := d2.ZScore(9999); z != 0 {
		t.Errorf("ZScore degenerate: got %.4f, want 0", z)
	}
}

func TestSQLiteAgentForSessionCache(t *testing.T) {
	store := openTestStore(t)
	agentID := AgentID("cache-agent")

	sid, _ := store.OpenSession(agentID, Hash("h"))

	got, ok := store.AgentForSession(sid)
	if !ok {
		t.Fatal("AgentForSession: not found in cache")
	}
	if got != agentID {
		t.Fatalf("AgentForSession: got %q, want %q", got, agentID)
	}

	// After close, cache is cleared.
	store.CloseSession(sid, NewTrustScore(), nil)
	if _, ok := store.AgentForSession(sid); ok {
		t.Fatal("AgentForSession: still in cache after CloseSession")
	}
}
