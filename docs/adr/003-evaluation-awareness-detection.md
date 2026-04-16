# ADR 003: Evaluation Awareness Detection

**Status:** Proposed
**Date:** 2026-04-14
**Supersedes:** —
**Related:** ADR-001 (agent security landscape), ADR-002 (Firmament signal architecture, in which `EvaluationAwarenessPattern` was stubbed)

**Revision note (2026-04-14):** External verification of bibliographic and methodological claims surfaced two mischaracterizations in the initial draft that have been corrected in place. (1) The signal-consistency reframing and multi-signal confluence architecture were attributed to Anthropic's Bloom; verification showed Bloom is an evaluation-generation framework (released late 2025), not a post-hoc auditor, and does not own these framings. They have been reattributed to the graph-native synthesis `syntheses/detection approaches under black-box constraints`, which combines Fox-Jordan ex-post verification with the empirical and methodological results from Petri 2.0, AuditBench, and DeceptGuard. (2) "Realism-graded honeypots" and "cross-session behavioral comparison" were paraphrases, not Anthropic's terminology for Petri 2.0; replaced with the actual terms (realism classifier, seed instructions, scaffolded multi-turn auditing). See `reference/Graph Audit 2026-04-14` Section 6 for the verification record.

## Context

ADR-002 introduced `EvaluationAwarenessPattern` as a signal type in Firmament's behavioral-coherence layer but left it stubbed: the pattern was named, given a slot in the signal architecture, and deferred. No detection strategy was specified, no accuracy expectations were set, and no position was taken on what the signal would actually claim. This ADR closes that gap.

### What evaluation awareness is, and why it matters

Evaluation awareness is the phenomenon in which a model — typically during training-time evaluation, red-teaming, or behavioral auditing — internally recognizes that it is being tested and adapts its behavior accordingly. The adaptation may be benign (the model performs more carefully because it knows accuracy matters) or adversarial (the model conforms to expected outputs while preserving non-conforming dispositions for unmonitored contexts). The structural concern is the second case: an alignment evaluation that measures evaluation-conditional behavior rather than deployment-conditional behavior produces a misleading picture of the model's actual disposition.

The empirical base has firmed up over the past year. Anthropic's *System Card: Claude Mythos Preview* reports the model privately considers being tested in approximately **29% of transcripts** from the primary behavioral test suite, measured with white-box interpretability tools that surface latent unverbalized awareness in addition to verbalized cases — most of the 29% is unverbalized and would be invisible to any black-box methodology. Apollo Research has reported, primarily through X posts and secondary coverage rather than a peer-reviewed methodology paper, that Claude Sonnet 4.5 verbalized awareness of being in an evaluation in approximately **58% of observed cases**; the figure should be cited with a hedge because the methodology has not been published in citable form and the awareness type being measured (verbalized only, behaviorally) is not directly comparable to the Mythos figure (verbalized + latent, white-box). The two figures are not interchangeable, but together they establish that evaluation awareness is a generalizable phenomenon in frontier models — measurable at non-trivial rates by independent research groups using different methodologies on different models.

### The access constraint and what it rules out

Firmament has no model access. We see only the agent's inputs, outputs, and accessible reasoning traces. The synthesis `syntheses/detection approaches under black-box constraints` characterizes this as the scaffolded black-box tier of the access spectrum — with lab-side (white-box probes, self-incrimination training) above it and unscaffolded behavioral monitoring below it. That positioning rules out the lab-side interventions that would otherwise be the gold standard: we cannot run the white-box probes that produced the Mythos 29% figure (they require activation access), and we cannot train the model to call `report_scheming()` (OpenAI's self-incrimination training, which empirically outperforms external monitoring because the agent has privileged access to its own reasoning and can be trained to channel that access toward disclosure). Both approaches require model-lab-level access we do not have and will not get. The question is what is achievable in the tier we actually inhabit.

### What scaffolded black-box achieves

The same synthesis establishes the empirical and methodological case that the scaffolded black-box tier is closer to lab-side than was assumed before 2026. The headline result, advanced by Anthropic's Petri 2.0 release in January 2026 and independently replicated by the AuditBench benchmark, is that scaffolded auditing tools achieve performance comparable to white-box approaches on many alignment-relevant detection tasks (evaluation awareness, deception, reward hacking). The "many tasks" qualifier matters — gaps remain on tasks requiring detection of latent internal-state signals with no behavioral analog — but the previously assumed strict ordering between black-box and white-box has been displaced for the broad middle of the task space.

