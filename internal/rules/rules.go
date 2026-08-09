// Package rules validates rule definitions from container labels and renders
// them into Prometheus rule files.
//
// Validation is intentionally lightweight: it checks the structure a rule must
// have (alert/record, expr, durations, label names) without embedding the full
// Prometheus PromQL parser. A syntactically broken PromQL expression is caught
// by Prometheus itself at reload time and surfaced in the logs. This keeps the
// dependency footprint tiny, which is a deliberate design choice.
package rules

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

const header = "# Managed by promtag. Do not edit by hand.\n"

var (
	metricNameRe = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
	labelNameRe  = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	durationRe   = regexp.MustCompile(`^(0|(\d+(ms|[smhdwy]))+)$`)
	unsafeRe     = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)
)

type groupsDoc struct {
	Groups []group `yaml:"groups"`
}

type group struct {
	Name  string      `yaml:"name"`
	Rules []yaml.Node `yaml:"rules"`
}

// ruleCheck mirrors the fields we validate. The original yaml.Node is used for
// rendering so unknown fields and formatting are preserved faithfully.
type ruleCheck struct {
	Alert         string            `yaml:"alert"`
	Record        string            `yaml:"record"`
	Expr          string            `yaml:"expr"`
	For           string            `yaml:"for"`
	KeepFiringFor string            `yaml:"keep_firing_for"`
	Labels        map[string]string `yaml:"labels"`
	Annotations   map[string]string `yaml:"annotations"`
}

// Render validates the rules given in a label value and renders a Prometheus
// rule file containing a single group. All rules must be valid; otherwise the
// whole group is rejected so a container never emits a partially applied file.
func Render(groupName, labelValue string) ([]byte, error) {
	var nodes []yaml.Node
	if err := yaml.Unmarshal([]byte(labelValue), &nodes); err != nil {
		return nil, fmt.Errorf("rules must be a YAML list: %w", err)
	}
	if len(nodes) == 0 {
		return nil, errors.New("no rules defined")
	}
	for i := range nodes {
		if err := validate(&nodes[i]); err != nil {
			return nil, fmt.Errorf("rule %d: %w", i+1, err)
		}
	}

	doc := groupsDoc{Groups: []group{{Name: groupName, Rules: nodes}}}
	var buf bytes.Buffer
	buf.WriteString(header)
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func validate(n *yaml.Node) error {
	var r ruleCheck
	if err := n.Decode(&r); err != nil {
		return fmt.Errorf("invalid structure: %w", err)
	}

	switch {
	case r.Alert == "" && r.Record == "":
		return errors.New("must set either 'alert' or 'record'")
	case r.Alert != "" && r.Record != "":
		return errors.New("must set only one of 'alert' or 'record'")
	}
	if r.Expr == "" {
		return errors.New("'expr' is required")
	}

	if r.Record != "" {
		if !metricNameRe.MatchString(r.Record) {
			return fmt.Errorf("invalid record metric name %q", r.Record)
		}
		if r.For != "" || r.KeepFiringFor != "" || len(r.Annotations) > 0 {
			return errors.New("recording rules cannot use 'for', 'keep_firing_for' or 'annotations'")
		}
	}

	for _, d := range []struct{ name, value string }{
		{"for", r.For},
		{"keep_firing_for", r.KeepFiringFor},
	} {
		if d.value != "" && !durationRe.MatchString(d.value) {
			return fmt.Errorf("invalid %s duration %q", d.name, d.value)
		}
	}

	for _, m := range []map[string]string{r.Labels, r.Annotations} {
		for k := range m {
			if !labelNameRe.MatchString(k) {
				return fmt.Errorf("invalid label name %q", k)
			}
		}
	}
	return nil
}

// Filename returns the rule file promtag manages for a container's rule group. A
// singleton group (name == container) maps to "<container>.yml"; a named group
// maps to "<container>.<group>.yml", keeping files unique on disk.
func Filename(container, group string) string {
	base := container
	if group != container {
		base = container + "." + group
	}
	name := unsafeRe.ReplaceAllString(base, "_")
	if name == "" {
		name = "unnamed"
	}
	return name + ".yml"
}
