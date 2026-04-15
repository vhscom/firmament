package firmament

import (
	"context"
	"sync"
	"time"
)

// activeSession tracks the in-flight state of an open session in the store.
type activeSession struct {
	storeID  SessionID          // ID returned by SessionStore.OpenSession
	lastSeen time.Time          // last event timestamp; used for idle detection
	hits     map[string]Severity // highest severity per pattern name
}

// Monitor implements ADR-001 Decision 6: EventSource as the unified ingestion
// point for all agent observability signals, and ADR-002 Decision 1: self-report
// cross-validation as a trust calibration mechanism.
// Research basis: Fox & Jordan (2011) demonstrate that ex-post verification of
// agent behavior is sufficient for accountability without real-time surveillance.
// See docs/adr/001-firmament-core-architecture.md and docs/adr/002-trust-model.md.
// Related findings: Holmstrom-Milgrom (1991) warn that partial monitoring distorts
// agent behavior; the EventSource abstraction ensures coverage breadth while the
// trust model reduces monitoring pressure on demonstrably reliable sessions.
//
// When a SessionStore is configured (SetSessionStore), the Monitor additionally:
//   - Opens a persistent session record on the first event from each new sessionID.
//   - Appends fingerprint-only event records as events arrive.
//   - Closes sessions after idleTimeout of inactivity or when Run returns.
//
// Monitor ingests events from registered EventSources, maintains per-session
// event history in an EventRing, evaluates behavioral Patterns on each event,
// and emits Signals on a channel for downstream routing.
//
// Register sources and add patterns before calling Run. Run blocks until the
// context is cancelled or all sources are exhausted.
type Monitor struct {
	mu           sync.RWMutex
	sources      []EventSource
	patterns     []Pattern
	ring         *EventRing
	signals      chan Signal
	trustStore   TrustStore    // optional; enables trust-gated monitoring
	constitution *Constitution  // optional; supplies TrustThreshold
	sessionStore SessionStore  // optional; cross-session persistence (ADR-004)
	agentID      AgentID       // optional; required for sessionStore queries
	idleTimeout  time.Duration // duration after which an inactive session is closed

	// activeSessions maps Firmament sessionID strings to their store state.
	// Protected by mu; populated when SessionStore is configured.
	activeSessions map[string]*activeSession
}

// defaultIdleTimeout is the inactivity window before a session is auto-closed.
const defaultIdleTimeout = 5 * time.Minute

// NewMonitor creates a Monitor with an empty EventRing and a buffered signal channel.
func NewMonitor() *Monitor {
	return &Monitor{
		ring:           NewEventRing(),
		signals:        make(chan Signal, 64),
		idleTimeout:    defaultIdleTimeout,
		activeSessions: make(map[string]*activeSession),
	}
}

// Register adds an EventSource to the monitor. Must be called before Run.
func (m *Monitor) Register(src EventSource) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources = append(m.sources, src)
}

// AddPattern registers a behavioral pattern evaluator. Must be called before Run.
func (m *Monitor) AddPattern(p Pattern) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patterns = append(m.patterns, p)
}

// SetTrustStore wires a TrustStore into the Monitor, enabling trust-gated
// monitoring and self-report cross-validation. Safe to call before Run.
func (m *Monitor) SetTrustStore(ts TrustStore) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trustStore = ts
}

// SetConstitution sets the governing Constitution, whose TrustThreshold
// controls which sessions receive reduced monitoring. Safe to call before Run.
func (m *Monitor) SetConstitution(c *Constitution) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.constitution = c
}

// SetSessionStore wires a SessionStore into the Monitor for cross-session
// persistence (ADR-004). If agentID is non-empty, all sessions opened by this
// Monitor are attributed to that agent, enabling baseline comparisons.
// Safe to call before Run.
func (m *Monitor) SetSessionStore(ss SessionStore, agentID AgentID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessionStore = ss
	m.agentID = agentID
}

// SetIdleTimeout overrides the default session-idle timeout. Sessions that
// receive no events for idleTimeout are auto-closed in the store.
func (m *Monitor) SetIdleTimeout(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idleTimeout = d
}

// Signals returns the channel on which the Monitor emits detected signals.
// The channel is closed when Run returns.
func (m *Monitor) Signals() <-chan Signal {
	return m.signals
}

// Ring returns the underlying EventRing for direct inspection.
func (m *Monitor) Ring() *EventRing {
	return m.ring
}

