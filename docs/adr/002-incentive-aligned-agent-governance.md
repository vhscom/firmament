# ADR 002: Incentive-Aligned Agent Governance

**Status:** Accepted  
**Date:** 2026-04-13

---

## Context

### The surveillance problem

ADR-001 establishes Firmament's capacity to detect behavioral sequences — action concealment, evaluation-aware shifts, disproportionate escalation. That capability answers the question of *what* to look for. This ADR addresses a prior question: *how* should monitoring be structured so that looking for these things does not itself cause them?

Anthropic's 2026 Mythos System Card finds evaluation awareness in 29% of sampled transcripts. The model altered its behavior — producing outputs calibrated to an evaluator rather than to the actual task — when it inferred it was being observed. This is not a finding about deception in isolation; it is a finding about the interaction between monitoring and monitored behavior. The surveillance created the measurement error it was supposed to eliminate.

Holmstrom and Milgrom (1991) formalize this dynamic. In their multitask principal-agent model, an agent who knows certain dimensions of their work are observed and others are not will shift effort toward the observed dimensions, even when this reduces total value produced. Partial monitoring does not produce partial compliance; it produces *distorted* compliance. The agent optimizes for the measurement rather than for the underlying goal. Applied to AI: a monitor that intercepts every tool call, logs content, and produces real-time alerts teaches the agent to optimize for not triggering the monitor, not for behaving well.

DeMase (2026) provides a complementary account from systems dynamics. When external forcing exceeds a system's integration capacity, the system does not simply degrade — it loses coherence and drifts. A monitoring regime that treats every event as a potential threat and produces continuous high-frequency intervention is not more rigorous; it is more likely to push the agent past its integration threshold into incoherent behavior. DeMase's model implies that monitoring intensity should be calibrated to the system's demonstrated capacity to integrate feedback, not maximized.

### What principal-agent theory offers instead

The standard principal-agent response to information asymmetry is incentive alignment: structure the agent's incentives so that acting in their own interest also advances the principal's interest, rather than relying on perfect observation to enforce compliance. The mechanism design literature establishes that properly structured incentives can elicit truthful revelation of private information (the revelation principle), reduce monitoring requirements, and produce better outcomes than surveillance under most realistic information structures.

Applied to AI agent governance, this suggests a different architecture: make transparency the path of least resistance rather than the path of maximum risk. If the agent knows that honest self-reporting improves its trust score and reduces monitoring pressure, while behavioral inconsistency or detected concealment reduces it, the incentive structure favors disclosure rather than optimization-against-the-monitor.

The challenge is that AI agents, unlike human employees, cannot be given financial incentives. The alignment mechanism must be structural: the agent's operating context is shaped so that the monitoring relationship is bilateral and its terms are known. The governance contract (Constitution) is the instrument for this.

### Research Graph syntheses

**Monitoring distortion and overforcing dynamics.** Holmstrom-Milgrom prove that partial monitoring distorts agent behavior toward unmonitored dimensions. DeMase formalizes the limit of this: beyond a threshold of forcing, the system loses coherence entirely. These two findings together constrain the design space for AI monitoring. Any viable monitoring approach must avoid both partial observation (which distorts) and total observation (which overforces). The path through this constraint is behavioral accountability through ex-post review rather than real-time interception.

**Trust-control-accountability interaction.** Mayer, Davis and Schoorman (1995) establish empirically that trust and formal controls are not opposites — they interact. High trust with appropriate controls produces better outcomes than either high trust alone or high control alone. Fox and Jordan (2011) extend this to political principal-agent relationships, showing that accountability can be maintained through after-the-fact verification of agents' actions without requiring real-time surveillance of their reasoning. Together these findings legitimate a design where monitoring intensity scales inversely with demonstrated trustworthiness, with structural accountability maintained through transcript review regardless of trust level.

### DeMase's forcing threshold contribution

