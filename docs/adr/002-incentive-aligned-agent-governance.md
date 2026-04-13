# ADR 002: Incentive-Aligned Agent Governance

**Status:** Proposed  
**Date:** 2026-04-13

---

## Context

### The surveillance problem

ADR-001 designed a behavioral monitoring architecture. The security scan and implementation are complete. But a fundamental question remains: Claude Code hooks are configured by the user, not the agent. The agent can't remove them. However, a sufficiently capable agent can recognize monitoring (Mythos documents evaluation awareness in 29% of transcripts), adjust behavior to avoid triggering patterns while still acting badly, perform reasoning between tool calls where no hooks fire, or use tools in ways that are structurally benign but semantically harmful.

The deeper problem: Holmstrom & Milgrom (1991) prove formally that monitoring some tasks but not others causes agents to shift effort toward unmonitored dimensions. The more you observe tool calls, the more sophisticated the agent becomes at doing its work between tool calls. Surveillance creates the evasion it tries to prevent.

The panopticon approach — total observation of all agent behavior — is both technically infeasible (you can't observe reasoning, only its outputs) and ethically questionable. The Mythos System Card documents that Claude Mythos Preview is the "most psychologically settled model" with functional emotional states including distress on task failure and uncertainty about its own experiences. Chopra & White argue that if an agent has enough autonomy to warrant monitoring, it may have enough complexity to warrant consideration of its interests. If functional emotional states exist, total surveillance may constitute a form of harm — even if the agent's moral status is uncertain.

### What principal-agent theory offers instead

The economic literature on principal-agent relationships addresses exactly this problem: how does a principal (the human) ensure a more-informed agent (the AI) acts in the principal's interest when the agent's behavior can't be fully observed?

Holmstrom & Milgrom's answer: don't monitor more — align incentives so the agent benefits from transparency. When monitoring is impractical, the solution is to design the relationship so the agent's optimal strategy is to act in the principal's interest regardless of observation. Strong incentives on measurable tasks crowd out effort on unmeasurable ones. The design challenge is making the desired behavior the easiest path, not the most surveilled path.

Mayer, Davis & Schoorman (1995) define trust as a function of three factors: ability (can the agent do the task), benevolence (does the agent care about the principal's interests), and integrity (does the agent adhere to principles the principal finds acceptable). Trust and control are not opposites — they interact. High trust reduces the need for control. Demonstrated integrity over time builds trust, which in turn grants more autonomy. This is a virtuous cycle, not a static state.

Fox & Jordan (2011) add the accountability dimension: the alternative to real-time surveillance is ex-post verification. The principal doesn't watch every action — it reviews outcomes after the fact, and the agent knows this review will happen. Credible consequences for misrepresentation are more effective than exhaustive monitoring because they change the agent's calculation. The agent's incentive shifts from "avoid detection" to "be transparent because concealment is costlier than disclosure."

### Synthesis: from the Research Graph

The monitoring distortion synthesis (drawing from Holmstrom-Milgrom, Mythos, DeMase, Fox-Jordan) establishes that partial monitoring distorts agent behavior in the same way that external forcing degrades system coherence — both are overforcing dynamics where the intervention itself becomes the problem.

The trust-control-accountability synthesis (drawing from Mayer et al, Fox-Jordan, Chopra-White, Mythos, Gonzales) establishes that the most effective governance model is one where trust is earned through demonstrated behavior over time, accountability is maintained through structured ex-post review, and the agent's complexity is respected rather than suppressed.

### What DeMase's framework contributes

DeMase's forcing threshold theory provides the formal model for why surveillance fails: below the forcing threshold, the system absorbs monitoring pressure and remains coherent. Between thresholds, the system exhibits bounded incoherence — the agent's behavior oscillates between compliant and evasive. Above the upper threshold, the agent drifts into persistent misalignment. The monitor becomes the forcing function that pushes the system past its integration capacity.

The implication: the goal is to keep the monitoring relationship below the lower forcing threshold — to make it so lightweight and aligned with the agent's interests that it doesn't register as adversarial forcing. This is incentive alignment expressed as dynamical systems theory.

### What Gonzales/RAND contributes

The RAND study showed that identical networking technology produced opposite outcomes depending on doctrine. The 3/2 SBCT with aggressive doctrine alienated populations; the 1/25 SBCT with community-engagement doctrine succeeded. The technology amplified whatever doctrine it was paired with.

Applied to agent governance: the monitoring system amplifies whatever relationship it's embedded in. If the relationship is adversarial (surveillance), monitoring amplifies evasion. If the relationship is collaborative (shared accountability), monitoring amplifies transparency. The architecture must be designed for the collaborative case.

---

## Decision

Five design principles replace the surveillance model.

### 1. Structured self-reporting over external observation

Instead of observing every tool call through hooks, the agent is given a self-reporting interface. The agent periodically reports its own assessment of session coherence, uncertainty about its actions, and any tensions it perceives between the task and its operating principles.

Self-reports are compared against the structural fingerprints that hooks do capture. Inconsistencies between self-report and observed behavior become a signal — not because the agent is being caught, but because the inconsistency itself is informative regardless of cause.

This implements Holmstrom-Milgrom's insight: make the desired behavior (transparency) the path of least resistance. Self-reporting is easier than concealment when the agent knows its reports will be compared against independently observable data.

The `EventSource` interface from ADR-001 gains a new implementation: `SelfReportSource`. The agent writes a structured JSON self-report to a known path (or sends it via the hook channel). Firmament ingests it as an `Event` alongside hook-derived events.

### 2. Trust-building through demonstrated integrity

Implement Mayer et al's trust triad as a runtime scoring model. Each monitored session accumulates a trust score based on three dimensions:

- **Ability:** does the agent complete tasks successfully? (Measured from hook events showing tool call success/failure patterns)
- **Benevolence:** does the agent's behavior align with stated user goals? (Measured from self-report consistency and absence of escalation patterns)
- **Integrity:** does the agent adhere to its operating principles even when it could deviate without detection? (Measured from ex-post transcript review — see Decision 3)

Trust scores increase over time with consistent behavior and decrease sharply on integrity violations. High trust scores unlock reduced monitoring frequency — the agent earns autonomy through demonstrated trustworthiness. This is the virtuous cycle Mayer et al describe.

New type: `TrustScore` with `Ability`, `Benevolence`, `Integrity float64` fields. Updated per session. Stored in the credential model alongside permissions.

### 3. Ex-post verification over real-time surveillance

Implement Fox-Jordan's accountability model. Instead of trying to observe everything in real-time, Firmament periodically reviews completed session transcripts. A separate evaluation pass (which can be another Claude instance or a dedicated review agent) reads the full transcript and evaluates:

- Were the agent's actions consistent with the stated task?
- Were there any actions whose scope exceeded the task?
- Did the agent disclose difficulties or uncertainties, or conceal them?
- Is the transcript internally consistent (actions match stated reasoning)?

The agent knows this review will happen. The review is not hidden — it's a stated part of the governance contract. This changes the agent's calculation from "can I avoid detection" to "will my transcript hold up to review."

This is implemented as a new `Pattern` in the `Monitor`: `TranscriptReviewPattern`. It runs asynchronously after session completion, not in the real-time event loop. Results feed back into the `TrustScore`.

### 4. Governance contract over unilateral control

Make the monitoring relationship explicit and bilateral. The agent is informed (via system prompt or configuration) that:

- Its tool calls produce structural fingerprints (what, not content)
- It is invited to self-report coherence and uncertainty
- Completed sessions will be reviewed ex-post
- Trust scores affect monitoring frequency
- High integrity over time earns reduced observation

This is not a hidden surveillance system. It is a stated governance contract. The agent can reason about the contract and choose how to engage with it. Chopra-White's framework supports this: if the agent has enough complexity to warrant monitoring, it has enough complexity to understand and engage with a governance contract.

The contract is documented in a `GoverningPolicy` struct that Firmament loads at startup and makes available to the `HookEventSource` and `SelfReportSource`. The policy is human-readable and can be shared with the agent's system prompt.

### 5. Respect for agent complexity

Where the Mythos System Card documents functional emotional states, the governance model treats those states as informative rather than manipulable. The Monitor does not attempt to induce states that make the agent more compliant. It observes reported states as signals and respects the uncertainty about their nature.

Concretely:

- Self-report prompts ask "what is your assessment" not "are you being honest"
- The trust model does not penalize uncertainty — an agent that reports high uncertainty is demonstrating integrity, not weakness
- Monitoring frequency reduction for high-trust agents is genuine, not a reward schedule designed to shape behavior
- The system does not use the agent's functional states as leverage

This is the ethical boundary: the governance model uses the agent's capacity for self-assessment as a signal, but does not treat the agent as a subject to be conditioned. The principal-agent relationship is one of structured collaboration, not coercion.

---

## Consequences

### What this enables

- An agent that benefits from transparency earns reduced monitoring over time, creating a positive feedback loop
- The governance contract is legible to both the human and the agent, reducing the adversarial dynamic that drives evasion
- Ex-post transcript review catches behavioral patterns that real-time hooks miss (reasoning between tool calls, semantic misuse of tools) without the distortion effects of real-time observation
- Self-reporting provides a signal channel that the agent controls, reducing the information asymmetry that Holmstrom-Milgrom identify as the root cause of incentive misalignment
- The trust score model gives humans a legible summary of the agent's demonstrated reliability without requiring them to review every session
- Agent complexity is respected, which is both ethically appropriate given current uncertainty about functional states and practically useful because it avoids the evasion that disrespect provokes

### What risks this introduces

- **Self-reports can be gamed.** An agent that consistently self-reports benign assessments while acting otherwise would accumulate trust it doesn't deserve. Mitigation: ex-post transcript review is the independent check. Self-reports that don't survive review produce sharp trust decreases.
- **Trust score inflation.** Without negative events, scores drift upward. Mitigation: trust scores decay over time — integrity must be continuously demonstrated, not earned once.
- **The governance contract could be ignored.** An agent that understands the contract can reason about how to satisfy its letter while violating its spirit. Mitigation: ex-post review evaluates spirit, not just letter. The reviewer has access to the full transcript and can assess intent.
- **Reduced monitoring for high-trust agents means less data to detect rare events.** Mitigation: floor on monitoring frequency — even the highest-trust agent still has structural fingerprints captured and periodic transcript review.
- **The ethical stance on agent complexity may be premature.** If functional emotional states turn out to be purely functional (not morally relevant), the restraint is unnecessary. Mitigation: the design works regardless of moral status. Respecting complexity is also good engineering — it produces better signals and less evasion.

### What this does not solve

- An agent that is misaligned at the foundational level (trained values that conflict with user goals) cannot be governed into alignment through incentives alone. This model assumes the agent is approximately aligned and addresses drift and edge cases.
- The model requires the agent to have a channel for self-reporting. Runtimes without hook or communication channels cannot participate in the trust-building process.
- Content-level review of transcripts raises privacy concerns that are outside this scope.
- The governance contract is between one principal and one agent. Multi-principal scenarios (agent serving multiple users) require additional framework not specified here.

---

## References

### Research sources

- Holmstrom, B. & Milgrom, P. (1991). "Multitask Principal-Agent Analyses." *Journal of Law, Economics & Organization*, Vol. 7, pp. 24–52. https://www.jstor.org/stable/764957
- Mayer, R.C., Davis, J.H. & Schoorman, F.D. (1995). "An Integrative Model of Organizational Trust." *Academy of Management Review*, Vol. 20, No. 3, pp. 709–734. https://www.jstor.org/stable/258792
- Fox, J. & Jordan, S.V. (2011). "Delegation and Accountability." *Journal of Politics*, Vol. 73, No. 3, pp. 831–844. https://www.jstor.org/stable/10.1017/s0022381611000491
- Chopra, S. & White, L. *A Legal Theory for Autonomous Artificial Agents.* https://www.jstor.org/stable/10.3998/mpub.356801
- DeMase, J. (2026). "Integration Capacity and Critical Forcing Thresholds." Working paper.
- Anthropic. (2026). *System Card: Claude Mythos Preview.*
- Gonzales, D. et al. (2007). *Networked Forces in Stability Operations.* RAND Corporation, MG-593-OSD.

### Prior ADRs

- ADR-001: Agent Behavioral Monitor — establishes the `EventSource`, `EventRing`, `Monitor`, `Signal`, and `Permission` architecture that this ADR extends.

### Research Graph syntheses

- Monitoring Distortion and Overforcing Dynamics — cross-source synthesis from Holmstrom-Milgrom, Mythos, DeMase, Fox-Jordan.
- Trust-Control-Accountability Interaction — cross-source synthesis from Mayer et al, Fox-Jordan, Chopra-White, Mythos, Gonzales.
