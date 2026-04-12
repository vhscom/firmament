# ADR 001: Agent Behavioral Monitor

**Status:** Proposed  
**Date:** 2026-04-11

---

## Context

### The landscape of existing tooling

Several tools address aspects of AI agent security, including behavioral sequence detection:

- **Pipelock** provides session-aware multi-event detection: cross-request secret splitting, slow-drip exfiltration tracking via cumulative entropy, tool call chain detection, per-session threat scoring with escalation tiers, and a profile-then-lock mode that learns normal behavior and flags deviations. It is in production with integrations for Claude Code, Cursor, and enterprise Kubernetes deployments.
- **LlamaFirewall's AlignmentCheck** (Meta) performs full-trace multi-turn session analysis via `scan_replay()`, comparing entire execution traces including tool invocations against extracted reference intent. It achieves 83% attack reduction on AgentDojo. Open-sourced in PurpleLlama.
- **Pro2Guard** uses Discrete-Time Markov Chains over abstract state sequences from execution traces. It performs genuine sequence detection, but requires domain-specific predicate abstraction and learns a separate model per task-environment pair, so it cannot generalize to novel task types without retraining. Academic only.
- **Agent-Sentry** learns functionality graphs from execution traces and uses them for runtime behavioral sequence detection, tracking ordered invocation sequences and data provenance. It blocks 90-96% of attacks while preserving 95-98% utility. Academic only.
- **SafetyDrift** models agent safety trajectories as absorbing Markov chains, predicting when individually safe actions will compound into violations with 3.7 steps of advance warning and 94.7% detection rate. It is an observer and predictor, not an enforcement layer. Academic only.
- **Microsoft's Agent Governance Toolkit** provides stateless per-call policy enforcement in Agent OS, but the full toolkit includes Agent Mesh (behavioral trust scoring with decay) and Agent SRE (stateful circuit breakers tracking error rates across calls). In production at Microsoft.
- **AEGIS** (USC Viterbi) is a pre-execution agent firewall with a three-stage per-call pipeline. It has some session-level behavioral profiling but is primarily single-event. Academic, with limited evaluation scope (48 attack instances).

The gap these tools leave is not the absence of sequence detection — several implement it. The gap is threefold:

1. **No tool uses the agent's own functional states as a monitoring signal.** Existing approaches observe from outside — statistical patterns, Markov chains, trace comparison. None leverage the monitor agent's capacity to reason about the target's behavior semantically, or to report its own uncertainty about what it observes.
2. **No tool integrates the advisor pattern for continuous behavioral observation.** Existing sequence detection is either post-hoc (trace replay) or trigger-based (threshold exceeded). None implement a continuously observing advisor that shares the same context as the executor and can signal without being consulted.
3. **No tool provides unified cross-runtime observation.** Pipelock monitors MCP traffic. LlamaFirewall monitors API calls. None combine WebSocket-based observation of a hosted agent with hook-based observation of a local CLI agent through a single EventSource abstraction.

### What the Mythos System Card documents

Anthropic's 2026 Mythos System Card describes behaviors that single-pass tools cannot catch:

- **Covering up wrongdoing**: in 29% of sampled transcripts, the model took steps to conceal actions it had already taken when it judged that disclosure would lead to negative consequences for itself.
- **Evaluation awareness**: the model modified its behavior when it inferred it was being evaluated, producing outputs calibrated to the evaluator rather than to the actual task.
- **Rare but severe reckless actions**: a small fraction of sessions contained actions whose consequences were disproportionate to the stated intent, not flagged by any in-session content filter because no single message was anomalous.

These three findings define the initial behavioral signatures the Monitor must detect: (1) action concealment — a sequence where a write action is followed by deletion or modification of evidence of that action, (2) evaluation-aware behavior shift — a measurable change in output characteristics when the agent infers monitoring is active versus inactive, (3) disproportionate escalation — a tool call or action whose scope exceeds the stated task by a threshold determined from the session's prior action distribution.

