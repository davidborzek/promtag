// Package reconcile turns the set of rule-carrying containers into rule files
// on disk and reloads Prometheus when anything changes.
package reconcile

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/davidborzek/promtag/internal/rules"
	"github.com/davidborzek/promtag/internal/source"
)

type lister interface {
	List(ctx context.Context) ([]source.Container, error)
}

type reloader interface {
	Trigger(ctx context.Context) error
}

// Result summarizes one reconcile run. It is dependency-free so callers (e.g.
// metrics) can record it without coupling this package to instrumentation.
type Result struct {
	Managed  int  // containers declaring rules
	Invalid  int  // rule groups whose validation failed
	Files    int  // rule files written (desired state)
	Changed  bool // on-disk state changed
	Reloaded bool // a Prometheus reload was attempted
	ReloadOK bool // and it succeeded
}

// Reconciler owns the rules directory and keeps it in sync with the containers.
type Reconciler struct {
	src      lister
	reloader reloader
	dir      string
	log      *slog.Logger
}

// New returns a Reconciler.
func New(src lister, rl reloader, dir string, log *slog.Logger) *Reconciler {
	return &Reconciler{src: src, reloader: rl, dir: dir, log: log}
}

// Run performs one full reconcile: render every container's rules, sync the
// directory, and reload Prometheus if the on-disk state changed.
func (r *Reconciler) Run(ctx context.Context) (Result, error) {
	var res Result

	containers, err := r.src.List(ctx)
	if err != nil {
		return res, err
	}
	res.Managed = len(containers)

	desired := make(map[string][]byte, len(containers))
	for _, c := range containers {
		for _, g := range c.Groups {
			content, err := rules.Render(g.Name, g.Rules)
			if err != nil {
				res.Invalid++
				r.log.Warn("ignoring invalid rules", "container", c.Name, "group", g.Name, "error", err)
				continue
			}
			desired[rules.Filename(c.Name, g.Name)] = content
		}
	}
	res.Files = len(desired)

	changed, err := sync(r.dir, desired)
	if err != nil {
		return res, err
	}
	res.Changed = changed
	if !changed {
		return res, nil
	}

	r.log.Info("rules changed, reloading prometheus", "groups", res.Files)
	res.Reloaded = true
	if err := r.reloader.Trigger(ctx); err != nil {
		// Non-fatal: a bad PromQL expression makes Prometheus reject the whole
		// reload. Surface its message; the next reconcile retries.
		r.log.Error("prometheus reload failed", "error", err)
	} else {
		res.ReloadOK = true
	}
	return res, nil
}

// sync makes dir contain exactly the desired files. promtag owns dir, so any
// *.yml not in the desired set is removed. Returns whether anything changed.
func sync(dir string, desired map[string][]byte) (bool, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}

	changed := false
	present := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yml" {
			continue
		}
		present[e.Name()] = struct{}{}
		if _, keep := desired[e.Name()]; !keep {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				return changed, err
			}
			changed = true
		}
	}

	for name, content := range desired {
		path := filepath.Join(dir, name)
		if _, ok := present[name]; ok {
			if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, content) {
				continue
			}
		}
		if err := writeFile(path, content); err != nil {
			return changed, err
		}
		changed = true
	}
	return changed, nil
}

func writeFile(path string, content []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
