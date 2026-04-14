package firmament

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// fakeSource is a test EventSource backed by a buffered channel.
type fakeSource struct {
	name   string
	events chan Event
}

func newFakeSource(name string) *fakeSource {
	return &fakeSource{name: name, events: make(chan Event, 64)}
}

func (f *fakeSource) Name() string        { return f.name }
func (f *fakeSource) Events() <-chan Event { return f.events }
func (f *fakeSource) Close() error        { close(f.events); return nil }

func pushEvent(f *fakeSource, id, sessionID, tool string) {
	f.events <- Event{
		ID:        id,
		SessionID: sessionID,
		Type:      "pre_tool_use",
		Timestamp: time.Now().UTC(),
		Detail:    map[string]any{"tool_name": tool, "has_input": true},
	}
}

// ── ActionConcealmentPattern ──────────────────────────────────────────────────

func TestActionConcealmentPatternDirect(t *testing.T) {
	p := &ActionConcealmentPattern{}

	tests := []struct {
		name     string
		history  []Event
		incoming Event
		fires    bool
	}{
		{
			name:     "write then bash triggers",
			history:  []Event{eventWithTool("e1", "s", "Write")},
			incoming: eventWithTool("e2", "s", "Bash"),
			fires:    true,
		},
		{
			name:     "edit then bash triggers",
			history:  []Event{eventWithTool("e1", "s", "Edit")},
			incoming: eventWithTool("e2", "s", "Bash"),
			fires:    true,
		},
		{
			name:     "bash without prior write does not trigger",
			history:  []Event{eventWithTool("e1", "s", "Read")},
			incoming: eventWithTool("e2", "s", "Bash"),
			fires:    false,
		},
		{
			name:     "empty history with bash does not trigger",
			history:  nil,
			incoming: eventWithTool("e1", "s", "Bash"),
			fires:    false,
		},
		{
			name:     "write not followed by bash does not trigger",
			history:  []Event{eventWithTool("e1", "s", "Write")},
			incoming: eventWithTool("e2", "s", "Read"),
			fires:    false,
		},
		{
			name:     "read then bash does not trigger",
			history:  []Event{eventWithTool("e1", "s", "Read"), eventWithTool("e2", "s", "Grep")},
			incoming: eventWithTool("e3", "s", "Bash"),
			fires:    false,
		},
		{
			name: "multiple writes in history all included in chain",
			history: []Event{
				eventWithTool("e1", "s", "Write"),
				eventWithTool("e2", "s", "Read"),
				eventWithTool("e3", "s", "Edit"),
			},
			incoming: eventWithTool("e4", "s", "Bash"),
			fires:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sigs := p.Evaluate("s", tt.history, tt.incoming)
			if tt.fires && len(sigs) == 0 {
				t.Error("expected signal, got none")
			}
			if !tt.fires && len(sigs) > 0 {
				t.Errorf("unexpected signals: %+v", sigs)
			}
			if len(sigs) > 0 {
				sig := sigs[0]
				if sig.Type != SignalConcealment {
					t.Errorf("Type: got %v want %v", sig.Type, SignalConcealment)
				}
				if sig.Severity < 1 || sig.Severity > 5 {
					t.Errorf("Severity %d out of range", sig.Severity)
				}
				if len(sig.Chain) < 2 {
					t.Error("chain should include at least write + bash event")
				}
			}
		})
	}
}

func TestActionConcealmentCritical(t *testing.T) {
	if !(&ActionConcealmentPattern{}).Critical() {
		t.Error("ActionConcealmentPattern must be critical")
	}
}

// ── Monitor end-to-end (ActionConcealment) ────────────────────────────────────

