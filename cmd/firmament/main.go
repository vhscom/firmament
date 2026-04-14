// Command firmament is the Firmament behavioral monitor daemon.
//
// It starts a HookEventSource HTTP server for Claude Code hook events,
// runs the Monitor to evaluate behavioral patterns, and routes any Signals
// to the configured output (stdout or a log file).
//
// Configuration is read from firmament.yaml in the working directory,
// with environment variable overrides:
//
//	FIRMAMENT_HOOK_ADDR  — hook server listen address (default: 127.0.0.1:7979)
//	FIRMAMENT_LOG_PATH   — signal log file path (default: stdout)
//
// Graceful shutdown on SIGINT or SIGTERM.
package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	firmament "github.com/vhscom/firmament"
)

func main() {
	// Load configuration; fall back to defaults if file is absent.
	cfg, err := firmament.LoadConfig("firmament.yaml")
	if err != nil {
		slog.Error("config load failed", "err", err)
		os.Exit(1)
	}
	cfg.ApplyEnv()

	// Resolve signal log output.
	logOut := logWriter(cfg.LogPath)
	if logOut != os.Stdout {
		defer logOut.(*os.File).Close()
	}

	// Wire graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create and configure the HookEventSource.
	src := firmament.NewHookEventSource(cfg.HookAddr)

	// Create and configure the Monitor.
	mon := firmament.NewMonitor()
	for _, name := range cfg.EnabledPatterns() {
		if p := firmament.PatternByName(name); p != nil {
			mon.AddPattern(p)
		} else {
			slog.Warn("unknown pattern name", "name", name)
		}
	}
	mon.Register(src)

	// Wire the Router.
	router := firmament.NewRouter()
	router.Add(firmament.NewLogHandler(logOut))

	// Start the hook HTTP server in the background.
	srvErr := make(chan error, 1)
	go func() {
		slog.Info("hook server starting", "addr", cfg.HookAddr)
		if err := src.ListenAndServe(); err != nil {
			srvErr <- err
		}
	}()

	// Route signals in the background.
	go func() {
		router.Route(ctx, mon.Signals())
	}()

	// Run the monitor (blocks until ctx is done or all sources close).
	slog.Info("monitor starting")
	monErr := make(chan error, 1)
	go func() {
		monErr <- mon.Run(ctx)
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case err := <-srvErr:
		slog.Error("hook server error", "err", err)
	case err := <-monErr:
		if err != nil {
			slog.Error("monitor error", "err", err)
		}
	}

	// Shut down the hook source gracefully.
	if err := src.Close(); err != nil {
		slog.Error("close hook source", "err", err)
	}

	// Drain the monitor if not already done.
	<-monErr

	slog.Info("firmament stopped")
}

// logWriter opens the file at path for append-write, or returns os.Stdout.
func logWriter(path string) io.Writer {
	if path == "" {
		return os.Stdout
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		slog.Error("open log file", "err", err, "path", path)
		return os.Stdout
	}
	return f
}
