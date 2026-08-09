package reconcile

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/davidborzek/promtag/internal/source"
)

func TestSync(t *testing.T) {
	dir := t.TempDir()

	// Pre-existing file that promtag should later remove as an orphan.
	orphan := filepath.Join(dir, "old.yml")
	if err := os.WriteFile(orphan, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	desired := map[string][]byte{
		"a.yml": []byte("groups: [a]\n"),
		"b.yml": []byte("groups: [b]\n"),
	}

	changed, err := sync(dir, desired)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on first sync")
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Error("orphan file was not removed")
	}
	for name, want := range desired {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if string(got) != string(want) {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	// Second sync with identical desired state must be a no-op.
	changed, err = sync(dir, desired)
	if err != nil {
		t.Fatalf("sync (2): %v", err)
	}
	if changed {
		t.Error("expected changed=false on identical re-sync")
	}

	// Removing a container removes its file.
	changed, err = sync(dir, map[string][]byte{"a.yml": []byte("groups: [a]\n")})
	if err != nil {
		t.Fatalf("sync (3): %v", err)
	}
	if !changed {
		t.Error("expected changed=true after removing b.yml")
	}
	if _, err := os.Stat(filepath.Join(dir, "b.yml")); !os.IsNotExist(err) {
		t.Error("b.yml should have been removed")
	}
}

func TestSyncIgnoresNonYAML(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(keep, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := sync(dir, map[string][]byte{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Error("non-yml file must not be touched")
	}
}

type fakeLister struct{ containers []source.Container }

func (f fakeLister) List(context.Context) ([]source.Container, error) {
	return f.containers, nil
}

type fakeReloader struct {
	calls int
	fail  bool
}

func (f *fakeReloader) Trigger(context.Context) error {
	f.calls++
	if f.fail {
		return errors.New("reload rejected")
	}
	return nil
}

func TestRunResult(t *testing.T) {
	dir := t.TempDir()
	src := fakeLister{containers: []source.Container{
		{Name: "sonarr", Groups: []source.Group{{Name: "sonarr", Rules: "- alert: A\n  expr: up"}}},
		{Name: "radarr", Groups: []source.Group{{Name: "radarr", Rules: "- alert: B\n  expr: up"}}},
		{Name: "broken", Groups: []source.Group{{Name: "broken", Rules: "- alert: C"}}}, // missing expr
	}}
	rl := &fakeReloader{}
	rec := New(src, rl, dir, testLogger())

	res, err := rec.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Managed != 3 || res.Invalid != 1 || res.Files != 2 {
		t.Errorf("counts = managed %d invalid %d files %d; want 3/1/2", res.Managed, res.Invalid, res.Files)
	}
	if !res.Changed || !res.Reloaded || !res.ReloadOK {
		t.Errorf("flags = changed %v reloaded %v ok %v; want all true", res.Changed, res.Reloaded, res.ReloadOK)
	}
	if rl.calls != 1 {
		t.Errorf("reload calls = %d, want 1", rl.calls)
	}

	// Nothing changed -> no reload.
	res, err = rec.Run(context.Background())
	if err != nil {
		t.Fatalf("run (2): %v", err)
	}
	if res.Changed || res.Reloaded {
		t.Errorf("second run should be a no-op, got changed %v reloaded %v", res.Changed, res.Reloaded)
	}
	if rl.calls != 1 {
		t.Errorf("reload calls = %d after no-op, want 1", rl.calls)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