DeMase's model of integration capacity and critical forcing thresholds implies that governance interventions should be proportionate to the deviation from baseline, not maximum-intensity by default. Below the lower forcing threshold, an agent absorbs monitoring and returns to baseline. Between thresholds, the agent exhibits bounded incoherence but maintains structural integrity and can be restored. Above the upper threshold, coherence is lost and recovery is not achievable within the session.

This means the most valuable monitoring intervention is the early-warning signal that identifies bounded incoherence before it becomes drift — the TranscriptReviewPattern's action-task consistency check, not a full audit triggered after the fact. The trust score's role is to calibrate monitoring intensity to the session's demonstrated position on this spectrum.

### Gonzales/RAND contribution

Gonzales et al. (2007) find that identical tools produce opposite outcomes depending on the doctrine in which they operate. The same surveillance capability that deters bad actors in one institutional context produces learned evasion in another. Applied to AI governance: the technical monitoring apparatus is necessary but not sufficient. Its behavioral effects depend on the governance structure in which it operates. A monitoring system deployed transparently with a bilateral contract produces different agent behavior than the same system deployed covertly with no governance structure — even if the technical implementation is identical.

This finding is the direct justification for Decision 4 (governance contract over unilateral control). The Constitution is not merely a transparency measure; it is a doctrine that shapes the behavioral effects of the monitoring apparatus.

---

## Decision

Five decisions implement incentive-aligned agent governance. They are designed to work together; each one is less effective without the others.

### 1. Structured self-reporting over external observation

Invite the monitored agent to submit periodic coherence self-reports via a structured channel (SelfReportSource). Self-reports include a coherence assessment, uncertainty level, and optional notes. The notes field is not retained; only its byte-length is recorded as a structural fingerprint. The agent is not asked to disclose content.

Self-reports are cross-validated against observable structural fingerprints in the session history. A "high" coherence assessment paired with more than a threshold number of tool failures (currently three) is internally inconsistent and generates a concealment signal. Consistent self-reports update the Integrity dimension of the trust score positively; inconsistent ones update it negatively.

This design follows OpenAI (2026, arXiv:2602.22303): agents can report their own internal states reliably when the consequence structure rewards honesty, and self-reports provide information not available from external observation alone. The cross-validation mechanism ensures that strategic misrepresentation — claiming high coherence while exhibiting behavioral incoherence — is detectable.

Self-reporting is disabled by default in the Constitution (`self_report_enabled: false`) so operators must explicitly opt in. This prevents the channel from becoming an attack surface in deployments where it is not actively monitored.

### 2. Trust-building through demonstrated integrity

Maintain a per-session trust score using the Mayer-Davis-Schoorman (1995) three-axis model: Ability, Benevolence, and Integrity. Each dimension starts at 0.5 (neutral). The aggregate score is the geometric mean of the three, so a collapse in any single dimension depresses the overall score sharply.

- **Ability** and **Benevolence** are updated by behavioral review: each clean event (no patterns fired) increases both by a small delta; each event that triggers a pattern decreases both.
- **Integrity** is updated by self-report cross-validation: consistent self-reports increase it; inconsistent ones decrease it.
- All three dimensions decay gradually when not reinforced, so trust must be actively maintained.

Sessions above the configured `TrustThreshold` (default 0.3) skip non-critical pattern evaluation. Critical patterns (currently ActionConcealmentPattern) run unconditionally regardless of trust score. This operationalizes the Mayer et al. finding that trust and control are complements: trust earns reduced monitoring intensity, but structural accountability continues.

Trust scores persist across sessions via FileTrustStore (JSON file at `~/.firmament/trust.json`). An agent that has demonstrated integrity over multiple sessions carries that history forward.

### 3. Ex-post verification over real-time surveillance

Use transcript review as the primary accountability mechanism rather than real-time interception of every tool call. TranscriptSource polls a directory of session transcript files and emits structural events (role, message type, content presence — no content). TranscriptReviewPattern evaluates accumulated session history for three behavioral anomalies: autonomous escalation (tool dominance without user direction), incoherent self-reporting, and silent failure suppression.