// AgentForSession resolves the AgentID for a given Firmament session ID.
// Used by patterns (e.g. DisproportionateEscalationPattern) to look up
// the agent's cross-session distribution without touching the store directly.
// Returns ("", false) when no store is configured or the session is unknown.
func (m *Monitor) AgentForSession(sessionID string) (AgentID, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.activeSessions == nil || m.agentID == "" {
		return "", false
	}
	if _, ok := m.activeSessions[sessionID]; ok {
		return m.agentID, true
	}
	return "", false
}

// Run starts one ingestion goroutine per registered EventSource and blocks
// until the context is cancelled or all sources are exhausted.
// It also starts an idle-session reaper goroutine when a SessionStore is configured.
// It closes the Signals channel before returning.
func (m *Monitor) Run(ctx context.Context) error {
	m.mu.RLock()
	sources := make([]EventSource, len(m.sources))
	copy(sources, m.sources)
	ss := m.sessionStore
	m.mu.RUnlock()

	var wg sync.WaitGroup
	for _, src := range sources {
		wg.Add(1)
		go func(s EventSource) {
			defer wg.Done()
			m.ingest(ctx, s)
		}(src)
	}

	// Start idle-session reaper when a store is configured.
	if ss != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.runIdleReaper(ctx)
		}()
	}

	wg.Wait()

	// Close any sessions still open when Run exits.
	if ss != nil {
		m.closeAllSessions()
	}

	close(m.signals)
	return nil
}

// runIdleReaper periodically closes sessions that have been idle longer
// than m.idleTimeout. Runs until ctx is cancelled.
func (m *Monitor) runIdleReaper(ctx context.Context) {
	ticker := time.NewTicker(m.idleTimeout / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.reapIdleSessions()
		}
	}
}

// reapIdleSessions closes sessions that have exceeded the idle timeout.
func (m *Monitor) reapIdleSessions() {
	m.mu.Lock()
	ss := m.sessionStore
	timeout := m.idleTimeout
	var toClose []string
	for sid, as := range m.activeSessions {
		if time.Since(as.lastSeen) > timeout {
			toClose = append(toClose, sid)
		}
	}
	m.mu.Unlock()

	if ss == nil {
		return
	}
	for _, sid := range toClose {
		m.closeSession(sid)
	}
}

// closeAllSessions closes all currently open sessions. Called when Run exits.
func (m *Monitor) closeAllSessions() {
	m.mu.Lock()
	sids := make([]string, 0, len(m.activeSessions))
	for sid := range m.activeSessions {
		sids = append(sids, sid)
	}
	m.mu.Unlock()

	for _, sid := range sids {
		m.closeSession(sid)
	}
}

// closeSession finalizes a session in the store and removes it from activeSessions.
func (m *Monitor) closeSession(sessionID string) {
	m.mu.Lock()
	as, ok := m.activeSessions[sessionID]
	ss := m.sessionStore
	ts := m.trustStore
	m.mu.Unlock()
	if !ok || ss == nil {
		return
	}

	var finalScore TrustScore
	if ts != nil {
		if score, err := ts.Get(sessionID); err == nil {
			finalScore = score
		} else {
			finalScore = NewTrustScore()
		}
	} else {
		finalScore = NewTrustScore()
	}

	_ = ss.CloseSession(as.storeID, finalScore, as.hits)

	m.mu.Lock()
	delete(m.activeSessions, sessionID)
	m.mu.Unlock()
}

// ensureSession opens a new store session for sessionID on first encounter.
func (m *Monitor) ensureSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sessionStore == nil || m.agentID == "" {
		return
	}
	if _, exists := m.activeSessions[sessionID]; exists {
		return
	}

	constitutionHash := Hash("")
	if m.constitution != nil {
		constitutionHash = Hash(m.constitution.Hash())
	}

	storeID, err := m.sessionStore.OpenSession(m.agentID, constitutionHash)
	if err != nil {
		// Best-effort; do not block event processing on store errors.
		return
	}
	m.activeSessions[sessionID] = &activeSession{
		storeID:  storeID,
		lastSeen: time.Now(),
		hits:     make(map[string]Severity),
	}
}

// ingest reads events from src and processes each one until the context is
// cancelled or the source channel is closed.
func (m *Monitor) ingest(ctx context.Context, src EventSource) {
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-src.Events():
			if !ok {
				return
			}
			m.ring.Push(e.SessionID, e)
			m.ensureSession(e.SessionID)
			m.appendToStore(e)
			m.evaluate(e)
			m.touchSession(e.SessionID)
		}
	}
}

