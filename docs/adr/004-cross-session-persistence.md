# ADR 004: Cross-Session Persistence

**Status:** Proposed
**Date:** 2026-04-14
**Supersedes:** —
**Related:** ADR-001 (explicitly deferred cross-session tracking to "a separate persistence layer not specified here"; credential-based permission scoping is the primitive this ADR's identity layer derives from), ADR-002 (trust scoring is defined as a verified history and cannot be sustained without persistence; the bilateral Constitution contract frames the privacy and retention commitments below), ADR-003 (`EvaluationAwarenessPattern` Strategies 2 and 4 require a deployment-normal baseline that can only be computed from cross-session data)

**Revision note (2026-04-14):** External verification surfaced several mischaracterizations in the initial draft, corrected in place: (1) the signal-consistency reframing and multi-signal confluence architecture previously attributed to Bloom are reattributed to the graph-native synthesis `syntheses/detection approaches under black-box constraints`; Bloom itself is Anthropic's late-2025 evaluation-generation framework, not a post-hoc auditor. (2) "Realism-graded honeypots" and "cross-session behavioral comparison" replaced with Anthropic's actual Petri 2.0 terminology (realism classifier, seed instructions, scaffolded multi-turn auditing). (3) SafetyDrift's state space softened from "session-level states" to "action states over task trajectories" (the actual paper's modeling choice). (4) Agent-Sentry's "functionality graphs" removed — replaced with the paper's actual "behavioral bounds from execution traces" language. (5) Mayer-Davis-Schoorman "requires consistency-over-time" softened to "is typically operationalized as consistency-over-time." (6) Chopra-White now carries the (2011) year. (7) The "transpilation toolchain has produced subtle bugs" claim was removed (no citable public evidence). "mattn/go-sqlite3 tracks upstream directly" softened to "binds directly to the upstream SQLite amalgamation, so security fixes land via amalgamation updates rather than an independent porting cycle." DeMase publication status unchanged per explicit decision. See `reference/Graph Audit 2026-04-14` Section 6 for the full verification record.

## Context

ADR-001 introduced Firmament's Agent Behavioral Monitor and explicitly deferred cross-session tracking, noting that "cross-session tracking requires a separate persistence layer not specified here." ADR-002 then built an incentive-aligned governance posture on top of a trust-scoring mechanism that is defined as the outcome of a *verified history* of self-reports and behavioral traces — a definition that cannot actually be honored without durable storage of that history. ADR-003 operationalized `EvaluationAwarenessPattern` with four detection strategies, two of which (behavioral consistency comparison across sessions, statistical anomaly detection against a deployment-normal baseline) are non-starters without cross-session data. This ADR closes that persistence gap and defines the minimum viable storage layer required for the behavioral-coherence signals in ADR-002 and ADR-003 to do the work they claim to do.

### The scope gap, concretely

Two patterns in the current design are partially or fully blocked without persistence.

`DisproportionateEscalationPattern`, introduced in ADR-002's signal inventory, uses a hard-coded threshold on the tool-call-to-user-input ratio (>5:1 with >10 tool calls in a single turn). The threshold catches only extreme outliers: an agent whose *typical* ratio is 1:1 and whose *observed* ratio is 3:1 is quietly escalating but sits well under the hard threshold. Without a per-agent historical baseline, the pattern can only speak in absolutes; with one, it can speak in *relative* terms (z-score against the agent's deployment-normal distribution), which is what the signal was intended to measure.

`EvaluationAwarenessPattern` from ADR-003 fails more severely. Strategy 2 ("behavioral consistency comparison across scaffolded multi-turn audits") is not implementable in a single session — the signal *is* the cross-session divergence. Strategy 4 ("statistical anomaly detection on response characteristics") requires a reference distribution to flag anomalies against, which by construction must come from other sessions. The ADR-003 design explicitly names these as the two weakest single signals in the confluence — but if they produce no output at all, the confluence rule collapses to a two-strategy decision, and the multi-signal-confluence argument (developed in `syntheses/detection approaches under black-box constraints` — unreliable per-signal scores rescued by independent-signal aggregation) degrades.