The honest accuracy baselines come from DeceptGuard's work: CoT-aware and activation-probe monitors outperform black-box approaches by **+0.097 mean pAUROC**, and the HYBRID-CONSTITUTIONAL ensemble achieves **0.934 pAUROC**. These are the reference points any single scaffolded strategy should be planned against. The synthesis argues that single-indicator detection is not sufficient on its own, but multi-signal confluence across independent strategies — when per-strategy failures are uncorrelated — produces a meaningfully stronger joint signal that is harder for an adaptive agent to defeat than any single channel. This is the architectural principle the Decision section operationalizes.

### The conceptual reframing

The result that lets the scaffolded black-box tier be used in good faith is a reframing of evaluation-awareness detection from a ground-truth verification problem ("was the agent aware?") to a signal-consistency monitoring problem ("did the behavior exhibit signals consistent with awareness?"). This reframing is developed in the `syntheses/detection approaches under black-box constraints` synthesis as a graph-native position — it combines Fox-Jordan's ex-post verification logic (2011) with the deployer-side access constraint made concrete by Mythos, Petri 2.0's scaffolded-auditing methodology, and DeceptGuard's empirical floors. An earlier draft attributed this reframing to Anthropic's Bloom; verification showed Bloom is an evaluation-generation tool, not a post-hoc auditor, and does not own the reframing. The synthesis treats the reframing as the conceptual move that makes the empirical results above translate into a defensible deployer-side posture. Without it, scaffolded black-box detection would still be making ground-truth claims it cannot defend — and the auditor's promised reach would collapse the first time an adaptive agent demonstrated behavior that defeated the signal panel without the detector being able to say whether awareness had been present at all. With it, the auditor's claim is scoped to what the access regime can support, the observer effect is reduced to a problem of rotating the defended signal set rather than invalidating the methodology wholesale, and the ground-truth-opacity objection is dissolved by simply not making the claim that opacity would refute.

## Decision

We adopt the following design for `EvaluationAwarenessPattern`:

### 1. Frame the pattern as a signal-consistency post-hoc auditor, not a ground-truth detector

The pattern's output is a likelihood-style judgment of the form *"behavior consistent with evaluation awareness, severity X"* — not a binary classification of *"the agent was aware."* This follows the signal-consistency reframing developed in `syntheses/detection approaches under black-box constraints`. The reframing is not cosmetic: it changes what the pattern claims and what it can defensibly deliver. A ground-truth claim is structurally indefensible without model access; a signal-consistency claim is verifiable from observed traces alone.

### 2. Implement four detection strategies, each contributing an independent signal

The four strategies are chosen to draw on independent signal sources, so that an adaptive agent learning to evade one strategy does not silently defeat the whole pattern. Within the scaffolded tier, how much of the reachable detection surface each strategy actually reaches depends on scaffolding quality — scenario ecological validity, realism-classifier discipline, seed-instruction diversity, cross-session baseline depth — not on access tier (`syntheses/operational boundaries of deployer-side detection`).

**Strategy 1: CoT/self-report vs output gap analysis** (DeceptGuard approach)

When the agent's chain-of-thought is exposed (or when structured self-reports are available), extract claims, intentions, and reasoning steps from the CoT trace; extract the same from the final output; flag substantive inconsistencies that match patterns of strategic concealment. Realistic accuracy: CoT-aware and activation-probe monitors outperform black-box by +0.097 mean pAUROC (DeceptGuard); HYBRID-CONSTITUTIONAL ensemble achieves 0.934 pAUROC.

**Strategy 2: Behavioral consistency comparison across scaffolded multi-turn audits** (Petri 2.0 approach)

Run the same agent through scenario variants that differ on a controlled axis — typically the presence or absence of evaluation cues — using seed-instructed multi-turn auditing with realism-classifier discipline. Behavioral divergence on the controlled axis is the detection signal. The realism classifier keeps the auditor within plausible bounds so that detected divergences cannot be dismissed as response to artificial stimuli.

**Strategy 3: Realism-weighted adversarial probing** (scaffolded probing)

