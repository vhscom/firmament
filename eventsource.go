package firmament

// EventSource is the interface implemented by every agent runtime adapter.
// The Monitor consumes events through this interface and does not interact
// with any runtime protocol directly.
//
// Implementations must close the channel returned by Events when Close is called
// or when the source is exhausted. The Monitor treats a closed channel as
// end-of-stream for that source.
type EventSource interface {
	// Name returns a stable identifier for this source, e.g. "hook" or "ws".
	Name() string

	// Events returns a channel on which the source emits observed events.
	// The channel is closed when the source shuts down.
	Events() <-chan Event

	// Close shuts down the source and causes Events() to be closed.
	Close() error
}
