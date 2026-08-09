---
name: Bug report
about: Report a problem with promtag
labels: bug
---

**What happened**

A clear description of the bug.

**Expected behaviour**

What you expected to happen instead.

**Configuration**

- Relevant `PROMTAG_*` variables (rules dir, reload URL, label prefix,
  intervals):
- The container `promtag.rules` label that triggered the behaviour:
- `/metrics` output if relevant (e.g. `promtag_invalid_rules`,
  `promtag_reloads_total`):

**Logs**

Run with `PROMTAG_LOG_LEVEL=debug` and paste the relevant output.

**Environment**

- promtag image tag / version:
- Docker version:
- Prometheus version:
