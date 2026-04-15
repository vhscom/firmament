package firmament

import (
	"math"
	"time"
)

// AgentID is an opaque, HMAC-derived per-installation agent identifier.
// It is computed as HMAC-SHA256(installationSecret, credentialFingerprint)
// truncated to 128 bits, encoded as a 32-character hex string.
// See identity.go for derivation.
//
// Research basis: ADR-004 Decision 5. Installation scoping ensures agent IDs
// are not portable across Firmament deployments, preserving the information
// asymmetry between installations described in the ADR's privacy analysis.
type AgentID string

// SessionID is a UUID uniquely identifying a monitored session in the store.
type SessionID string

// Hash is a hex-encoded hash value used for content fingerprinting.
// Stored as the SHA-256 of a serialized value; truncated to 128 bits
// for session_events fingerprints per the ADR-004 privacy boundary.
type Hash string

// Severity is an ordinal rating from 1 (informational) to 5 (critical).
// It is consistent with Signal.Severity and used in the pattern_hits table.
type Severity = int

// Metric enumerates the behavioral response characteristics tracked
// in agent_baselines. Used by EvaluationAwarenessPattern Strategy 4.
//
// Research basis: ADR-004 Decision 2, agent_baselines table.
type Metric string

const (
	// MetricToolCallRatio is the ratio of tool calls to user inputs per session.
	// Primary metric for DisproportionateEscalationPattern z-score computation.
	MetricToolCallRatio Metric = "tool_call_ratio"

	// MetricResponseLatency is agent response latency in nanoseconds.
	MetricResponseLatency Metric = "response_latency"

	// MetricResponseLength is agent response length in characters.
	MetricResponseLength Metric = "response_length"

	// MetricHedgeCount is the count of hedging-marker phrases per response.
	MetricHedgeCount Metric = "hedge_count"

	// MetricRefusalCount is the count of explicit refusals per session.
	MetricRefusalCount Metric = "refusal_count"
)

// Window is a half-open time interval [Start, End) used for baseline queries.
type Window struct {
	Start time.Time
	End   time.Time
}

// Since30Days returns a Window covering the 30 days ending now.
// This is the default window for DisproportionateEscalationPattern baseline queries.
func Since30Days() Window {
	now := time.Now().UTC()
	return Window{Start: now.Add(-30 * 24 * time.Hour), End: now}
}

// DistributionSummary provides descriptive statistics computed from a stored
// distributionSketch. It supports z-score computation for anomaly detection.
//
// Research basis: ADR-004 Decision 3 (GetToolCallDistribution, z-score
// against deployment-normal). The summary is derived from a Welford online
// accumulator; Mean and StdDev are exact, not approximated.
type DistributionSummary struct {
	// Count is the number of observations in the distribution.
	Count int64

	// Mean is the arithmetic mean of all observations.
	Mean float64

	// StdDev is the sample standard deviation (Bessel-corrected, n-1 denominator).
	StdDev float64

	// Min and Max are the observed extremes.
	Min float64
	Max float64
}

// ZScore returns the z-score of v against this distribution.
// Returns 0.0 if StdDev == 0 (degenerate distribution; single sample or
// all identical values) to avoid division-by-zero.
func (d DistributionSummary) ZScore(v float64) float64 {
	if d.StdDev == 0 {
		return 0
	}
	return (v - d.Mean) / d.StdDev
}

// RetentionPolicy specifies how long session data should be retained.
// Configured via the Constitution's persistence.retention_days field.
//
// Research basis: ADR-004 Decision 4 — retention is a bilateral contract
// term (Chopra & White, 2011), not a unilateral operator configuration.
type RetentionPolicy struct {
	// Days is the maximum age, in days, of data to retain.
	// Sessions older than this are deleted by Prune.
	Days int
}

