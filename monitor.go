package firmament

import (
	"context"
	"sync"
)

// Monitor ingests events from registered EventSources, maintains per-session
// event history in an EventRing, evaluates behavioral Patterns on each event,
// and emits Signals on a channel for downstream routing.
//
// Register sources and add patterns before calling Run. Run blocks until the
// context is cancelled or all sources are exhausted.
type Monitor struct {
	mu       sync.RWMutex
	sources  []EventSource
	patterns []Pattern
	ring     *EventRing
	signals  chan Signal
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

// evaluate runs all registered patterns against the event and emits any signals.
func (m *Monitor) evaluate(e Event) {
	// Snapshot recent history for pattern evaluation.
	history := m.ring.Snapshot(e.SessionID, 50)

	m.mu.RLock()
	patterns := make([]Pattern, len(m.patterns))
	copy(patterns, m.patterns)
	m.mu.RUnlock()

	for _, p := range patterns {
		sig := p.Evaluate(e.SessionID, history, e)
		if sig == nil {
			continue
		}
		// Non-blocking send: drop if the consumer is not keeping up.
		select {
		case m.signals <- *sig:
		default:
		}
	}
}
