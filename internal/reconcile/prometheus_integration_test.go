//go:build integration

// Integration test against a real Prometheus in Docker: a labelled container is
// rendered into a rule file, synced to a bind-mounted rules directory, and a
// live reload is triggered. Prometheus must then serve the rule via its API.
package reconcile

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/davidborzek/promtag/internal/reload"
	"github.com/davidborzek/promtag/internal/source"
)

const promConfig = `global:
  scrape_interval: 15s
rule_files:
  - /etc/prometheus/rules/*.yml
`

// promStubLister feeds a fixed set of containers into the reconciler.
type promStubLister struct{ containers []source.Container }

func (s promStubLister) List(context.Context) ([]source.Container, error) {
	return s.containers, nil
}

func TestReconcileReloadsPrometheus(t *testing.T) {
	ctx := context.Background()

	// Host-side rules directory promtag owns; bind-mounted read-only into
	// Prometheus. World-traversable/readable so the container's user can read it.
	rulesDir := t.TempDir()
	if err := os.Chmod(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}

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

	ctr, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
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
	base := "http://" + host + ":" + port.Port()

	label := `- alert: MyAppDown
  expr: up == 0
  for: 2m
  labels: {severity: critical}
  annotations: {summary: "myapp is down"}
`
	lister := promStubLister{containers: []source.Container{
		{Name: "myapp", Groups: []source.Group{{Name: "myapp", Rules: label}}},
	}}
	rec := New(lister, reload.New(base+"/-/reload"), rulesDir, slog.New(slog.DiscardHandler))

	res, err := rec.Run(ctx)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.Managed != 1 || res.Files != 1 {
		t.Fatalf("managed=%d files=%d, want 1/1", res.Managed, res.Files)
	}
	if !res.Reloaded || !res.ReloadOK {
		t.Fatalf("reloaded=%v ok=%v, want true/true", res.Reloaded, res.ReloadOK)
	}

	// Prometheus applies rules asynchronously after reload; poll its API.
	deadline := time.Now().Add(20 * time.Second)
	for {
		if rulesAPIHasAlert(t, base, "MyAppDown") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("MyAppDown never appeared in Prometheus /api/v1/rules")
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func rulesAPIHasAlert(t *testing.T, base, alert string) bool {
	t.Helper()
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
