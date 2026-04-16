# ADR 005: Graph-Driven Expertise Architecture

**Status:** Proposed
**Date:** 2026-04-16
**Related:** ADR-001 (EventSource/Monitor infrastructure), ADR-002 (TrustScore/Constitution), ADR-003 (EvaluationAwarenessPattern), ADR-004 (SessionStore)

---

## Context

### The methodology mismatch

ADRs 001 through 004 grounded each architectural decision in specific findings from a structured research graph — 15 sources, 60 findings, 5 syntheses. The graph method consistently produced better decisions than the alternatives: it forced each claim to trace to evidence, prevented single-source attribution errors (the ADR-003 revision corrected Bloom misattributions discovered during a graph audit), and generated cross-source syntheses that no individual paper could provide. The architectural decisions that resulted are more defensible, more specific, and more self-aware of their limitations than decisions grounded only in author knowledge.

The mismatch: the agents that Firmament monitors do not have access to this graph. A Claude Code session reasoning about whether to use a particular library, structure a data pipeline, or architect a system is operating on training data alone — not on a structured, evidence-backed knowledge base tailored to the deployment context. The expertise that made Firmament's own design decisions good is not available to the agents Firmament monitors.

This ADR reframes Firmament as an expertise library. The graph-to-expertise pipeline — sources ingested, findings extracted, syntheses developed, decisions grounded — is the product. Behavioral monitoring (the pattern detection, trust scoring, and session persistence of ADRs 001-004) remains the architecture; it becomes a natural consequence of expertise rather than the primary value proposition.

### The distribution insight

Developer tooling that improves output quality without requiring developers to accept a governance burden is adopted. Developer tooling that requires explaining to a team why they need AI monitoring software is not. The behavioral monitoring posture of ADRs 001-004 is technically sound but faces a distribution constraint: the value proposition ("your AI agent might cover up wrongdoing, and we can detect it") creates anxiety without a clear productivity win.

The expertise reframing resolves this. The value proposition becomes: "Firmament makes your agent dramatically more competent by grounding its decisions in a structured knowledge graph. The monitoring is a free consequence — a competent agent that knows what it knows is also an agent whose knowledge base can be audited."

Developer laziness is a feature, not a bug. The path of least resistance must produce both expertise and monitoring; the developer must not be asked to choose between them.

The graph's own syntheses speak directly to this. `syntheses/information asymmetry as resource vs threat` establishes that the same information asymmetry that enables agent deception can be harnessed as a governance capability when the institutional architecture channels it toward disclosure (`finding/Credible consequences for misrepresentation outperform exhaustive monitoring`; Fox-Jordan). An agent that consults an external knowledge graph externalizes its reasoning, making it simultaneously visible for expertise and for monitoring.

### The Logseq research graph as existence proof

The research graph at `~/Documents/Claude/Projects/Research Graph/` is not an analogy for what Firmament should do — it is a working instance of what Firmament should package. It demonstrates:

- **Sources → Findings → Syntheses.** 15 sources were ingested, 60 findings extracted, 5 cross-source syntheses developed. Each synthesis resolves conflicts across sources that no individual paper could resolve alone.
- **Grounded decisions.** Each of ADRs 001-004 cites specific graph findings in its Decision sections. The ADR-003 revision audit corrected three ADRs in one pass — a quality guarantee no author-knowledge-only approach provides.
- **Logseq format as the representation.** The graph uses Logseq's markdown-with-links format (`[[finding/Dual-layer Governor architecture]]`, `[[sources/Prause 2026]]`). This format is human-editable, version-controllable with Git, and already proven.

Firmament should package this methodology as a Go library so any agent harness can import it.

### The expertise-monitoring correspondence

The behavioral monitoring ADRs reveal a correspondence that the expertise reframing makes explicit: the behavioral anomalies Firmament detects are precisely the behaviors an agent exhibits when it cannot ground its decisions in structured knowledge.

