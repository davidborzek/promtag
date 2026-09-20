# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.1](https://github.com/davidborzek/promtag/compare/v0.1.0...v0.1.1) (2026-09-20)


### Bug Fixes

* bump go toolchain to 1.26.7, add nix dev shell ([#14](https://github.com/davidborzek/promtag/issues/14)) ([c553803](https://github.com/davidborzek/promtag/commit/c553803d2186c49029ca857cd3afd03c8bc83b82))
* **deps:** update module github.com/moby/moby/api to v1.56.0 ([#13](https://github.com/davidborzek/promtag/issues/13)) ([abe6985](https://github.com/davidborzek/promtag/commit/abe6985c4f123d48466f1b6e383a1abc7b77f2ae))
* **deps:** update module github.com/urfave/cli/v3 to v3.12.0 ([#11](https://github.com/davidborzek/promtag/issues/11)) ([e22bd7c](https://github.com/davidborzek/promtag/commit/e22bd7c6460d27047db7fb9ce7175a4138ac0899))

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
