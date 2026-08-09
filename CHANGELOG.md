# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-08-09

Initial release.

### Added

- Event-driven reconcile over the Docker API (debounced, with periodic resync).
- Rules from container labels: `promtag.rules` (singleton group) and
  `promtag.rules.<name>` (named groups); each is validated and rendered to its
  own file.
- Owns the rules directory — orphaned files are pruned to match running containers.
- Reloads Prometheus via the `--web.enable-lifecycle` endpoint after changes.
- `promtag_*` metrics on `/metrics` and a `/healthz` liveness probe.
- Configuration via `PROMTAG_*` environment variables.
- Multi-arch image (`linux/amd64`, `linux/arm64`) on `ghcr.io/davidborzek/promtag`.
