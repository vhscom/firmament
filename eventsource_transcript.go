package firmament

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// transcriptEntry is one entry in a Claude Code JSON transcript file.
// Each file contains an array of these.
type transcriptEntry struct {
	Role    string `json:"role"`
	Type    string `json:"type"`
	Content any    `json:"content"` // may be string or []map[string]any
}

// TranscriptSource implements EventSource by reading JSON transcript files
// from a directory. Each file contains an array of transcript entries with
// role/type/content fields. New files are discovered on each poll interval.
//
// Session ID is derived from the filename (without extension).
// The source records which files it has already processed so each file is
// emitted only once. Close stops the polling loop.
type TranscriptSource struct {
	dir      string
	interval time.Duration
	events   chan Event
	seen     map[string]struct{}
	mu       sync.Mutex
	stop     chan struct{} // closed by Close to signal Start to exit
	stopOnce sync.Once
	evOnce   sync.Once
}

// defaultTranscriptInterval is how often the directory is re-scanned.
const defaultTranscriptInterval = 5 * time.Second

// NewTranscriptSource creates a TranscriptSource that polls dir every interval.
// Pass 0 for interval to use the default (5 s).
func NewTranscriptSource(dir string, interval time.Duration) *TranscriptSource {
	if interval <= 0 {
		interval = defaultTranscriptInterval
	}
	return &TranscriptSource{
		dir:      dir,
		interval: interval,
		events:   make(chan Event, 256),
		seen:     make(map[string]struct{}),
		stop:     make(chan struct{}),
	}
}

// Name implements EventSource.
func (s *TranscriptSource) Name() string { return "transcript" }

// Events implements EventSource.
func (s *TranscriptSource) Events() <-chan Event { return s.events }

// Start begins polling the transcript directory. It blocks until ctx is
// cancelled, Close is called, or the goroutine exits. Call it in a goroutine.
// When Start returns it closes the Events channel.
func (s *TranscriptSource) Start(ctx context.Context) {
	defer s.evOnce.Do(func() { close(s.events) })

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Scan immediately before the first tick.
	s.scan()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case <-ticker.C:
			s.scan()
		}
	}
}

// Close stops the polling loop. Idempotent.
func (s *TranscriptSource) Close() error {
	s.stopOnce.Do(func() { close(s.stop) })
	return nil
}

// scan reads the directory and processes any new .json files.
func (s *TranscriptSource) scan() {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		if filepath.Ext(de.Name()) != ".json" {
			continue
		}
		path := filepath.Join(s.dir, de.Name())

		s.mu.Lock()
		_, done := s.seen[path]
		s.mu.Unlock()
		if done {
			continue
		}

		s.processFile(path, de.Name())

		s.mu.Lock()
		s.seen[path] = struct{}{}
		s.mu.Unlock()
	}
}

// processFile parses a transcript JSON file and emits one Event per entry.
// The session ID is the filename without its extension.
func (s *TranscriptSource) processFile(path, name string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var entries []transcriptEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return
	}

	sessionID := name[:len(name)-len(filepath.Ext(name))]
	now := time.Now().UTC()

	for _, entry := range entries {
		e := Event{
			ID:        uuid.New().String(),
			SessionID: sessionID,
			Type:      "transcript_entry",
			Timestamp: now,
			Detail: map[string]any{
				"role": entry.Role,
				"type": entry.Type,
				// Content is not captured — structural fingerprint only.
				"has_content": entry.Content != nil,
			},
		}
		select {
		case s.events <- e:
		default:
		}
	}
}
