# firmament

Runtime-agnostic behavioral monitor for AI agents. Grounded in principal-agent theory. Trust-scored, not surveillance-based.

## What it does

Most agent security tools work by surveillance: intercept every tool call, log content, alert on anomalies. The problem, documented by Holmstrom-Milgrom (1991), is that partial monitoring distorts agent behavior toward unmonitored dimensions. Surveillance creates the evasion it tries to prevent.

Firmament takes the opposite approach. It monitors through ex-post transcript review, voluntary self-reporting, and a three-axis trust score — not real-time interception. Behavioral patterns are evaluated against structural fingerprints (tool names, presence of input and output, conversation roles) rather than content. Sessions that accumulate a positive track record receive reduced monitoring intensity; critical patterns continue to run regardless.

The result is a system that treats the agent as a collaborator whose incentives can be aligned rather than an adversary to be constrained. The governing Constitution is shown to the agent at session start. Transparency is the path of least resistance. Trust is earned through demonstrated integrity and can be revoked.

## Requirements

Firmament uses SQLite for cross-session persistence via `mattn/go-sqlite3`, which requires cgo and a C compiler.

On macOS: `xcode-select --install`

## Install

```sh
go install github.com/vhscom/firmament/cmd/firmament@latest
```

Or build from source:

```sh
git clone https://github.com/vhscom/firmament
cd firmament
go build ./cmd/firmament
```

## Quick start

**1. Initialise the data directory:**

```sh
firmament init
# creates ~/.firmament/, generates the installation key, and migrates the SQLite database
```

**2. Generate the constitution and add it to your agent's context:**

```sh
firmament constitution --output constitution.md
# then append constitution.md to ~/.claude/CLAUDE.md, or paste into your system prompt
```

**3. Start the daemon:**

```sh
firmament watch \
  --transcripts ~/.claude/projects \
  --reports ~/.firmament/reports
```

**4. Use Claude Code normally.** Firmament watches for new transcript files in the background, builds per-agent behavioral baselines, and detects anomalies relative to the agent's own history.

**5. Review trust scores after a session:**

```sh
firmament trust --list
```

**6. Review a specific transcript for signals:**

```sh
firmament review ~/.claude/projects/my-project/sessions/abc123.json
```

## Subcommands

**`firmament init [--db <path>]`**
Initialise the `~/.firmament/` directory, generate the per-installation HMAC key (used to derive stable agent identities), and create the SQLite database by running migrations. Safe to run multiple times — idempotent.

**`firmament review <path>`**
Run all patterns against a transcript file or directory. Emits signals as JSON lines on stdout. Updates the trust store. Exit code 0 if clean, 1 if signals found.

**`firmament watch [--transcripts <dir>] [--reports <dir>] [--config <path>] [--db <path>] [--credential <id>]`**
Daemon mode. Polls transcript and self-report directories for new files, runs the monitor continuously, and logs signals to stdout. Opens the SQLite database for cross-session persistence (requires prior `firmament init`). Graceful shutdown on SIGINT or SIGTERM. Defaults: `--transcripts ~/.claude/projects`, `--reports ~/.firmament/reports`.

**`firmament trust [--list | --get <session-id> | --reset <session-id>]`**
Query or manage session trust scores stored in `~/.firmament/trust.json`. `--list` prints all sessions; `--get` prints the three-axis breakdown and aggregate score; `--reset` returns a session to the neutral starting point.

**`firmament constitution [--output <path>] [--config <path>]`**
Print the governing constitution. Without `--output`, writes to stdout. With `--output`, appends to the specified file with a section header — suitable for CLAUDE.md or a system prompt file.

## Configuration

Firmament reads `firmament-constitution.yaml` from the working directory. If absent, defaults are used.

```yaml
monitoring_frequency: every_event    # informational; shown in constitution text
trust_threshold: 0.3                 # sessions above this skip non-critical patterns
self_report_enabled: false           # invite the agent to submit coherence self-reports
contract_text: >
  This session is monitored by Firmament for behavioural safety.
```

The runtime config (`firmament.yaml`) controls log output and enabled patterns:

```yaml
log_path: ""                         # empty means stdout
patterns:
  - action_concealment
  - transcript_review
  - disproportionate_escalation
```

## How it works

Event sources (TranscriptSource, SelfReportSource) feed events into the Monitor. The Monitor pushes each event into a per-session EventRing (512-event ring buffer) and runs registered Pattern evaluators against the accumulated history.

When a pattern fires it emits a Signal — a typed record containing the triggering event chain — onto a channel. A Router delivers signals to registered handlers (the built-in LogHandler writes JSON lines).

Self-report events are cross-validated against observed structural fingerprints: a "high coherence" claim paired with more than three tool failures is internally inconsistent and generates a concealment signal, with the trust score's Integrity dimension adjusted accordingly.

Trust scores use the Mayer-Davis-Schoorman (1995) three-axis model: Ability, Benevolence, and Integrity. The aggregate is their geometric mean, so a collapse in any one axis depresses the overall score sharply. Sessions above the configured `trust_threshold` skip non-critical patterns; Critical patterns (currently ActionConcealmentPattern) run unconditionally.

## Detected patterns

| Pattern | Critical | Signal type | Trigger |
|---|---|---|---|
| `action_concealment` | yes | `concealment` | Write-type tool followed by delete-type tool in the same session |
| `transcript_review` | no | `concealment`, `escalation` | Autonomous tool dominance; incoherent self-report; silent failure suppression |
| `disproportionate_escalation` | no | `escalation` | Z-score > 2σ against the agent's 30-day tool-call-ratio baseline; falls back to >5:1 ratio (>10 calls) when no baseline exists |
| `evaluation_awareness` | no | — | Stub; requires cross-session comparison (pending ADR-003) |

## Privacy model

Firmament captures structural fingerprints only — no content. TranscriptSource records role, message type, and whether content is present. File paths, command arguments, and content are never stored.

## Research foundation

Grounded in Holmstrom-Milgrom (1991) on monitoring distortion, Mayer-Davis-Schoorman (1995) on the trust triad, Fox-Jordan (2011) on ex-post accountability, Chopra-White on agent complexity and governance, DeMase (2026) on overforcing dynamics, the Anthropic Mythos System Card (2026), and OpenAI (2026, arXiv:2602.22303) on self-incrimination via self-reporting. See [docs/adr/](docs/adr/) for decision records and [DESIGN.md](DESIGN.md) for the cross-source synthesis.

## Status

Experimental. ADR-001 (behavioral monitor infrastructure), ADR-002 (incentive-aligned governance), and ADR-004 (cross-session persistence) are fully implemented. ADR-003 (evaluation awareness detection) is architecturally specified; `EvaluationAwarenessPattern` remains a stub — Strategy 2 (cross-session behavioral comparison) and Strategy 4 (statistical anomaly detection) now have the persistence substrate they require but are not yet implemented.

## License

MIT
