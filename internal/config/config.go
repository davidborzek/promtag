// Package config loads promtag settings from the environment.
package config

import (
	"os"
	"time"
)

// Config holds all runtime settings. Everything is sourced from the
// environment to keep the tool trivially configurable in a compose file.
type Config struct {
	// RulesDir is the directory promtag owns and writes rule files into.
	// It must be shared (read-only) with Prometheus.
	RulesDir string
	// ReloadURL is the Prometheus reload endpoint (needs --web.enable-lifecycle).
	ReloadURL string
	// LabelPrefix is the container label namespace, e.g. "promtag" ->
	// "promtag.rules" and "promtag.rules.<name>".
	LabelPrefix string
	// ResyncInterval is the periodic full reconcile interval, a safety net
	// on top of event-driven reconciliation.
	ResyncInterval time.Duration
	// DebounceDelay coalesces bursts of container events into one reconcile.
	DebounceDelay time.Duration
	// LogLevel is one of debug, info, warn, error.
	LogLevel string
	// MetricsAddr is the listen address for the /metrics endpoint. Blank disables it.
	MetricsAddr string
}

func Load() Config {
	return Config{
		RulesDir:       env("PROMTAG_RULES_DIR", "/rules"),
		ReloadURL:      env("PROMTAG_RELOAD_URL", "http://localhost:9090/-/reload"),
		LabelPrefix:    env("PROMTAG_LABEL_PREFIX", "promtag"),
		ResyncInterval: envDuration("PROMTAG_RESYNC_INTERVAL", 60*time.Second),
		DebounceDelay:  envDuration("PROMTAG_DEBOUNCE_DELAY", time.Second),
		LogLevel:       env("PROMTAG_LOG_LEVEL", "info"),
		MetricsAddr:    env("PROMTAG_METRICS_ADDR", ":9333"),
	}
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
