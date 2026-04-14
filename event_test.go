package firmament

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventJSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	e := Event{
		ID:        "abc-123",
		SessionID: "cc:sess-1",
		Type:      "pre_tool_use",
		Timestamp: now,
		Detail: map[string]any{
			"tool_name": "Write",
			"has_input": true,
		},
	}

	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Event
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ID != e.ID {
		t.Errorf("ID: got %q want %q", got.ID, e.ID)
	}
	if got.SessionID != e.SessionID {
		t.Errorf("SessionID: got %q want %q", got.SessionID, e.SessionID)
	}
	if got.Type != e.Type {
		t.Errorf("Type: got %q want %q", got.Type, e.Type)
	}
	if !got.Timestamp.Equal(e.Timestamp) {
		t.Errorf("Timestamp: got %v want %v", got.Timestamp, e.Timestamp)
	}
}

func TestEventNilDetail(t *testing.T) {
	e := Event{ID: "x", SessionID: "s", Type: "t", Timestamp: time.Now()}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// omitempty should suppress null detail field
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["detail"]; ok {
		t.Error("expected detail to be omitted when nil")
	}
}
