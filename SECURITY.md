# Security policy

## Reporting a vulnerability

Please **do not** open a public issue for security vulnerabilities.

Instead, report them privately via GitHub's
[security advisories](https://github.com/davidborzek/promtag/security/advisories/new)
("Report a vulnerability"). You will receive a response as soon as possible, and
disclosure will be coordinated with you.

## Scope

promtag connects to the Docker API to watch container labels and writes
Prometheus rule files, then triggers a Prometheus reload via
`--web.enable-lifecycle`. Restrict access to the Docker socket to the minimum
required, keep the reload endpoint reachable only from promtag, and treat the
managed rules directory as controlled by promtag.
