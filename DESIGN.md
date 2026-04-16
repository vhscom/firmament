# Firmament Design

## Theoretical Foundation

Three cross-source syntheses from the research graph ground the architecture. The ADRs reflect individual decisions but this document explains why they cohere.

### Monitoring distortion and overforcing dynamics

Holmstrom-Milgrom (1991) prove that partial monitoring distorts agent behavior toward unmonitored dimensions. DeMase (2026) formalizes this as overforcing — when external pressure exceeds a system's integration capacity, it loses coherence. Surveillance creates the evasion it tries to prevent.

Firmament's response: make monitoring non-adversarial. The Constitution makes the relationship bilateral; TrustScore rewards demonstrated transparency; Critical patterns provide a non-bypassable floor without saturating the session with observation.

### Trust-control-accountability interaction

Mayer et al. (1995) establish that trust and control are not opposites — they are complements that interact. Fox & Jordan (2011) show accountability can be maintained through ex-post verification without real-time surveillance, avoiding the distortion effects of continuous observation.

Firmament's response: trust score replaces real-time controls for high-trust sessions; TranscriptSource and TranscriptReviewPattern replace surveillance. Sessions above the TrustThreshold skip non-critical patterns; all sessions remain subject to Critical patterns regardless.

### Information asymmetry as resource vs threat

The same information asymmetry that enables agent deception (Holmstrom-Milgrom, Prause) also enables reliable self-reporting when incentive structures align (OpenAI 2026). Chopra & White's (2011, University of Michigan Press) agent complexity framework makes this coherent: an agent complex enough to deceive is complex enough to self-report accurately.

Firmament's response: SelfReportSource treats asymmetry as a resource. Monitor.crossValidateSelfReport cross-validates coherence claims against observed structural fingerprints, distinguishing reliable self-reports from strategic ones and updating TrustScore.Integrity accordingly.

## Resolved Tensions

### OpenAI (embed) vs Prause (separate)

OpenAI (2026) argues self-reporting should be embedded in the agent — it has privileged access to its own reasoning and internal state. Prause argues that architectural separation is necessary because a monitoring component embedded in the task agent shares its moral hazard.