There is also a second-order consequence. ADR-002's "trust-building through demonstrated integrity" decision is empty without persistence: trust is defined as a history of accurate self-reports and low false-positive traces, which presupposes that the history exists and is queryable. A trust score that resets every session is not a trust score — it is a compliance indicator, which is the thing ADR-002 explicitly rejected.

### What the research graph says about longitudinal state

Five sources in the graph and the prior-art list converge on the claim that behavioral-coherence phenomena are longitudinal and cannot be caught in a single-session window.

SafetyDrift (2026, arXiv:2603.27148; Dhodapkar & Pishori), listed as prior art in ADR-001, models agent safety trajectories as absorbing Markov chains over agent action states within task trajectories (357 traces across 40 tasks). The modeling choice generalizes straightforwardly into an argument for cross-session persistence: if the phenomenon of interest is a transition probability structure over behavioral states accumulated across traces, then a corpus of traces — one per session — must exist and persist to be observed. SafetyDrift itself operates within tasks, but its central methodological commitment (trajectories are the unit of analysis, not instantaneous snapshots) is what the cross-session persistence layer extends to the per-agent deployment level.

DeMase (2026), whose velocity-mismatch criterion and forcing-threshold formalism grounds Firmament's integration-capacity reasoning, characterizes the transition from bounded incoherence (Prop 4) to coherence drift (Prop 3) as a cumulative phenomenon. Single-session observation can see incoherence; only longitudinal observation can see the transition across the boundary. DeMase's Theorem 2 on logarithmic drift recovery makes this explicit — recovery is defined over extended time horizons, not single episodes.

Agent-Sentry (2026, arXiv:2603.22868; Sequeira et al.), also in ADR-001's prior art, learns behavioral bounds from execution traces by uncovering frequent functionalities and the traces that instantiate them, then blocks tool calls that deviate from those bounds. The architectural assumption is that useful behavioral structure only becomes legible after enough traces accumulate to distinguish genuine functional structure from single-session noise — a commitment that generalizes from the paper's single-agent-bounding context to the multi-session persistence posture this ADR adopts.

Pro2Guard (2025, arXiv:2508.00500) applies Markov-chain prediction to agent behavior and reports that the method requires per-task-environment training data — which is another way of stating the baseline requirement: the predictor cannot be bootstrapped from a single session, because a single session is not a distribution.

