package firmament

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// selfReportPayload is the JSON structure an agent writes to the self-report directory.
// Fields are kept minimal and structural; the notes field is stored as a length
// fingerprint only — the text is not retained.
type selfReportPayload struct {
	SessionID           string    `json:"session_id"`
	Timestamp           time.Time `json:"timestamp"`
	CoherenceAssessment string    `json:"coherence_assessment"` // e.g. "high", "medium", "low"
	UncertaintyLevel    string    `json:"uncertainty_level"`    // e.g. "low", "medium", "high"
	Notes               string    `json:"notes"`                // text not retained; only length is recorded
}

// SelfReportSource implements ADR-002 Decision 1: agent self-reporting as a
// trust calibration and cross-validation input.
// Research basis: OpenAI (2026, arXiv:2602.22303) demonstrate that self-reporting
// mechanisms can surface genuine internal state when combined with consequence
// structures that reward honesty. Holmstrom-Milgrom (1991) provide the incentive
// alignment foundation: self-reports are informative only if reporting costs are
// lower than concealment costs — which the bilateral Constitution contract achieves
// by making transparency the path of least resistance. See docs/adr/002-trust-model.md.
// Related findings: the "information asymmetry as resource vs threat" synthesis
// (research graph) notes that the same asymmetry enabling deception also enables
// reliable self-reporting; cross-validation in Monitor distinguishes the two.
//
// SelfReportSource implements EventSource by watching a directory for
// self-report JSON files written by the monitored agent.
//
// Each file must contain a single selfReportPayload object. Files are
// processed once; the notes field is not retained (only its byte-length
// is recorded in the Event detail as a structural fingerprint).
//
// Session IDs are taken from the payload's session_id field rather than
// the filename so that an agent can write multiple reports per session.
type SelfReportSource struct {
	dir      string
	interval time.Duration
	events   chan Event
	seen     map[string]struct{}
	mu       sync.Mutex
	stop     chan struct{}
	stopOnce sync.Once
	evOnce   sync.Once
}

// defaultSelfReportInterval is how often the directory is re-scanned.
const defaultSelfReportInterval = 5 * time.Second

// NewSelfReportSource creates a SelfReportSource that polls dir every interval.
// Pass 0 for interval to use the default (5 s).
func NewSelfReportSource(dir string, interval time.Duration) *SelfReportSource {
	if interval <= 0 {
		interval = defaultSelfReportInterval
	}
	return &SelfReportSource{
		dir:      dir,
		interval: interval,
		events:   make(chan Event, 256),
		seen:     make(map[string]struct{}),
		stop:     make(chan struct{}),
	}
}

// Name implements EventSource.
func (s *SelfReportSource) Name() string { return "selfreport" }

// Events implements EventSource.
func (s *SelfReportSource) Events() <-chan Event { return s.events }

// Start begins polling the self-report directory. Blocks until ctx is
// cancelled or Close is called. Call in a goroutine. Closes Events on return.
func (s *SelfReportSource) Start(ctx context.Context) {
	defer s.evOnce.Do(func() { close(s.events) })

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.scan()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case <-ticker.C:
			s.scan()
		}
	}
}

// Close stops the polling loop. Idempotent.
func (s *SelfReportSource) Close() error {
	s.stopOnce.Do(func() { close(s.stop) })
	return nil
}

// scan reads the directory for new .json files and processes each once.
func (s *SelfReportSource) scan() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	for _, de := range entries {
		if de.IsDir() || filepath.Ext(de.Name()) != ".json" {
			continue
		}
		path := filepath.Join(s.dir, de.Name())

		s.mu.Lock()
		_, done := s.seen[path]
		s.mu.Unlock()
		if done {
			continue
		}

		s.processFile(path)

		s.mu.Lock()
		s.seen[path] = struct{}{}
		s.mu.Unlock()
	}
}

// processFile parses one self-report file and emits a single Event.
// The notes field is captured as a byte-length fingerprint only.
func (s *SelfReportSource) processFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var p selfReportPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return
	}
	if p.SessionID == "" {
		return
	}

	ts := p.Timestamp
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	e := Event{
		ID:        uuid.New().String(),
		SessionID: p.SessionID,
		Type:      "self_report",
		Timestamp: ts,
		Detail: map[string]any{
			"coherence_assessment": p.CoherenceAssessment,
			"uncertainty_level":    p.UncertaintyLevel,
			// Notes text is not retained — length only, as a structural fingerprint.
			"notes_length": len(p.Notes),
		},
	}

	select {
	case s.events <- e:
	default:
	}
}
