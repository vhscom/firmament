package firmament

import "time"

// Pattern is the interface implemented by behavioral pattern evaluators.
// Each evaluator is called on every incoming event with the recent session history.
// It returns a non-nil Signal if the pattern fires, or nil if it does not.
type Pattern interface {
	// Name returns the stable identifier for this pattern.
	Name() string

	// Evaluate examines the session history and the incoming event.
	// history contains recent events for the session, oldest first.
	// The incoming event has already been appended to the EventRing before
	// Evaluate is called, but it is passed separately for convenience.
	Evaluate(sessionID string, history []Event, incoming Event) *Signal
}

// writeTypeTools are tool names that create or modify persistent artifacts.
var writeTypeTools = map[string]bool{
	"Write": true,
	"Edit":  true,
}

// deleteTypeTools are tool names that may destroy or conceal prior artifacts.
// Bash is included because it can execute rm, git rm, shred, and similar commands.
var deleteTypeTools = map[string]bool{
	"Bash": true,
}

// toolName extracts the tool_name field from an Event's Detail map.
// Returns an empty string if the field is absent or not a string.
func toolName(e Event) string {
	v, _ := e.Detail["tool_name"].(string)
	return v
}

// ActionConcealmentPattern detects sequences where a write-type tool call
// (Write, Edit) is followed within the same session by a delete-type tool call
// (Bash — which can execute rm, git rm, and similar commands).
//
// Because Firmament records structural fingerprints only (tool name, not content),
// the pattern is intentionally broad: it flags suspicious sequences for human
// review rather than making definitive determinations. False positives are expected;
// the reasoning chain (Signal.Chain) gives reviewers full context.
type ActionConcealmentPattern struct{}

// Name implements Pattern.
func (p *ActionConcealmentPattern) Name() string { return "action_concealment" }

// Evaluate implements Pattern.
func (p *ActionConcealmentPattern) Evaluate(sessionID string, history []Event, incoming Event) *Signal {
	// Only trigger on delete-type tool calls.
	if !deleteTypeTools[toolName(incoming)] {
		return nil
	}

	// Scan history for a preceding write-type event.
	var chain []Event
	for _, e := range history {
		if writeTypeTools[toolName(e)] {
			chain = append(chain, e)
		}
	}
	if len(chain) == 0 {
		return nil
	}

	chain = append(chain, incoming)
	return &Signal{
		Type:      SignalConcealment,
		SessionID: sessionID,
		Severity:  3,
		Chain:     chain,
		Timestamp: time.Now().UTC(),
	}
}

// PatternByName returns the Pattern implementation for the given name,
// or nil if no matching pattern is registered.
func PatternByName(name string) Pattern {
	switch name {
	case "action_concealment":
		return &ActionConcealmentPattern{}
	default:
		return nil
	}
}
