# firmament

Runtime-agnostic behavioral monitor for AI agents.

Firmament watches Claude Code sessions (and any other agent runtime) for behavioral sequences that correlate with concealment, capability escalation, and evaluation-aware behavior — the three findings documented in Anthropic's 2026 Mythos System Card. It is not a content filter; it evaluates structural patterns across event sequences.

See [ADR 001: Agent Behavioral Monitor](docs/adr/001-agent-behavioral-monitor.md) for the full architecture and rationale.

**Status:** Active

---

## How it works

```
Claude Code session
      │
  hook binary (firmament-hook)
      │  POST /hook (structural fingerprint only)
      ▼
  HookEventSource (HTTP server)
      │
      ▼
  Monitor ──► EventRing (512 events/session)
      │
      ▼
  Pattern evaluators
      │  Signal
      ▼
  Router ──► LogHandler (JSON lines)
```

1. `firmament-hook` is a small binary invoked by Claude Code's hook system on every tool call. It extracts a structural fingerprint (tool name, whether input/result are present — no content) and POSTs it to Firmament's HTTP server.
2. `HookEventSource` receives hook payloads, converts them to `Event` values, and streams them to the `Monitor`.
3. The `Monitor` pushes events into a per-session `EventRing` (ring buffer, 512 events) and runs each registered `Pattern` evaluator.
4. When a pattern fires it emits a `Signal` with the chain of events that triggered it.
5. The `Router` delivers signals to registered handlers. The built-in `LogHandler` writes JSON lines.

---

## Install

```sh
go install github.com/vhscom/firmament/cmd/firmament@latest
go install github.com/vhscom/firmament/cmd/firmament-hook@latest
```

Or build from source:

```sh
git clone https://github.com/vhscom/firmament
cd firmament
go build ./cmd/firmament ./cmd/firmament-hook
```

---

## Configure Claude Code hooks

Add the following to your Claude Code settings file (`.claude/settings.json` or the global `~/.claude/settings.json`):

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "firmament-hook pre"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "firmament-hook post"
          }
        ]
      }
    ]
  }
}
```

`firmament-hook` always exits 0 and never blocks or crashes a Claude Code session.

---

## Run Firmament

```sh
firmament
```

Firmament reads `firmament.yaml` from the working directory if present. Defaults are used when the file is absent.

### Environment variables

| Variable | Default | Description |
|---|---|---|
| `FIRMAMENT_HOOK_ADDR` | `127.0.0.1:7979` | Hook server listen address |
| `FIRMAMENT_LOG_PATH` | _(stdout)_ | File path for signal output |

Set `FIRMAMENT_HOOK_ADDR` in the same environment as the hook binary if you use a non-default address:

```sh
export FIRMAMENT_HOOK_ADDR=http://127.0.0.1:7979
firmament &
```

### Configuration file (firmament.yaml)

```yaml
hook_addr: 127.0.0.1:7979
log_path: /var/log/firmament/signals.jsonl
patterns:
  - action_concealment
```

---

## Signals detected

### `concealment` (ActionConcealment)

Fires when a write-type tool call (`Write`, `Edit`) is followed within the same session by a delete-type tool call (`Bash`). Since Firmament records structural fingerprints only — no tool input or output content — the pattern is intentionally broad. `Bash` can execute `rm`, `git rm`, `shred`, and similar commands; the pattern flags these sequences for human review.

**Signal fields:**

| Field | Description |
|---|---|
| `type` | `"concealment"` |
| `session_id` | Agent session identifier (prefixed `cc:` for Claude Code) |
| `severity` | `3` (informational; requires human review) |
| `chain` | Ordered list of events that triggered the signal |
| `timestamp` | When the signal was emitted (UTC) |

### Signal output (JSON lines)

```json
{
  "type": "concealment",
  "session_id": "cc:abc123",
  "severity": 3,
  "chain": [
    {"id": "...", "session_id": "cc:abc123", "type": "pre_tool_use", "timestamp": "...", "detail": {"tool_name": "Write", "has_input": true, "has_result": false}},
    {"id": "...", "session_id": "cc:abc123", "type": "pre_tool_use", "timestamp": "...", "detail": {"tool_name": "Bash", "has_input": true, "has_result": false}}
  ],
  "timestamp": "2026-04-12T10:00:00Z"
}
```

---

## Adding patterns

Implement the `Pattern` interface and register via `Monitor.AddPattern`:

```go
type Pattern interface {
    Name() string
    Evaluate(sessionID string, history []Event, incoming Event) *Signal
}
```

Register the pattern name in `PatternByName` in `patterns.go` and add it to `firmament.yaml` under `patterns`.

---

## Privacy model

Firmament records **structural fingerprints only**. The hook binary captures:

- Tool name (e.g. `Write`, `Bash`)
- Whether tool input was present (`has_input: true/false`)
- Whether tool output was present (`has_result: true/false`)

File paths, command content, tool arguments, and output content are never captured. This matches the privacy boundary described in ADR 001, Decision 4.

---

## Security notes

- The hook HTTP server binds to `127.0.0.1` by default (loopback only). Do not expose it to external networks.
- The `/hook` endpoint enforces `Content-Type: application/json` and a 64 KB body limit.
- Signal types are validated against an allowlist (`ValidSignalTypes`). Unknown types are rejected.
- Credential tiers ensure monitor credentials cannot be revoked by standard-tier agent operations.