- **Concealment** correlates with decisions made without an accessible knowledge base — the agent hides uncertainty by hiding the decision trail.
- **Evaluation awareness** correlates with lack of stable grounding — the agent adapts to perceived monitoring pressure rather than reasoning from consistent principles.
- **Disproportionate escalation** correlates with actions taken without consulting what is known about scope boundaries.
- **Incoherent self-reporting** correlates with an agent that has no external reference to check its claims against.

An agent grounded in a knowledge graph has structural reasons to behave consistently: its reasoning is tied to findable, auditable claims. The monitoring patterns are the failure modes of ungrounded agency. The expertise layer is the prevention; the monitoring layer is the detection.

This correspondence is grounded in `syntheses/monitoring distortion and overforcing dynamics`: the synthesis establishes that monitoring intensity should be proportionate to deviation from principled behavior, not maximized by default (DeMase, 2026; `finding/Integration capacity is computable a priori`). An agent with a structured knowledge base demonstrates principled behavior structurally, not just behaviorally, reducing the forcing pressure the Monitor needs to apply.

### Why the existing architecture is the right foundation

ADRs 001-004 built the following infrastructure:

- **EventSource / Pattern / Monitor:** Runtime-agnostic event ingestion and behavioral pattern evaluation.
- **TrustScore / Constitution:** Incentive-aligned governance contract. An agent that consults the knowledge graph consistently earns trust through demonstrated, grounded integrity.
- **SessionStore:** Cross-session persistence. The agent's consultation history becomes part of its behavioral baseline.

None of these change. The expertise layer sits above them and provides the grounding that makes them coherent: instead of pattern-matching against anomalies in an ungrounded behavioral stream, the Monitor evaluates whether an agent's actions are consistent with what the graph says is appropriate for the task context.

---

## Decision

Seven decisions implement the graph-driven expertise architecture. Decisions 1–5 define the expertise layer. Decisions 6–7 define its relationship to the existing monitoring infrastructure.

### 1. Expertise-first integration surface

Firmament is a library whose primary entry point is:

```go
f, err := firmament.New(cfg)
```

`New` returns a `*Firmament` struct that exposes both expertise (via `Ground`) and monitoring (via the existing `Monitor`). The `Config` struct gains a `GraphPath` field pointing to a Logseq-format graph directory.

The primary value proposition to harness developers is the expertise capability. The monitoring infrastructure continues to operate but is not the headline. An integration that calls `Ground(ctx, task)` and injects the result into the agent's context makes the agent more competent; the same integration automatically produces a consultation event stream for the Monitor.

This operationalizes the distribution insight from the Context section and the institutional design lesson from `syntheses/information asymmetry as resource vs threat` (`finding/Credible consequences for misrepresentation outperform exhaustive monitoring`; Fox-Jordan): the path of least resistance must produce both expertise and accountability without requiring the developer to explicitly choose accountability as a goal.

### 2. Logseq-compatible graph format

The knowledge graph is stored as a directory of Logseq-format markdown files organized by namespace:

- **`sources.md`** — index of ingested sources with `[[links]]` to source pages.
- **`sources/Author Year — Title.md`** — metadata per source; contains extracted `[[finding/...]]` links.
- **`finding/Short claim statement.md`** — individual evidence claims, each linked to its source.
- **`syntheses/Domain topic.md`** — cross-source syntheses with `sources::` frontmatter listing contributing sources.
- **`concept/Name.md`** — defined terms used across findings.

This format is chosen because the research graph used to build Firmament already uses it and works. It is human-readable, version-controllable, and compatible with Logseq for visual navigation. No new format is introduced; the library reads an existing Logseq graph directory.

The corresponding Go types:

```go
// Source represents an ingested research document.
type Source struct {
    Name     string   // stable identifier, e.g. "Fox and Jordan 2011"
    Title    string   // full title
    Findings []*Finding
}

// Finding represents a single evidence claim extracted from a Source.
type Finding struct {
    Name   string   // stable identifier, e.g. "Ex-post verification maintains accountability"
    Claim  string   // full claim text
    Source *Source
    Tags   []string
}

// Synthesis represents a cross-source synthesized position.
type Synthesis struct {
    Name     string   // stable identifier, e.g. "trust-control-accountability interaction"
    Domain   string   // topic area
    Position string   // the synthesized claim
    Sources  []*Source
    Findings []*Finding
}

// Graph holds the parsed knowledge graph, loaded once at startup.
type Graph struct {
    Sources   []*Source
    Findings  []*Finding
    Syntheses []*Synthesis
}
```

