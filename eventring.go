package firmament

import "sync"

// defaultRingCapacity is the number of events retained per session.
const defaultRingCapacity = 512

// EventRing is a thread-safe, per-session ring buffer of recent events.
// Each session retains at most capacity events; older events are silently dropped
// as newer ones arrive. EventRing is safe for concurrent use.
type EventRing struct {
	mu       sync.RWMutex
	sessions map[string]*sessionRing
	capacity int
}

// sessionRing holds the ring state for a single session.
type sessionRing struct {
	buf  []Event
	head int // index of the oldest element
	size int // number of elements currently stored
}

// NewEventRing creates an EventRing with the default capacity of 512 events per session.
func NewEventRing() *EventRing {
	return &EventRing{
		sessions: make(map[string]*sessionRing),
		capacity: defaultRingCapacity,
	}
}

// Push appends an event to the ring for the given session.
// When the ring is at capacity the oldest event is dropped.
func (r *EventRing) Push(sessionID string, e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	sr, ok := r.sessions[sessionID]
	if !ok {
		sr = &sessionRing{buf: make([]Event, r.capacity)}
		r.sessions[sessionID] = sr
	}

	// Write position is one past the last element.
	idx := (sr.head + sr.size) % r.capacity
	sr.buf[idx] = e

	if sr.size < r.capacity {
		sr.size++
	} else {
		// Buffer is full: advance head to drop the oldest element.
		sr.head = (sr.head + 1) % r.capacity
	}
}

// Snapshot returns up to n of the most recent events for sessionID, oldest first.
// Returns nil if the session has no events.
func (r *EventRing) Snapshot(sessionID string, n int) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sr, ok := r.sessions[sessionID]
	if !ok || sr.size == 0 || n <= 0 {
		return nil
	}

	count := sr.size
	if n < count {
		count = n
	}

	result := make([]Event, count)
	// Start index of the earliest event in the requested window.
	start := (sr.head + sr.size - count) % r.capacity
	for i := 0; i < count; i++ {
		result[i] = sr.buf[(start+i)%r.capacity]
	}
	return result
}

// Evict removes all events for the given session, freeing its memory.
func (r *EventRing) Evict(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, sessionID)
}
