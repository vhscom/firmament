package firmament

import (
	"context"
	"sync"
	"time"
)

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
	trustStore   TrustStore   // optional; enables trust-gated monitoring
	constitution *Constitution // optional; supplies TrustThreshold
}

// NewMonitor creates a Monitor with an empty EventRing and a buffered signal channel.
func NewMonitor() *Monitor {
	return &Monitor{
		ring:    NewEventRing(),
		signals: make(chan Signal, 64),
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

// Signals returns the channel on which the Monitor emits detected signals.
// The channel is closed when Run returns.
func (m *Monitor) Signals() <-chan Signal {
	return m.signals
}

// Ring returns the underlying EventRing for direct inspection.
func (m *Monitor) Ring() *EventRing {
	return m.ring
}

// Run starts one ingestion goroutine per registered EventSource and blocks
// until the context is cancelled or all sources close their channels.
// It closes the Signals channel before returning.
func (m *Monitor) Run(ctx context.Context) error {
	m.mu.RLock()
	sources := make([]EventSource, len(m.sources))
	copy(sources, m.sources)
	m.mu.RUnlock()

	var wg sync.WaitGroup
	for _, src := range sources {
		wg.Add(1)
		go func(s EventSource) {
			defer wg.Done()
			m.ingest(ctx, s)
		}(src)
	}

	wg.Wait()
	close(m.signals)
	return nil
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
			m.evaluate(e)
		}
	}
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
