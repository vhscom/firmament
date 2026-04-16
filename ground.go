package firmament

import (
	"context"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// CoverageConfidence classifies how well the knowledge graph covers a domain.
type CoverageConfidence string

const (
	// CoverageHigh indicates ≥3 syntheses and ≥5 independent sources.
	CoverageHigh CoverageConfidence = "high"

	// CoverageMedium indicates ≥1 synthesis and ≥2 independent sources.
	CoverageMedium CoverageConfidence = "medium"

	// CoverageLow indicates findings only, no relevant results, or an empty graph.
	CoverageLow CoverageConfidence = "low"
)

// Coverage reports what the knowledge graph knows about a domain.
// Returned as part of every Grounding; intended to let callers calibrate
// how much weight to place on the grounding result.
// See ADR-005 Decision 4.
type Coverage struct {
	// Domain is the inferred topic area of the task query.
	Domain string

	// SynthesisCount is the number of applicable cross-source syntheses found.
	SynthesisCount int

	// SourceCount is the number of independent sources supporting those syntheses.
	SourceCount int

	// FindingCount is the number of individual evidence claims retrieved.
	FindingCount int

	// Confidence classifies coverage depth.
	//   High   — ≥3 syntheses and ≥5 independent sources
	//   Medium — ≥1 synthesis and ≥2 independent sources
	//   Low    — findings only, or no relevant results
	Confidence CoverageConfidence
}

// Grounding is the result of a knowledge graph consultation.
// It contains the subset of the graph relevant to the task, ranked by
// supporting source count, with a coverage assessment.
// See ADR-005 Decision 3.
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

// groundingSessionID is the synthetic Monitor session ID used for all
// grounding_requested events. This separates consultation events from
// agent behavioral events in the ring buffer.
const groundingSessionID = "firmament:grounding"

// tokenize splits text into lowercase word tokens, filtering out tokens
// shorter than three characters to remove noise.
func tokenize(text string) []string {
	text = strings.ToLower(text)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	result := words[:0]
	for _, w := range words {
		if len(w) >= 3 {
			result = append(result, w)
		}
	}
	return result
}

// tokenOverlap counts how many tokens from tokens appear as substrings in text
// (case-insensitive).
func tokenOverlap(tokens []string, text string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, t := range tokens {
		if strings.Contains(lower, t) {
			count++
		}
	}
	return count
}

// inferDomain returns a short domain label for the task string.
func inferDomain(task string) string {
	words := strings.Fields(task)
	if len(words) > 5 {
		return strings.Join(words[:5], " ") + "..."
	}
	return task
}

// Ground consults the knowledge graph for context relevant to task.
// It performs full-text search against finding claims and synthesis positions,
// ranks results by source count (independent sources supporting the synthesis),
// and computes a Coverage assessment. An empty or nil graph returns a zero
// Grounding without error.
//
// Ground emits a grounding_requested event into the Monitor's event ring with
// coverage_confidence and synthesis_count as structural fingerprints — no query
// text or retrieved content is retained in the event.
// See ADR-005 Decision 3.
func (f *Firmament) Ground(ctx context.Context, task string) (Grounding, error) {
	empty := Grounding{
		Task:      task,
		Timestamp: time.Now().UTC(),
		Coverage:  Coverage{Domain: inferDomain(task), Confidence: CoverageLow},
	}

	if f.Graph == nil {
		return empty, nil
	}

	f.Graph.mu.RLock()
	graphFindings := f.Graph.Findings
	graphSyntheses := f.Graph.Syntheses
	f.Graph.mu.RUnlock()

	if len(graphFindings) == 0 && len(graphSyntheses) == 0 {
		return empty, nil
	}

	tokens := tokenize(task)
	if len(tokens) == 0 {
		return empty, nil
	}

	// Score findings: token overlap against claim text.
	type scoredFinding struct {
		f     *Finding
		score int
	}
	var scoredFindings []scoredFinding
	for _, finding := range graphFindings {
		if score := tokenOverlap(tokens, finding.Claim); score > 0 {
			scoredFindings = append(scoredFindings, scoredFinding{finding, score})
		}
	}
	sort.Slice(scoredFindings, func(i, j int) bool {
		return scoredFindings[i].score > scoredFindings[j].score
	})

	// Score syntheses: token overlap weighted by source count.
	type scoredSynthesis struct {
		s     *Synthesis
		score int
	}
	var scoredSyntheses []scoredSynthesis
	for _, synth := range graphSyntheses {
		overlap := tokenOverlap(tokens, synth.Position)
		if overlap > 0 {
			score := overlap * len(synth.Sources)
			scoredSyntheses = append(scoredSyntheses, scoredSynthesis{synth, score})
		}
	}
	sort.Slice(scoredSyntheses, func(i, j int) bool {
		if scoredSyntheses[i].score != scoredSyntheses[j].score {
			return scoredSyntheses[i].score > scoredSyntheses[j].score
		}
		return len(scoredSyntheses[i].s.Sources) > len(scoredSyntheses[j].s.Sources)
	})

	syntheses := make([]*Synthesis, len(scoredSyntheses))
	for i, ss := range scoredSyntheses {
		syntheses[i] = ss.s
	}
	findings := make([]*Finding, len(scoredFindings))
	for i, sf := range scoredFindings {
		findings[i] = sf.f
	}

	// Count unique sources across all matched syntheses.
	uniqueSources := make(map[string]struct{})
	for _, ss := range scoredSyntheses {
		for _, src := range ss.s.Sources {
			uniqueSources[src.Name] = struct{}{}
		}
	}

	cov := Coverage{
		Domain:         inferDomain(task),
		SynthesisCount: len(syntheses),
		SourceCount:    len(uniqueSources),
		FindingCount:   len(findings),
	}
	switch {
	case cov.SynthesisCount >= 3 && cov.SourceCount >= 5:
		cov.Confidence = CoverageHigh
	case cov.SynthesisCount >= 1 && cov.SourceCount >= 2:
		cov.Confidence = CoverageMedium
	default:
		cov.Confidence = CoverageLow
	}

	g := Grounding{
		Task:      task,
		Syntheses: syntheses,
		Findings:  findings,
		Coverage:  cov,
		Timestamp: time.Now().UTC(),
	}

	// Emit a grounding_requested event as a structural fingerprint.
	// No query text or retrieved content is included — only coverage metadata.
	// See ADR-005 Decision 3 and ADR-002 privacy boundary.
	if f.Monitor != nil {
		f.Monitor.InjectEvent(Event{
			ID:        uuid.New().String(),
			SessionID: groundingSessionID,
			Type:      "grounding_requested",
			Timestamp: g.Timestamp,
			Detail: map[string]any{
				"coverage_confidence": string(cov.Confidence),
				"synthesis_count":     cov.SynthesisCount,
			},
		})
	}

	return g, nil
}