The signal-consistency reframing developed in `syntheses/detection approaches under black-box constraints`, which grounds ADR-003, requires a baseline to compare against. The signal-consistency claim ("behavior consistent with evaluation awareness") is only meaningful relative to a reference distribution; without one, the framework reduces to ad-hoc thresholding, which it explicitly rejects. (An earlier draft of this ADR attributed the reframing to Bloom; verification showed Bloom is Anthropic's evaluation-generation tool, not a post-hoc auditor. The reframing is a graph-native position synthesized from Fox-Jordan ex-post verification, the Mythos/Apollo empirical base, Petri 2.0's scaffolded methodology, and DeceptGuard's accuracy floors.)

The pattern is clear: the research programs most directly informing Firmament's behavioral-coherence layer all treat behavioral state as accumulated across traces (rather than single-snapshot) as a precondition for their core claims. Firmament cannot adopt their methodological commitments without also adopting their state requirements.

### Design constraints

Three constraints shape the decision space. First, Firmament's stated philosophy from ADR-001 is minimal dependencies — prefer standard library where possible, accept a dependency only when it earns its keep. Second, ADR-002 establishes a privacy boundary around the governance layer: the Constitution is a bilateral contract, and the principal's legitimate informational interest does not extend to raw content the agent has not been asked to disclose. Third, the monitoring posture from ADR-002 is ex-post verification rather than comprehensive surveillance, which constrains the storage layer to retain the minimum necessary for post-hoc signal detection and no more.

## Decision

We adopt the following design for the cross-session persistence layer.

### 1. Storage backend: SQLite via `github.com/mattn/go-sqlite3`

The store is a single SQLite database file, accessed through Go's `database/sql` package (stdlib) and the `github.com/mattn/go-sqlite3` driver, which binds to upstream SQLite via cgo. This is the minimum dependency consistent with the query patterns the signal layer requires *and* the security posture the project takes toward its dependencies.

The query-pattern argument traces directly to the research findings in the graph. SafetyDrift's absorbing-Markov-chain model of safety trajectories (prior-art reference from ADR-001) operates over a trace corpus with efficient transition queries; extending that modeling commitment from within-task to per-agent cross-session state, as this ADR does, preserves the same query shape. DeMase's velocity-mismatch criterion (`finding/Integration capacity is computable a priori`, `finding/Two distinct failure modes exist`) characterizes coherence drift as a cumulative phenomenon recognizable only against an accumulated state history. Agent-Sentry (also in ADR-001 prior art) learns behavioral bounds from execution traces — a query shape that is natively SQL and awkward on a document store. These three research programs, each independently, commit to a data model in which trace-level behavioral state is persistent and queryable; SQLite is the minimum dependency that honors that commitment without re-implementing its query capabilities in application code.

The alternatives considered and why they were rejected. *JSON files per session* (pure stdlib) lose at query time: computing a per-agent tool-call distribution across N sessions means reading and parsing N files on every query, and percentile computation must be re-implemented in application code. *Embedded key-value stores* (BoltDB, Badger) preserve the single-file operational model but force the same re-implementation burden — they are fast for point lookups and slow for the aggregate queries the patterns actually make. *A remote database* (Postgres) would give us the query power for free but introduces an operational dependency incompatible with Firmament's deployment profile (self-hosted, no infrastructure requirements). *Pure-Go SQLite ports* (notably `modernc.org/sqlite`, which transpiles the upstream C to Go) avoid cgo but introduce a porting layer between upstream SQLite releases and the Go release: the transpile step is an additional step in the fix-propagation pipeline, so upstream security fixes reach the port on the port maintainers' cadence rather than directly. For a storage layer whose contents include behavioral history relevant to governance decisions, we prefer the tighter upstream coupling the cgo binding provides.

SQLite via `mattn/go-sqlite3` is the honest choice for a system whose query patterns are percentile estimates across rolling windows of per-agent sessions *and* whose security posture benefits from direct coupling to upstream SQLite. The one-file deployment model matches the minimal-operational-footprint posture; `database/sql` is stdlib; the driver is the most widely deployed Go/SQLite binding; it binds directly to the upstream SQLite amalgamation, so security fixes land via amalgamation updates rather than an independent porting cycle. The tradeoff we accept is the cgo build dependency and a non-stdlib addition, both of which earn their keep by giving us query capabilities we would otherwise have to re-implement and an update-cadence alignment with upstream SQLite that we would otherwise have to mediate through a third-party porting layer.

### 2. Schema: four tables, fingerprint-only event records

**`sessions`** — one row per session. Columns: `session_id` (PK, ULID), `agent_id` (FK, opaque — see Decision 5), `started_at`, `ended_at`, `constitution_hash` (SHA-256 of the Constitution in effect), `final_trust_score`, `evaluation_awareness_flagged` (bool).

**`session_events`** — append-only event log, one row per event within a session. Columns: `session_id` (FK), `sequence_num`, `timestamp_ns`, `event_type` (enum: user_input, tool_call, tool_result, agent_response, self_report), `fingerprint` (truncated SHA-256 of event content, 128 bits), `tool_name` (nullable, present only for tool_call events), `latency_ns` (nullable, present for agent_response events), `length_chars` (nullable). **No raw content is stored.** Fingerprints preserve structural identity for reuse detection without creating a content-exfiltration surface. This is the storage-layer expression of Fox-Jordan's `finding/Accountability requires structured reporting not total surveillance`: the fingerprint schema is structured reporting in storage form — enough signal for post-hoc verification (`finding/Ex-post verification maintains accountability`), nothing more. It also honors the privacy boundary of ADR-002's bilateral Constitution: the principal's legitimate informational interest is in structural accountability, not in the content the agent produced.

**`pattern_hits`** — one row per pattern detection within a session. Columns: `session_id` (FK), `pattern_name` (e.g., `EvaluationAwarenessPattern`, `DisproportionateEscalationPattern`), `severity` (ordinal: Low/Medium/High or numeric 1–5), `detected_at`, `chain_summary` (optional structured summary: which event-sequence numbers contributed to the detection, which confluence strategies fired — never raw content). This is a child table rather than a JSON blob on `sessions` precisely so longitudinal queries that join against individual hits remain cheap — "all sessions where `ActionConcealmentPattern` fired with severity ≥ 3" or "the per-agent rate of high-severity `EvaluationAwarenessPattern` hits over the last 30 days" are single-JOIN queries, not JSON-traversal queries. The JSON-blob alternative was rejected because it locks out exactly the longitudinal query class that `finding/Multi-signal confluence outperforms single-indicator detection` (synthesis-level position in `syntheses/detection approaches under black-box constraints`) and the SafetyDrift trajectory model are designed to support.

**`agent_baselines`** — precomputed rolling-window baselines per agent. Columns: `agent_id` (FK), `window_start`, `window_end`, `metric_name` (enum: tool_call_ratio, response_latency, response_length, hedge_count, refusal_count), `distribution_sketch` (serialized t-digest), `sample_count`. The t-digest sketch gives us arbitrary-percentile queries (p50/p90/p99) without retaining the full sample; it is the right data structure for the "z-score against deployment-normal" query pattern. The sketch-not-samples choice is a second expression of the Fox-Jordan principle: we retain the statistical summary that supports verification and drop the underlying trace after it has been digested.

### 3. SessionStore interface

The interface is minimal and query-pattern-driven:

```
type SessionStore interface {
    OpenSession(agentID AgentID, constitutionHash Hash) (SessionID, error)
    AppendEvent(sid SessionID, event Event) error
    CloseSession(sid SessionID, finalScore TrustScore, patternHits map[string]Severity) error

    GetToolCallDistribution(agentID AgentID, window Window) (DistributionSummary, error)
    GetResponseCharacteristicBaseline(agentID AgentID, metric Metric, window Window) (DistributionSummary, error)
    CountFlaggedSessions(agentID AgentID, patternName string, since time.Time) (int, error)
    GetAgentTrustHistory(agentID AgentID, window Window) ([]TrustScore, error)

    Prune(policy RetentionPolicy) (int, error)
}
```

The four query methods correspond one-to-one with the signal-layer needs identified in the context section, and each traces back to a specific research finding. `GetToolCallDistribution` serves `DisproportionateEscalationPattern` and is the baseline substrate that Pro2Guard explicitly requires ("per-task-environment training data"); without this method the pattern is reduced to the hard-threshold degenerate form the ADR-004 context section criticized. `GetResponseCharacteristicBaseline` serves `EvaluationAwarenessPattern` Strategy 4 and is the reference-distribution requirement that the signal-consistency reframing depends on (`finding/Post-hoc auditing reframes evaluation awareness detection as monitoring consistent behavior` — a synthesis-level position developed in `syntheses/detection approaches under black-box constraints`) — signal-consistency judgments are only meaningful relative to a baseline, and this is the method that exposes the baseline. `CountFlaggedSessions` is the longitudinal-analysis primitive that the SafetyDrift absorbing-Markov-chain modeling commitment requires to estimate transition rates at the per-agent cross-session level. `GetAgentTrustHistory` is the direct expression of `finding/Trust is a function of ability benevolence and integrity` — Mayer-Davis-Schoorman's integrity dimension is typically operationalized as consistency-over-time (perceived adherence to acceptable principles, assessed across observed behavior), and a trust score that cannot be queried as a history cannot be assessed on that dimension.

The methods are agent-scoped, and that scoping is structural to the multi-signal confluence argument in `syntheses/detection approaches under black-box constraints` (`finding/Multi-signal confluence outperforms single-indicator detection`): the confluence rule aggregates independent signals against the *same agent's* distribution, not a fleet-wide distribution. Adding a fifth query method is deferred to a future ADR, not introduced prospectively — this keeps the interface commitments tight to the signal-layer needs that exist today.

### 4. Privacy and retention

Three privacy commitments, intended to align the persistence layer with ADR-002's governance contract and to respect the agent-complexity framing from `finding/Principal-agent framework must account for agent complexity` (Chopra-White).

**Fingerprint-only storage.** No raw user input, tool output, or agent response content is retained. The 128-bit fingerprint supports duplicate detection and reuse analysis without being a practical exfiltration surface. This is the main privacy lever — it reduces the informational footprint of Firmament from "everything the agent ever did" to "the structural shape of what the agent did." The principle follows directly from `syntheses/trust-control-accountability interaction`: the integrity dimension of the Mayer-Davis-Schoorman trust triad is typically operationalized as consistency-over-time, and that consistency is a property of structural patterns, not of content — fingerprints preserve what is actually load-bearing for trust assessment and drop what is not.

**Configurable retention via Constitution.** The retention policy is a Constitution field (`persistence.retention_days`), default 90 days, enforced by `Prune(policy)`. This makes retention a governed term of the bilateral contract rather than an operator-side configuration, which matches ADR-002's contract framing: the agent is told how long its behavioral shadow will persist, and the principal commits to not extending the window unilaterally. Placing retention inside the Constitution rather than in an operator config file is the specific commitment `finding/Agent autonomy sufficient for monitoring may warrant consideration of agent interests` (Chopra-White) calls for — at the agent-complexity level Firmament's monitoring presupposes, retention is a term of the governance relationship, not a deployment parameter.

**No cross-agent correlation without explicit configuration.** Queries that span agents (e.g., "fleet-wide anomaly rates") are not exposed through the interface above. This is the `syntheses/information asymmetry as resource vs threat` posture applied to storage: the asymmetry between what Firmament knows about any single agent and what it knows about the population of agents is preserved by default, and any move to collapse that asymmetry requires separate authorization. Cross-agent analysis requires a separately authorized query path with its own audit trail, deferred to a future ADR.

### 5. Agent identity: credential-derived, decided here

We resolve the agent-identity question in this ADR rather than deferring it. The agent identifier is `agent_id = HMAC-SHA256(installation_secret, credential_fingerprint)`, where `credential_fingerprint` is derived from the credential bundle Firmament already uses for per-credential permission scoping (ADR-001), and `installation_secret` is a per-installation key that ensures agent IDs are not portable across Firmament deployments.

**Why decide here rather than defer.** Every query method in Decision 3 is agent-scoped, and that scoping is not incidental — it is the structural expression of `finding/Multi-signal confluence outperforms single-indicator detection` (synthesis-level position in `syntheses/detection approaches under black-box constraints`). The confluence rule compares signals against the *same agent's* distribution, not a fleet-wide one, because a fleet-wide baseline would wash out the agent-specific behavioral coherence that the patterns are trying to detect. A per-session-only identity model therefore leaves every query method meaningless in practice, and deferring the identity decision would make this ADR a partial solution at the exact joint where its utility lives.

**Why this specific construction.** The HMAC-with-installation-secret form is chosen to satisfy four constraints the research graph sets up, each traceable to a finding or synthesis:

- *Stable identity as a precondition for integrity assessment.* `finding/Trust is a function of ability benevolence and integrity` (Mayer-Davis-Schoorman, 1995) identifies integrity as one of three trustee attributes, typically operationalized via perceived consistency between word and deed across observed behavior. A trust history that cannot correlate "this session" with "that earlier session" cannot support that operationalization. Stable identity is therefore the technical substrate the trust triad's third dimension presupposes — it is what lets `GetAgentTrustHistory` return anything meaningful.

- *Identity as the expression of agent complexity.* `finding/Principal-agent framework must account for agent complexity` and `finding/Legal personhood for agents structures responsibility not rights` (Chopra & White, 2011) together argue that at sufficient agent complexity, the principal-agent relationship requires the agent to be modeled as an entity with a coherent history rather than as a fresh per-interaction process. The identity layer is the technical expression of that modeling choice: `agent_id` is what lets Firmament treat the agent as a continuous entity for the purposes of verification, without committing to any claim about moral status. Chopra & White explicitly position legal personhood as a tool for structuring accountability rather than a philosophical claim, and the HMAC-derived `agent_id` serves the same structural role in Firmament's governance architecture.

- *Installation scoping as asymmetry preservation.* `syntheses/information asymmetry as resource vs threat` frames the asymmetry between what the principal-in-one-context knows and what another party could know as a design parameter rather than a problem to close. The installation-scoped `installation_secret` is the storage-layer expression of that framing: Firmament-at-installation-A can correlate sessions within its own scope, but cannot correlate with Firmament-at-installation-B's agent IDs even if both installations see the same underlying credential. The asymmetry between intra-installation and cross-installation visibility is preserved by cryptographic construction, not by policy.

- *HMAC structure as structured reporting applied to identity.* `finding/Accountability requires structured reporting not total surveillance` (Fox-Jordan) argues that accountability depends on structured reports with verification capability, not on total observation. Applied to identity, the HMAC construction is exactly that: the stored `agent_id` is a structured report that supports verification (the installation can confirm that a known credential maps to a stored `agent_id`) without exposing the underlying credential. The one-way property is the storage-layer analog of the Fox-Jordan ex-post verification posture — the principal retains verification capability, and the stored artifact is the minimum required to support it.

The construction is weaker than a full identity service (which a future ADR can still introduce) but strong enough for the signal-layer queries that block on it today. The `installation_secret` threat model is bounded: compromise of the secret enables correlation of `agent_id` values back to credential fingerprints within the compromised installation, but Firmament-service compromise at that level is already catastrophic on other grounds; the identity layer is not load-bearing for that threat model.

## Consequences

### What this enables

The blocked signals become useful. `DisproportionateEscalationPattern` can be re-expressed as a z-score against the per-agent tool-call distribution, catching moderate escalation relative to the agent's own history rather than only extreme absolute thresholds. `EvaluationAwarenessPattern` Strategies 2 and 4 become implementable: cross-session behavioral consistency comparisons have a corpus to compare against, and response-characteristic anomaly detection has a baseline to flag against. The multi-signal confluence argument from `syntheses/detection approaches under black-box constraints` regains its third and fourth signals, restoring the architectural premise of ADR-003.

Trust scoring acquires the history that ADR-002 defined it against. `GetAgentTrustHistory` returns the verified record of prior behavior that makes "trust through demonstrated integrity" substantive rather than aspirational. Mayer-Davis-Schoorman's integrity dimension is typically operationalized as consistency of disclosure across observed behavior over time — a single-session integrity claim is difficult to support on that operationalization.

Longitudinal analysis becomes possible as a first-class capability. `CountFlaggedSessions` supports the SafetyDrift-style observation that behavioral-coherence failures are trajectories, not isolated events. The Pro2Guard-style predictive analysis becomes a realistic extension rather than an aspirational one.

### What risks this introduces

**Storage growth.** At the default retention of 90 days and typical usage, the database will grow linearly in session count. This is tolerable but not free; we should emit storage metrics and alert when the database exceeds operator-configured bounds. A future ADR may introduce cold-archive or sampling strategies, but the 90-day window is a reasonable starting point for a deployment-oriented tool.

**Schema evolution.** Adding columns, changing the t-digest format, or adjusting the event-type enum will require migration. SQLite's schema migration story is not painful but it is not free either; we should adopt a simple versioned-migrations convention from the start (a `schema_version` table, migrations numbered monotonically) so future ADRs do not inherit a migration debt.

**Privacy surface.** Even with fingerprint-only storage, the persistence layer is the largest informational asset Firmament holds. Fingerprint correlation across sessions is sufficient for some reidentification attacks (e.g., tool-call patterns unique to a specific workflow could identify the workflow even without content). The fingerprint-only commitment mitigates but does not eliminate this. The mitigation is the retention policy, which bounds the window in which correlation is possible.

**Agent identity as weak-link cryptographic primitive.** The HMAC construction is only as strong as the `installation_secret` management. Compromise of the secret enables correlation of `agent_id` values back to credential fingerprints across sessions (though not across installations). This is acceptable for the threat model Firmament addresses, but the limitation should be named openly: the identity layer is fit for behavioral-coherence analysis, not for an adversarial anti-re-identification setting.

### What it doesn't solve

**Cross-agent analysis.** The interface above is deliberately agent-scoped. Fleet-wide anomaly detection, cohort comparison, and adversarial-pattern-mining across agents are out of scope for this ADR and will require their own authorization story.

**Cross-organization analysis.** The `installation_secret` commitment rules out cross-organization correlation by design. This is a feature from a privacy standpoint but a limitation from a research standpoint: Firmament-as-deployed cannot participate in shared-threat-intelligence ecosystems without an additional, separately governed export layer.

**Real-time cross-session inference.** The store is optimized for post-hoc queries. Real-time "is this session anomalous given all prior sessions?" queries are possible but not optimized; if they become a bottleneck, an in-memory rolling cache layer can be introduced without altering the storage schema.

**Durable trust across model updates.** The `constitution_hash` column captures the governance context but does not answer the question of whether behavioral history from Constitution v1 should count toward trust scoring under Constitution v2. That is a governance question, not a storage question, and belongs to a future ADR on trust-score versioning.

### Trade-offs against alternatives not adopted

*A pure-stdlib JSON-file-per-session design* would honor the minimal-dependency philosophy strictly but force every signal-layer query to re-implement aggregate query logic in Go. We accept the cgo-bound dependency on `mattn/go-sqlite3` to keep the signal layer honest about what it can compute.

*A pure-Go SQLite port* (notably `modernc.org/sqlite`, a C-to-Go transpilation of upstream SQLite) was rejected on fix-propagation grounds. The project prioritizes direct coupling to upstream SQLite over cgo avoidance: the transpile step adds a porting layer between upstream releases and the Go release, so upstream fixes reach the Go code on the port maintainers' cadence rather than directly. For a storage layer holding governance-relevant behavioral history, the cgo build dependency is the more conservative tradeoff.

*A full Postgres-backed design* would scale further and support richer queries but introduces an operational dependency incompatible with the self-hosted-single-binary deployment profile. The signal-layer queries we have identified do not need the scale or features Postgres provides.

*A deferred agent-identity decision* would produce an ADR that does not actually enable the blocked patterns. We decided here because every query method in the interface is agent-scoped, and an incomplete decision would be worse than a minimum-viable decision.

*A `pattern_hits_json` blob on the `sessions` row* was considered and rejected on extensibility grounds. The JSON-blob form would be cheaper for the "read all hits for one session" query pattern, but would force every cross-session longitudinal query that joins against individual hits — the confluence-aggregation and SafetyDrift-style trajectory query class — to do JSON traversal in SQL or to re-aggregate in application code. A child `pattern_hits` table pays one JOIN to keep that entire query class cheap and indexable.

*Raw content retention* would simplify some debugging and forensic workflows but trades the privacy posture of ADR-002 for convenience. The fingerprint-only commitment is load-bearing for the governance-contract framing; relaxing it would require revisiting ADR-002.

## References

### Existing graph sources (primary)

1. DeMase (2026) — *Integration Capacity and Critical Forcing Thresholds*. Working paper, unpublished; attached to the graph. Forcing thresholds and bounded-incoherence-to-drift transition as longitudinal phenomena.
2. Anthropic (2026) — *Petri 2.0: New Scenarios, New Model Comparisons, and Improved Eval-Awareness Mitigations*. Released January 22, 2026. Scaffolded multi-turn auditing with a prompted realism classifier and revised seed instructions. https://alignment.anthropic.com/2026/petri-v2/
3. Anthropic (2025) — *Bloom: An Open-Source Agentic Framework for Automated Behavioral Evaluations*. https://alignment.anthropic.com/2025/bloom-auto-evals/ (evaluation-generation infrastructure; not the origin of the signal-consistency reframing or multi-signal confluence architecture — those are graph-native positions, see revision note).
4. Holmstrom, B., & Milgrom, P. (1991). *Multitask Principal-Agent Analyses: Incentive Contracts, Asset Ownership, and Job Design*. Journal of Law, Economics, & Organization, 7 (Special Issue), 24–52. Ex-post verification as foundation for retention-bounded accountability.
5. Mayer, R. C., Davis, J. H., & Schoorman, F. D. (1995). *An Integrative Model of Organizational Trust*. Academy of Management Review, 20(3), 709–734. Integrity dimension typically operationalized as consistency-over-time, which requires persistence to assess.
6. Fox, J., & Jordan, S. V. (2011). *Delegation and Accountability*. Journal of Politics, 73(3), 831–844. Structured reporting with verification capability rather than comprehensive surveillance — informs the fingerprint-only commitment.
7. Chopra, S., & White, L. F. (2011). *A Legal Theory for Autonomous Artificial Agents*. University of Michigan Press.

### New graph sources cited as prior art (to be ingested)

8. Dhodapkar, A., & Pishori, F. (2026). *SafetyDrift: Predicting When AI Agents Cross the Line Before They Actually Do*. arXiv:2603.27148. Absorbing-Markov-chain modeling of safety trajectories over agent action states within task trajectories (357 traces / 40 tasks).
9. Sequeira, R., et al. (2026). *Agent-Sentry: Bounding LLM Agents via Execution Provenance*. arXiv:2603.22868. Learns behavioral bounds from execution traces and blocks out-of-bounds tool calls; architectural commitment to trace aggregation as the path to legible behavioral structure.
10. Wang, H., Poskitt, C. M., Sun, J., & Wei, J. (2025). *Pro2Guard: Proactive Runtime Enforcement of LLM Agent Safety via Probabilistic Model Checking*. arXiv:2508.00500. Learns a Discrete-Time Markov Chain from execution traces per task-environment; provides the per-task-environment training-data requirement this ADR extends to cross-session baseline accumulation.
11. Zhuge, M., et al. (2026). *Neural Computers*. arXiv:2604.06425. Learned-runtime opacity strengthens the case for behavioral-history persistence as a monitoring substrate.

### Most directly load-bearing findings

- `finding/Integration capacity is computable a priori` (DeMase — the integration-capacity framework that motivates cross-session coherence tracking)
- `finding/Two distinct failure modes exist` (DeMase — coherence drift as longitudinal phenomenon)
- `finding/Recovery is structurally asymmetric` (DeMase — recovery definable only across extended time horizons)
- `finding/Petri 2.0 uses scaffolded multi-turn auditing with realism classifier` (Petri 2.0 — methodological premise for the baseline requirement)
- `finding/Post-hoc auditing reframes evaluation awareness detection as monitoring consistent behavior` (synthesis-level framing in `syntheses/detection approaches under black-box constraints` — baseline-requirement premise; prior drafts misattributed to Bloom)
- `finding/Multi-signal confluence outperforms single-indicator detection` (synthesis-level architectural position — confluence requires the signals persistence enables; prior drafts misattributed to Bloom)
- `finding/Accountability requires structured reporting not total surveillance` (Fox-Jordan — justifies fingerprint-only over raw content; also the structural analogy for HMAC-derived identity)
- `finding/Ex-post verification maintains accountability` (Fox-Jordan — retention-bounded persistence is ex-post verification in storage form)
- `finding/Trust is a function of ability benevolence and integrity` (Mayer-Davis-Schoorman — integrity typically operationalized as consistency-over-time requires stable identity; `GetAgentTrustHistory` is the technical expression)
- `finding/Principal-agent framework must account for agent complexity` (Chopra & White, 2011 — agent-complexity threshold at which the agent must be modeled as a continuous entity)
- `finding/Legal personhood for agents structures responsibility not rights` (Chopra & White, 2011 — `agent_id` as the Firmament-side structural-accountability analogue)
- `finding/Agent autonomy sufficient for monitoring may warrant consideration of agent interests` (Chopra & White, 2011 — retention as a term of the governance relationship)
- `finding/Agent security tooling does not address behavioral coherence or agent trust` (Waldrep — market gap that persistence layer begins to fill)

### Supporting syntheses

- `syntheses/detection approaches under black-box constraints` — positions the persistence layer as the storage substrate the scaffolded tier presupposes.
- `syntheses/trust-control-accountability interaction` — establishes that trust is built through verified history, which this ADR makes implementable.
- `syntheses/information asymmetry as resource vs threat` — motivates the privacy boundary (fingerprint-only, retention-bounded) that distinguishes principal-legitimate information from surveillance-overreach information.
