//go:build e2e

// Package e2e drives the whole promtag binary end to end: a labelled demo
// container is read from the real Docker daemon, its rules are rendered and
// synced, and a live Prometheus reload makes the rule queryable via the API.
// Everything runs against a real Docker daemon (testcontainers).
// Run with `go test -tags e2e ./test/e2e/`.
package e2e

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const promConfig = `global:
  scrape_interval: 15s
rule_files:
  - /etc/prometheus/rules/*.yml
`

const ruleLabel = `- alert: MyAppDown
  expr: up == 0
  for: 2m
  labels: {severity: critical}
  annotations: {summary: "myapp is down"}
`

func TestEndToEnd(t *testing.T) {
	ctx := context.Background()

	rulesDir := t.TempDir()
	if err := os.Chmod(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	bin := buildPromtag(t)
	promBase := startPrometheus(t, ctx, rulesDir)
	startDemo(t, ctx)

	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(),
		"PROMTAG_RULES_DIR="+rulesDir,
		"PROMTAG_RELOAD_URL="+promBase+"/-/reload",
		"PROMTAG_LABEL_PREFIX=promtag",
		"PROMTAG_RESYNC_INTERVAL=2s",
		"PROMTAG_DEBOUNCE_DELAY=200ms",
		"PROMTAG_METRICS_ADDR=",
		"PROMTAG_LOG_LEVEL=info",
	)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start promtag: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	deadline := time.Now().Add(30 * time.Second)
	for {
		if rulesAPIHasAlert(promBase, "MyAppDown") {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("MyAppDown never appeared in Prometheus /api/v1/rules")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func buildPromtag(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "promtag")
	out, err := exec.Command("go", "build", "-o", bin, "github.com/davidborzek/promtag/cmd/promtag").CombinedOutput()
	if err != nil {
		t.Fatalf("build promtag: %v\n%s", err, out)
	}
	return bin
}

func startPrometheus(t *testing.T, ctx context.Context, rulesDir string) string {
	t.Helper()
	req := tc.ContainerRequest{
		Image:        "prom/prometheus:v3.5.0",
		ExposedPorts: []string{"9090/tcp"},
		Cmd: []string{
			"--config.file=/etc/prometheus/prometheus.yml",
			"--web.enable-lifecycle",
		},
		Files: []tc.ContainerFile{{
			Reader:            strings.NewReader(promConfig),
			ContainerFilePath: "/etc/prometheus/prometheus.yml",
			FileMode:          0o644,
		}},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Binds = append(hc.Binds, rulesDir+":/etc/prometheus/rules:ro")
		},
		WaitingFor: wait.ForHTTP("/-/ready").WithPort("9090/tcp").WithStartupTimeout(60 * time.Second),
	}
	ctr, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: req, Started: true})
	tc.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatalf("start prometheus: %v", err)
	}
	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := ctr.MappedPort(ctx, "9090/tcp")
	if err != nil {
		t.Fatal(err)
	}
	return "http://" + host + ":" + port.Port()
}

func startDemo(t *testing.T, ctx context.Context) {
	t.Helper()
	req := tc.ContainerRequest{
		Image: "alpine:3",
		Cmd:   []string{"sleep", "infinity"},
		Labels: map[string]string{
			"promtag.rules.e2e": ruleLabel,
		},
	}
	ctr, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: req, Started: true})
	tc.CleanupContainer(t, ctr)
	if err != nil {
		t.Fatalf("start demo container: %v", err)
	}
}

func rulesAPIHasAlert(base, alert string) bool {
	resp, err := http.Get(base + "/api/v1/rules")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var parsed struct {
		Data struct {
			Groups []struct {
				Rules []struct {
					Name string `json:"name"`
				} `json:"rules"`
			} `json:"groups"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return false
	}
	for _, g := range parsed.Data.Groups {
		for _, r := range g.Rules {
			if r.Name == alert {
				return true
			}
		}
	}
	return false
}
