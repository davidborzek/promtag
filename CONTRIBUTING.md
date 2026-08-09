# Contributing

Thanks for your interest in promtag! Contributions are welcome.

## Development

promtag is a standard Go module (Go 1.26+). No code generation or extra
tooling is required.

```sh
go build ./...      # build
go vet ./...        # vet
go test ./...       # unit tests
```

Formatting is enforced in CI — run `gofmt -w .` before committing.

### Integration and e2e tests

Two tiers exercise real infrastructure via Docker (behind build tags, skipped
automatically when Docker is unavailable):

```sh
# reconcile ↔ Prometheus: render, sync the rules dir, reload
go test -tags integration ./internal/reconcile/

# full stack: the promtag binary + a labelled demo container -> Prometheus
go test -tags e2e ./test/e2e/
```

### Extending promtag

The architecture is deliberately small — promtag watches the Docker API for
containers carrying a `promtag.rules` label, renders each into a Prometheus
alerting/recording rule file, syncs the rules directory, and reloads Prometheus.
See the [README](README.md) and the existing implementations under
`internal/source`, `internal/rules`, `internal/reconcile`, and
`internal/reload`.

## Pull requests

- Keep changes focused; one logical change per PR.
- Use [Conventional Commits](https://www.conventionalcommits.org/) for commit
  messages (`feat:`, `fix:`, `docs:`, `refactor:`, `ci:` …).
- Add or update tests for behavioural changes.
- Make sure `gofmt`, `go vet`, and `go test ./...` pass.

## Reporting issues

Use the issue templates. For security-sensitive reports, see
[SECURITY.md](SECURITY.md).

## Releases

Releases are automated — no manual tagging:

- **[release-please](https://github.com/googleapis/release-please)** watches
  `main` and, from the Conventional Commit history, maintains a "release PR"
  that bumps the version and updates `CHANGELOG.md`. Merging it creates the tag
  and the GitHub release.
- **[goreleaser](https://goreleaser.com/)** then builds the binaries and the
  multi-arch (`amd64`/`arm64`) image, pushes it to
  `ghcr.io/davidborzek/promtag`, and attaches archives + checksums to the
  release — in the same workflow run (hanging off release-please's output, so no
  PAT is needed).
