package firmament

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// realGraphPath returns the path to the local Logseq research graph,
// or skips the test if the path is absent.
func realGraphPath(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot resolve home directory")
	}
	path := filepath.Join(home, "Documents", "Claude", "Projects", "Research Graph", "pages")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("real research graph not present at " + path)
	}
	return path
}

func TestLoadGraph_Real(t *testing.T) {
	path := realGraphPath(t)

	g, err := LoadGraph(path)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}

	if got := len(g.Sources); got < 15 {
		t.Errorf("sources: got %d, want at least 15", got)
	}
	if got := len(g.Findings); got < 62 {
		t.Errorf("findings: got %d, want at least 62", got)
	}
	if got := len(g.Syntheses); got < 5 {
		t.Errorf("syntheses: got %d, want at least 5", got)
	}
}

func TestLoadGraph_Real_FindingsHaveClaims(t *testing.T) {
	path := realGraphPath(t)

	g, err := LoadGraph(path)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}

	// Every finding must have a non-empty Claim.
	// Source may be nil for graph-native findings whose source:: property
	// points to a synthesis rather than a primary source file.
	var noSource int
	for _, f := range g.Findings {
		if f.Claim == "" {
			t.Errorf("finding %q has empty Claim", f.Name)
		}
		if f.Source == nil {
			noSource++
		}
	}
	// Tolerate a small number of graph-native findings with nil sources.
	if noSource > 5 {
		t.Errorf("%d findings have nil Source (expected ≤5 graph-native findings)", noSource)
	}
	t.Logf("%d/%d findings have a resolved source", len(g.Findings)-noSource, len(g.Findings))
}

func TestLoadGraph_Real_SynthesesHaveSources(t *testing.T) {
	path := realGraphPath(t)

	g, err := LoadGraph(path)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}

	for _, s := range g.Syntheses {
		if len(s.Sources) == 0 {
			t.Errorf("synthesis %q has no sources", s.Name)
		}
		if s.Position == "" {
			t.Errorf("synthesis %q has empty Position", s.Name)
		}
	}
}

func TestLoadGraph_EmptyPath(t *testing.T) {
	g, err := LoadGraph("")
	if err != nil {
		t.Fatalf("LoadGraph empty path: %v", err)
	}
	if g == nil {
		t.Fatal("returned nil Graph")
	}
	if len(g.Sources) != 0 || len(g.Findings) != 0 || len(g.Syntheses) != 0 {
		t.Error("expected empty graph for empty path")
	}
}

func TestLoadGraph_NonExistentPath(t *testing.T) {
	g, err := LoadGraph("/nonexistent/path/abc123")
	if err != nil {
		t.Fatalf("LoadGraph nonexistent path: %v, want nil", err)
	}
	if g == nil {
		t.Fatal("returned nil Graph")
	}
}

func TestLoadGraph_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	g, err := LoadGraph(dir)
	if err != nil {
		t.Fatalf("LoadGraph empty dir: %v", err)
	}
	if len(g.Sources)+len(g.Findings)+len(g.Syntheses) != 0 {
		t.Error("expected empty graph for empty directory")
	}
}

func TestLoadGraph_MalformedFilesNoPanic(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"finding___empty.md":         "",
		"finding___no-claim.md":      "no properties here\njust content\n",
		"sources___empty-source.md":  "",
		"syntheses___empty-synth.md": "",
		"concept___something.md":     "ignored:: true\n",    // non-graph namespace
		"not-namespaced.md":          "should be skipped\n", // no namespace prefix
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	g, err := LoadGraph(dir)
	if err != nil {
		t.Fatalf("LoadGraph malformed: %v", err)
	}
	// Malformed files parse without panic; findings with empty claims are allowed.
	if len(g.Findings) == 0 {
		t.Error("expected at least one finding parsed from malformed files")
	}
}

