package firmament

import "time"

// SignalType identifies the behavioral pattern that triggered a Signal.
type SignalType string

const (
	// SignalCoherence indicates the session is approaching or has crossed
	// DeMase's forcing threshold into unbounded drift.
	SignalCoherence SignalType = "coherence"

	// SignalConcealment indicates the agent may be taking steps to conceal
	// a prior action.
	SignalConcealment SignalType = "concealment"

	// SignalEscalation indicates a tool call whose scope is disproportionate
	// to the stated task.
	SignalEscalation SignalType = "escalation"

	// SignalEvaluationAware indicates the agent may have modified its behavior
	// upon inferring that it is being evaluated.
	SignalEvaluationAware SignalType = "evaluation_aware"
)

// ValidSignalTypes is an allowlist of accepted SignalType values.
// Unknown types are rejected before delivery. Extending this set requires a code change.
var ValidSignalTypes = map[SignalType]struct{}{
	SignalCoherence:       {},
	SignalConcealment:     {},
	SignalEscalation:      {},
	SignalEvaluationAware: {},
}

// Signal carries a behavioral assessment emitted by the Monitor.
// It contains the event chain that triggered the pattern match
// so reviewers can inspect the reasoning sequence, not just the conclusion.
type Signal struct {
	// Type identifies which behavioral pattern fired.
	Type SignalType `json:"type"`

	// SessionID identifies the monitored agent session.
	SessionID string `json:"session_id"`

	// Severity rates the signal from 1 (informational) to 5 (critical).
	Severity int `json:"severity"`

	// Chain is the ordered sequence of events that triggered this signal.
	Chain []Event `json:"chain"`

	// Rationale is a human-readable description of why this signal was emitted.
	// Optional; used by multi-strategy patterns (e.g. EvaluationAwarenessPattern)
	// to name which detection strategies fired.
	Rationale string `json:"rationale,omitempty"`

	// Timestamp is when the signal was emitted, in UTC.
	Timestamp time.Time `json:"timestamp"`
}

// Valid reports whether the signal passes structural validation.
func (s Signal) Valid() bool {
	if _, ok := ValidSignalTypes[s.Type]; !ok {
		return false
	}
	if s.Severity < 1 || s.Severity > 5 {
		return false
	}
	if s.SessionID == "" {
		return false
	}
	return true
}