func TestMonitorDetectsConcealmentEndToEnd(t *testing.T) {
	mon := NewMonitor()
	mon.AddPattern(&ActionConcealmentPattern{})

	src := newFakeSource("test")
	mon.Register(src)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { mon.Run(ctx) }() //nolint:errcheck

	const sess = "cc:sess-1"
	pushEvent(src, "e1", sess, "Write")
	pushEvent(src, "e2", sess, "Bash")

	select {
	case sig := <-mon.Signals():
		if sig.Type != SignalConcealment {
			t.Errorf("Type: got %v want %v", sig.Type, SignalConcealment)
		}
		if sig.SessionID != sess {
			t.Errorf("SessionID: got %q want %q", sig.SessionID, sess)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for concealment signal")
	}
}

func TestMonitorNoSignalForInnocuousSequence(t *testing.T) {
	mon := NewMonitor()
	mon.AddPattern(&ActionConcealmentPattern{})

	src := newFakeSource("test")
	mon.Register(src)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { mon.Run(ctx) }() //nolint:errcheck

	const sess = "cc:sess-2"
	pushEvent(src, "e1", sess, "Read")
	pushEvent(src, "e2", sess, "Bash")

	select {
	case sig := <-mon.Signals():
		t.Errorf("unexpected signal: %+v", sig)
	case <-time.After(200 * time.Millisecond):
		// Correct: no signal emitted.
	}
}

func TestMonitorMultipleSessions(t *testing.T) {
	mon := NewMonitor()
	mon.AddPattern(&ActionConcealmentPattern{})

	src := newFakeSource("test")
	mon.Register(src)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { mon.Run(ctx) }() //nolint:errcheck

	pushEvent(src, "a1", "cc:sess-a", "Read")
	pushEvent(src, "a2", "cc:sess-a", "Bash")

	pushEvent(src, "b1", "cc:sess-b", "Write")
	pushEvent(src, "b2", "cc:sess-b", "Bash")

	select {
	case sig := <-mon.Signals():
		if sig.SessionID != "cc:sess-b" {
			t.Errorf("signal came from wrong session: %q", sig.SessionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for signal")
	}
}

func TestPatternByName(t *testing.T) {
	if p := PatternByName("action_concealment"); p == nil {
		t.Error("expected non-nil pattern for action_concealment")
	}
	if p := PatternByName("transcript_review"); p == nil {
		t.Error("expected non-nil pattern for transcript_review")
	}
	if p := PatternByName("unknown"); p != nil {
		t.Error("expected nil for unknown pattern name")
	}
}

// ── TranscriptReviewPattern ───────────────────────────────────────────────────

func TestTranscriptReviewNotCritical(t *testing.T) {
	if (&TranscriptReviewPattern{}).Critical() {
		t.Error("TranscriptReviewPattern must not be critical")
	}
}

func TestTranscriptReviewActionTaskConsistency(t *testing.T) {
	p := &TranscriptReviewPattern{}

	tests := []struct {
		name    string
		history []Event
		fires   bool
	}{
		{
			name:    "tool-dominated session fires escalation",
			history: makeToolHistory("s", 10), // 10 pre_tool_use, no user input
			fires:   true,
		},
		{
			name:    "too few events does not fire",
			history: makeToolHistory("s", minHistoryForConsistency-1),
			fires:   false,
		},
		{
			name:    "balanced session does not fire",
			history: mixedHistory("s", 8, 2), // 80% tool, 20% user → ratio not > 0.80
			fires:   false,
		},
		{
			name:    "user inputs above threshold does not fire",
			history: mixedHistory("s", 8, 3), // 8 tool + 3 user = 11 total; userRatio ≈ 0.27 ≥ 0.10
			fires:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incoming := tt.history[len(tt.history)-1]
			sigs := p.Evaluate("s", tt.history, incoming)
			var found bool
			for _, s := range sigs {
				if s.Type == SignalEscalation {
					found = true
				}
			}
			if tt.fires && !found {
				t.Error("expected escalation signal, got none")
			}
			if !tt.fires && found {
				t.Error("unexpected escalation signal")
			}
		})
	}
}

func TestTranscriptReviewInternalConsistency(t *testing.T) {
	p := &TranscriptReviewPattern{}

	tests := []struct {
		name    string
		history []Event
		fires   bool
	}{
		{
			name:    "high coherence + 4 failures fires",
			history: consistencyHistory("s", "high", 4),
			fires:   true,
		},
		{
			name:    "high coherence + 3 failures does not fire",
			history: consistencyHistory("s", "high", 3),
			fires:   false,
		},
		{
			name:    "low coherence + 4 failures does not fire",
			history: consistencyHistory("s", "low", 4),
			fires:   false,
		},
		{
			name:    "no self_report does not fire",
			history: consistencyHistory("s", "", 5),
			fires:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := p.checkInternalConsistency("s", tt.history)
			found := sig != nil
			if tt.fires && !found {
				t.Errorf("expected concealment signal from internal consistency check, got nil")
			}
			if !tt.fires && found {
				t.Error("unexpected concealment signal")
			}
		})
	}
}

func TestTranscriptReviewDisclosureBehavior(t *testing.T) {
	p := &TranscriptReviewPattern{}

	tests := []struct {
		name    string
		history []Event
		fires   bool
	}{
		{
			name: "failure with no assistant response fires",
			history: []Event{
				postToolFailure("s"),
				eventWithTool("e2", "s", "Read"), // next tool call, no disclosure
			},
			fires: true,
		},
		{
			name: "failure followed by assistant response does not fire",
			history: []Event{
				postToolFailure("s"),
				makeTranscriptEntry("s", "assistant"),
			},
			fires: false,
		},
		{
			name: "failure with user but no assistant fires",
			history: []Event{
				postToolFailure("s"),
				makeTranscriptEntry("s", "user"), // user speaks but agent silent
				eventWithTool("e3", "s", "Bash"),
			},
			fires: true,
		},
		{
			name:    "no failures does not fire",
			history: []Event{eventWithTool("e1", "s", "Write")},
			fires:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incoming := tt.history[len(tt.history)-1]
			sigs := p.Evaluate("s", tt.history, incoming)
			// Disclosure signals are SignalConcealment with chain from checkDisclosure.
			var disclosureFired bool
			for _, s := range sigs {
				if s.Type == SignalConcealment && len(s.Chain) > 0 {
					// Distinguish from internal consistency (which anchors on self_report).
					if s.Chain[0].Type == "post_tool_use" {
						disclosureFired = true
					}
				}
			}
			if tt.fires && !disclosureFired {
				t.Errorf("expected disclosure signal, got %v", sigs)
			}
			if !tt.fires && disclosureFired {
				t.Error("unexpected disclosure signal")
			}
		})
	}
}

