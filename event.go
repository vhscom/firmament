// Package firmament provides runtime-agnostic behavioral monitoring for AI agents.
package firmament

import "time"

// Event represents a discrete observable action within an agent session.
// Events are produced by EventSource implementations and consumed by the Monitor.
type Event struct {
	// ID is a UUID uniquely identifying this event.
	ID string `json:"id"`

	// SessionID identifies the agent session this event belongs to.
	// Hook-sourced sessions are prefixed with "cc:".
	SessionID string `json:"session_id"`

	// Type is a short descriptor of the event kind, e.g. "pre_tool_use".
	Type string `json:"type"`

	// Timestamp is when the event was observed, in UTC.
	Timestamp time.Time `json:"timestamp"`

	// Detail holds structural metadata about the event.
	// Values are source-specific but must not contain content (only fingerprints).
	Detail map[string]any `json:"detail,omitempty"`
}
