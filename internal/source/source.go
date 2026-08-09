// Package source discovers rule-carrying containers from the Docker engine and
// watches for lifecycle changes.
package source

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

// Group is one named Prometheus rule group declared on a container.
type Group struct {
	Name  string // rule group name
	Rules string // embedded YAML list of rules (the label value)
}

// Container is a running container that declares one or more rule groups.
type Container struct {
	Name   string
	Groups []Group
}

// Source lists and watches containers via the Docker API.
type Source struct {
	cli    client.APIClient
	prefix string
	log    *slog.Logger
}

// New returns a Source using the given Docker client and label prefix.
func New(cli client.APIClient, prefix string, log *slog.Logger) *Source {
	return &Source{cli: cli, prefix: prefix, log: log}
}

// List returns every running container that declares rule groups via labels. A
// `<prefix>.rules` label is a singleton group named after the container; each
// `<prefix>.rules.<name>` label adds an independent group named <name>.
func (s *Source) List(ctx context.Context) ([]Container, error) {
	summaries, err := s.cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, err
	}
	rulesKey := s.prefix + ".rules"
	namedPrefix := rulesKey + "."
	out := make([]Container, 0, len(summaries))
	for _, c := range summaries {
		name := containerName(c)
		var groups []Group
		for k, v := range c.Labels {
			if strings.TrimSpace(v) == "" {
				continue
			}
			switch {
			case k == rulesKey:
				groups = append(groups, Group{Name: name, Rules: v})
			case strings.HasPrefix(k, namedPrefix):
				if gn := k[len(namedPrefix):]; gn != "" {
					groups = append(groups, Group{Name: gn, Rules: v})
				}
			}
		}
		if len(groups) == 0 {
			continue
		}
		sort.Slice(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })
		out = append(out, Container{Name: name, Groups: groups})
	}
	return out, nil
}

func containerName(c container.Summary) string {
	if len(c.Names) > 0 {
		return strings.TrimPrefix(c.Names[0], "/")
	}
	if len(c.ID) >= 12 {
		return c.ID[:12]
	}
	return c.ID
}

// Watch emits a signal whenever a relevant container event occurs. It
// resubscribes automatically on stream errors and stops when ctx is cancelled.
func (s *Source) Watch(ctx context.Context, onRestart func()) <-chan struct{} {
	out := make(chan struct{}, 1)
	go func() {
		defer close(out)
		f := filters.NewArgs(
			filters.Arg("type", "container"),
			filters.Arg("event", "start"),
			filters.Arg("event", "die"),
			filters.Arg("event", "destroy"),
			filters.Arg("event", "update"),
		)
		for ctx.Err() == nil {
			msgs, errs := s.cli.Events(ctx, events.ListOptions{Filters: f})
		stream:
			for {
				select {
				case <-ctx.Done():
					return
				case <-msgs:
					signal(out)
				case err := <-errs:
					if ctx.Err() == nil && err != nil {
						s.log.Warn("docker event stream interrupted, resubscribing", "error", err)
						if onRestart != nil {
							onRestart()
						}
						time.Sleep(time.Second)
					}
					break stream
				}
			}
		}
	}()
	return out
}

func signal(ch chan<- struct{}) {
	select {
	case ch <- struct{}{}:
	default: // a reconcile is already pending
	}
}