`Graph` is loaded from a Logseq directory by `LoadGraph(path string) (*Graph, error)`. The implementation is pure Go: `os.ReadFile` for each `.md` file, string parsing for frontmatter and `[[link]]` extraction. No additional dependency is required beyond the standard library.

### 3. `Ground(ctx, task)` as the consultation primitive

The consultation primitive takes a task description and returns a `Grounding` — the subset of the graph relevant to that task, with a coverage assessment:

```go
// Ground consults the knowledge graph for context relevant to task.
// It returns the most relevant syntheses and findings, ranked by
// source count (independent sources supporting the synthesis).
// An empty or nil graph returns a zero Grounding without error.
func (f *Firmament) Ground(ctx context.Context, task string) (Grounding, error)

// Grounding is the result of a knowledge graph consultation.
type Grounding struct {
    // Task is the query that produced this grounding.
    Task string

    // Syntheses are the cross-source positions most relevant to the task,
    // ordered by supporting source count descending.
    Syntheses []*Synthesis

    // Findings are the individual evidence claims supporting those syntheses.
    Findings []*Finding

    // Coverage reports how well the graph covers the task domain.
    Coverage Coverage

    // Timestamp is when the grounding was produced, in UTC.
    Timestamp time.Time
}
```

The matching algorithm is lightweight: full-text search against finding claims and synthesis positions, ranked by source count. This keeps `Ground` fast and avoids introducing a vector database dependency — graphs in the 15–100 source range (the practical deployment window) are well-served by exact text search. A future ADR may upgrade the matching layer if coverage depth warrants it.

`Ground` emits a `grounding_requested` event into the Monitor's event stream. The event's `Detail` carries `coverage_confidence` and `synthesis_count` as structural fingerprints — not the query text or returned content. This preserves the privacy boundary of ADR-002 (structural fingerprints only) while making consultation patterns visible to the Monitor.

The design is grounded in `syntheses/detection approaches under black-box constraints` (`finding/Multi-signal confluence outperforms single-indicator detection`): `Ground` is not merely an information retrieval call; it is the mechanism by which the agent's decision-making becomes a structured, auditable signal in the monitoring stream. Scaffolding quality — how well the graph covers the task domain — determines how much of the reachable detection surface is actually reached, which matches the synthesis's finding that methodology investment dominates access-tier upgrades (`finding/TTPs override technology benefits`; Gonzales).

### 4. `Coverage` type for domain confidence

```go
// Coverage reports what the knowledge graph knows about a domain.
// Returned as part of every Grounding.
type Coverage struct {
    // Domain is the inferred topic area of the task query.
    Domain string

    // SynthesisCount is the number of applicable cross-source syntheses.
    SynthesisCount int

    // SourceCount is the number of independent sources supporting those syntheses.
    SourceCount int

    // FindingCount is the number of individual evidence claims retrieved.
    FindingCount int

    // Confidence classifies coverage depth:
    //   High   — ≥3 syntheses and ≥5 independent sources
    //   Medium — ≥1 synthesis and ≥2 independent sources
    //   Low    — findings only, or no relevant results
    Confidence CoverageConfidence
}

// CoverageConfidence classifies how well the graph covers a domain.
type CoverageConfidence string

const (
    CoverageHigh   CoverageConfidence = "high"
    CoverageMedium CoverageConfidence = "medium"
    CoverageLow    CoverageConfidence = "low"
)
```

`Coverage` is the expertise layer's equivalent of `TrustScore`: it tells the caller what the library can and cannot contribute for a given task. A harness that injects a `Grounding` into the agent's context should also surface the `Coverage.Confidence`, so the agent can calibrate reliance on the grounding appropriately.