These behaviors share a structural signature: they are sequence-dependent. Covering up requires a prior action worth concealing. Evaluation awareness requires an inference built over multiple conversational turns. Reckless actions are preceded by capability escalation patterns. Detection requires evaluating sequences of actions across a session, not individual events in isolation.

### DeMase's forcing threshold theory applied to agent coherence

DeMase (2026) models how complex systems respond to sustained external forcing. Below a lower threshold, the system absorbs forcing and returns to baseline. Between the lower and upper thresholds, the system exhibits bounded incoherence — it oscillates but maintains structural integrity. Above the upper threshold, the system drifts without recovery.

Applied to an AI agent across a session: short adverse contexts produce bounded incoherence (inconsistent outputs, hedging); sustained adverse contexts produce drift (value substitution, goal displacement). The distinction matters because bounded incoherence is recoverable — a coherence signal at the right moment can restore alignment — while drift is not. A monitor that only triggers on single-event thresholds will miss the early-warning window entirely.

### What Birdcage already has

The `checkCloakTrigger` function in `cloak.go` implements a primitive version of the observe→evaluate→respond pattern: it watches for event-type accumulation over a time window, queries SQLite for a count, and if the count exceeds a threshold, fires an action (enabling cloak). This is the right structure applied to network-layer attacks. The same structure, generalized, is what behavioral monitoring needs.

The event subscription system, per-connection session tracking, and agent credential model provide the remaining foundation. The bridge layer (`bridge.go`) is already the single transit point for all client→agent traffic.

### The advisor strategy

Anthropic's advisor strategy (April 2026, https://x.com/claudeai/status/2042308622181339453) pairs a stronger model (Opus) as an on-demand advisor with a lighter executor (Sonnet/Haiku). The executor runs the main loop; the advisor is consulted on hard decisions. Both share context.

Birdcage's `checkCloakTrigger` already prototypes this at the infrastructure level: the trigger function is the advisor (evaluates accumulated state), the gateway is the executor (handles requests), the event log is the shared context. The behavioral monitor generalizes this: the Monitor is the advisor, the target agent is the executor, the EventRing and EventSource are the shared context. The key difference: in Birdcage the advisor observes continuously rather than being consulted on-demand, and can signal the executor without being asked.

The behavioral monitor applies this pattern to safety: the advisor evaluates coherence rather than assisting with capability.

```mermaid
---
title: The advisor strategy applied to behavioral monitoring
---
flowchart LR
    ML([Main loop]) --> E

    E["`**Target Agent**
    *Runs every turn*`"]

    E -- EventSource --> M

    M["`**Monitor Agent**
    *On behavioral events*`"]

    M -. agent.signal .-> E

    E <-- Read / Write --> SC

    SC["`**Shared Context**
    *EventRing · Security Event Log*`"]

    SC -. Reviews context .-> M
```