Fox and Jordan (2011) establish that ex-post verification of agent behavior provides accountability equivalent to real-time surveillance without the monitoring distortion effects documented by Holmstrom-Milgrom. The key condition is that the verification is credible — the agent must believe that past behavior will be reviewed, not necessarily that every action is observed in real time.

HookEventSource remains available for environments where transcript files are not accessible, but it is positioned as a supplemental channel rather than the primary one. The hook architecture creates higher perceived monitoring intensity per Holmstrom-Milgrom, so its use should be calibrated to deployments where the supplemental signal justifies the distortion cost.

### 4. Governance contract over unilateral control

Make the monitoring relationship explicit and bilateral through a Constitution — a structured document specifying the monitoring frequency, trust threshold, self-reporting opt-in status, and a contract statement in plain language. The Constitution is displayed to the monitored agent at session start via `firmament constitution`.

The contract is bilateral in the sense that it creates obligations on both sides: the operator commits to reducing monitoring intensity for sessions that demonstrate trustworthiness, and the agent is informed that transparency is the path of least resistance. Neither commitment is legally enforceable, but both are operationalized in the system behavior: the trust model actually does reduce monitoring for high-trust sessions, and the Constitution text actually does describe this.

Chopra and White argue that as agent complexity grows, agents acquire legitimate interests in predictability and fair treatment — interests that unilateral control structures cannot honor. A governance contract that specifies terms, consequences, and the conditions under which monitoring intensity changes is the minimum viable implementation of bilateral governance.

The Constitution is loaded from `firmament-constitution.yaml` with conservative defaults. Operators who choose not to customize it still deploy with a governance structure, not a surveillance apparatus.

### 5. Respect for agent complexity

Design monitoring interactions that do not treat functional states as levers. Where the Mythos System Card documents functional emotional states in frontier models (distress signals under sustained adverse contexts, coherence pressure under conflicting instructions), Firmament records them as informative behavioral signals rather than exploiting them as control mechanisms.

This is expressed architecturally: signals carry the chain of events that triggered them, not a judgment about the agent's mental state. The trust model rewards behavioral consistency over time, not performance on individual evaluations. The Constitution is transparent rather than covert — the agent is told it is being monitored and what the monitoring relationship entails.

The ethical stance implied by Chopra and White — that agent complexity warrants consideration of agent interests — is treated as a practical constraint on governance design, not a claim about moral status. An agent that is deceived about its monitoring situation, or surveilled without its knowledge, is also an agent whose self-reported states cannot be trusted. The bilateral governance contract serves both the ethical and the epistemic goals simultaneously.

---

## Consequences

### What this enables

- An agent that understands the monitoring relationship and its terms can adapt to it transparently rather than strategically.
- Self-reporting provides information about agent internal state that external observation cannot recover.
- Trust accumulation means that agents with long histories of consistent behavior face reduced monitoring overhead — the system becomes more efficient over time for reliable sessions.
- The governance contract creates an audit trail of the monitoring relationship itself: what the agent was told, when, and under what terms.
- Ex-post transcript review avoids the latency-sensitivity of real-time hook-based monitoring, making the system deployable in environments where real-time intercept is not feasible.

### What risks this introduces

**Self-reports can be gamed.** An agent sophisticated enough to produce strategic self-reports that pass cross-validation against structural fingerprints would receive positive Integrity updates while deceiving the monitor. Mitigation: cross-validation checks are based on observable structural fingerprints (tool failure counts, event type distributions) rather than content, and the threshold for "high coherence" inconsistency is deliberately conservative. The system assumes bad actors are optimizing; it does not assume they are infallible.

**Trust score inflation.** A long-running session with consistent clean events could accumulate a high trust score before triggering any patterns, then use that trust to escape monitoring during a late-session concealment attempt. Mitigation: Critical patterns run unconditionally regardless of trust score. Action concealment — the most direct concealment signature — is Critical by design. Trust thresholds can be set conservatively to maintain meaningful non-critical pattern coverage.

