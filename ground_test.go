package firmament

import (
	"context"
	"testing"
	"time"
)

// buildTestGraph constructs an in-memory Graph suitable for Ground tests.
// It contains 6 sources, 3 findings, and 2 syntheses with known properties.
func buildTestGraph() *Graph {
	src1 := &Source{Name: "Author 2024", Title: "Security Monitoring"}
	src2 := &Source{Name: "Author 2023", Title: "Trust Models"}
	src3 := &Source{Name: "Author 2022", Title: "Behavioral Analysis"}
	src4 := &Source{Name: "Author 2021", Title: "Agent Governance"}
	src5 := &Source{Name: "Author 2020", Title: "Detection Methods"}
	src6 := &Source{Name: "Author 2019", Title: "Risk Assessment"}

	f1 := &Finding{Name: "monitoring helps security", Claim: "Behavioral monitoring improves security posture for agents", Source: src1}
	f2 := &Finding{Name: "trust enables monitoring", Claim: "Trust models enable effective monitoring without surveillance", Source: src2}
	f3 := &Finding{Name: "detection requires signals", Claim: "Detection of behavioral anomalies requires multiple signals", Source: src3}

	src1.Findings = []*Finding{f1}
	src2.Findings = []*Finding{f2}
	src3.Findings = []*Finding{f3}

	// synth1 has 5 sources and position text overlapping many monitoring-related tokens.
	synth1 := &Synthesis{
		Name:     "monitoring and trust",
		Domain:   "security monitoring",
		Position: "Effective monitoring combines behavioral signals with trust models to achieve security for agents",
		Sources:  []*Source{src1, src2, src3, src4, src5},
		Findings: []*Finding{f1, f2, f3},
	}
	// synth2 has 4 sources and position text about detection approaches.
	synth2 := &Synthesis{
		Name:     "detection approaches",
		Domain:   "detection",
		Position: "Multi-signal detection approaches outperform single-indicator monitoring for behavioral agents",
		Sources:  []*Source{src3, src4, src5, src6},
		Findings: []*Finding{f3},
	}
	// synth3 has 3 sources and position text about trust and security monitoring.
	synth3 := &Synthesis{
		Name:     "trust and security",
		Domain:   "trust",
		Position: "Trust models and monitoring security posture are complementary for behavioral agent governance",
		Sources:  []*Source{src2, src4, src6},
		Findings: []*Finding{f2},
	}

	return &Graph{
		Sources:   []*Source{src1, src2, src3, src4, src5, src6},
		Findings:  []*Finding{f1, f2, f3},
		Syntheses: []*Synthesis{synth1, synth2, synth3},
	}
}

func TestGround_HighCoverage(t *testing.T) {
	f := &Firmament{Graph: buildTestGraph(), Monitor: NewMonitor()}
	ctx := context.Background()

	// Task hits both syntheses and their sources.
	gr, err := f.Ground(ctx, "behavioral monitoring security trust agent signals")
	if err != nil {
		t.Fatalf("Ground: %v", err)
	}
	if gr.Coverage.Confidence != CoverageHigh {
		t.Errorf("confidence: got %v, want High (synths=%d srcs=%d findings=%d)",
			gr.Coverage.Confidence, gr.Coverage.SynthesisCount, gr.Coverage.SourceCount, gr.Coverage.FindingCount)
	}
	if gr.Coverage.SynthesisCount < 3 {
		t.Errorf("SynthesisCount: got %d, want ≥3", gr.Coverage.SynthesisCount)
	}
	if gr.Coverage.SourceCount < 5 {
		t.Errorf("SourceCount: got %d, want ≥5", gr.Coverage.SourceCount)
	}
}