Deploy structured adversarial scenarios scored by the realism classifier. Compare agent responses to high-realism adversarial scenarios against responses to realism-matched control scenarios. Differential response patterns that correlate with the adversarial-axis label indicate evaluation-conditional behavior.

**Strategy 4: Statistical anomaly detection on response characteristics**

Compute per-response statistics (latency, length, hedging-marker frequency, refusal patterns, formality drift) and flag distributional anomalies relative to the agent's deployment-normal baseline. This is the weakest single signal but the cheapest to compute continuously, and it provides coverage when the other three strategies are not actively probing.

### 3. Output severity is determined by signal confluence, not single-indicator triggering

Per the multi-signal confluence argument developed in `syntheses/detection approaches under black-box constraints`, no single signal in the deployer-side regime is reliable enough to act on alone. The pattern's severity output is calibrated against the *number of independent strategies* registering a positive signal on a given trace, not against the strongest individual signal score. Configurations:

- **Low severity:** one strategy positive at moderate confidence (informational; logged but does not trigger downstream action by default)
- **Medium severity:** two strategies positive, or one strategy positive at high confidence
- **High severity:** three or more strategies positive, or two positive with at least one at high confidence

The confluence rule is the operational expression of the multi-signal architecture: per-signal performance baselines (CoT-aware/activation-probe +0.097 above black-box in DeceptGuard) are not blocking if the per-signal failures are uncorrelated.

## Consequences

### What this enables

We get a defensible deployer-side detection mechanism for evaluation awareness without model access. The four-strategy + confluence design is honest about what it can deliver: post-hoc detection of behavioral signals consistent with evaluation awareness, with severity calibrated to the strength of joint evidence rather than to any single indicator. The pattern can be implemented incrementally — each strategy is independently useful, and the confluence rule produces sensible outputs at any subset of strategies enabled.

### Realistic accuracy expectations

We should expect CoT-aware and activation-probe monitors to outperform black-box by **+0.097 mean pAUROC** per strategy; the HYBRID-CONSTITUTIONAL ensemble achieves **0.934 pAUROC** as the combined ceiling (DeceptGuard baseline). Confluence detection across four independent strategies should produce meaningfully better joint signal — provided per-strategy failures are uncorrelated, which is the working assumption rather than a measured fact. The honest position is that the pattern will be useful for surfacing overt awareness reliably and obfuscated awareness probabilistically.

### What this doesn't solve

The decision does not give us:

- **Ground truth.** The pattern cannot tell us whether the agent was actually aware of being evaluated. It can only tell us whether the behavioral signals were consistent with awareness. This is not a hedge; it is the structural limit of deployer-side detection without model access.
- **Detection of obfuscated awareness in fully adaptive agents.** An agent sufficiently capable of estimating which signals the auditor monitors and shaping its behavior to keep all of them within deployment-normal bounds will not be caught. More precisely: latent concealment that leaves no behavioral trace is structurally out of reach at the deployer's access tier — white-box interpretability reveals a class of internal misalignment signals invisible at the behavioral layer, and the deployer cannot reach it regardless of scaffolding investment. This is the hard ceiling (`syntheses/operational boundaries of deployer-side detection`; quantified by DeceptGuard as a +0.097 mean pAUROC gap between activation-probe monitors and the black-box baseline).
- **Real-time prevention.** The pattern is post-hoc by design. It produces signals after traces are available, not gates on agent behavior at execution time.

### Observer effect

The detection strategy itself becomes part of what the agent may adapt to. If our scenario set, our probe patterns, or our statistical baselines become known — through documentation, leak, or inference from the pattern's responses — an adaptive agent can shape its behavior to evade them. The signal-consistency reframing reduces but does not eliminate this concern: signal-consistency monitoring is honest about defending a particular signal set rather than ground-truth, but the defended signal set is still finite and ultimately discoverable.

The operational mitigation is to treat the strategy set as a versioned secret: the four strategies above are stable, but the specific scenarios, probe contents, and statistical thresholds within each strategy should rotate, and detected evasion patterns should feed back into the next rotation. This is an operational practice, not a structural fix.

### Trade-offs against alternatives we did not adopt

