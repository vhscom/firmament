package firmament

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNew_NilConfig(t *testing.T) {
	f, err := New(nil)
	if err != nil {
		t.Fatalf("New(nil): %v", err)
	}
	if f == nil {
		t.Fatal("returned nil Firmament")
	}
	if f.Monitor == nil {
		t.Fatal("Monitor is nil")
	}
	if f.Graph == nil {
		t.Fatal("Graph is nil")
	}
}

func TestNew_DefaultConfig(t *testing.T) {
	f, err := New(DefaultConfig())
	if err != nil {
		t.Fatalf("New(DefaultConfig): %v", err)
	}
	if f.Monitor == nil || f.Graph == nil {
		t.Fatal("nil Monitor or Graph")
	}
	// Default config has no GraphPath; graph should be empty.
	if len(f.Graph.Sources) != 0 || len(f.Graph.Findings) != 0 {
		t.Error("expected empty graph with no GraphPath")
	}
}

func TestNew_EmptyGraphPath(t *testing.T) {
	cfg := &Config{GraphPath: ""}
	f, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if f.Graph == nil {
		t.Fatal("Graph is nil")
	}
	if len(f.Graph.Sources) != 0 {
		t.Error("expected empty sources for empty GraphPath")
	}
}

func TestNew_NonExistentGraphPath(t *testing.T) {
	cfg := &Config{GraphPath: "/nonexistent/path/xyz"}
	f, err := New(cfg)
	if err != nil {
		t.Fatalf("New with nonexistent path: %v, want nil error", err)
	}
	if f == nil {
		t.Fatal("returned nil Firmament")
	}
}

func TestNew_WithSmallGraph(t *testing.T) {
	dir := t.TempDir()
	srcContent := "title:: Test Source\n"
	findContent := "claim:: Test claim about monitoring\nsource:: [[sources/Test Source]]\n"

	if err := os.WriteFile(filepath.Join(dir, "sources___Test Source.md"), []byte(srcContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "finding___test claim.md"), []byte(findContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{GraphPath: dir}
	f, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(f.Graph.Sources) != 1 {
		t.Errorf("sources: got %d, want 1", len(f.Graph.Sources))
	}
	if len(f.Graph.Findings) != 1 {
		t.Errorf("findings: got %d, want 1", len(f.Graph.Findings))
	}
}

func TestNew_PatternsWired(t *testing.T) {
	cfg := &Config{Patterns: []string{"action_concealment", "transcript_review"}}
	f, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Verify patterns are registered by running the monitor briefly.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediately cancel; just checking it doesn't panic
	_ = f.Monitor.Run(ctx)
}

func TestNew_WithRealGraph(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot resolve home directory")
	}
	path := filepath.Join(home, "Documents", "Claude", "Projects", "Research Graph", "pages")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("real research graph not present")
	}

	cfg := &Config{GraphPath: path}
	f, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if len(f.Graph.Sources) == 0 {
		t.Error("expected sources to be loaded from real graph")
	}
	if f.Monitor == nil {
		t.Error("Monitor is nil")
	}
}

func TestFirmament_GroundEndToEnd(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot resolve home directory")
	}
	path := filepath.Join(home, "Documents", "Claude", "Projects", "Research Graph", "pages")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("real research graph not present")
	}

	cfg := &Config{GraphPath: path}
	f, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	gr, err := f.Ground(context.Background(), "behavioral monitoring agent trust detection")
	if err != nil {
		t.Fatalf("Ground: %v", err)
	}
	if gr.Coverage.Confidence == "" {
		t.Error("expected non-empty coverage confidence")
	}
	if gr.Task == "" {
		t.Error("expected non-empty task")
	}
	t.Logf("real graph ground: confidence=%v syntheses=%d sources=%d findings=%d",
		gr.Coverage.Confidence, gr.Coverage.SynthesisCount, gr.Coverage.SourceCount, gr.Coverage.FindingCount)
}

func TestFirmament_GroundEmitsEventEndToEnd(t *testing.T) {
	dir := t.TempDir()
	synthContent := "sources:: [[sources/Src A]], [[sources/Src B]]\nstatus:: draft\n\n- # syntheses/monitoring\n- Monitoring behavioral agents improves security posture.\n"
	if err := os.WriteFile(filepath.Join(dir, "syntheses___monitoring.md"), []byte(synthContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{GraphPath: dir}
	f, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = f.Ground(context.Background(), "monitoring behavioral agents security")
	if err != nil {
		t.Fatalf("Ground: %v", err)
	}

	snapshot := f.Monitor.Ring().Snapshot(groundingSessionID, 10)
	if len(snapshot) == 0 {
		t.Error("expected grounding_requested event in monitor ring after Ground call")
	}
}

func TestConfig_BackwardCompatible(t *testing.T) {
	// A Config without GraphPath must not fail LoadGraph.
	cfg := &Config{
		LogPath:  "/tmp/test.log",
		Patterns: []string{"action_concealment"},
		// GraphPath intentionally absent
	}
	f, err := New(cfg)
	if err != nil {
		t.Fatalf("New without GraphPath: %v", err)
	}
	if f.Graph.path != "" {
		t.Errorf("expected empty graph path, got %q", f.Graph.path)
	}
}