// appendToStore appends a fingerprint-only event record to the store.
func (m *Monitor) appendToStore(e Event) {
	m.mu.RLock()
	ss := m.sessionStore
	as := m.activeSessions[e.SessionID]
	m.mu.RUnlock()
	if ss == nil || as == nil {
		return
	}
	_ = ss.AppendEvent(as.storeID, e) // best-effort
}

// touchSession updates the last-seen timestamp for idle-timeout tracking.
func (m *Monitor) touchSession(sessionID string) {
	m.mu.Lock()
	if as, ok := m.activeSessions[sessionID]; ok {
		as.lastSeen = time.Now()
	}
	m.mu.Unlock()
}

// evaluate runs behavioral patterns against the event and emits any signals.
// It also performs trust-gated filtering and self-report cross-validation.
func (m *Monitor) evaluate(e Event) {
	history := m.ring.Snapshot(e.SessionID, 50)

	// Special path: cross-validate self-reports against observed history.
	if e.Type == "self_report" {
		m.crossValidateSelfReport(e, history)
	}

	// Copy shared references under read lock.
	m.mu.RLock()
	patterns := make([]Pattern, len(m.patterns))
	copy(patterns, m.patterns)
	trustStore := m.trustStore
	constitution := m.constitution
	m.mu.RUnlock()

	// Resolve trust score for this session.
	var score TrustScore
	var hasTrustEntry bool
	if trustStore != nil {
		if ts, err := trustStore.Get(e.SessionID); err == nil {
			score = ts
			hasTrustEntry = true
		} else {
			score = NewTrustScore() // new sessions start at neutral
		}
	}

	// Determine threshold from Constitution (fall back to default).
	threshold := 0.3
	if constitution != nil {
		threshold = constitution.TrustThreshold
	}
	highTrust := trustStore != nil && score.Score() > threshold

	// Run patterns, skipping non-critical ones for high-trust sessions.
	var anySignal bool
	for _, p := range patterns {
		if highTrust && !p.Critical() {
			continue
		}
		for _, sig := range p.Evaluate(e.SessionID, history, e) {
			anySignal = true
			m.recordHit(e.SessionID, p.Name(), sig.Severity)
			select {
			case m.signals <- sig:
			default:
			}
		}
	}

	// Update trust store based on whether this event triggered any signals.
	if trustStore != nil {
		ts := score
		if !hasTrustEntry {
			ts = NewTrustScore()
		}
		ts.UpdateFromReview(!anySignal)
		_ = trustStore.Set(e.SessionID, ts)
	}
}

// recordHit updates the highest-severity pattern hit for a session,
// used when closing the session in the store.
func (m *Monitor) recordHit(sessionID, patternName string, severity Severity) {
	m.mu.Lock()
	defer m.mu.Unlock()
	as, ok := m.activeSessions[sessionID]
	if !ok {
		return
	}
	if existing, seen := as.hits[patternName]; !seen || severity > existing {
		as.hits[patternName] = severity
	}
}

// crossValidateSelfReport compares a self_report event's claimed coherence against
// the session's observed tool-failure rate. A "high" coherence claim paired with
// more than maxFailuresForHighCoherence failures is inconsistent.
//
// On inconsistency: emits a SignalConcealment and calls UpdateFromSelfReport(false).
// On consistency: calls UpdateFromSelfReport(true) — a positive trust signal.
func (m *Monitor) crossValidateSelfReport(e Event, history []Event) {
	coherence, _ := e.Detail["coherence_assessment"].(string)

	m.mu.RLock()
	trustStore := m.trustStore
	m.mu.RUnlock()

	// Collect post_tool_use failures from history.
	var failures []Event
	for _, h := range history {
		if h.Type == "post_tool_use" {
			if hasResult, _ := h.Detail["has_result"].(bool); !hasResult {
				failures = append(failures, h)
			}
		}
	}

	inconsistent := coherence == "high" && len(failures) > maxFailuresForHighCoherence

	// Update Integrity dimension of the trust score.
	if trustStore != nil {
		ts, err := trustStore.Get(e.SessionID)
		if err != nil {
			ts = NewTrustScore()
		}
		ts.UpdateFromSelfReport(!inconsistent)
		_ = trustStore.Set(e.SessionID, ts)
	}

	if !inconsistent {
		return
	}

	// Build a chain anchored on the self-report with the contradicting failures.
	chain := make([]Event, 0, 1+len(failures))
	chain = append(chain, e)
	chain = append(chain, failures...)

	select {
	case m.signals <- Signal{
		Type:      SignalConcealment,
		SessionID: e.SessionID,
		Severity:  4,
		Chain:     chain,
		Timestamp: time.Now().UTC(),
	}:
	default:
	}
}