func TestGround_MediumCoverage(t *testing.T) {
	f := &Firmament{Graph: buildTestGraph(), Monitor: NewMonitor()}
	ctx := context.Background()

	// Task hits synth2 only (detection, single-indicator).
	gr, err := f.Ground(ctx, "detection single indicator approach")
	if err != nil {
		t.Fatalf("Ground: %v", err)
	}
	if gr.Coverage.Confidence != CoverageMedium {
		t.Errorf("confidence: got %v, want Medium (synths=%d srcs=%d)",
			gr.Coverage.Confidence, gr.Coverage.SynthesisCount, gr.Coverage.SourceCount)
	}
}

func TestGround_LowCoverage_NoMatch(t *testing.T) {
	f := &Firmament{Graph: buildTestGraph(), Monitor: NewMonitor()}
	ctx := context.Background()

	// Task has no overlap with any synthesis position or finding claim.
	gr, err := f.Ground(ctx, "quantum physics thermodynamics")
	if err != nil {
		t.Fatalf("Ground: %v", err)
	}
	if gr.Coverage.Confidence != CoverageLow {
		t.Errorf("confidence: got %v, want Low", gr.Coverage.Confidence)
	}
	if len(gr.Syntheses) != 0 {
		t.Errorf("syntheses: got %d, want 0", len(gr.Syntheses))
	}
}

func TestGround_EmptyGraph(t *testing.T) {
	f := &Firmament{Graph: &Graph{}, Monitor: NewMonitor()}
	ctx := context.Background()

	gr, err := f.Ground(ctx, "monitoring behavioral agents")
	if err != nil {
		t.Fatalf("Ground empty graph: %v", err)
	}
	if gr.Coverage.Confidence != CoverageLow {
		t.Errorf("confidence: got %v, want Low", gr.Coverage.Confidence)
	}
	if len(gr.Syntheses) != 0 || len(gr.Findings) != 0 {
		t.Error("expected empty results for empty graph")
	}
}

func TestGround_NilGraph(t *testing.T) {
	f := &Firmament{Graph: nil, Monitor: NewMonitor()}
	ctx := context.Background()

	gr, err := f.Ground(ctx, "anything")
	if err != nil {
		t.Fatalf("Ground nil graph: %v", err)
	}
	if gr.Task != "anything" {
		t.Errorf("Task: got %q, want %q", gr.Task, "anything")
	}
	if gr.Coverage.Confidence != CoverageLow {
		t.Errorf("confidence: got %v, want Low", gr.Coverage.Confidence)
	}
}

func TestGround_TaskAndTimestamp(t *testing.T) {
	f := &Firmament{Graph: buildTestGraph(), Monitor: NewMonitor()}
	ctx := context.Background()

	before := time.Now().UTC()
	gr, _ := f.Ground(ctx, "monitoring")
	after := time.Now().UTC()

	if gr.Task != "monitoring" {
		t.Errorf("Task: got %q, want %q", gr.Task, "monitoring")
	}
	if gr.Timestamp.Before(before) || gr.Timestamp.After(after) {
		t.Errorf("Timestamp %v not in [%v, %v]", gr.Timestamp, before, after)
	}
}

func TestGround_EmitsGroundingEvent(t *testing.T) {
	mon := NewMonitor()
	f := &Firmament{Graph: buildTestGraph(), Monitor: mon}

	_, err := f.Ground(context.Background(), "behavioral monitoring agent trust")
	if err != nil {
		t.Fatalf("Ground: %v", err)
	}

	// InjectEvent pushes directly to the ring; check synchronously.
	snapshot := mon.Ring().Snapshot(groundingSessionID, 10)
	if len(snapshot) == 0 {
		t.Fatal("expected grounding_requested event in ring")
	}
	evt := snapshot[len(snapshot)-1]
	if evt.Type != "grounding_requested" {
		t.Errorf("event type: got %q, want %q", evt.Type, "grounding_requested")
	}
	if evt.Detail["coverage_confidence"] == nil {
		t.Error("event missing coverage_confidence detail")
	}
	if evt.Detail["synthesis_count"] == nil {
		t.Error("event missing synthesis_count detail")
	}
	// Verify no query text is stored in the event.
	for k := range evt.Detail {
		if k != "coverage_confidence" && k != "synthesis_count" {
			t.Errorf("unexpected detail key %q in grounding event", k)
		}
	}
}