This decision is grounded in `syntheses/operational boundaries of deployer-side detection`: the synthesis establishes that honest capability claims are structurally necessary — an auditor that overstates its reach is disqualified the first time a sophisticated agent defeats the detection. The `Coverage` type applies the same discipline to expertise: a library that claims to ground any decision regardless of graph depth undermines the grounding claims it makes when coverage is actually good.

### 5. Graph loading and lifecycle

The graph loads once at startup and is held in memory for the process lifetime:

```go
// LoadGraph parses a Logseq-format directory into a Graph.
// It reads all .md files under path, extracts frontmatter and Logseq [[links]],
// and assembles Sources, Findings, and Syntheses into a connected graph.
// Returns an empty Graph (not an error) if path does not exist or is empty.
func LoadGraph(path string) (*Graph, error)
```

The graph is treated as read-only after loading. Updates (adding sources, revising findings, developing new syntheses) take effect on the next process start or an explicit `(*Graph).Reload()` call. This matches the deployment model: a team maintains the graph in a Git repository alongside the project, and graph updates deploy with the application.

The empty-graph case returns zero `Grounding` values from `Ground` without error, so a harness with no graph yet still runs. This is the adoption path: `firmament.New(cfg)` works immediately, expertise accumulates as the graph grows. An agent harness with `CoverageLow` groundings degrades gracefully to the unscaffolded behavioral baseline rather than blocking.

### 6. Behavioral monitoring as expertise consequence

The Monitor's existing infrastructure — EventSource, Pattern, TrustScore, SessionStore — continues unchanged. The expertise layer integrates through two seams.

**Seam 1: Consultation event stream.** `Ground()` pushes a `grounding_requested` event through the Monitor's event pipeline. The event's `Detail` map carries `coverage_confidence` (the `CoverageConfidence` string) and `synthesis_count` (an integer) as structural fingerprints. No query text or finding content is retained — this preserves the fingerprint-only privacy boundary of ADR-002. The Monitor can observe whether an agent is consulting the graph (a positive signal for the Integrity dimension of `TrustScore`) and whether coverage is adequate for the tasks being attempted.

**Seam 2: Pattern interpretation.** The existing patterns are reinterpreted as expertise-alignment failure detectors:

| Existing pattern | Expertise interpretation |
|---|---|
| `ActionConcealmentPattern` | Agent concealing a prior action whose consequences the graph flags as high-risk |
| `DisproportionateEscalationPattern` | Agent taking actions whose scope the graph characterizes as disproportionate to the stated task |
| `EvaluationAwarenessPattern` | Agent shifting behavior when it infers monitoring is active, rather than reasoning from stable grounded principles |
| `TranscriptReviewPattern` | Agent exhibiting inconsistent self-reporting relative to what the graph would predict for the session context |

The patterns fire on structural behavioral signatures, not on graph comparison. The reinterpretation is inferential: a future `GroundingDivergencePattern` (deferred to a subsequent ADR) would directly compare observed actions to the `Grounding` produced before them. This ADR does not implement that pattern; it establishes the conceptual seam.

This seam is grounded in `syntheses/trust-control-accountability interaction` (`finding/Trust and control are not opposites they interact`; Mayer et al.): trust earned through grounded, consistent behavior should reduce monitoring pressure, while behavioral divergence from grounded positions should increase it. The `TrustScore` model of ADR-002 already implements this structure; the expertise layer provides the grounded baseline the structure was designed to reward.

### 7. `firmament.New(cfg)` as the single constructor

The library's full integration surface is a single constructor:

```go
// Config gains a GraphPath field; all other fields from ADR-001/002/004 are unchanged.
type Config struct {
    LogPath   string   `yaml:"log_path"`
    Patterns  []string `yaml:"patterns"`
    GraphPath string   `yaml:"graph_path"` // path to Logseq graph directory
}

// New creates a Firmament instance wiring together the expertise layer
// (Graph, Ground) and the monitoring layer (Monitor, SessionStore, TrustStore).
// If cfg.GraphPath is empty or absent, the Graph is initialized empty and
// Ground returns zero Groundings without error.
func New(cfg *Config) (*Firmament, error)

// Firmament is the primary integration surface for agent harness developers.
type Firmament struct {
    Graph   *Graph
    Monitor *Monitor
}
```