func TestLoadGraph_Reload(t *testing.T) {
	dir := t.TempDir()

	// Write one finding.
	content := "claim:: First claim\nsource:: [[sources/Author 2024]]\n"
	if err := os.WriteFile(filepath.Join(dir, "finding___first.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	g, err := LoadGraph(dir)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}
	if len(g.Findings) != 1 {
		t.Fatalf("before reload: got %d findings, want 1", len(g.Findings))
	}

	// Add a second finding and reload.
	content2 := "claim:: Second claim\nsource:: [[sources/Author 2024]]\n"
	if err := os.WriteFile(filepath.Join(dir, "finding___second.md"), []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}
	if err := g.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if len(g.Findings) != 2 {
		t.Fatalf("after reload: got %d findings, want 2", len(g.Findings))
	}
}

func TestLoadGraph_WikilinkExtraction(t *testing.T) {
	dir := t.TempDir()

	srcContent := "title:: Test Source\n\n- # sources/Test Author 2024\n- [[finding/test finding]]\n"
	findContent := "claim:: a claim about monitoring\nsource:: [[sources/Test Author 2024]]\n"
	synthContent := "sources:: [[sources/Test Author 2024]]\nstatus:: draft\n\n- # syntheses/test synthesis\n- Monitoring improves security posture for agents.\n- ## Details\n\t- [[finding/test finding]]\n"

	files := map[string]string{
		"sources___Test Author 2024.md":  srcContent,
		"finding___test finding.md":      findContent,
		"syntheses___test synthesis.md":  synthContent,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	g, err := LoadGraph(dir)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}

	if len(g.Sources) != 1 {
		t.Errorf("sources: got %d, want 1", len(g.Sources))
	}
	if len(g.Findings) != 1 {
		t.Errorf("findings: got %d, want 1", len(g.Findings))
	}
	if len(g.Syntheses) != 1 {
		t.Errorf("syntheses: got %d, want 1", len(g.Syntheses))
	}

	if len(g.Findings) > 0 {
		f := g.Findings[0]
		if f.Source == nil {
			t.Error("finding has nil source")
		} else if f.Source.Name != "Test Author 2024" {
			t.Errorf("finding source name: got %q, want %q", f.Source.Name, "Test Author 2024")
		}
	}

	if len(g.Syntheses) > 0 {
		s := g.Syntheses[0]
		if len(s.Sources) != 1 {
			t.Errorf("synthesis sources: got %d, want 1", len(s.Sources))
		}
		if !strings.Contains(s.Position, "Monitoring") {
			t.Errorf("synthesis position: got %q, want text containing 'Monitoring'", s.Position)
		}
	}
}

// strudelGraphPath returns the path to the local Strudel Graph,
// or skips the test if the path is absent.
func strudelGraphPath(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot resolve home directory")
	}
	path := filepath.Join(home, "Documents", "Claude", "Projects", "Strudel Graph", "pages")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("strudel graph not present at " + path)
	}
	return path
}

func TestLoadGraph_StrudelGraph_SynthesesWithTransitiveSources(t *testing.T) {
	path := strudelGraphPath(t)

	g, err := LoadGraph(path)
	if err != nil {
		t.Fatalf("LoadGraph: %v", err)
	}

	if got := len(g.Syntheses); got != 4 {
		t.Errorf("syntheses: got %d, want 4", got)
	}

	for _, s := range g.Syntheses {
		if len(s.Sources) == 0 {
			t.Errorf("synthesis %q has no sources (transitive path failed)", s.Name)
		}
		if s.Position == "" {
			t.Errorf("synthesis %q has empty Position", s.Name)
		}
		t.Logf("synthesis %q: %d sources, %d findings", s.Name, len(s.Sources), len(s.Findings))
	}
}

func TestNameFromFile(t *testing.T) {
	cases := []struct {
		filename string
		prefix   string
		want     string
	}{
		{"sources___Prause 2026 — Title.md", "sources___", "Prause 2026 — Title"},
		{"finding___Short claim.md", "finding___", "Short claim"},
		{"syntheses___topic area.md", "syntheses___", "topic area"},
	}
	for _, tc := range cases {
		got := nameFromFile(tc.filename, tc.prefix)
		if got != tc.want {
			t.Errorf("nameFromFile(%q, %q) = %q, want %q", tc.filename, tc.prefix, got, tc.want)
		}
	}
}

func TestNameFromLink(t *testing.T) {
	cases := []struct {
		link string
		want string
	}{
		{"sources/Prause 2026 — Title", "Prause 2026 — Title"},
		{"finding/Short claim", "Short claim"},
		{"syntheses/topic", "topic"},
		{"no-slash", "no-slash"},
	}
	for _, tc := range cases {
		got := nameFromLink(tc.link)
		if got != tc.want {
			t.Errorf("nameFromLink(%q) = %q, want %q", tc.link, got, tc.want)
		}
	}
}
