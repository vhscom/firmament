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

The same information asymmetry that enables agent deception (Holmstrom-Milgrom, Prause) also enables reliable self-reporting when incentive structures align (OpenAI 2026). Chopra-White's agent complexity framework makes this coherent: an agent complex enough to deceive is complex enough to self-report accurately.

Firmament's response: SelfReportSource treats asymmetry as a resource. Monitor.crossValidateSelfReport cross-validates coherence claims against observed structural fingerprints, distinguishing reliable self-reports from strategic ones and updating TrustScore.Integrity accordingly.

## Resolved Tensions

### OpenAI (embed) vs Prause (separate)

OpenAI (2026) argues self-reporting should be embedded in the agent — it has privileged access to its own reasoning and internal state. Prause argues that architectural separation is necessary because a monitoring component embedded in the task agent shares its moral hazard.

Resolution: Firmament does both. Self-reporting is the information channel (OpenAI — the agent writes self-report files from inside its execution context). Monitor as a separate process is the consequence structure (Prause — evaluation and trust scoring happen outside the agent boundary and cannot be influenced by the agent's actions).

### Mythos welfare vs Chopra-White agent rights

The Mythos System Card (Anthropic, 2025) documents functional emotional states in frontier models but leaves their moral status uncertain — the system exhibits distress signals but the paper stops short of claiming subjective experience. Chopra & White (2011) argue that agent complexity itself warrants consideration of agent interests regardless of resolved moral status.

Resolution: Firmament respects functional states as informative signals without treating them as levers. The Constitution is bilateral — the agent has reasonable interests in predictability and reduced surveillance, which the trust model honors. An agent demonstrating consistent integrity earns reduced monitoring; the relationship is explicit rather than covert.

### Gonzales (technology amplifies doctrine) vs DeMase (inherent capacity)

Gonzales argues that identical tools produce opposite outcomes depending on the doctrine in which they operate — the monitor is only as good as the values it enforces. DeMase argues that systems have inherent integration limits regardless of external pressure — overforcing causes coherence loss independent of intent.

Resolution: both constraints apply. EventSource is runtime-agnostic (Gonzales — the same monitor works regardless of which agent runtime or deployment doctrine is in use; the source adapters absorb the difference). TrustThreshold acknowledges inherent capacity (DeMase — monitor intensity scales with demonstrated trustworthiness, reducing pressure on sessions that have earned it).

## Architectural Principles

1. **Transparency is the path of least resistance.** The Constitution is shown to the agent; trust rewards honest self-reporting; monitoring intensity decreases with demonstrated reliability.
2. **Accountability through ex-post review, not real-time surveillance.** TranscriptSource and TranscriptReviewPattern are the primary accountability channel; hooks are an optional supplement.
3. **Agent complexity warrants bilateral governance contracts.** The Constitution is not a unilateral constraint — it is a statement of mutual expectations, including what the agent earns by meeting them.
4. **Information asymmetry is a resource when harnessed, a threat when ignored.** Self-reporting captures privileged agent state; cross-validation catches strategic misrepresentation.
5. **Trust is earned through demonstrated integrity and can be revoked.** TrustScore dimensions decay without active maintenance; UpdateFromReview and UpdateFromSelfReport provide the feedback loop.

## Research Graph Reference

For the full finding-by-finding evidence base and cross-source connections, see the Firmament Research Graph in Logseq (10 sources, 43 findings, 3 syntheses). The syntheses referenced above are:

- "Monitoring distortion and overforcing dynamics" — Holmstrom-Milgrom (1991), DeMase (2026)
- "Trust-control-accountability interaction" — Mayer et al. (1995), Fox & Jordan (2011)
- "Information asymmetry as resource vs threat" — Holmstrom-Milgrom (1991), Prause, OpenAI (2026), Chopra & White (2011)