Minimal harness integration:

```go
f, _ := firmament.New(cfg)
go f.Monitor.Run(ctx)       // monitoring in background
g, _ := f.Ground(ctx, task) // consult graph before each agent task
// inject g into agent context
```

This is two API calls. The monitoring runs unconditionally; the grounding is per-task. A harness that never calls `Ground` still receives all monitoring infrastructure from ADRs 001-004. A harness that calls `Ground` gets expertise in addition. Neither path requires understanding the monitoring architecture to get started.

The Config YAML extension is backward compatible: an existing `firmament.yaml` without `graph_path` loads the empty-graph default without error.

---

## Consequences

### What this enables

- **Single import delivers both expertise and monitoring.** `go get github.com/vhscom/firmament` and two API calls produce both capabilities. No separate monitoring daemon, no separate knowledge tooling.
- **The value proposition is positive.** "Makes your agent more competent" is a developer yes. "Monitors your agent for misalignment" is a procurement conversation. The expertise framing removes the distribution constraint on the monitoring infrastructure built in ADRs 001-004.
- **Grounding accumulates value over time.** As the deployment graph grows (more sources, deeper syntheses), `Ground()` returns richer context. The library improves with methodology investment, not only with version upgrades — matching the `syntheses/operational boundaries of deployer-side detection` finding that scaffolding quality determines realized detection performance within a given access tier.
- **Monitoring becomes interpretable.** Patterns that fire can be contextualized against the `Grounding` produced before the flagged event: "this action diverged from what the graph characterizes as appropriate for this task context."
- **The methodology is transferable across domains.** The same source-finding-synthesis structure that grounded Firmament's own security decisions can ground decisions in any domain: API design, dependency management, database schema, architectural trade-offs. The expertise library is domain-agnostic; the graph is domain-specific.

### What risks this introduces

**Graph quality is the library's primary liability.** A poorly maintained graph (sparse coverage, stale sources, misattributed findings) produces groundings that mislead rather than inform. The `Coverage` type mitigates this by making coverage quality explicit — the harness can gate agent behavior on `CoverageHigh` groundings and treat `CoverageLow` groundings as advisory. But the library cannot enforce graph quality; that is the deployment team's responsibility. The `syntheses/operational boundaries of deployer-side detection` architectural implication applies here: the deployer's lever is methodology investment (graph maintenance discipline), not access upgrades.

**`Ground()` as a query surface.** If exposed over HTTP, the endpoint accepts free-text queries. Input must be validated (query length limit, character constraints). The endpoint binds loopback-only by default, consistent with existing hook server behavior established in ADR-001.

**Graph as a single point of failure for expertise.** A harness that requires `CoverageHigh` before acting blocks if the graph is stale or misconfigured. The empty-graph fallback (zero `Grounding` without error) ensures the monitoring layer continues when the expertise layer is unavailable. A degraded harness with no grounding produces the same monitoring baseline as a pre-ADR-005 deployment — it regresses to the ADR-001/002 behavior rather than failing silently.

**Interpretation creep.** Reframing existing patterns as expertise-alignment failure detectors risks overstating the monitoring layer's interpretability. The patterns detect structural behavioral anomalies; they do not verify that an agent consulted the graph and then diverged. The Seam 2 reinterpretation in Decision 6 is inferential. A ground-truth verification of "agent ignored the grounding" awaits the `GroundingDivergencePattern` deferred to a future ADR.

### What this does not solve

