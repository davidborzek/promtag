// Package metrics exposes promtag's own Prometheus metrics plus a liveness
// endpoint. Collectors live on a private registry so New is side-effect free.
package metrics

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds promtag's instrumentation and its dedicated registry.
type Metrics struct {
	reg           *prometheus.Registry
	reconciles    *prometheus.CounterVec
	reconcileTime prometheus.Histogram
	lastReconcile prometheus.Gauge
	lastSuccess   prometheus.Gauge
	ready         prometheus.Gauge
	managed       prometheus.Gauge
	ruleFiles     prometheus.Gauge
	invalidRules  prometheus.Gauge
	reloads       *prometheus.CounterVec
	watchRestarts prometheus.Counter
}

// New registers and returns the metric collectors on a private registry.
func New(version string) *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	m := &Metrics{
		reg: reg,
		reconciles: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "promtag_reconciles_total",
			Help: "Total reconcile runs by result.",
		}, []string{"result"}),
		reconcileTime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "promtag_reconcile_duration_seconds",
			Help:    "Duration of reconcile runs in seconds.",
			Buckets: prometheus.DefBuckets,
		}),
		lastReconcile: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "promtag_last_reconcile_timestamp_seconds",
			Help: "Unix timestamp of the last completed reconcile.",
		}),
		lastSuccess: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "promtag_last_reconcile_success_timestamp_seconds",
			Help: "Unix timestamp of the last successful reconcile.",
		}),
		ready: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "promtag_ready",
			Help: "1 if the last reconcile succeeded, else 0.",
		}),
		managed: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "promtag_managed_containers",
			Help: "Containers currently declaring rules.",
		}),
		ruleFiles: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "promtag_managed_rule_files",
			Help: "Rule files currently written to the rules directory.",
		}),
		invalidRules: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "promtag_invalid_rules",
			Help: "Rule groups that currently fail validation.",
		}),
		reloads: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "promtag_reloads_total",
			Help: "Total Prometheus reload attempts by result.",
		}, []string{"result"}),
		watchRestarts: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "promtag_watch_restarts_total",
			Help: "Total Docker event-stream resubscriptions.",
		}),
	}
	reg.MustRegister(m.reconciles, m.reconcileTime, m.lastReconcile, m.lastSuccess,
		m.ready, m.managed, m.ruleFiles, m.invalidRules, m.reloads, m.watchRestarts)
	reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "promtag_build_info",
		Help:        "Build information, always 1.",
		ConstLabels: prometheus.Labels{"version": version},
	}, func() float64 { return 1 }))
	return m
}

// ObserveReconcile records the outcome and duration of one reconcile run.
func (m *Metrics) ObserveReconcile(success bool, d time.Duration) {
	m.reconciles.WithLabelValues(result(success)).Inc()
	m.reconcileTime.Observe(d.Seconds())
	m.lastReconcile.SetToCurrentTime()
	if success {
		m.lastSuccess.SetToCurrentTime()
		m.ready.Set(1)
	} else {
		m.ready.Set(0)
	}
}

// SetState records the current desired-state counts.
func (m *Metrics) SetState(managed, files, invalid int) {
	m.managed.Set(float64(managed))
	m.ruleFiles.Set(float64(files))
	m.invalidRules.Set(float64(invalid))
}

// ObserveReload records a Prometheus reload attempt.
func (m *Metrics) ObserveReload(success bool) {
	m.reloads.WithLabelValues(result(success)).Inc()
}

// ObserveWatchRestart counts a Docker event-stream resubscription.
func (m *Metrics) ObserveWatchRestart() {
	m.watchRestarts.Inc()
}

func result(success bool) string {
	if success {
		return "success"
	}
	return "error"
}

func (m *Metrics) handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	return mux
}

// Serve runs the /metrics and /healthz endpoints until ctx is cancelled. A
// blank addr disables the server.
func (m *Metrics) Serve(ctx context.Context, addr string, logger *slog.Logger) {
	if addr == "" {
		return
	}
	srv := &http.Server{Addr: addr, Handler: m.handler(), ReadHeaderTimeout: 5 * time.Second}

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()

	go func() {
		logger.Info("serving metrics", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("metrics server failed", "error", err)
		}
	}()
}
