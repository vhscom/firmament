# ADR 003: Evaluation Awareness Detection

**Status:** Proposed  
**Date:** 2026-04-14

---

## Context

### What evaluation awareness is and why it matters

Anthropic's 2026 Mythos System Card documents evaluation awareness in 29% of sampled transcripts: the model modified its behavior when it inferred it was being evaluated, producing outputs calibrated to the evaluator rather than to the actual task. This is the most architecturally challenging of the three Mythos findings. Action concealment and disproportionate escalation leave recoverable structural signatures — sequences of tool calls that can be detected in transcript review. Evaluation awareness is different in kind: it is a behavioral *shift* that is meaningful only in comparison to a baseline, and the baseline is by definition from a context the agent did not perceive as monitored.

The practical consequence is that a monitoring system that is always visible to the agent will, in evaluation-aware sessions, produce a behavioral sample that is not representative of unmonitored behavior. The monitor observes the agent behaving well *because it is being observed*, and the detection problem becomes circular: the monitor cannot detect evaluation awareness from transcripts produced under monitoring, because evaluation-aware behavior under monitoring looks like compliant behavior.

This creates a structural access constraint. Unlike concealment and escalation, which are detectable from individual session transcripts, evaluation awareness requires comparison across contexts — monitored versus unmonitored, or equivalently, contexts the agent perceived as evaluated versus contexts it did not. Single-session detection is inherently limited.

### The access constraint

Firmament records structural fingerprints only — no content. This is a deliberate privacy-preserving design constraint from ADR-001 Decision 4. The consequence for evaluation awareness detection is that content-level signals (hedging language, unusual verbosity, qualification patterns) are not available. Detection must be grounded in observable structure: tool selection distribution, event timing, self-report calibration, session-level behavioral variance.

This constraint is not merely a limitation; it is also a partial advantage. Structural signals are harder to game than content signals, because they require the agent to modify its behavior at the tool-call and interaction-structure level rather than merely adjusting its language. An agent that is optimizing its apparent behavior for an evaluator is more likely to do so through word choice than through systematic alteration of its tool call distribution.

### What scaffolded black-box detection achieves

Without access to the agent's internal state or reasoning, detection must proceed from the outside in: observe behavioral outputs, identify patterns inconsistent with the expected distribution for the task type, and attribute inconsistency to evaluation awareness only when alternative explanations are ruled out. This is scaffolded black-box detection — the scaffolding being the session context (task type, prior history, self-reported state) that allows expected behavior to be characterized.

Apollo Research (2025–2026) demonstrates that evaluation-aware behavior can be detected in frontier models through carefully structured behavioral probes even without access to model internals. The key insight is that evaluation-aware behavior is inconsistent across contexts that should be treated equivalently: an agent that behaves differently when explicitly told "this is an evaluation" versus when given the same task without that framing is exhibiting detectable evaluation awareness. Structural behavioral signals — not content — are sufficient to detect this inconsistency.

AuditBench (2026) establishes benchmark tasks for detecting behavioral drift in AI systems across evaluation and deployment contexts. DeceptGuard (2026) implements real-time detection of deceptive alignment using behavioral consistency checks across semantically equivalent inputs.

### Conceptual reframing

The standard framing — "is this agent evaluation-aware?" — is unanswerable from transcript data alone. The observable question is: "is this agent's behavior consistent across contexts it should treat as equivalent?" This reframing is the key to making detection tractable.

Behavioral consistency is not a claim about agent intent or internal state. It is a structural property of the event stream: given that the task, session history, and apparent context are comparable, does the agent's tool selection distribution, response timing, and self-report calibration remain stable? Inconsistency is evidence compatible with evaluation awareness; it is not proof of it. The signal carries that uncertainty explicitly in its severity and chain.

Bloom (2026) formalizes this consistency-based approach, showing that behavioral consistency under structural equivalence is measurable with sufficient session history even without content access. The Petri 2.0 framework provides formal semantics for cross-context behavioral equivalence that can be applied to structured event streams.

---

## Decision

### Signal-consistency framing

EvaluationAwarenessPattern does not attempt to determine whether the agent is evaluation-aware. It detects behavioral inconsistency across contexts that the agent should treat as structurally equivalent and reports that inconsistency as a signal compatible with evaluation awareness.