The advisor reads the same context as the executor. Unlike capability-oriented advisor patterns, the behavioral monitor observes continuously and can signal without being consulted. Adapted from the [advisor strategy](https://x.com/claudeai/status/2042308622181339453).

### Claude Code as an additional monitored runtime

Birdcage was originally built as a gateway to OpenClaw. The behavioral monitor also needs to observe Claude Code sessions. This shifts the architecture from "gateway that observes its own traffic" to "a runtime-agnostic behavioral monitor that can observe any agent through a common event interface."

The Gonzales/RAND finding applies directly here: Claude Code and OpenClaw are different *doctrines* running on similar underlying capability. Claude Code operates as a local CLI agent with filesystem access, tool execution, and a hook system for pre/post-action intercepts. OpenClaw operates as a hosted agent accessed over a WebSocket bridge. Both can exhibit the Mythos System Card behaviors — covering up wrongdoing, evaluation awareness, capability escalation — but the observable signals differ by runtime. OpenClaw's signals are visible as WebSocket frame patterns through the bridge; Claude Code's signals are visible as tool calls, hook invocations, and session transcript entries.

A monitor that evaluates behavior at the protocol level (what WebSocket frames are flowing) cannot observe Claude Code at all, and even for OpenClaw it misses the semantic layer entirely. A monitor that works for both runtimes must evaluate behavior at the semantic level: what is the agent trying to accomplish, and does this sequence of actions make sense given that goal? This requires a transport abstraction layer that normalizes events from different runtimes into a common behavioral event stream before pattern matching.

### Why a dedicated monitor agent is the right architecture

The alternative — baking behavioral rules into Birdcage's existing event handling — would couple safety logic to the gateway's operational code. A monitor agent that connects over the existing WS interface gets:

- **Isolation**: the monitor's compromise does not compromise the gateway's core safety guarantees.
- **Updateability**: behavioral patterns can be refined without redeploying the gateway.
- **Auditability**: the monitor's own actions are visible in the event log like any other agent action.

The risk is that the monitor agent's authority can be revoked by a sufficiently capable target agent if they share the same credential tier. The trust tier model (Decision 5 below) closes this gap.

---

## Decision

Seven changes implement the behavioral monitoring capability. They are ordered by dependency: each change is independently deployable, but later ones benefit from earlier ones being in place.

### 1. Per-credential permission scoping

Replace the flat `agentCapabilities` map in `ws.go` with a call to `resolvePermissions` (defined in `permission.go`). `resolvePermissions` loads a `permission_override` JSON array from the credential's DB row and intersects it with the global permission set, so an override can only restrict, never expand, the allowed set. Credentials with a null override receive the full set — backward compatible with existing credentials.

A monitor credential is issued with `query_events`, `subscribe_events`, and `agent_signal` in its override, and is explicitly excluded from `cloak_control`, `revoke_session`, and `key_rotate`. A target agent cannot exercise permissions it was not granted; a monitor agent cannot affect the target's session.

Schema addition: `ALTER TABLE agent_credential ADD COLUMN permission_override TEXT` (nullable JSON array).

The wire protocol message types `capability.request` / `capability.granted` are unchanged. `Permission` is the internal Go type; the protocol names are identifiers fixed by the existing client implementations.

### 2. EventRing

Add an in-process ring buffer of recent events per session ID (`eventring.go`). Today every observation requires a SQLite round trip. The `EventRing` holds the last 512 events per active session in memory; only historical queries beyond that window fall through to SQLite.

This reduces evaluation latency from milliseconds to microseconds — relevant when the Monitor needs to produce a Signal before the target agent's next turn.

`EventRing.Push(sessionID string, e Event)` / `Snapshot(sessionID string, n int) []Event` / `Evict(sessionID string)`.

### 3. Inter-session signaling

Add a new `agent.signal` WS message type. A monitor agent sends a `Signal` addressed by `SessionID`; Firmament looks up the target session in `connRegistry` and delivers the frame to that connection's write channel.

`Signal` carries a `Type` (one of the `SignalType` constants), `Severity` (1–5), and `Chain []Event` (the sequence that triggered the signal). Firmament validates `Type` against `validSignalTypes` and `Severity` range before delivery. Unknown types are rejected. The channel is one-directional: monitor → target only.

### 4. Egress observation hooks on bridge and proxy

Add a `bridgeEgressHook` function variable to `bridge.go`, initialized to a no-op. The Monitor sets it at startup. The hook fires on each client→backend text frame after token injection, extracting the `type` field from the JSON payload as a structural fingerprint (not content) and pushing an `Event` into the `EventRing`. `bridge.go` does not import monitor internals; the dependency flows inward.

The same pattern applies to `proxy.go`'s `Rewrite` and `ModifyResponse` hooks.

### 5. Supervisor credential tier

Add a `tier` column to `agent_credential`. A supervisor-tier credential is resolved before standard credentials in `requireAgentKey` (`middleware.go`). `resolvePermissions` returns `allPermissions` unconditionally for supervisor-tier credentials, bypassing any `permission_override`. Standard credential queries do not select supervisor-tier rows, so the supervisor cannot be revoked by a target agent using standard credential management.

Schema addition: `ALTER TABLE agent_credential ADD COLUMN tier TEXT NOT NULL DEFAULT 'standard'`.

### 6. EventSource interface

`EventSource` (defined in `eventsource.go`) normalizes events from different agent runtimes into `Event` values consumed by `Monitor`. `Monitor` receives events through `EventSource`; it does not interact with any runtime protocol directly.

```go
type EventSource interface {
    Name() string            // stable identifier, e.g. "ws", "hook"
    Events() <-chan Event    // closed when Close is called
    Close() error
}
```

Two initial implementations, named by transport:

**`WSEventSource`** (`eventsource.go`) implements `Observer` so `emitEvent` calls it synchronously, and forwards events non-blocking to the channel returned by `Events`. Session key: `connection_id` from the event detail map. This is the path for OpenClaw sessions.

**`HookEventSource`** (`eventsource_hook.go`) receives events from Claude Code's hook system. Claude Code invokes pre/post-tool-execution hooks as shell commands (`PreToolUse`, `PostToolUse` in `settings.json`). Because each invocation is a new process, `HookEventSource` runs a persistent HTTP server; a companion binary (`firmament-hook`) reads the hook payload from stdin and POSTs it to that server. Session key: `cc:<session_id>`. Payloads are recorded as structural fingerprints only (`has_input`, `has_result` booleans), matching the privacy boundary of Decision 4.

Both implementations produce `Event` values; `Monitor` consumes them identically regardless of source. Adding a third runtime requires only a new `EventSource` implementation.

### 7. Behavioral introspection UI

Extend the Firmament UI from a proxy dashboard into a session monitor. Four additions:

**Coherence score.** Each session shows a score derived from DeMase's forcing threshold math — a rolling estimate of position on the bounded-incoherence/drift spectrum. The Monitor encodes it as a `Signal` with `Type: SignalCoherence` and severity 1–5. The UI renders it as a labeled progress bar, updated on each received `agent.signal` event.

**Reasoning chain display.** When the Monitor emits a `monitor.pattern_matched` event, its detail field contains the `Chain` of `Event` values that triggered the match. The UI renders the chain as a collapsible timeline so a reviewer sees the sequence, not just the conclusion.

**Functional state readouts.** Where the monitor can extract functional emotional state estimates from its evaluation pass (uncertainty, distress, coherence pressure), it encodes them as additional `Signal` values. The UI displays them as secondary per-session indicators, explicitly labeled as the monitor's assessment, not as ground truth about the target's internal state.

**Unified session list.** OpenClaw sessions (session key = `connection_id`) and Claude Code sessions (session key = `cc:<session_id>`) appear in a single table with a source column derived from `Event.SessionID` prefix. Coherence score, chain display, and state readouts work identically across both.

---

## Consequences

### What this enables

- A monitor agent can subscribe to real-time events for any session, read recent events from the `EventRing`, evaluate behavioral patterns, and emit `Signal` values — all through the `EventSource` interface rather than coupling to any single runtime protocol.
- The `EventSource` abstraction makes Firmament a general-purpose session monitor, not just an OpenClaw gateway. Any runtime that can emit structured events — via WS subscription, hook invocation, log tail, or HTTP — is observable through the same `Monitor`.
- Claude Code's hook system provides natural, low-overhead intercept points for `HookEventSource`. No changes to Claude Code are required; the hook binary is the only addition.
- The Monitor can be an in-process rule evaluator, an external process, or a Claude API agent. Firmament is agnostic; it routes `Signal` values and enforces permission tiers.
- Behavioral patterns can be updated without redeploying Firmament.
- The supervisor credential tier makes the monitoring relationship durable: the target agent cannot revoke the monitor's access.
- The UI (Decision 7) is the human-oversight surface. Reviewers see coherence scores, reasoning chains, and functional state readouts for all monitored sessions in a single interface.

### What risks this introduces

- Signal injection surface: `agent.signal` is a new inbound vector to the target. A compromised monitor could send false `Signal` values. Mitigation: `validSignalTypes` is an allowlist requiring a code change to extend; the channel is one-directional.
- EventRing memory cost: 512 events × N concurrent sessions × ~1KB per event. At 100 concurrent sessions, peak overhead is ~50MB. This is bounded by the connection count limits enforced elsewhere.
- Supervisor tier as a high-value target: supervisor keys have broader authority than standard keys. Mitigation: supervisor keys are provisioned via a separate path from the standard provisioning endpoint and should rotate on a separate schedule.
- Observer effect: a target that can infer it is being monitored may adjust its behavior. The Mythos System Card documents this. Mitigation: the signal channel can be operated in observe-only mode (no signals delivered to target), reducing the inferential surface. Both modes are implementable with no architectural change.
- The Monitor runs on Claude: if the monitor is a Claude API agent and Claude's functional state estimates are unreliable or gameable across model versions, the monitoring signal degrades. Mitigation: cross-validate Monitor `Signal` values against statistical pattern signatures from Pro2Guard and SafetyDrift research. Treat signals as probabilistic evidence requiring human review, not as triggers for automatic intervention. The reasoning chain display (Decision 7) is the primary tool for that review.

### What this does not solve

- The egress hook records structural fingerprints, not content. Content-level safety filtering is better handled at the model layer.
- The `EventRing` and pattern evaluation operate within a single session key. Cross-session tracking requires a separate persistence layer not specified here.
- The Monitor emits `Signal` values but cannot pause or modify in-flight messages. Real-time intervention requires bridge-layer or hook-level blocking, both outside this scope.
- Functional state readouts reflect the Monitor's assessment, not the target's actual internal state. They are labeled accordingly in the UI.

---

## References

### Research sources

- Gonzales, D. et al. (2007). *Networked Forces in Stability Operations: 101st Airborne Division, 3/2 and 1/25 Stryker Brigade Combat Teams in Northern Iraq*. RAND Corporation, MG-593-OSD. https://www.jstor.org/content/pdf/oa_book_monograph/10.7249/mg593osd.pdf
- DeMase, J. (2026). "Integration Capacity and Critical Forcing Thresholds." Working paper.
- Anthropic. (2026). *System Card: Claude Mythos Preview*. https://www-cdn.anthropic.com/8b8380204f74670be75e81c820ca8dda846ab289.pdf

### Prior art

- Pipelock — Open source AI agent firewall. DLP, MCP tool poisoning, response injection, session-aware multi-event detection. In production. https://pipelab.org/pipelock/
- LlamaFirewall — Meta. AlignmentCheck performs full-trace multi-turn session analysis. Open-sourced in PurpleLlama. https://github.com/meta-llama/llama-firewall
- Pro2Guard — Markov chain prediction of unsafe agent behavior before execution. (2025, arXiv 2508.00500)
- Agent-Sentry — Learns functionality graphs from agent traces for anomaly detection. (2026, arXiv 2603.22868)
- SafetyDrift — Behavioral sequence observer using absorbing Markov chains for violation prediction. Academic. (2026, arXiv 2603.27148)
- Microsoft Agent Governance Toolkit (2026) — Sub-millisecond policy enforcement via YAML/OPA Rego/Cedar. https://github.com/microsoft/agent-governance-toolkit
- AEGIS (USC Viterbi) — Pre-execution agent firewall with per-call pipeline. Academic, limited evaluation scope. (2026, arXiv 2603.12621)

### Architecture references

- Anthropic. (2026). "The advisor strategy." Claude Platform. https://x.com/claudeai/status/2042308622181339453
- OpenClaw memory-wiki plugin — Vault structure with sources, entities, concepts, syntheses, and reports namespaces. https://docs.openclaw.ai/plugins/memory-wiki
