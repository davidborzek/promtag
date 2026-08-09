package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestObserve(t *testing.T) {
	m := New("test")
	m.ObserveReconcile(true, 10*time.Millisecond)
	m.SetState(3, 3, 1)
	m.ObserveReload(true)

	if got := testutil.ToFloat64(m.reconciles.WithLabelValues("success")); got != 1 {
		t.Errorf("success reconciles = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.managed); got != 3 {
		t.Errorf("managed = %v, want 3", got)
	}
	if got := testutil.ToFloat64(m.invalidRules); got != 1 {
		t.Errorf("invalid rules = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.reloads.WithLabelValues("success")); got != 1 {
		t.Errorf("reload success = %v, want 1", got)
	}
}

func TestReadyAndWatchRestarts(t *testing.T) {
	m := New("test")

	m.ObserveReconcile(true, time.Millisecond)
	if testutil.ToFloat64(m.ready) != 1 {
		t.Error("ready should be 1 after a success")
	}
	if testutil.ToFloat64(m.lastSuccess) == 0 {
		t.Error("last-success timestamp must be set after a success")
	}

	m.ObserveReconcile(false, time.Millisecond)
	if testutil.ToFloat64(m.ready) != 0 {
		t.Error("ready should be 0 after an error")
	}

	m.ObserveWatchRestart()
	m.ObserveWatchRestart()
	if got := testutil.ToFloat64(m.watchRestarts); got != 2 {
		t.Errorf("watch restarts = %v, want 2", got)
	}
}

func TestHandler(t *testing.T) {
	m := New("test")
	m.ObserveReload(true)
	ts := httptest.NewServer(m.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("healthz = %d %q, want 200 \"ok\"", resp.StatusCode, body)
	}

	resp, err = http.Get(ts.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), "promtag_reloads_total") {
		t.Fatalf("metrics missing promtag_reloads_total:\n%s", body)
	}
}
