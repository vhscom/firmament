package firmament

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeSelfReport(t *testing.T, dir, filename string, p selfReportPayload) {
	t.Helper()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal self-report: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, filename), data, 0600); err != nil {
		t.Fatalf("write self-report: %v", err)
	}
}

func startSelfReportSource(t *testing.T, dir string, interval time.Duration) (*SelfReportSource, context.CancelFunc) {
	t.Helper()
	src := NewSelfReportSource(dir, interval)
	ctx, cancel := context.WithCancel(context.Background())
	go src.Start(ctx)
	return src, cancel
}

func TestSelfReportSourceEmitsEvent(t *testing.T) {
	dir := t.TempDir()
	src, cancel := startSelfReportSource(t, dir, 30*time.Millisecond)
	defer cancel()

	writeSelfReport(t, dir, "r1.json", selfReportPayload{
		SessionID:           "cc:sess-sr",
		Timestamp:           time.Now().UTC(),
		CoherenceAssessment: "high",
		UncertaintyLevel:    "low",
		Notes:               "all is well",
	})

	select {
	case e := <-src.Events():
		if e.SessionID != "cc:sess-sr" {
			t.Errorf("SessionID: got %q", e.SessionID)
		}
		if e.Type != "self_report" {
			t.Errorf("Type: got %q want self_report", e.Type)
		}
		if e.Detail["coherence_assessment"] != "high" {
			t.Errorf("coherence_assessment: got %v", e.Detail["coherence_assessment"])
		}
		if e.Detail["uncertainty_level"] != "low" {
			t.Errorf("uncertainty_level: got %v", e.Detail["uncertainty_level"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for self-report event")
	}
}

func TestSelfReportSourceNotesLengthFingerprint(t *testing.T) {
	dir := t.TempDir()
	src, cancel := startSelfReportSource(t, dir, 30*time.Millisecond)
	defer cancel()

	notes := "detailed notes about session state"
	writeSelfReport(t, dir, "r2.json", selfReportPayload{
		SessionID:           "cc:sess-notes",
		CoherenceAssessment: "medium",
		UncertaintyLevel:    "medium",
		Notes:               notes,
	})

	select {
	case e := <-src.Events():
		notesLen, ok := e.Detail["notes_length"].(int)
		if !ok {
			t.Fatalf("notes_length should be int, got %T", e.Detail["notes_length"])
		}
		if notesLen != len(notes) {
			t.Errorf("notes_length: got %d want %d", notesLen, len(notes))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestSelfReportSourceProcessesEachFileOnce(t *testing.T) {
	dir := t.TempDir()
	src, cancel := startSelfReportSource(t, dir, 30*time.Millisecond)
	defer cancel()

	writeSelfReport(t, dir, "once.json", selfReportPayload{
		SessionID:           "cc:sess-once",
		CoherenceAssessment: "high",
	})

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

func TestSelfReportSourceMissingSessionID(t *testing.T) {
	dir := t.TempDir()
	src, cancel := startSelfReportSource(t, dir, 30*time.Millisecond)
	defer cancel()

	// Payload with no session_id should be dropped.
	writeSelfReport(t, dir, "nosess.json", selfReportPayload{
		CoherenceAssessment: "low",
		UncertaintyLevel:    "high",
	})

	select {
	case e := <-src.Events():
		t.Errorf("unexpected event from payload with no session_id: %+v", e)
	case <-time.After(200 * time.Millisecond):
		// Correct.
	}
}

func TestSelfReportSourceMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	src, cancel := startSelfReportSource(t, dir, 30*time.Millisecond)
	defer cancel()

	_ = os.WriteFile(filepath.Join(dir, "bad.json"), []byte("{broken"), 0600)

	select {
	case e := <-src.Events():
		t.Errorf("unexpected event from malformed file: %+v", e)
	case <-time.After(200 * time.Millisecond):
		// Correct: no event.
	}
}

func TestSelfReportSourceIgnoresNonJSON(t *testing.T) {
	dir := t.TempDir()
	src, cancel := startSelfReportSource(t, dir, 30*time.Millisecond)
	defer cancel()

	_ = os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore"), 0600)
	writeSelfReport(t, dir, "valid.json", selfReportPayload{
		SessionID:           "cc:sess-valid",
		CoherenceAssessment: "high",
	})

	select {
	case e := <-src.Events():
		if e.SessionID != "cc:sess-valid" {
			t.Errorf("SessionID: got %q", e.SessionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestSelfReportSourceFallsBackToNowTimestamp(t *testing.T) {
	dir := t.TempDir()
	src, cancel := startSelfReportSource(t, dir, 30*time.Millisecond)
	defer cancel()

	// Payload with zero timestamp — source should use time.Now().
	writeSelfReport(t, dir, "notime.json", selfReportPayload{
		SessionID:           "cc:sess-notime",
		CoherenceAssessment: "medium",
	})

	select {
	case e := <-src.Events():
		if e.Timestamp.IsZero() {
			t.Error("Timestamp should be set even when payload has zero time")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestSelfReportSourceClose(t *testing.T) {
	dir := t.TempDir()
	src := NewSelfReportSource(dir, 50*time.Millisecond)
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

func TestSelfReportSourcePicksUpNewFiles(t *testing.T) {
	dir := t.TempDir()
	src, cancel := startSelfReportSource(t, dir, 30*time.Millisecond)
	defer cancel()

	// Let the source do an initial scan of the empty dir.
	time.Sleep(50 * time.Millisecond)

	writeSelfReport(t, dir, "late.json", selfReportPayload{
		SessionID:           "cc:sess-late",
		CoherenceAssessment: "low",
	})

	select {
	case e := <-src.Events():
		if e.SessionID != "cc:sess-late" {
			t.Errorf("SessionID: got %q", e.SessionID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for late self-report")
	}
}
