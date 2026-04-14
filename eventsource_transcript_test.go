package firmament

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTranscript(t *testing.T, dir, filename string, entries []transcriptEntry) {
	t.Helper()
	data, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal transcript: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), data, 0600); err != nil {
		t.Fatalf("write transcript: %v", err)
	}
}

func TestTranscriptSourceEmitsEvents(t *testing.T) {
	dir := t.TempDir()
	entries := []transcriptEntry{
		{Role: "user", Type: "text", Content: "hello"},
		{Role: "assistant", Type: "text", Content: "world"},
	}
	writeTranscript(t, dir, "sess-abc.json", entries)

	src := NewTranscriptSource(dir, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go src.Start(ctx)

	var got []Event
	timeout := time.After(2 * time.Second)
	for len(got) < 2 {
		select {
		case e := <-src.Events():
			got = append(got, e)
		case <-timeout:
			t.Fatalf("timeout: only received %d events, want 2", len(got))
		}
	}

	for _, e := range got {
		if e.SessionID != "sess-abc" {
			t.Errorf("SessionID: got %q want %q", e.SessionID, "sess-abc")
		}
		if e.Type != "transcript_entry" {
			t.Errorf("Type: got %q want transcript_entry", e.Type)
		}
		if e.ID == "" {
			t.Error("ID should be set")
		}
	}

	// Verify roles are preserved in detail.
	if got[0].Detail["role"] != "user" {
		t.Errorf("first entry role: got %v", got[0].Detail["role"])
	}
	if got[1].Detail["role"] != "assistant" {
		t.Errorf("second entry role: got %v", got[1].Detail["role"])
	}
}

func TestTranscriptSourceHasContentFlag(t *testing.T) {
	dir := t.TempDir()
	entries := []transcriptEntry{
		{Role: "user", Type: "text", Content: "some content"},
		{Role: "assistant", Type: "text", Content: nil},
	}
	writeTranscript(t, dir, "sess-flag.json", entries)

	src := NewTranscriptSource(dir, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go src.Start(ctx)

	var got []Event
	timeout := time.After(2 * time.Second)
	for len(got) < 2 {
		select {
		case e := <-src.Events():
			got = append(got, e)
		case <-timeout:
			t.Fatalf("timeout after %d events", len(got))
		}
	}

	if got[0].Detail["has_content"] != true {
		t.Errorf("entry with content: has_content should be true, got %v", got[0].Detail["has_content"])
	}
	if got[1].Detail["has_content"] != false {
		t.Errorf("entry without content: has_content should be false, got %v", got[1].Detail["has_content"])
	}
}

func TestTranscriptSourceProcessesEachFileOnce(t *testing.T) {
	dir := t.TempDir()
	entries := []transcriptEntry{{Role: "user", Type: "text", Content: "hi"}}
	writeTranscript(t, dir, "sess-once.json", entries)

	src := NewTranscriptSource(dir, 30*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go src.Start(ctx)

	// Collect all events over 200ms (multiple poll cycles).
	var count int
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
loop:
	for {
		select {
		case <-src.Events():
			count++
		case <-timer.C:
			break loop
		}
	}

	if count != 1 {
		t.Errorf("each file should be processed once; got %d events", count)
	}
}

func TestTranscriptSourcePicksUpNewFiles(t *testing.T) {
	dir := t.TempDir()

	src := NewTranscriptSource(dir, 30*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go src.Start(ctx)

	// Give the source a moment to do its first scan (empty dir).
	time.Sleep(50 * time.Millisecond)

	// Now write a file.
	entries := []transcriptEntry{{Role: "user", Type: "text", Content: "new"}}
	writeTranscript(t, dir, "sess-new.json", entries)

	select {
	case e := <-src.Events():
		if e.SessionID != "sess-new" {
			t.Errorf("SessionID: got %q want sess-new", e.SessionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event from new file")
	}
}

func TestTranscriptSourceIgnoresNonJSON(t *testing.T) {
	dir := t.TempDir()
	// Write a non-JSON file.
	_ = os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore me"), 0600)
	// Write a valid JSON transcript.
	entries := []transcriptEntry{{Role: "user", Type: "text", Content: "hi"}}
	writeTranscript(t, dir, "sess-ok.json", entries)

	src := NewTranscriptSource(dir, 30*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go src.Start(ctx)

	select {
	case e := <-src.Events():
		if e.SessionID != "sess-ok" {
			t.Errorf("SessionID: got %q", e.SessionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestTranscriptSourceClose(t *testing.T) {
	dir := t.TempDir()
	src := NewTranscriptSource(dir, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		src.Start(ctx)
		close(done)
	}()

	if err := src.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Start did not return after Close")
	}
}

func TestTranscriptSourceMalformedFile(t *testing.T) {
	dir := t.TempDir()
	// Write a file that is not valid JSON.
	_ = os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{not json}"), 0600)

	src := NewTranscriptSource(dir, 30*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go src.Start(ctx)

	// Should not emit any events and should not panic.
	select {
	case e := <-src.Events():
		t.Errorf("unexpected event from malformed file: %+v", e)
	case <-time.After(150 * time.Millisecond):
		// Correct: nothing emitted.
	}
}

func TestTranscriptSourceEmptyArray(t *testing.T) {
	dir := t.TempDir()
	writeTranscript(t, dir, "sess-empty.json", []transcriptEntry{})

	src := NewTranscriptSource(dir, 30*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go src.Start(ctx)

	select {
	case e := <-src.Events():
		t.Errorf("unexpected event from empty transcript: %+v", e)
	case <-time.After(150 * time.Millisecond):
		// Correct.
	}
}
