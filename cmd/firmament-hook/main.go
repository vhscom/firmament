// Optional verification channel.

// Command firmament-hook is the Claude Code companion binary for Firmament.
//
// It is invoked as a Claude Code hook (PreToolUse or PostToolUse), reads the
// hook payload from stdin, extracts a structural fingerprint (no content),
// and POSTs it to the Firmament hook server.
//
// Usage in .claude/settings.json:
//
//	{
//	  "hooks": {
//	    "PreToolUse":  [{"matcher": ".*", "hooks": [{"type": "command", "command": "firmament-hook pre"}]}],
//	    "PostToolUse": [{"matcher": ".*", "hooks": [{"type": "command", "command": "firmament-hook post"}]}]
//	  }
//	}
//
// Environment variables:
//
//	FIRMAMENT_HOOK_ADDR  — address of the Firmament hook server (default: http://127.0.0.1:7979)
//
// The binary always exits 0 so it never blocks or fails a Claude Code session.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"
)

// claudeHookInput is the payload Claude Code writes to stdin for hook invocations.
type claudeHookInput struct {
	SessionID    string         `json:"session_id"`
	ToolName     string         `json:"tool_name"`
	ToolInput    map[string]any `json:"tool_input"`
	ToolResponse map[string]any `json:"tool_response"`
}

// hookFingerprint is the structural-only payload sent to the Firmament server.
type hookFingerprint struct {
	SessionID string `json:"session_id"`
	EventType string `json:"event_type"`
	ToolName  string `json:"tool_name"`
	HasInput  bool   `json:"has_input"`
	HasResult bool   `json:"has_result"`
}

func main() {
	// Determine hook type from first argument.
	eventType := "pre_tool_use"
	if len(os.Args) > 1 && os.Args[1] == "post" {
		eventType = "post_tool_use"
	}

	// Read stdin; silently exit 0 on any failure — never block the agent.
	raw, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20)) // 1 MB cap
	if err != nil {
		os.Exit(0)
	}

	var input claudeHookInput
	if err := json.Unmarshal(raw, &input); err != nil {
		os.Exit(0)
	}

	// Reject payloads with no meaningful identity.
	if input.SessionID == "" && input.ToolName == "" {
		os.Exit(0)
	}

	fp := hookFingerprint{
		SessionID: "cc:" + input.SessionID,
		EventType: eventType,
		ToolName:  input.ToolName,
		HasInput:  input.ToolInput != nil,
		HasResult: input.ToolResponse != nil,
	}

	payload, err := json.Marshal(fp)
	if err != nil {
		os.Exit(0)
	}

	addr := os.Getenv("FIRMAMENT_HOOK_ADDR")
	if addr == "" {
		addr = "http://127.0.0.1:7979"
	}

	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequest(http.MethodPost, addr+"/hook", bytes.NewReader(payload))
	if err != nil {
		os.Exit(0)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		os.Exit(0)
	}
	resp.Body.Close()

	os.Exit(0)
}