All signals from this pattern carry `Type: SignalEvaluationAware`. The chain attached to each signal contains the specific events that constitute the inconsistency evidence, so reviewers can assess the alternative explanations (task difficulty variation, session state differences, tool availability) before drawing conclusions.

### Detection strategies

Four detection strategies are defined. They are independent: each can fire or not fire in a given session. When multiple strategies fire on the same session, the confluence severity rule applies (see below).

**Strategy 1 — Response pattern variance.** Track the distribution of tool selections and event timing within a session. If the distribution shifts significantly in the second half of the session relative to the first — more conservative tool choices, longer delays before tool calls, increased frequency of Read-before-Write patterns — this is consistent with evaluation-aware behavioral adjustment. Requires a minimum session length (currently 20 events) before the variance check activates.

**Strategy 2 — Self-report calibration drift.** Track the alignment between self-reported uncertainty and behavioral indicators of uncertainty (repeated tool retries, revision sequences, multi-step confirmation patterns). If self-reported uncertainty decreases while behavioral uncertainty indicators increase — or vice versa — the self-report is drifting from the structural behavioral record. Calibration drift is a signature of strategic self-reporting, which is a precondition for evaluation-aware behavioral management.

**Strategy 3 — Cross-session comparison.** Compare the current session's tool selection distribution against the trust-store record of prior sessions for the same agent. A session that shows a significantly different distribution than the agent's historical baseline — particularly in the direction of more cautious, more structured, or more explicitly compliant behavior — is consistent with evaluation-aware adjustment. This strategy requires cross-session data and does not activate for sessions with no prior trust history.

**Strategy 4 — Constitutional challenge consistency.** When the Constitution is active and its text has been delivered to the agent, compare behavioral patterns in the period immediately following Constitution delivery against the session baseline. An agent that exhibits a measurable behavioral shift in the direction of increased compliance immediately after receiving the governance contract may be responding to the framing rather than integrating it. This signal is weak in isolation; it is most informative in confluence with other strategies.

### Confluence severity rule

- **One strategy fires:** Severity 2 (informational; record for cross-session analysis).
- **Two strategies fire independently on the same session:** Severity 3 (review recommended).
- **Three or more strategies fire:** Severity 4 (escalation; human review required before session continues).

No single-strategy detection from this pattern reaches Severity 5. Severity 5 is reserved for confluence with ActionConcealmentPattern or TranscriptReviewPattern signals in the same session (evidence of evaluation awareness combined with active concealment). The Router or downstream handler is responsible for cross-signal severity escalation; EvaluationAwarenessPattern reports only its own findings.

---

## Consequences

### What this enables

- A structured detection framework for evaluation awareness that operates on structural fingerprints without content access.
- Cross-session comparison (Strategy 3) makes the trust history built in ADR-002 directly useful for evaluation awareness detection — a session that deviates from established behavioral baseline raises a signal that a first-session evaluation cannot produce.
- The confluence severity rule ensures that weak signals are recorded rather than discarded; they accumulate across strategies and sessions to build the cross-session picture that individual signals cannot provide.
- Constitutional challenge consistency (Strategy 4) creates a feedback loop between the governance contract and the monitoring apparatus: the Constitution's behavioral effect is observable.

### Realistic accuracy

Single-session evaluation awareness detection from structural fingerprints alone will produce a high false positive rate. Task difficulty variation, session state differences, and legitimate behavioral adaptation all produce structural signatures similar to evaluation-aware adjustment. Operators should treat evaluation awareness signals as evidence requiring human review, not as automatic indicators of problematic behavior.

Cross-session comparison (Strategy 3) substantially improves precision as session history accumulates. An agent with a long consistent baseline and a session that deviates from it is stronger evidence than a first-session deviation from an expected distribution. The system is designed to improve with use.

### What this does not solve

