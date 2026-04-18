package firmament

import (
	"bufio"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Source represents an ingested research document.
// Sources are the primary units of evidence in the knowledge graph;
// each source contributes one or more Findings that are cross-referenced
// by Syntheses.
type Source struct {
	// Name is the stable identifier derived from the Logseq filename,
	// e.g. "Fox and Jordan 2011 — Delegation and Accountability".
	Name string

	// Title is the full document title extracted from the title:: property.
	Title string

	// Findings are the evidence claims extracted from this source.
	Findings []*Finding
}

// Finding represents a single evidence claim extracted from a Source.
// Each finding is a discrete, citable assertion linked to exactly one source.
type Finding struct {
	// Name is the stable identifier derived from the Logseq filename,
	// e.g. "Accountability requires structured reporting not total surveillance".
	Name string

	// Claim is the full claim text from the claim:: property.
	Claim string

	// Source is the originating research document.
	Source *Source

	// Tags are the taxonomy links from the supports:: property.
	Tags []string
}

// Synthesis represents a cross-source synthesized position.
// Each synthesis resolves conflicts and builds positions across multiple Sources
// in a way no individual source could provide alone.
type Synthesis struct {
	// Name is the stable identifier derived from the Logseq filename,
	// e.g. "detection approaches under black-box constraints".
	Name string

	// Domain is the topic area; derived from Name.
	Domain string

	// Position is the synthesized claim; extracted from the lead paragraph
	// of the synthesis body following the title heading.
	Position string

	// Sources are the independent research documents supporting this synthesis.
	Sources []*Source

	// Findings are the individual evidence claims this synthesis draws on.
	Findings []*Finding
}

// Graph holds the parsed knowledge graph, loaded once at startup.
// Methods on Graph are safe for concurrent use.
type Graph struct {
	mu        sync.RWMutex
	path      string
	Sources   []*Source
	Findings  []*Finding
	Syntheses []*Synthesis
}

// wikilinkRe matches Logseq [[wikilink]] references.
var wikilinkRe = regexp.MustCompile(`\[\[([^\[\]]+)\]\]`)

// extractWikiLinks returns all [[target]] references found in text.
func extractWikiLinks(text string) []string {
	matches := wikilinkRe.FindAllStringSubmatch(text, -1)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		result = append(result, m[1])
	}
	return result
}

// parseLogseqProperties extracts Logseq key:: value pairs from file content.
// Handles both "key:: value" and "key::" (empty value) forms.
func parseLogseqProperties(content string) map[string]string {
	props := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if idx := strings.Index(line, ":: "); idx >= 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+3:])
			if key != "" {
				props[key] = val
			}
		} else if strings.HasSuffix(line, "::") && !strings.Contains(line, " ") {
			key := strings.TrimSuffix(line, "::")
			if key != "" {
				props[key] = ""
			}
		}
	}
	return props
}

// extractSynthesisPosition extracts the lead paragraph from a synthesis file.
// It locates the title heading line ("- # syntheses/...") and returns
// the text of the first non-empty, non-heading bullet that follows.
func extractSynthesisPosition(content string) string {
	foundTitle := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !foundTitle {
			if strings.Contains(trimmed, "# syntheses/") || strings.Contains(trimmed, "# synthesis/") {
				foundTitle = true
			}
			continue
		}
		if trimmed == "" {
			continue
		}
		// Strip bullet marker.
		text := strings.TrimPrefix(trimmed, "- ")
		if text == trimmed {
			// No bullet marker; skip non-bullet lines (e.g. raw property leftovers).
			continue
		}
		// Skip sub-headings (## Core Thesis, etc.).
		if strings.HasPrefix(text, "#") {
			continue
		}
		if text == "" {
			continue
		}
		// Strip wikilinks, keeping the leaf name.
		text = wikilinkRe.ReplaceAllStringFunc(text, func(m string) string {
			inner := wikilinkRe.FindStringSubmatch(m)[1]
			if idx := strings.LastIndex(inner, "/"); idx >= 0 {
				return inner[idx+1:]
			}
			return inner
		})
		return text
	}
	return ""
}

// nameFromFile derives the entity name from a Logseq filename by stripping
// the namespace prefix and .md suffix.
// E.g. nameFromFile("sources___Prause 2026 — Title.md", "sources___") → "Prause 2026 — Title"
func nameFromFile(filename, prefix string) string {
	name := strings.TrimSuffix(filename, ".md")
	return strings.TrimPrefix(name, prefix)
}

// nameFromLink extracts the entity name from a Logseq wikilink by stripping
// the namespace prefix.
// E.g. nameFromLink("sources/Prause 2026 — Title") → "Prause 2026 — Title"
func nameFromLink(link string) string {
	if idx := strings.Index(link, "/"); idx >= 0 {
		return link[idx+1:]
	}
	return link
}

// LoadGraph parses a Logseq-format directory into a Graph.
// It reads all .md files under path, extracts Logseq key:: value properties
// and [[wikilinks]], and assembles Sources, Findings, and Syntheses into a
// connected graph. Returns an empty Graph (not an error) if path does not
// exist or is empty.
func LoadGraph(path string) (*Graph, error) {
	g := &Graph{path: path}
	if path == "" {
		return g, nil
	}
	if err := g.load(); err != nil {
		return g, err
	}
	return g, nil
}

// Reload re-reads the graph directory, replacing the in-memory data.
// Safe for concurrent use; callers may continue reading the graph during reload.
func (g *Graph) Reload() error {
	return g.load()
}

