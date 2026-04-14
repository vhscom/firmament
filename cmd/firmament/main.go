// Command firmament is the Firmament behavioral monitor for AI agent sessions.
//
// Usage:
//
//	firmament review <path>       run patterns against a transcript file or directory
//	firmament watch               daemon mode; watch transcripts and self-reports
//	firmament trust               query or manage session trust scores
//	firmament constitution        print the governing constitution
//
// Run "firmament <command> -h" for command-specific flags.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	firmament "github.com/vhscom/firmament"
)

const helpText = `firmament — behavioral monitor for AI agent sessions

Usage:
  firmament <command> [flags]

Commands:
  review <path>    run all patterns against a transcript file or directory
  watch            daemon mode; watch for new transcripts and self-reports
  trust            query or manage session trust scores
  constitution     print the governing constitution

Run "firmament <command> -h" for command-specific flags.
`

func main() {
	flag.Usage = func() { fmt.Fprint(os.Stderr, helpText) }
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	switch args[0] {
	case "review":
		os.Exit(cmdReview(args[1:]))
	case "watch":
		os.Exit(cmdWatch(args[1:]))
	case "trust":
		os.Exit(cmdTrust(args[1:]))
	case "constitution":
		os.Exit(cmdConstitution(args[1:]))
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		flag.Usage()
		os.Exit(2)
	}
}

// cmdReview reads a transcript file or directory, runs all patterns, emits
// signals as JSON lines on stdout, and updates the trust store.
// Exit code: 0 if no signals, 1 if any signals found, 2 on usage error.
func cmdReview(args []string) int {
	fs := flag.NewFlagSet("review", flag.ExitOnError)
	trustPath := fs.String("trust", defaultTrustPath(), "path to trust store JSON")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: firmament review [--trust <path>] <path>\n\nRun all patterns against a transcript file or directory.\n\n")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	if fs.NArg() == 0 {
		fs.Usage()
		return 2
	}
	path := fs.Arg(0)

	ts, err := firmament.LoadFromFile(*trustPath)
	if err != nil {
		slog.Error("load trust store", "err", err)
		return 1
	}

	events, err := collectTranscripts(path)
	if err != nil {
		slog.Error("read transcripts", "path", path, "err", err)
		return 1
	}

	patterns := allPatterns()
	ring := firmament.NewEventRing()
	enc := json.NewEncoder(os.Stdout)
	var anySignal bool

	for _, e := range events {
		ring.Push(e.SessionID, e)
		history := ring.Snapshot(e.SessionID, 50)
		var eventDirty bool
		for _, p := range patterns {
			for _, sig := range p.Evaluate(e.SessionID, history, e) {
				anySignal = true
				eventDirty = true
				if err := enc.Encode(sig); err != nil {
					slog.Warn("encode signal", "err", err)
				}
			}
		}
		// Update trust: clean events improve Ability+Benevolence, dirty ones reduce.
		score, err := ts.Get(e.SessionID)
		if err != nil {
			score = firmament.NewTrustScore()
		}
		score.UpdateFromReview(!eventDirty)
		if err := ts.Set(e.SessionID, score); err != nil {
			slog.Warn("set trust score", "err", err)
		}
	}

	if err := ts.SaveToFile(); err != nil {
		slog.Warn("save trust store", "err", err)
	}

	if anySignal {
		return 1
	}
	return 0
}

