package rules

import (
	"strings"
	"testing"
)

func TestRenderValid(t *testing.T) {
	spec := `
- alert: SonarrDown
  expr: up{job="sonarr"} == 0
  for: 5m
  labels: {severity: critical}
  annotations: {summary: "Sonarr is down"}
- record: job:up:count
  expr: count(up)
`
	out, err := Render("sonarr", spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := string(out)
	for _, want := range []string{"groups:", "name: sonarr", "alert: SonarrDown", "record: job:up:count", "Managed by promtag"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered output missing %q\n---\n%s", want, got)
		}
	}
}

func TestRenderInvalid(t *testing.T) {
	cases := map[string]string{
		"not a list":        `alert: X`,
		"empty":             ``,
		"missing expr":      "- alert: X",
		"alert and record":  "- alert: X\n  record: Y\n  expr: up",
		"neither":           "- expr: up",
		"bad for duration":  "- alert: X\n  expr: up\n  for: 5minutes",
		"record with for":   "- record: my_metric\n  expr: up\n  for: 5m",
		"bad record name":   "- record: \"1bad\"\n  expr: up",
		"bad label name":    "- alert: X\n  expr: up\n  labels: {\"bad-name\": v}",
		"record annotation": "- record: my_metric\n  expr: up\n  annotations: {a: b}",
	}
	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Render("grp", spec); err == nil {
				t.Errorf("expected error for %q, got none", name)
			}
		})
	}
}

func TestFilename(t *testing.T) {
	cases := []struct {
		container, group, want string
	}{
		{"sonarr", "sonarr", "sonarr.yml"},
		{"my/app:1", "my/app:1", "my_app_1.yml"},
		{"proj-web-1", "proj-web-1", "proj-web-1.yml"},
		{"", "", "unnamed.yml"},
		{"web", "disk", "web.disk.yml"},
		{"web", "a/b", "web.a_b.yml"},
	}
	for _, c := range cases {
		if got := Filename(c.container, c.group); got != c.want {
			t.Errorf("Filename(%q, %q) = %q, want %q", c.container, c.group, got, c.want)
		}
	}
}