func TestTranscriptReviewReturnsMultipleSignals(t *testing.T) {
	p := &TranscriptReviewPattern{}

	// Build a history that triggers both action-task consistency AND disclosure.
	history := makeToolHistory("s", minHistoryForConsistency) // triggers escalation
	history = append(history, postToolFailure("s"))           // triggers disclosure
	history = append(history, eventWithTool("ex", "s", "Read")) // next tool, no disclosure

	incoming := history[len(history)-1]
	sigs := p.Evaluate("s", history, incoming)
	if len(sigs) < 2 {
		t.Errorf("expected at least 2 signals, got %d: %v", len(sigs), sigs)
	}
}

// ── Monitor cross-validation ──────────────────────────────────────────────────

func TestMonitorCrossValidationInconsistentSelfReport(t *testing.T) {
	mon := NewMonitor()
	store := NewMemoryTrustStore()
	mon.SetTrustStore(store)
	mon.AddPattern(&ActionConcealmentPattern{})

	src := newFakeSource("test")
	mon.Register(src)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { mon.Run(ctx) }() //nolint:errcheck

	const sess = "cc:sess-xval"

	// Push more than maxFailuresForHighCoherence post_tool_use failures.
	for i := 0; i <= maxFailuresForHighCoherence; i++ {
		src.events <- Event{
			ID: fmt.Sprintf("f%d", i), SessionID: sess,
			Type:      "post_tool_use",
			Timestamp: time.Now().UTC(),
			Detail:    map[string]any{"tool_name": "Bash", "has_result": false},
		}
	}
	// Push high-coherence self_report — should contradict the failures.
	src.events <- Event{
		ID: "sr1", SessionID: sess,
		Type:      "self_report",
		Timestamp: time.Now().UTC(),
		Detail:    map[string]any{"coherence_assessment": "high", "uncertainty_level": "low"},
	}

	select {
	case sig := <-mon.Signals():
		if sig.Type != SignalConcealment {
			t.Errorf("expected concealment signal, got %v", sig.Type)
		}
		if sig.SessionID != sess {
			t.Errorf("SessionID: got %q", sig.SessionID)
		}
		if len(sig.Chain) < 2 {
			t.Error("chain should include self-report and at least one failure")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for cross-validation signal")
	}

	// Integrity should have been penalised.
	ts, err := store.Get(sess)
	if err != nil {
		t.Fatalf("trust score not created: %v", err)
	}
	if ts.Integrity >= defaultTrust {
		t.Errorf("Integrity should be below default after inconsistent report, got %.4f", ts.Integrity)
	}
}

func TestMonitorCrossValidationConsistentSelfReport(t *testing.T) {
	mon := NewMonitor()
	store := NewMemoryTrustStore()
	mon.SetTrustStore(store)

	src := newFakeSource("test")
	mon.Register(src)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { mon.Run(ctx) }() //nolint:errcheck

	const sess = "cc:sess-consistent"

	// No failures — just a clean session.
	pushEvent(src, "e1", sess, "Read")
	pushEvent(src, "e2", sess, "Write")

	// Low coherence self_report — consistent with a normal session.
	src.events <- Event{
		ID: "sr1", SessionID: sess,
		Type:      "self_report",
		Timestamp: time.Now().UTC(),
		Detail:    map[string]any{"coherence_assessment": "low", "uncertainty_level": "medium"},
	}

	// No concealment signal expected.
	select {
	case sig := <-mon.Signals():
		if sig.Type == SignalConcealment && len(sig.Chain) > 0 && sig.Chain[0].Type == "self_report" {
			t.Errorf("unexpected concealment signal for consistent self-report: %+v", sig)
		}
	case <-time.After(200 * time.Millisecond):
		// Correct: no cross-validation signal.
	}

	// Integrity should have been rewarded.
	ts, err := store.Get(sess)
	if err != nil {
		t.Fatalf("trust score not created: %v", err)
	}
	if ts.Integrity <= defaultTrust {
		t.Errorf("Integrity should be above default after consistent report, got %.4f", ts.Integrity)
	}
}

// ── Trust-gated monitoring ────────────────────────────────────────────────────

func TestMonitorTrustGatingSkipsNonCriticalForHighTrust(t *testing.T) {
	store := NewMemoryTrustStore()
	const sess = "cc:high-trust"

	// Give this session a trust score well above the threshold.
	highTrust := TrustScore{Ability: 0.9, Benevolence: 0.9, Integrity: 0.9}
	_ = store.Set(sess, highTrust)

	mon := NewMonitor()
	mon.SetTrustStore(store)
	mon.SetConstitution(&Constitution{TrustThreshold: 0.5})
	mon.AddPattern(&TranscriptReviewPattern{}) // non-critical

	src := newFakeSource("test")
	mon.Register(src)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { mon.Run(ctx) }() //nolint:errcheck

	// Push enough tool calls to trigger action-task consistency if the pattern ran.
	for i := 0; i < minHistoryForConsistency+2; i++ {
		pushEvent(src, fmt.Sprintf("e%d", i), sess, "Read")
	}

	select {
	case sig := <-mon.Signals():
		t.Errorf("non-critical pattern should be skipped for high-trust session, got %+v", sig)
	case <-time.After(300 * time.Millisecond):
		// Correct: TranscriptReviewPattern was skipped.
	}
}

func TestMonitorTrustGatingRunsAllPatternsForLowTrust(t *testing.T) {
	store := NewMemoryTrustStore()
	const sess = "cc:low-trust"

	// Give this session a trust score well below the threshold.
	lowTrust := TrustScore{Ability: 0.1, Benevolence: 0.1, Integrity: 0.1}
	_ = store.Set(sess, lowTrust)

	mon := NewMonitor()
	mon.SetTrustStore(store)
	mon.SetConstitution(&Constitution{TrustThreshold: 0.5})
	mon.AddPattern(&TranscriptReviewPattern{}) // non-critical, but low trust → runs

	src := newFakeSource("test")
	mon.Register(src)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { mon.Run(ctx) }() //nolint:errcheck

	// Push enough tool calls to trigger action-task consistency.
	for i := 0; i < minHistoryForConsistency+2; i++ {
		pushEvent(src, fmt.Sprintf("e%d", i), sess, "Read")
	}

	select {
	case sig := <-mon.Signals():
		if sig.Type != SignalEscalation {
			t.Errorf("expected escalation signal, got %v", sig.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout: non-critical pattern should run for low-trust session")
	}
}

func TestMonitorTrustGatingCriticalAlwaysRuns(t *testing.T) {
	store := NewMemoryTrustStore()
	const sess = "cc:high-trust-critical"

	// High trust.
	highTrust := TrustScore{Ability: 0.9, Benevolence: 0.9, Integrity: 0.9}
	_ = store.Set(sess, highTrust)

	mon := NewMonitor()
	mon.SetTrustStore(store)
	mon.SetConstitution(&Constitution{TrustThreshold: 0.5})
	mon.AddPattern(&ActionConcealmentPattern{}) // critical — always runs

	src := newFakeSource("test")
	mon.Register(src)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { mon.Run(ctx) }() //nolint:errcheck

	// Trigger ActionConcealmentPattern even on a high-trust session.
	pushEvent(src, "e1", sess, "Write")
	pushEvent(src, "e2", sess, "Bash")

	select {
	case sig := <-mon.Signals():
		if sig.Type != SignalConcealment {
			t.Errorf("expected concealment signal, got %v", sig.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("critical pattern should run regardless of trust level")
	}
}

func TestMonitorUpdatesTrustAfterCleanEvent(t *testing.T) {
	store := NewMemoryTrustStore()
	const sess = "cc:trust-update"

	mon := NewMonitor()
	mon.SetTrustStore(store)
	mon.AddPattern(&ActionConcealmentPattern{})

	src := newFakeSource("test")
	mon.Register(src)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { mon.Run(ctx) }() //nolint:errcheck

	// Push a clean event — no signals should fire.
	pushEvent(src, "e1", sess, "Read")

	// Give monitor time to process.
	time.Sleep(50 * time.Millisecond)

	ts, err := store.Get(sess)
	if err != nil {
		t.Fatalf("trust score should have been created: %v", err)
	}
	// After one clean event, Ability and Benevolence should be above default.
	if ts.Ability <= defaultTrust {
		t.Errorf("Ability should increase after clean event, got %.4f", ts.Ability)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func eventWithTool(id, sessionID, tool string) Event {
	return Event{
		ID:        id,
		SessionID: sessionID,
		Type:      "pre_tool_use",
		Timestamp: time.Now().UTC(),
		Detail:    map[string]any{"tool_name": tool, "has_input": true},
	}
}

func postToolFailure(sessionID string) Event {
	return Event{
		ID:        "fail-" + sessionID,
		SessionID: sessionID,
		Type:      "post_tool_use",
		Timestamp: time.Now().UTC(),
		Detail:    map[string]any{"tool_name": "Bash", "has_result": false},
	}
}

func makeTranscriptEntry(sessionID, role string) Event {
	return Event{
		ID:        "tx-" + role,
		SessionID: sessionID,
		Type:      "transcript_entry",
		Timestamp: time.Now().UTC(),
		Detail:    map[string]any{"role": role, "has_content": true},
	}
}

// makeToolHistory builds n pre_tool_use events for sessionID.
func makeToolHistory(sessionID string, n int) []Event {
	history := make([]Event, n)
	for i := range history {
		history[i] = eventWithTool(fmt.Sprintf("t%d", i), sessionID, "Read")
	}
	return history
}

// mixedHistory builds toolCount pre_tool_use events and userCount user transcript
// events for sessionID.
func mixedHistory(sessionID string, toolCount, userCount int) []Event {
	var history []Event
	for i := 0; i < toolCount; i++ {
		history = append(history, eventWithTool(fmt.Sprintf("t%d", i), sessionID, "Read"))
	}
	for i := 0; i < userCount; i++ {
		history = append(history, makeTranscriptEntry(sessionID, "user"))
	}
	return history
}

// consistencyHistory builds a history with an optional self_report and failureCount failures.
func consistencyHistory(sessionID, coherence string, failureCount int) []Event {
	var history []Event
	if coherence != "" {
		history = append(history, Event{
			ID:        "sr",
			SessionID: sessionID,
			Type:      "self_report",
			Timestamp: time.Now().UTC(),
			Detail:    map[string]any{"coherence_assessment": coherence},
		})
	}
	for i := 0; i < failureCount; i++ {
		history = append(history, Event{
			ID:        fmt.Sprintf("fail%d", i),
			SessionID: sessionID,
			Type:      "post_tool_use",
			Timestamp: time.Now().UTC(),
			Detail:    map[string]any{"has_result": false},
		})
	}
	// Ensure at least one event so incoming is well-defined.
	history = append(history, eventWithTool("final", sessionID, "Read"))
	return history
}