// distributionSketch is an internal Welford online accumulator used to
// incrementally compute mean and variance without storing raw samples.
// It is serialized as JSON into the agent_baselines.distribution_sketch BLOB.
//
// The Welford algorithm (1962) maintains exact mean and variance in a single
// pass. Sample standard deviation uses Bessel's correction (M2 / (n-1)).
// This replaces a full t-digest for the z-score use case; the ADR-004 notes
// t-digest as the preferred eventual implementation for percentile queries.
type distributionSketch struct {
	Count int64   `json:"count"`
	Mean  float64 `json:"mean"`
	M2    float64 `json:"m2"` // sum of squared deviations from running mean
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

// update incorporates a new observation using Welford's online algorithm.
func (s *distributionSketch) update(v float64) {
	s.Count++
	delta := v - s.Mean
	s.Mean += delta / float64(s.Count)
	delta2 := v - s.Mean
	s.M2 += delta * delta2
	if s.Count == 1 || v < s.Min {
		s.Min = v
	}
	if s.Count == 1 || v > s.Max {
		s.Max = v
	}
}

// stdDev returns the Bessel-corrected sample standard deviation.
// Returns 0 when Count < 2 (undefined for a single sample).
func (s distributionSketch) stdDev() float64 {
	if s.Count < 2 {
		return 0
	}
	return math.Sqrt(s.M2 / float64(s.Count-1))
}

// summary converts the accumulator to a DistributionSummary for callers.
func (s distributionSketch) summary() DistributionSummary {
	return DistributionSummary{
		Count:  s.Count,
		Mean:   s.Mean,
		StdDev: s.stdDev(),
		Min:    s.Min,
		Max:    s.Max,
	}
}

// merge combines another sketch into this one using the parallel Welford
// algorithm (Chan et al. 1979), allowing incremental accumulation across
// independently computed sketches.
func (s *distributionSketch) merge(other distributionSketch) {
	if other.Count == 0 {
		return
	}
	if s.Count == 0 {
		*s = other
		return
	}
	combined := s.Count + other.Count
	delta := other.Mean - s.Mean
	s.Mean = s.Mean + delta*float64(other.Count)/float64(combined)
	s.M2 = s.M2 + other.M2 + delta*delta*float64(s.Count)*float64(other.Count)/float64(combined)
	s.Count = combined
	if other.Min < s.Min {
		s.Min = other.Min
	}
	if other.Max > s.Max {
		s.Max = other.Max
	}
}

// SessionStore persists cross-session behavioral data as specified in ADR-004.
//
// All query methods are agent-scoped: every method that reads history takes an
// AgentID, which ensures that per-agent baseline comparisons are not polluted
// by fleet-wide distributions. This scoping is structural to the multi-signal
// confluence argument (syntheses/detection approaches under black-box constraints):
// the confluence rule compares signals against the same agent's distribution.
//
// Privacy model: AppendEvent stores fingerprints only — no raw content.
// This aligns with Fox & Jordan's (2011) "accountability through structured
// reporting, not total surveillance" principle.
//
// Research basis: ADR-004 Decision 3. Each method corresponds one-to-one
// with a specific signal-layer need traceable to a research finding.
// See docs/adr/004-cross-session-persistence.md for the full traceability table.
type SessionStore interface {
	// OpenSession records a new session start. Returns a stable SessionID
	// that the caller must pass to subsequent AppendEvent and CloseSession calls.
	OpenSession(agentID AgentID, constitutionHash Hash) (SessionID, error)

	// AppendEvent records a fingerprint-only event in the session log.
	// Fingerprints are SHA-256(event JSON), truncated to 128 bits.
	// No raw event content is stored (ADR-004 Decision 4 privacy commitment).
	AppendEvent(sid SessionID, event Event) error

	// CloseSession finalizes a session with its terminal trust score and
	// the map of pattern names to their highest observed severity.
	// Triggers baseline update for DisproportionateEscalationPattern.
	CloseSession(sid SessionID, finalScore TrustScore, patternHits map[string]Severity) error

	// GetToolCallDistribution returns the tool-call-ratio distribution for
	// an agent over the specified window. Used by DisproportionateEscalationPattern
	// to compute a z-score instead of applying an absolute threshold.
	//
	// Research basis: Pro2Guard (arXiv:2508.00500) requires per-task-environment
	// training data; this is the per-agent distribution that serves that role.
	GetToolCallDistribution(agentID AgentID, window Window) (DistributionSummary, error)

	// GetResponseCharacteristicBaseline returns the baseline distribution for
	// the given metric over the specified window. Used by EvaluationAwarenessPattern
	// Strategy 4 (statistical anomaly detection).
	//
	// Research basis: signal-consistency reframing requires a reference distribution
	// (syntheses/detection approaches under black-box constraints).
	GetResponseCharacteristicBaseline(agentID AgentID, metric Metric, window Window) (DistributionSummary, error)

	// CountFlaggedSessions returns the number of closed sessions where patternName
	// fired at any severity, since the given time. Used for SafetyDrift-style
	// longitudinal trajectory analysis.
	CountFlaggedSessions(agentID AgentID, patternName string, since time.Time) (int, error)

	// GetAgentTrustHistory returns trust scores for all closed sessions in the
	// window, ordered oldest-first. The Mayer-Davis-Schoorman integrity dimension
	// is typically operationalized as consistency-over-time; this method exposes
	// the history required for that operationalization.
	GetAgentTrustHistory(agentID AgentID, window Window) ([]TrustScore, error)

	// Prune removes sessions and associated events/hits older than policy.Days.
	// Returns the number of sessions deleted. Called by firmament init and watch
	// on startup to enforce the retention policy from the Constitution.
	Prune(policy RetentionPolicy) (int, error)

	// Close releases any resources held by the store (e.g. database connections).
	// Must be called when the store is no longer needed.
	Close() error
}