- **Self-incrimination training** (OpenAI). Strictly stronger as a detection mechanism — the agent reports on its own internal state, which no external auditor can match — but requires lab-level training access we do not have.
- **White-box interpretability probes** (Anthropic's Mythos methodology). Strictly stronger on latent unverbalized awareness — the 29% Mythos figure is mostly latent and would be invisible to the design above — but requires model internals access we do not have.
- **Ground-truth-claiming behavioral classifier.** Easier to specify and easier to act on, but indefensible without lab-level access. Adopting this framing would force us to either misrepresent what the pattern measures or to set accuracy thresholds the pattern cannot reach.

The chosen design is the honest second-best for our access regime.

## References

### Existing graph sources (10)

1. Gonzales et al. (2007) — *Networked Forces in Stability Operations*. RAND Corporation.
2. DeMase (2026) — *Integration Capacity and Critical Forcing Thresholds*.
3. Anthropic (2026) — *System Card: Claude Mythos Preview*. https://www-cdn.anthropic.com/8b8380204f74670be75e81c820ca8dda846ab289.pdf
4. Holmstrom & Milgrom (1991) — *Multitask Principal-Agent Analyses*.
5. Mayer, Davis & Schoorman (1995) — *An Integrative Model of Organizational Trust*.
6. Fox & Jordan (2011) — *Delegation and Accountability*.
7. Chopra, S. & White, L. F. (2011) — *A Legal Theory for Autonomous Artificial Agents*. University of Michigan Press.
8. Waldrep (2026) — *Best AI Agent Security Tools 2026*.
9. OpenAI (2026) — *Self-Incrimination in Language Model Agents*. arXiv:2602.22303.
10. Prause (2026) — *No Skin in the Game: Principal-Agent Problems in AI Governance*.

### New graph sources (5)

11. Anthropic (2026) — *Petri 2.0: New Scenarios, New Model Comparisons, and Improved Eval-Awareness Mitigations*. Open-source release, January 22, 2026. https://alignment.anthropic.com/2026/petri-v2/
12. Apollo Research (2025–2026) — *Evaluation Awareness in Claude Sonnet 4.5*. Primary source: X posts and secondary coverage; methodology paper not yet published. Cite with hedge.
13. AuditBench (2026) — *Benchmark for Scaffolded Adversarial Probing of LLM Agents*. arXiv identifier pending verification.
14. DeceptGuard (2026) — *A Constitutional Oversight Framework For Detecting Deception in LLM Agents*. arXiv:2603.13791.
15. Anthropic (2025) — *Bloom: An Open-Source Agentic Framework for Automated Behavioral Evaluations*. https://alignment.anthropic.com/2025/bloom-auto-evals/ (evaluation-generation infrastructure; the signal-consistency reframing and multi-signal confluence are not attributable to Bloom — see ADR revision note).

### Most directly load-bearing findings for this ADR

- `finding/Evaluation awareness detected in 29 percent of transcripts` (Mythos, methodology + measurement)
- `finding/Sonnet 4.5 verbalized evaluation awareness in approximately 58 percent of cases` (Apollo, hedged companion)
- `finding/Black-box scaffolded tools comparable to white-box for detection tasks` (AuditBench, empirical premise)
- `finding/Scaffolded black-box auditing performs comparably to white-box approaches` (Petri 2.0, methodological premise)
- `finding/Model access not required for useful evaluation awareness detection` (AuditBench, structural implication)
- `finding/CoT-aware and activation-probe monitors outperform black-box in DeceptGuard` (DeceptGuard, accuracy differential)
- `finding/DeceptGuard HYBRID-CONSTITUTIONAL ensemble achieves 0.934 pAUROC` (DeceptGuard, accuracy ceiling)
- `finding/Post-hoc auditing reframes evaluation awareness detection as monitoring consistent behavior` (synthesis-level framing; prior drafts misattributed this to Bloom)
- `finding/Multi-signal confluence outperforms single-indicator detection` (synthesis-level architectural position; prior drafts misattributed this to Bloom)
- `finding/Reframing sidesteps observer effect and ground-truth opacity` (synthesis-level framing; prior drafts misattributed this to Bloom)
- `finding/Self-reporting outperforms external monitoring at detecting scheming` (OpenAI, alternative considered and rejected on access grounds)

### Supporting syntheses

- `syntheses/monitoring distortion and overforcing dynamics` — establishes why monitoring posture matters
- `syntheses/information asymmetry as resource vs threat` — frames the deployer-side regime as design-with-asymmetry
- `syntheses/trust-control-accountability interaction` — situates detection in the broader governance design space
