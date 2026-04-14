package firmament

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func makeSignal(sessionID string) Signal {
	return Signal{
		Type:      SignalConcealment,
		SessionID: sessionID,
		Severity:  3,
		Timestamp: time.Now().UTC(),
	}
}

func TestRouterCallbackHandler(t *testing.T) {
	var received []Signal
	var mu sync.Mutex

	h := NewCallbackHandler(func(s Signal) error {
		mu.Lock()
		received = append(received, s)
		mu.Unlock()
		return nil
	})

	r := NewRouter()
	r.Add(h)

	ch := make(chan Signal, 4)
	ch <- makeSignal("s1")
	ch <- makeSignal("s2")
	close(ch)

	ctx := context.Background()
	r.Route(ctx, ch)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Errorf("want 2 signals, got %d", len(received))
	}
}

func TestRouterLogHandler(t *testing.T) {
	var buf bytes.Buffer
	h := NewLogHandler(&buf)

	r := NewRouter()
	r.Add(h)

	ch := make(chan Signal, 2)
	ch <- makeSignal("s1")
	close(ch)

	r.Route(context.Background(), ch)

	// Verify JSON was written.
	var sig Signal
	if err := json.NewDecoder(&buf).Decode(&sig); err != nil {
		t.Fatalf("decode log output: %v", err)
	}
	if sig.SessionID != "s1" {
		t.Errorf("SessionID: got %q", sig.SessionID)
	}
}

func TestRouterMultipleHandlers(t *testing.T) {
	var countA, countB atomic.Int64

	a := NewCallbackHandler(func(Signal) error { countA.Add(1); return nil })
	b := NewCallbackHandler(func(Signal) error { countB.Add(1); return nil })

	r := NewRouter()
	r.Add(a)
	r.Add(b)

	ch := make(chan Signal, 3)
	for i := 0; i < 3; i++ {
		ch <- makeSignal("s")
	}
	close(ch)

	r.Route(context.Background(), ch)

	if countA.Load() != 3 || countB.Load() != 3 {
		t.Errorf("want 3 each, got A=%d B=%d", countA.Load(), countB.Load())
	}
}

func TestRouterHandlerErrorDoesNotStopRouting(t *testing.T) {
	var count atomic.Int64
	errHandler := NewCallbackHandler(func(Signal) error { return errors.New("boom") })
	okHandler := NewCallbackHandler(func(Signal) error { count.Add(1); return nil })

	r := NewRouter()
	r.Add(errHandler)
	r.Add(okHandler)

	ch := make(chan Signal, 2)
	ch <- makeSignal("s1")
	ch <- makeSignal("s2")
	close(ch)

	r.Route(context.Background(), ch)

	if count.Load() != 2 {
		t.Errorf("ok handler should receive all signals, got %d", count.Load())
	}
}

func TestRouterContextCancellation(t *testing.T) {
	var count atomic.Int64
	h := NewCallbackHandler(func(Signal) error { count.Add(1); return nil })

	r := NewRouter()
	r.Add(h)

	// Unbuffered channel: nothing will be sent before cancellation.
	ch := make(chan Signal)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		r.Route(ctx, ch)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// Correct: Route returned after context cancellation.
	case <-time.After(time.Second):
		t.Fatal("Route did not return after context cancellation")
	}

	if count.Load() != 0 {
		t.Errorf("no signals should have been handled, got %d", count.Load())
	}
}

func TestRouterClosedChannelExits(t *testing.T) {
	r := NewRouter()

	ch := make(chan Signal)
	close(ch) // immediately closed

	done := make(chan struct{})
	go func() {
		r.Route(context.Background(), ch)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Route did not return on closed channel")
	}
}
