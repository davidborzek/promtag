// Package reload triggers a Prometheus configuration reload.
package reload

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Reloader posts to the Prometheus reload endpoint.
type Reloader struct {
	url string
	cli *http.Client
}

// New returns a Reloader for the given Prometheus reload URL.
func New(url string) *Reloader {
	return &Reloader{url: url, cli: &http.Client{Timeout: 10 * time.Second}}
}

// Trigger requests a configuration reload. A non-2xx response is returned as an
// error including Prometheus' message, which names the offending rule file.
func (r *Reloader) Trigger(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, nil)
	if err != nil {
		return err
	}
	resp, err := r.cli.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("reload returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}
