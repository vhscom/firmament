// Optional verification channel.
package firmament

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// maxHookBodySize is the maximum request body size accepted on /hook.
// Structural fingerprints are tiny; 64 KB is generous and prevents abuse.
const maxHookBodySize = 64 * 1024

// hookFingerprint is the payload posted by the firmament-hook companion binary.
// It contains only structural metadata — no tool input or output content.
type hookFingerprint struct {
	SessionID string `json:"session_id"`
	EventType string `json:"event_type"` // "pre_tool_use" or "post_tool_use"
	ToolName  string `json:"tool_name"`
	HasInput  bool   `json:"has_input"`
	HasResult bool   `json:"has_result"`
}

// HookEventSource listens on a local HTTP address for POST /hook requests
// from the firmament-hook companion binary and converts them into Events.
//
// Session keys are expected to be prefixed with "cc:" by the companion binary.
type HookEventSource struct {
	addr   string
	events chan Event
	server *http.Server
	once   sync.Once
}

// NewHookEventSource creates a HookEventSource that will listen on addr
// (e.g. "127.0.0.1:7979"). Call ListenAndServe to start accepting requests.
func NewHookEventSource(addr string) *HookEventSource {
	src := &HookEventSource{
		addr:   addr,
		events: make(chan Event, 256),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/hook", src.handleHook)
	src.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}
	return src
}

// Name implements EventSource.
func (h *HookEventSource) Name() string { return "hook" }

// Events implements EventSource.
func (h *HookEventSource) Events() <-chan Event { return h.events }

// Addr returns the configured listen address.
func (h *HookEventSource) Addr() string { return h.addr }

// ListenAndServe starts the HTTP server. It blocks until Close is called
// or a fatal server error occurs. Callers should run this in a goroutine.
func (h *HookEventSource) ListenAndServe() error {
	return h.server.ListenAndServe()
}

// Close shuts down the HTTP server and closes the Events channel.
func (h *HookEventSource) Close() error {
	err := h.server.Shutdown(context.Background())
	h.once.Do(func() { close(h.events) })
	return err
}

// handleHook is the HTTP handler for POST /hook.
func (h *HookEventSource) handleHook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxHookBodySize)
	var fp hookFingerprint
	if err := json.NewDecoder(r.Body).Decode(&fp); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if fp.SessionID == "" || fp.ToolName == "" {
		http.Error(w, "missing required fields", http.StatusBadRequest)
		return
	}

	e := Event{
		ID:        uuid.New().String(),
		SessionID: fp.SessionID,
		Type:      fp.EventType,
		Timestamp: time.Now().UTC(),
		Detail: map[string]any{
			"tool_name":  fp.ToolName,
			"has_input":  fp.HasInput,
			"has_result": fp.HasResult,
		},
	}

	// Non-blocking send: drop the event if the consumer is not keeping up
	// rather than blocking the HTTP handler.
	select {
	case h.events <- e:
	default:
	}

	w.WriteHeader(http.StatusOK)
}
