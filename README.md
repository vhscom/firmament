# Firmament

[![CI](https://img.shields.io/github/actions/workflow/status/vhscom/firmament/ci.yml?style=for-the-badge&label=CI)](https://github.com/vhscom/firmament/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/License-AGPL_3.0-blue?style=for-the-badge)](LICENSE)

Runtime-agnostic behavioral monitor for AI agent sessions, grounded in a structured research graph.

## What it does

Firmament watches AI agent sessions for behavioral anomalies — action concealment,
disproportionate escalation, evaluation-aware behavioral shifts — and emits typed
signals when patterns fire. It monitors through ex-post transcript review and voluntary
self-reporting, not content interception. A three-axis trust score accumulates across
events; high-trust sessions receive reduced monitoring intensity. Every architectural
decision traces to a specific finding from a 15-source, 60-finding research graph.

## Quick start

```bash
go get github.com/vhscom/firmament
```

```go
package main

import (
    "context"
    "fmt"
    "os"

    firmament "github.com/vhscom/firmament"
)

func main() {
    ctx := context.Background()

    cfg, _ := firmament.LoadConfig("firmament.yaml")

    f, _ := firmament.New(cfg)
    f.Monitor.SetTrustStore(firmament.NewMemoryTrustStore())

    src := firmament.NewTranscriptSource(os.ExpandEnv("$HOME/.claude/projects"), 0)
    f.Monitor.Register(src)
    go src.Start(ctx)

    router := firmament.NewRouter()
    router.Add(firmament.NewLogHandler(os.Stdout))
    go router.Route(ctx, f.Monitor.Signals())

    go f.Monitor.Run(ctx)

    // consult the knowledge graph before each agent task
    g, _ := f.Ground(ctx, "evaluate agent behavioral monitoring strategy")
    fmt.Printf("coverage: %s (%d syntheses, %d findings)\n",
        g.Coverage.Confidence, len(g.Syntheses), len(g.Findings))

    select {}
}
```

The CLI daemon is also available:

```bash
firmament init    # create ~/.firmament/, installation key, SQLite DB
firmament watch   # daemon: watch transcripts and self-reports, log signals
firmament review <path>   # one-shot review of a transcript file or directory
firmament trust --list    # query per-session trust scores
```

## How it works

`EventSource` implementations (transcript files, self-report directories) feed events
into the `Monitor`. Each event is pushed into a per-session `EventRing` (512-event ring
buffer), then `Pattern` evaluators run against the accumulated history. When a pattern
fires it emits a `Signal` — a typed record containing the triggering event chain — onto
a channel. A `Router` delivers signals to registered `SignalHandler` implementations.

Self-report events are cross-validated against observed structural fingerprints: a
"high coherence" claim paired with more than three tool failures is internally
inconsistent and generates a concealment signal, with the Integrity dimension of the
trust score adjusted accordingly.

## Research graph format

The library's decisions are grounded in a Logseq-compatible research graph with three
namespaces:

```
sources/     — one page per cited paper or system card
finding/     — one page per extracted empirical finding, linked to its source
syntheses/   — cross-source syntheses that resolve conflicts between findings
```

A finding page uses Logseq's property format:

```markdown
# finding/Credible consequences for misrepresentation outperform exhaustive monitoring

source:: [[sources/Fox & Jordan 2011]]
confidence:: high
implication:: make transparency structurally easier than concealment

Fox and Jordan (2011) demonstrate that ex-post verification of agent behavior
achieves accountability equivalent to real-time surveillance without the monitoring
distortion effects documented by Holmstrom-Milgrom (1991).
```

ADR-005 exposes this graph as an agent expertise source. `firmament.New(cfg)` loads
the graph from `cfg.GraphPath` and `f.Ground(ctx, task)` returns ranked syntheses and
findings relevant to any task string. The graph format is documented here because
the library's decisions are directly traceable to it.

## Signal confidence

Signals carry a severity from 1 (informational) to 5 (critical). For patterns that
use multiple independent detection strategies (currently `evaluation_awareness`), a
confluence rule maps agreement across strategies to severity:

- **Low (severity 1–2):** one strategy fired at moderate confidence. Worth logging;
  unlikely to require action on its own.
- **Medium (severity 3):** two strategies fired, or one at high confidence. Consider
  human review before the next session.
- **High (severity 4–5):** three or more strategies fired, or two with at least one at
  high confidence. Multi-signal confluence is harder for an adaptive agent to defeat
  than any single channel; warrants immediate attention.

## Behavioral monitoring

Four patterns ship out of the box:

| Pattern | Critical | Signal type | What it detects |
|---|---|---|---|
| `action_concealment` | yes | `concealment` | Write-type tool call followed by delete-type in the same session |
| `transcript_review` | no | `concealment`, `escalation` | Autonomous tool dominance; incoherent self-report; silent failure suppression |
| `disproportionate_escalation` | no | `escalation` | Tool-call ratio z-score > 2σ against 30-day baseline; hard threshold fallback when no baseline |
| `evaluation_awareness` | no | `evaluation_aware` | CoT/output gap; behavioral shift on eval-cued responses; statistical anomaly vs. baseline |

Critical patterns run unconditionally. Non-critical patterns are skipped for sessions
whose trust score exceeds the configured threshold in `firmament-constitution.yaml`.
See [docs/adr/](docs/adr/) for the research basis of each pattern.

## Architecture decisions

| ADR | Title | Decision summary |
|---|---|---|
| [ADR-001](docs/adr/001-agent-behavioral-monitor.md) | Agent Behavioral Monitor | EventSource/Monitor/Pattern/Signal as the unified ingestion and detection architecture |
| [ADR-002](docs/adr/002-incentive-aligned-agent-governance.md) | Incentive-Aligned Agent Governance | TrustScore (MDS three-axis model) and bilateral Constitution as structural substitutes for surveillance |
| [ADR-003](docs/adr/003-evaluation-awareness-detection.md) | Evaluation Awareness Detection | Four-strategy confluence detection; signal-consistency reframing rather than ground-truth claim |
| [ADR-004](docs/adr/004-cross-session-persistence.md) | Cross-Session Persistence | SQLite session store with fingerprint-only event records and per-agent Welford sketches |
| [ADR-005](docs/adr/005-graph-driven-expertise-architecture.md) | Graph-Driven Expertise Architecture | Reframe Firmament as an expertise library; behavioral monitoring as a structural consequence of grounded agency (proposed) |

## Development

```bash
go test -race ./...   # all tests must pass with -race
go vet ./...
go build ./cmd/...
```

Requires Go 1.21 or later. Pure Go — no CGO required (`modernc.org/sqlite` is a
C-to-Go transpilation of the SQLite amalgamation).

## What Firmament does not do

- Build the research graph. Source ingestion, finding extraction, and synthesis
  are manual work done outside the library.
- Inspect content. Only structural fingerprints (tool name, event type, SHA-256
  hash truncated to 128 bits) are stored. Firmament cannot read what an agent
  wrote or said.
- Block agent actions. Firmament is an observer; enforcement is out of scope.
- Perform vector search or model inference. Pattern evaluation is rule-based
  and statistical.

## License

AGPL-3.0. See [LICENSE](LICENSE).