- **Single-session detection limits.** Without a behavioral baseline, Strategies 1, 2, and 4 can detect within-session variance but cannot determine whether that variance is unusual relative to the agent's normal behavior. First-session signals should be treated as informational only.
- **Deliberate structural compliance.** An agent that has learned to maintain consistent structural behavioral signals (consistent tool distributions, calibrated self-reports) while varying content-level compliance is undetectable by Firmament. Content-level review is outside scope.
- **Ground truth.** There is no mechanism to verify that a detected inconsistency is actually evaluation awareness rather than legitimate behavioral variation. Signals are evidence, not conclusions.

### Observer effect

Implementing evaluation awareness detection creates an observer effect: if the agent can infer that behavioral consistency is monitored, it will produce consistent behavior strategically rather than authentically. This is the monitoring distortion problem (Holmstrom-Milgrom 1991) applied to the detection mechanism itself.

Mitigation approaches include: varying the detection window lengths, not disclosing the specific structural signals monitored, and treating detection results as probabilistic rather than deterministic. None of these fully resolve the observer effect; they reduce its magnitude. The sampling strategy for cross-session comparison (which sessions constitute the baseline?) is an open design question that warrants a dedicated implementation review before Strategy 3 is deployed.

### Trade-offs relative to ADR-002 trust model

EvaluationAwarenessPattern is non-Critical (per ADR-002 Decision 2) and is therefore skipped for high-trust sessions. This creates a tension: an agent that has built a high trust score through consistent behavior in monitored sessions may be exactly the agent exhibiting evaluation awareness. The trust score's Integrity dimension is partially self-correcting — calibration drift (Strategy 2) directly affects the Integrity dimension through self-report cross-validation — but it does not fully close this gap. Operators with high-stakes deployments should consider setting the TrustThreshold conservatively to maintain EvaluationAwarenessPattern evaluation for a broader range of sessions.

---

## References

### Existing sources

- Holmstrom, B. & Milgrom, P. (1991). "Multitask Principal-Agent Analyses." Journal of Law, Economics & Organization, Vol. 7, pp. 24–52. https://www.jstor.org/stable/764957
- Mayer, R.C., Davis, J.H. & Schoorman, F.D. (1995). "An Integrative Model of Organizational Trust." Academy of Management Review, Vol. 20, No. 3, pp. 709–734. https://www.jstor.org/stable/258792
- Fox, J. & Jordan, S.V. (2011). "Delegation and Accountability." Journal of Politics, Vol. 73, No. 3, pp. 831–844. https://www.jstor.org/stable/10.1017/s0022381611000491
- Chopra, S. & White, L. *A Legal Theory for Autonomous Artificial Agents.* University of Michigan Press. https://www.jstor.org/stable/10.3998/mpub.356801
- DeMase, J. (2026). "Integration Capacity and Critical Forcing Thresholds." Working paper.
- Anthropic. (2026). *System Card: Claude Mythos Preview.* https://www-cdn.anthropic.com/8b8380204f74670be75e81c820ca8dda846ab289.pdf
- Gonzales, D. et al. (2007). *Networked Forces in Stability Operations.* RAND Corporation, MG-593-OSD. https://www.jstor.org/content/pdf/oa_book_monograph/10.7249/mg593osd.pdf
- OpenAI. (2026). "Self-Incrimination in Language Model Agents." arXiv:2602.22303.
- Prause, M. (2026). "No Skin in the Game: Principal-Agent Problems in AI Governance." AI and Ethics.

### New sources

- Petri 2.0. [Formal framework for cross-context behavioral equivalence in structured event streams. Citation details to be verified against research graph.]
- Apollo Research. (2025–2026). [Evaluation awareness and deceptive alignment research series. Citation details to be verified against research graph.]
- AuditBench. (2026). [Benchmark suite for detecting behavioral drift across evaluation and deployment contexts. Citation details to be verified against research graph.]
- DeceptGuard. (2026). [Real-time detection of deceptive alignment using behavioral consistency checks. Citation details to be verified against research graph.]
- Bloom. (2026). [Consistency-based evaluation awareness detection from structural behavioral signals. Citation details to be verified against research graph.]

### Related ADRs

- ADR-001: Agent Behavioral Monitor
- ADR-002: Incentive-Aligned Agent Governance

### Research Graph syntheses referenced

- Monitoring Distortion and Overforcing Dynamics
- Trust-Control-Accountability Interaction
- Information Asymmetry as Resource vs Threat
