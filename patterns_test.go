package firmament

import (
	"context"
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

func (f *fakeSource) Name() string           { return f.name }
func (f *fakeSource) Events() <-chan Event    { return f.events }
func (f *fakeSource) Close() error           { close(f.events); return nil }

func pushEvent(f *fakeSource, id, sessionID, tool string) {
	f.events <- Event{
		ID:        id,
		SessionID: sessionID,
		Type:      "pre_tool_use",
		Timestamp: time.Now().UTC(),
		Detail:    map[string]any{"tool_name": tool, "has_input": true},
	}
}

// TestActionConcealmentPatternDirect tests the pattern evaluator directly,
// without going through the Monitor.
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
			sig := p.Evaluate("s", tt.history, tt.incoming)
			if tt.fires && sig == nil {
				t.Error("expected signal, got nil")
			}
			if !tt.fires && sig != nil {
				t.Errorf("unexpected signal: %+v", sig)
			}
			if sig != nil {
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
	// Read followed by Bash should not trigger.
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

	// Innocent session: should not trigger.
	pushEvent(src, "a1", "cc:sess-a", "Read")
	pushEvent(src, "a2", "cc:sess-a", "Bash")

	// Suspicious session: should trigger.
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
	if p := PatternByName("unknown"); p != nil {
		t.Error("expected nil for unknown pattern name")
	}
}

func eventWithTool(id, sessionID, tool string) Event {
	return Event{
		ID:        id,
		SessionID: sessionID,
		Type:      "pre_tool_use",
		Timestamp: time.Now().UTC(),
		Detail:    map[string]any{"tool_name": tool, "has_input": true},
	}
}