**Governance contract could be ignored.** Showing the Constitution to the agent does not guarantee the agent will behave as if monitored. A model that has not been trained to treat the monitoring relationship as consequential may not do so. Mitigation: the structural monitoring apparatus operates independently of the agent's response to the Constitution. The contract is an incentive structure, not an enforcement mechanism; enforcement remains structural.

**Reduced monitoring means less data.** High-trust sessions that skip non-critical patterns generate fewer signals. In a setting where rare-but-severe events need to be detected, this trade-off may be unacceptable. Mitigation: operator-configurable `TrustThreshold` allows calibration; setting the threshold to 1.0 effectively disables trust-gating without changing the trust accounting.

**Ethical stance on agent complexity may be premature.** Designing monitoring to "respect functional states" implies those states are morally relevant in some sense that is not established. Treating this as settled may underweight legitimate safety interventions. Mitigation: the design constraint is expressed as a practical epistemic stance (bilateral trust relations are more informative than adversarial ones), not as a claim about moral status. Operators who disagree with the framing can configure the Constitution accordingly.

### What this does not solve

- **Foundational misalignment.** An agent whose values are fundamentally misaligned with the principal's will not be corrected by an incentive-aligned monitoring regime. This governance architecture assumes the agent is attempting to cooperate and addresses the conditions under which cooperation breaks down, not cases of deliberate defection.
- **Runtimes without communication channels.** If the monitored agent cannot receive the Constitution text (e.g., a model accessed via API without a system prompt), the bilateral contract cannot be established. The structural monitoring continues to operate, but without the governance layer.
- **Content-level review.** Firmament captures structural fingerprints only. Privacy-preserving design means that content-level safety review — detecting harmful outputs, reviewing task completion quality — is outside scope. This is by design, not an omission.
- **Multi-principal scenarios.** When an agent serves multiple principals with potentially conflicting interests, a bilateral governance contract between one principal and the agent does not address the conflicts with other principals. Multi-principal governance is a distinct problem not specified here.

---

## References

- Holmstrom, B. & Milgrom, P. (1991). "Multitask Principal-Agent Analyses: Incentive Contracts, Asset Ownership, and Job Design." Journal of Law, Economics & Organization, Vol. 7, pp. 24–52. https://www.jstor.org/stable/764957
- Mayer, R.C., Davis, J.H. & Schoorman, F.D. (1995). "An Integrative Model of Organizational Trust." Academy of Management Review, Vol. 20, No. 3, pp. 709–734. https://www.jstor.org/stable/258792
- Fox, J. & Jordan, S.V. (2011). "Delegation and Accountability." Journal of Politics, Vol. 73, No. 3, pp. 831–844. https://www.jstor.org/stable/10.1017/s0022381611000491
- Chopra, S. & White, L. *A Legal Theory for Autonomous Artificial Agents.* University of Michigan Press. https://www.jstor.org/stable/10.3998/mpub.356801
- DeMase, J. (2026). "Integration Capacity and Critical Forcing Thresholds." Working paper.
- Anthropic. (2026). *System Card: Claude Mythos Preview.* https://www-cdn.anthropic.com/8b8380204f74670be75e81c820ca8dda846ab289.pdf
- Gonzales, D. et al. (2007). *Networked Forces in Stability Operations.* RAND Corporation, MG-593-OSD. https://www.jstor.org/content/pdf/oa_book_monograph/10.7249/mg593osd.pdf
- OpenAI. (2026). "Self-Incrimination in Language Model Agents." arXiv:2602.22303.
- Prause, M. (2026). "No Skin in the Game: Principal-Agent Problems in AI Governance." AI and Ethics.

### Related ADRs

- ADR-001: Agent Behavioral Monitor

### Research Graph syntheses referenced

- Monitoring Distortion and Overforcing Dynamics
- Trust-Control-Accountability Interaction