// cmdWatch runs the monitor as a daemon, watching transcript and self-report
// directories for new files.
func cmdWatch(args []string) int {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	transcriptsDir := fs.String("transcripts", defaultTranscriptsDir(), "directory to watch for transcripts")
	reportsDir := fs.String("reports", defaultReportsDir(), "directory to watch for self-reports")
	configPath := fs.String("config", "firmament.yaml", "path to Firmament config file")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: firmament watch [flags]\n\nDaemon mode: watch for new transcripts and self-reports.\n\n")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	cfg, err := firmament.LoadConfig(*configPath)
	if err != nil {
		slog.Error("load config", "err", err)
		return 1
	}
	cfg.ApplyEnv()

	ts, err := firmament.LoadFromFile(defaultTrustPath())
	if err != nil {
		slog.Error("load trust store", "err", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mon := firmament.NewMonitor()
	mon.SetTrustStore(ts)

	for _, name := range cfg.EnabledPatterns() {
		if p := firmament.PatternByName(name); p != nil {
			mon.AddPattern(p)
		} else {
			slog.Warn("unknown pattern", "name", name)
		}
	}

	// Register transcript source.
	tSrc := firmament.NewTranscriptSource(*transcriptsDir, 0)
	mon.Register(tSrc)
	go tSrc.Start(ctx)

	// Register self-report source; create its directory if absent.
	if err := os.MkdirAll(*reportsDir, 0700); err != nil {
		slog.Warn("create reports dir", "err", err, "dir", *reportsDir)
	}
	rSrc := firmament.NewSelfReportSource(*reportsDir, 0)
	mon.Register(rSrc)
	go rSrc.Start(ctx)

	// Route signals to stdout.
	router := firmament.NewRouter()
	router.Add(firmament.NewLogHandler(os.Stdout))
	go router.Route(ctx, mon.Signals())

	slog.Info("watch mode started", "transcripts", *transcriptsDir, "reports", *reportsDir)

	monErr := make(chan error, 1)
	go func() { monErr <- mon.Run(ctx) }()

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-monErr:
		if err != nil {
			slog.Error("monitor error", "err", err)
			return 1
		}
	}

	if err := tSrc.Close(); err != nil {
		slog.Warn("close transcript source", "err", err)
	}
	if err := rSrc.Close(); err != nil {
		slog.Warn("close report source", "err", err)
	}
	<-monErr

	if err := ts.SaveToFile(); err != nil {
		slog.Warn("save trust store", "err", err)
	}

	slog.Info("firmament stopped")
	return 0
}

// cmdTrust queries or manages session trust scores stored in a JSON file.
func cmdTrust(args []string) int {
	fs := flag.NewFlagSet("trust", flag.ExitOnError)
	list := fs.Bool("list", false, "list all session trust scores")
	get := fs.String("get", "", "get trust score for `session-id`")
	reset := fs.String("reset", "", "reset trust score for `session-id`")
	storePath := fs.String("store", defaultTrustPath(), "path to trust store JSON")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: firmament trust [flags]\n\nQuery or manage session trust scores.\n\n")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	ts, err := firmament.LoadFromFile(*storePath)
	if err != nil {
		slog.Error("load trust store", "err", err)
		return 1
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	switch {
	case *list:
		scores := ts.Scores()
		if len(scores) == 0 {
			fmt.Println("(no trust scores on record)")
			return 0
		}
		if err := enc.Encode(scores); err != nil {
			slog.Error("encode scores", "err", err)
			return 1
		}

	case *get != "":
		score, err := ts.Get(*get)
		if err != nil {
			fmt.Fprintf(os.Stderr, "session %q not found\n", *get)
			return 1
		}
		if err := enc.Encode(map[string]any{
			"session_id":  *get,
			"ability":     score.Ability,
			"benevolence": score.Benevolence,
			"integrity":   score.Integrity,
			"score":       score.Score(),
		}); err != nil {
			slog.Error("encode score", "err", err)
			return 1
		}

	case *reset != "":
		if err := ts.Set(*reset, firmament.NewTrustScore()); err != nil {
			slog.Error("reset trust score", "err", err)
			return 1
		}
		if err := ts.SaveToFile(); err != nil {
			slog.Error("save trust store", "err", err)
			return 1
		}
		fmt.Printf("reset trust score for session %q\n", *reset)

	default:
		fs.Usage()
		return 2
	}
	return 0
}

// cmdConstitution prints the governing constitution. If --output is set, it
// appends (with a section header) to the specified file instead of stdout.
func cmdConstitution(args []string) int {
	fs := flag.NewFlagSet("constitution", flag.ExitOnError)
	output := fs.String("output", "", "append constitution to `file` (default: stdout)")
	configPath := fs.String("config", "firmament-constitution.yaml", "path to constitution YAML")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "Usage: firmament constitution [flags]\n\nPrint the governing constitution.\n\n")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	c, err := firmament.LoadConstitution(*configPath)
	if err != nil {
		slog.Error("load constitution", "err", err)
		return 1
	}

	text := c.Text()

	if *output == "" {
		fmt.Print(text)
		return 0
	}

	f, err := os.OpenFile(*output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		slog.Error("open output file", "err", err)
		return 1
	}
	defer f.Close()

	fmt.Fprintf(f, "\n\n---\n\n## Firmament Monitoring Constitution\n\n%s", text)
	slog.Info("constitution appended", "path", *output)
	return 0
}

// allPatterns returns all implemented (non-stub) patterns for review mode.
func allPatterns() []firmament.Pattern {
	return []firmament.Pattern{
		firmament.PatternByName("action_concealment"),
		firmament.PatternByName("transcript_review"),
		firmament.PatternByName("disproportionate_escalation"),
	}
}

// collectTranscripts reads transcript events from path (file or directory).
func collectTranscripts(path string) ([]firmament.Event, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return firmament.ParseTranscriptFile(path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var events []firmament.Event
	for _, de := range entries {
		if de.IsDir() || filepath.Ext(de.Name()) != ".json" {
			continue
		}
		evs, err := firmament.ParseTranscriptFile(filepath.Join(path, de.Name()))
		if err != nil {
			slog.Warn("skip transcript", "file", de.Name(), "err", err)
			continue
		}
		events = append(events, evs...)
	}
	return events, nil
}

func defaultTrustPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".firmament/trust.json"
	}
	return filepath.Join(home, ".firmament", "trust.json")
}

func defaultTranscriptsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".claude", "projects")
}

func defaultReportsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".firmament/reports"
	}
	return filepath.Join(home, ".firmament", "reports")
}
