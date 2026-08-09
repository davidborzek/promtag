// Command promtag watches Docker containers and renders their labelled
// Prometheus alerting/recording rules into files, reloading Prometheus on change.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/urfave/cli/v2"

	"github.com/davidborzek/promtag/internal/config"
	"github.com/davidborzek/promtag/internal/metrics"
	"github.com/davidborzek/promtag/internal/reconcile"
	"github.com/davidborzek/promtag/internal/reload"
	"github.com/davidborzek/promtag/internal/source"
)

// version is overridable at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	app := &cli.App{
		Name:    "promtag",
		Usage:   "render Prometheus rules from Docker container labels and reload Prometheus",
		Version: version,
		Description: "Configured via PROMTAG_* environment variables (see the README). promtag\n" +
			"watches the Docker API for containers carrying a promtag.rules label, renders\n" +
			"each into a Prometheus rule file, and reloads Prometheus when anything changes.",
		Action: runApp,
	}
	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}

func runApp(_ *cli.Context) error {
	cfg := config.Load()
	logger := newLogger(cfg.LogLevel)
	if err := run(cfg, logger); err != nil {
		logger.Error("fatal", "error", err)
		return cli.Exit("", 1)
	}
	return nil
}

func run(cfg config.Config, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dockerCli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	defer func() { _ = dockerCli.Close() }()

	m := metrics.New(version)
	m.Serve(ctx, cfg.MetricsAddr, logger)

	src := source.New(dockerCli, cfg.LabelPrefix, logger)
	rec := reconcile.New(src, reload.New(cfg.ReloadURL), cfg.RulesDir, logger)

	logger.Info("promtag started",
		"version", version,
		"rules_dir", cfg.RulesDir,
		"reload_url", cfg.ReloadURL,
		"label_prefix", cfg.LabelPrefix,
		"resync_interval", cfg.ResyncInterval,
	)

	reconcileOnce(ctx, rec, m, logger)

	changes := src.Watch(ctx, m.ObserveWatchRestart)
	ticker := time.NewTicker(cfg.ResyncInterval)
	defer ticker.Stop()

	var debounce <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			logger.Info("shutting down")
			return nil
		case _, ok := <-changes:
			if !ok {
				changes = nil
				continue
			}
			if debounce == nil {
				debounce = time.After(cfg.DebounceDelay)
			}
		case <-debounce:
			debounce = nil
			reconcileOnce(ctx, rec, m, logger)
		case <-ticker.C:
			reconcileOnce(ctx, rec, m, logger)
		}
	}
}

func reconcileOnce(ctx context.Context, rec *reconcile.Reconciler, m *metrics.Metrics, logger *slog.Logger) {
	start := time.Now()
	res, err := rec.Run(ctx)

	m.ObserveReconcile(err == nil, time.Since(start))
	if err == nil {
		m.SetState(res.Managed, res.Files, res.Invalid)
	}
	if res.Reloaded {
		m.ObserveReload(res.ReloadOK)
	}

	if err != nil {
		logger.Error("reconcile failed", "error", err)
	}
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l}))
}