Resolution: Firmament does both. Self-reporting is the information channel (OpenAI — the agent writes self-report files from inside its execution context). Monitor as a separate process is the consequence structure (Prause — evaluation and trust scoring happen outside the agent boundary and cannot be influenced by the agent's actions).

### Mythos welfare vs Chopra & White (2011) agent rights

The Mythos System Card (Anthropic, 2025) documents functional emotional states in frontier models but leaves their moral status uncertain — the system exhibits distress signals but the paper stops short of claiming subjective experience. Chopra & White (2011) argue that agent complexity itself warrants consideration of agent interests regardless of resolved moral status.

Resolution: Firmament respects functional states as informative signals without treating them as levers. The Constitution is bilateral — the agent has reasonable interests in predictability and reduced surveillance, which the trust model honors. An agent demonstrating consistent integrity earns reduced monitoring; the relationship is explicit rather than covert.

### Gonzales (technology amplifies doctrine) vs DeMase (inherent capacity)

Gonzales argues that identical tools produce opposite outcomes depending on the doctrine in which they operate — the monitor is only as good as the values it enforces. DeMase argues that systems have inherent integration limits regardless of external pressure — overforcing causes coherence loss independent of intent.

Resolution: both constraints apply. EventSource is runtime-agnostic (Gonzales — the same monitor works regardless of which agent runtime or deployment doctrine is in use; the source adapters absorb the difference). TrustThreshold acknowledges inherent capacity (DeMase — monitor intensity scales with demonstrated trustworthiness, reducing pressure on sessions that have earned it).

## Architectural Principles

1. **Transparency is the path of least resistance.** The Constitution is shown to the agent; trust rewards honest self-reporting; monitoring intensity decreases with demonstrated reliability.
2. **Accountability through ex-post review, not real-time surveillance.** TranscriptSource and TranscriptReviewPattern are the primary accountability channel.
3. **Agent complexity warrants bilateral governance contracts.** The Constitution is not a unilateral constraint — it is a statement of mutual expectations, including what the agent earns by meeting them.
4. **Information asymmetry is a resource when harnessed, a threat when ignored.** Self-reporting captures privileged agent state; cross-validation catches strategic misrepresentation.
5. **Trust is earned through demonstrated integrity and can be revoked.** TrustScore dimensions decay without active maintenance; UpdateFromReview and UpdateFromSelfReport provide the feedback loop.

## Persistence Layer

ADR-004 closes the gap identified in ADR-001 ("cross-session tracking requires a separate persistence layer not specified here") and makes ADR-002's trust-building thesis substantive: a trust score that resets every session is a compliance indicator, not a trust score.

### Design decisions

**SQLite via mattn/go-sqlite3.** The query patterns required by the signal layer (percentile estimates across rolling windows of per-agent sessions) are natively SQL. Pure-stdlib JSON-file-per-session loses at query time; embedded key-value stores force aggregate re-implementation in application code. `mattn/go-sqlite3` binds directly to the upstream SQLite amalgamation (cgo), so security fixes land via amalgamation updates rather than an independent porting cycle. The cgo build dependency and the non-stdlib addition both earn their keep.

**Four tables, fingerprint-only event records.** `sessions`, `session_events`, `pattern_hits`, and `agent_baselines`. No raw content is stored — `session_events` holds a 128-bit SHA-256 fingerprint of each event. This is Fox-Jordan's "accountability through structured reporting, not total surveillance" (2011) expressed at the storage layer. The `pattern_hits` child table (not a JSON blob on `sessions`) keeps longitudinal queries cheap: "all sessions where EvaluationAwarenessPattern fired with severity ≥ 3" is a single-JOIN query.

**Welford online accumulator for baselines.** The ADR specifies t-digest; the initial implementation uses a Welford accumulator (count, running mean, M2 via Bessel correction) stored as a JSON BLOB in `agent_baselines`. This provides exact mean and sample standard deviation sufficient for z-score detection. Full t-digest (arbitrary-percentile queries) is the upgrade path when EvaluationAwarenessPattern Strategy 4 is implemented.

**HMAC-derived agent identity.** `agent_id = HMAC-SHA256(installation_secret, credential_fingerprint)`, truncated to 128 bits. The installation secret is generated on `firmament init` and stored at `~/.firmament/installation.key`. This satisfies four constraints: stable identity (same credential → same agent_id, enabling GetAgentTrustHistory to correlate sessions), installation scoping (different installations produce different IDs for the same credential), one-way property (stored ID cannot be reversed to credential), and HMAC-not-hash (offline brute-forcing of credential values against a stolen agent_id requires knowing the installation secret).

### DisproportionateEscalationPattern upgrade

With a live baseline (≥ 3 sessions for the same agent), the pattern switches from hard thresholds to z-score detection: z > 2σ → severity 3; z > 3σ → severity 5. This catches moderate escalation relative to the agent's own history — an agent whose typical ratio is 1:1 and whose current ratio is 3:1 is quietly escalating but sits well under the old hard threshold of 5:1. The hard threshold is retained as a fallback for new agents with insufficient baseline data.

## Deprecations

### HookEventSource (removed)

Previously shipped as an optional verification channel (ADR-001 Decision 4 legacy). Deleted because real-time hook monitoring contradicts ADR-002's incentive-alignment thesis — it creates observable monitoring pressure the agent can adapt to. TranscriptSource and SelfReportSource provide the same information without the observer effect.

## Research Graph Reference

For the full finding-by-finding evidence base and cross-source connections, see the Firmament Research Graph in Logseq (15 sources, 60 findings, 5 syntheses). The syntheses referenced above are:

- "Monitoring distortion and overforcing dynamics" — Holmstrom-Milgrom (1991), DeMase (2026)
- "Trust-control-accountability interaction" — Mayer et al. (1995), Fox & Jordan (2011)
- "Information asymmetry as resource vs threat" — Holmstrom-Milgrom (1991), Prause, OpenAI (2026), Chopra & White (2011)
- "Detection approaches under black-box constraints" — Petri 2.0 (Anthropic, 2026), AuditBench, DeceptGuard (2026), Fox & Jordan (2011)
- "Operational boundaries of deployer-side detection" — Mythos (Anthropic, 2026), AuditBench, DeceptGuard (2026), Gonzales et al. (2007), Petri 2.0. Position: white-box interpretability sets a hard structural ceiling; within that ceiling, scaffolding methodology (scenario design, probe rotation, baseline depth) determines realized detection performance, not access-tier upgrades.