// load reads and parses all .md files in g.path, replacing current graph data.
func (g *Graph) load() error {
	if g.path == "" {
		return nil
	}

	entries, err := os.ReadDir(g.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}

	type rawSource struct {
		name         string
		title        string
		findingLinks []string
	}
	type rawFinding struct {
		name       string
		claim      string
		sourceLink string
		tags       []string
	}
	type rawSynthesis struct {
		name         string
		sourceLinks  []string
		findingLinks []string
		position     string
	}

	var rawSrcs []rawSource
	var rawFinds []rawFinding
	var rawSynths []rawSynthesis

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		fname := entry.Name()
		data, err := os.ReadFile(filepath.Join(g.path, fname))
		if err != nil {
			continue // skip unreadable files without failing the load
		}
		content := string(data)

		switch {
		case strings.HasPrefix(fname, "sources___"):
			entityName := nameFromFile(fname, "sources___")
			props := parseLogseqProperties(content)
			title := props["title"]
			if title == "" {
				title = entityName
			}
			var findLinks []string
			for _, link := range extractWikiLinks(content) {
				if strings.HasPrefix(link, "finding/") {
					findLinks = append(findLinks, nameFromLink(link))
				}
			}
			rawSrcs = append(rawSrcs, rawSource{
				name:         entityName,
				title:        title,
				findingLinks: findLinks,
			})

		case strings.HasPrefix(fname, "finding___"):
			entityName := nameFromFile(fname, "finding___")
			props := parseLogseqProperties(content)
			claim := props["claim"]
			sourceLink := ""
			if sv := props["source"]; sv != "" {
				if links := extractWikiLinks(sv); len(links) > 0 {
					sourceLink = nameFromLink(links[0])
				}
			}
			var tags []string
			if sv := props["supports"]; sv != "" {
				tags = append(tags, extractWikiLinks(sv)...)
			}
			rawFinds = append(rawFinds, rawFinding{
				name:       entityName,
				claim:      claim,
				sourceLink: sourceLink,
				tags:       tags,
			})

		case strings.HasPrefix(fname, "syntheses___") || strings.HasPrefix(fname, "synthesis___"):
			prefix := "syntheses___"
			if strings.HasPrefix(fname, "synthesis___") {
				prefix = "synthesis___"
			}
			entityName := nameFromFile(fname, prefix)
			props := parseLogseqProperties(content)
			var srcLinks []string
			if sv := props["sources"]; sv != "" {
				for _, link := range extractWikiLinks(sv) {
					if strings.HasPrefix(link, "sources/") {
						srcLinks = append(srcLinks, nameFromLink(link))
					}
				}
			}
			seen := make(map[string]bool)
			var findLinks []string
			for _, link := range extractWikiLinks(content) {
				if strings.HasPrefix(link, "finding/") {
					fn := nameFromLink(link)
					if !seen[fn] {
						seen[fn] = true
						findLinks = append(findLinks, fn)
					}
				}
			}
			rawSynths = append(rawSynths, rawSynthesis{
				name:         entityName,
				sourceLinks:  srcLinks,
				findingLinks: findLinks,
				position:     extractSynthesisPosition(content),
			})
		}
	}

	// Build Source map.
	sourceMap := make(map[string]*Source, len(rawSrcs))
	sources := make([]*Source, 0, len(rawSrcs))
	for _, rs := range rawSrcs {
		s := &Source{Name: rs.name, Title: rs.title}
		sources = append(sources, s)
		sourceMap[rs.name] = s
	}

	// Build Finding map and wire Source back-references.
	findingMap := make(map[string]*Finding, len(rawFinds))
	findings := make([]*Finding, 0, len(rawFinds))
	for _, rf := range rawFinds {
		src := sourceMap[rf.sourceLink]
		f := &Finding{
			Name:   rf.name,
			Claim:  rf.claim,
			Source: src,
			Tags:   rf.tags,
		}
		findings = append(findings, f)
		findingMap[rf.name] = f
	}
	for _, f := range findings {
		if f.Source != nil {
			f.Source.Findings = append(f.Source.Findings, f)
		}
	}

	// Build Syntheses with resolved Source and Finding pointers.
	syntheses := make([]*Synthesis, 0, len(rawSynths))
	for _, rs := range rawSynths {
		var srcs []*Source
		for _, link := range rs.sourceLinks {
			if s, ok := sourceMap[link]; ok {
				srcs = append(srcs, s)
			}
		}
		var finds []*Finding
		for _, link := range rs.findingLinks {
			if f, ok := findingMap[link]; ok {
				finds = append(finds, f)
			}
		}
		syntheses = append(syntheses, &Synthesis{
			Name:     rs.name,
			Domain:   rs.name,
			Position: rs.position,
			Sources:  srcs,
			Findings: finds,
		})
	}

	// Populate Sources transitively from Findings when no direct sources:: property.
	for _, syn := range syntheses {
		if len(syn.Sources) == 0 {
			seen := make(map[string]bool)
			for _, f := range syn.Findings {
				if f.Source != nil && !seen[f.Source.Name] {
					seen[f.Source.Name] = true
					syn.Sources = append(syn.Sources, f.Source)
				}
			}
		}
	}

	g.mu.Lock()
	g.Sources = sources
	g.Findings = findings
	g.Syntheses = syntheses
	g.mu.Unlock()

	return nil
}
