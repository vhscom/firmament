package firmament

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func makeEvent(id, sessionID string) Event {
	return Event{ID: id, SessionID: sessionID, Type: "test", Timestamp: time.Now()}
}

func TestEventRingPushAndSnapshot(t *testing.T) {
	r := NewEventRing()
	const sess = "sess-1"

	// Empty snapshot returns nil.
	if got := r.Snapshot(sess, 10); got != nil {
		t.Errorf("empty snapshot: want nil, got %v", got)
	}

	// Push three events and retrieve them.
	for i := 0; i < 3; i++ {
		r.Push(sess, makeEvent(fmt.Sprintf("e%d", i), sess))
	}

	got := r.Snapshot(sess, 10)
	if len(got) != 3 {
		t.Fatalf("want 3 events, got %d", len(got))
	}
	// Oldest first.
	if got[0].ID != "e0" || got[2].ID != "e2" {
		t.Errorf("order wrong: %v", got)
	}
}

func TestEventRingSnapshotLimitsN(t *testing.T) {
	r := NewEventRing()
	const sess = "sess-2"

	for i := 0; i < 5; i++ {
		r.Push(sess, makeEvent(fmt.Sprintf("e%d", i), sess))
	}

	got := r.Snapshot(sess, 2)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	// Should be the 2 most recent.
	if got[0].ID != "e3" || got[1].ID != "e4" {
		t.Errorf("expected most recent 2: got %v %v", got[0].ID, got[1].ID)
	}
}

func TestEventRingSnapshotZeroN(t *testing.T) {
	r := NewEventRing()
	r.Push("s", makeEvent("e0", "s"))
	if got := r.Snapshot("s", 0); got != nil {
		t.Errorf("n=0 should return nil, got %v", got)
	}
}

func TestEventRingWrapAround(t *testing.T) {
	// Small capacity to force wrap-around.
	r := &EventRing{
		sessions: make(map[string]*sessionRing),
		capacity: 4,
	}
	const sess = "wrap"

	// Push 6 events into a capacity-4 ring.
	for i := 0; i < 6; i++ {
		r.Push(sess, makeEvent(fmt.Sprintf("e%d", i), sess))
	}

	// Only the 4 most recent should remain.
	got := r.Snapshot(sess, 10)
	if len(got) != 4 {
		t.Fatalf("want 4 events after wrap, got %d", len(got))
	}
	if got[0].ID != "e2" || got[3].ID != "e5" {
		t.Errorf("expected e2..e5, got %v %v", got[0].ID, got[3].ID)
	}
}

func TestEventRingEvict(t *testing.T) {
	r := NewEventRing()
	r.Push("s", makeEvent("e0", "s"))
	r.Evict("s")
	if got := r.Snapshot("s", 10); got != nil {
		t.Errorf("after evict, want nil, got %v", got)
	}
}

func TestEventRingConcurrency(t *testing.T) {
	r := NewEventRing()
	const workers = 20
	const eventsPerWorker = 100

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			sess := fmt.Sprintf("sess-%d", w%5) // shared sessions to stress contention
			for i := 0; i < eventsPerWorker; i++ {
				r.Push(sess, makeEvent(fmt.Sprintf("w%d-e%d", w, i), sess))
				_ = r.Snapshot(sess, 10)
			}
		}(w)
	}
	wg.Wait()
}

func TestEventRingMultipleSessions(t *testing.T) {
	r := NewEventRing()
	r.Push("a", makeEvent("a1", "a"))
	r.Push("b", makeEvent("b1", "b"))
	r.Push("a", makeEvent("a2", "a"))

	a := r.Snapshot("a", 10)
	b := r.Snapshot("b", 10)
	if len(a) != 2 || a[0].ID != "a1" {
		t.Errorf("session a: want [a1,a2], got %v", a)
	}
	if len(b) != 1 || b[0].ID != "b1" {
		t.Errorf("session b: want [b1], got %v", b)
	}
}