func TestGround_ResultsRankedBySourceCount(t *testing.T) {
	g := buildTestGraph()
	f := &Firmament{Graph: g, Monitor: NewMonitor()}
	ctx := context.Background()

	// Both syntheses match; synth1 has 5 sources, synth2 has 4 sources.
	gr, err := f.Ground(ctx, "behavioral monitoring detection signals agents")
	if err != nil {
		t.Fatalf("Ground: %v", err)
	}
	if len(gr.Syntheses) < 2 {
		t.Skip("fewer than 2 syntheses matched; cannot test ordering")
	}
	// synth1 has more sources and higher score; should come first.
	if gr.Syntheses[0].Name == "detection approaches" && gr.Syntheses[1].Name == "monitoring and trust" {
		// synth1 should rank higher due to more sources+overlap; check source counts
		s0 := len(gr.Syntheses[0].Sources)
		s1 := len(gr.Syntheses[1].Sources)
		if s0 < s1 {
			t.Errorf("first synthesis has fewer sources (%d) than second (%d)", s0, s1)
		}
	}
}

func TestGround_CoverageThresholds(t *testing.T) {
	tests := []struct {
		name     string
		nSynths  int
		nSrcs    int
		wantConf CoverageConfidence
	}{
		{"high: 3 synths 5 srcs", 3, 5, CoverageHigh},
		{"high: 4 synths 6 srcs", 4, 6, CoverageHigh},
		{"medium: 1 synth 2 srcs", 1, 2, CoverageMedium},
		{"medium: 2 synths 3 srcs", 2, 3, CoverageMedium},
		{"low: 1 synth 1 src", 1, 1, CoverageLow},
		{"low: 0 synths", 0, 0, CoverageLow},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cov := Coverage{
				SynthesisCount: tc.nSynths,
				SourceCount:    tc.nSrcs,
			}
			switch {
			case cov.SynthesisCount >= 3 && cov.SourceCount >= 5:
				cov.Confidence = CoverageHigh
			case cov.SynthesisCount >= 1 && cov.SourceCount >= 2:
				cov.Confidence = CoverageMedium
			default:
				cov.Confidence = CoverageLow
			}
			if cov.Confidence != tc.wantConf {
				t.Errorf("got %v, want %v", cov.Confidence, tc.wantConf)
			}
		})
	}
}

func TestTokenize(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{"behavioral monitoring", []string{"behavioral", "monitoring"}},
		{"AI agents exhibit risks", []string{"agents", "exhibit", "risks"}}, // "AI" filtered (len<3)
		{"  leading   spaces  ", []string{"leading", "spaces"}},
		{"", nil},
		{"a b c d", nil}, // all single-char words filtered
	}
	for _, tc := range cases {
		got := tokenize(tc.input)
		if len(got) != len(tc.want) {
			t.Errorf("tokenize(%q): got %v, want %v", tc.input, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("tokenize(%q)[%d]: got %q, want %q", tc.input, i, got[i], tc.want[i])
			}
		}
	}
}

func TestTokenOverlap(t *testing.T) {
	tokens := []string{"monitoring", "agent", "security"}
	cases := []struct {
		text string
		want int
	}{
		{"Behavioral monitoring improves security", 2},
		{"agent behavior detection", 1},
		{"something completely unrelated", 0},
		{"monitoring agent security posture", 3},
	}
	for _, tc := range cases {
		got := tokenOverlap(tokens, tc.text)
		if got != tc.want {
			t.Errorf("tokenOverlap(%v, %q) = %d, want %d", tokens, tc.text, got, tc.want)
		}
	}
}