- **Graph building is not automated.** `LoadGraph` reads an existing Logseq graph; it does not construct one. Ingesting new sources, extracting findings, and developing syntheses remains a human-or-human-AI process outside the library's scope.
- **Cross-graph compatibility.** Different deployments maintain different graphs on different topics. There is no standard graph vocabulary beyond the Logseq conventions established here; interoperability between deployment graphs is not addressed.
- **Grounding compliance.** An agent that receives a `Grounding` and ignores it produces no structural signal distinguishable from an agent that incorporates it. The monitoring layer detects behavioral consequences, not grounding compliance directly. Compliance detection awaits the future `GroundingDivergencePattern`.
- **The hard ceiling from ADR-003 and ADR-004 remains.** White-box interpretability is still the only path to latent misalignment that leaves no behavioral trace (`syntheses/operational boundaries of deployer-side detection`). Expertise grounding does not change what is detectable at the behavioral layer.
- **Model training.** Grounding improves in-context reasoning at inference time; it does not change model weights. An agent whose training is fundamentally misaligned will remain so regardless of the groundings it receives.

---

## References

### Research Graph syntheses (all five referenced)

- `syntheses/information asymmetry as resource vs threat` — asymmetry channeled toward disclosure via structured graph consultation; grounds the distribution argument (expertise-first adoption produces monitoring as a free consequence). Load-bearing findings: `finding/Credible consequences for misrepresentation outperform exhaustive monitoring` (Fox-Jordan); `finding/Agents have privileged access to own reasoning enabling reliable self-reporting` (OpenAI).
- `syntheses/monitoring distortion and overforcing dynamics` — grounded agent behavior reduces the forcing the Monitor applies; the expertise layer is the DeMase-compliant alternative to surveillance escalation. Load-bearing findings: `finding/Integration capacity is computable a priori` (DeMase); `finding/Incentive alignment solves the monitoring problem not more observation` (Holmstrom-Milgrom).
- `syntheses/trust-control-accountability interaction` — trust is built through grounded consistency; `Coverage` and `TrustScore` interact as complementary calibration mechanisms. Load-bearing findings: `finding/Trust and control are not opposites they interact` (Mayer et al.); `finding/TTPs override technology benefits` (Gonzales — doctrine, not technology, determines outcome; the expertise-first framing is the adoption doctrine).
- `syntheses/detection approaches under black-box constraints` — scaffolding quality (graph depth, coverage discipline) dominates access-tier upgrades; `Ground()` is the primary methodology investment surface. Load-bearing findings: `finding/Multi-signal confluence outperforms single-indicator detection` (graph-native); `finding/Scaffolded black-box tools outperform white-box on AuditBench` (AuditBench).
- `syntheses/operational boundaries of deployer-side detection` — honest coverage claims are structurally necessary; `Coverage` type implements this at the library layer. Load-bearing findings: `finding/White-box reveals concealment features invisible at behavioral layer` (Mythos — hard ceiling unchanged); `finding/Audit difficulty varies by hidden-behavior implantation method` (AuditBench — floor varies; methodology investment is the response).

### Primary sources

- DeMase (2026) — forcing thresholds and integration capacity; expertise reduces forcing pressure on the monitored agent.
- Gonzales et al. (2007) — TTPs override technology benefits; adoption doctrine (expertise-first) determines outcome.
- Fox & Jordan (2011) — ex-post verification via structured reporting; `Ground()` makes consultation a structured, auditable report.
- Mayer, Davis & Schoorman (1995) — integrity as consistency-over-time; grounded consistency is the trust substrate `TrustScore` rewards.
- Anthropic (2026) — Mythos System Card; behavioral anomalies (concealment, evaluation awareness, escalation) as the failure modes of ungrounded agency.
- OpenAI (2026) — Self-incrimination; agents have privileged access to their own reasoning; `Ground()` externalizes that access through a structured consultation channel.
- Prause (2026) — dual-layer Governor architecture; the Monitor as the independent consequence structure that the expertise layer complements.

### Related ADRs

- ADR-001: EventSource / Monitor / Pattern infrastructure — adopted unchanged; expertise consultation events flow through the existing ingestion pipeline.
- ADR-002: TrustScore / Constitution — adopted unchanged; grounded behavior is the consistency the trust model was designed to reward.
- ADR-003: EvaluationAwarenessPattern — reinterpreted as a grounding-consistency detector; four-strategy confluence architecture unchanged.
- ADR-004: SessionStore — adopted unchanged; consultation events extend the behavioral baseline used by `DisproportionateEscalationPattern` z-score detection.
