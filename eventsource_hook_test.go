package firmament

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestHookSource creates a HookEventSource wired to a test HTTP server.
func newTestHookSource(t *testing.T) (*HookEventSource, *httptest.Server) {
	t.Helper()
	src := &HookEventSource{events: make(chan Event, 16)}
	ts := httptest.NewServer(http.HandlerFunc(src.handleHook))
	t.Cleanup(ts.Close)
	return src, ts
}

func postHook(t *testing.T, ts *httptest.Server, body any, contentType string) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(ts.URL+"/hook", contentType, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	return resp
}

func TestHookEventSourceValidPayload(t *testing.T) {
	src, ts := newTestHookSource(t)

	fp := hookFingerprint{
		SessionID: "cc:sess-1",
		EventType: "pre_tool_use",
		ToolName:  "Write",
		HasInput:  true,
		HasResult: false,
	}
	resp := postHook(t, ts, fp, "application/json")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}

	select {
	case e := <-src.events:
		if e.SessionID != "cc:sess-1" {
			t.Errorf("SessionID: got %q want %q", e.SessionID, "cc:sess-1")
		}
		if e.Type != "pre_tool_use" {
			t.Errorf("Type: got %q want %q", e.Type, "pre_tool_use")
		}
		if e.Detail["tool_name"] != "Write" {
			t.Errorf("tool_name: got %v", e.Detail["tool_name"])
		}
		if e.ID == "" {
			t.Error("ID should be set")
		}
		if e.Timestamp.IsZero() {
			t.Error("Timestamp should be set")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestHookEventSourceWrongMethod(t *testing.T) {
	_, ts := newTestHookSource(t)

	resp, err := http.Get(ts.URL + "/hook")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", resp.StatusCode)
	}
}

func TestHookEventSourceWrongContentType(t *testing.T) {
	_, ts := newTestHookSource(t)

	resp, err := http.Post(ts.URL+"/hook", "text/plain", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("want 415, got %d", resp.StatusCode)
	}
}

func TestHookEventSourceMissingSessionID(t *testing.T) {
	_, ts := newTestHookSource(t)

	fp := hookFingerprint{ToolName: "Write", EventType: "pre_tool_use"}
	resp := postHook(t, ts, fp, "application/json")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestHookEventSourceMissingToolName(t *testing.T) {
	_, ts := newTestHookSource(t)

	fp := hookFingerprint{SessionID: "cc:sess-1", EventType: "pre_tool_use"}
	resp := postHook(t, ts, fp, "application/json")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestHookEventSourceBadJSON(t *testing.T) {
	_, ts := newTestHookSource(t)

	resp, err := http.Post(ts.URL+"/hook", "application/json", strings.NewReader("{not-json"))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestHookEventSourceOversizedBody(t *testing.T) {
	_, ts := newTestHookSource(t)

	// Generate a body larger than maxHookBodySize.
	large := make([]byte, maxHookBodySize+1)
	for i := range large {
		large[i] = 'x'
	}
	resp, err := http.Post(ts.URL+"/hook", "application/json", bytes.NewReader(large))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode == http.StatusOK {
		t.Error("oversized body should not return 200")
	}
}

func TestHookEventSourceContentTypeWithCharset(t *testing.T) {
	src, ts := newTestHookSource(t)

	fp := hookFingerprint{
		SessionID: "cc:sess-2",
		EventType: "post_tool_use",
		ToolName:  "Bash",
		HasInput:  true,
		HasResult: true,
	}
	data, _ := json.Marshal(fp)
	resp, err := http.Post(ts.URL+"/hook", "application/json; charset=utf-8", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}

	select {
	case e := <-src.events:
		if e.Type != "post_tool_use" {
			t.Errorf("Type: got %q", e.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}
