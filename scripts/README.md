# Cloud Scripts

This directory is the script index for `wt-media-cloud`. It states the
classification and the placement rule only — no per-file walkthrough, no runtime
parameters.

## Classification

| Purpose | What the category answers | Scripts (file names only) |
| --- | --- | --- |
| Operations | Bring Cloud up / confirm it is up / stop it | see [`../bin/control.sh`](../bin/control.sh) |
| Development | Rebuild / regenerate / migrate after a change | `bootstrap.sh`, `build.sh`, `migrate.sh`, `test.sh` |
| Verification | Prove something holds, or does not | `test.sh`, `verify-health.sh`, `verify/test-control.sh` |
| Shared | Required by the scripts above; not an entry point itself | `local-env.sh` |

Multiple membership is a property, not a mistake: `test.sh` builds and asserts,
so it is both; `verify-health.sh` is a verification script that happens to start
and stop a throwaway server, so it spans Verification and Operations while the
Operations entry point proper is [`../bin/control.sh`](../bin/control.sh);
`local-env.sh` is sourced by that entry point and by `migrate.sh`, and is never
invoked on its own.

The Operations row is the one that moved: process start/stop and liveness live in
`bin/`, not here.

## Where a new script goes

- A new **development** script → `scripts/dev/`.
- A new **verification / acceptance** script → `scripts/verify/`.
- **Start/stop and health checks always go to [`bin/`](../bin/)**; do not add them
  under `scripts/`.
- **Existing scripts do not move.** The flat files in this directory are the
  historical landing spots and are left alone. `dev/` and `verify/` take scripts
  written from here on — they are not a target shape to migrate the current files
  into.

`scripts/verify/test-control.sh` is the first real file under either
subdirectory; `scripts/dev/` still holds only its `.gitkeep`. It asserts the
*entry* properties of [`../bin/control.sh`](../bin/control.sh) — tracked,
executable in the git index and on disk, directly invocable, the four verbs
listed, unknown verb refused with exit 2 — and starts nothing, so `test.sh`
runs it after `go test` and `npm test`. Every line it prints is prefixed
`[control]`: `scripts/verify_m3_acceptance.py` parses `test.sh`'s log for
`^ok\s`, `^FAIL`, `Test Files N passed (N)` and `Tests N passed (N)`, and an
unprefixed `ok`/`FAIL` from a shell check would be counted as a Go package
result or a vitest summary.

## What this file does not own

Per-file facts are not here. Directory facts and the no-scan zones belong to
[`../DIRECTORY_MAP.md`](../DIRECTORY_MAP.md); the server listen address and the
other runtime connection values belong to `config/`; the probe address belongs to
[`local-env.sh`](local-env.sh) and is described in [`../README.md`](../README.md).

**Numeric runtime parameters — ports, addresses, credentials, timeout and
retention defaults — have their single landing point in configuration files.**
Neither this file nor [`../bin/control.sh`](../bin/control.sh) restates them:
`bin/control.sh` dispatches verbs only.

The script-level operational overrides stay script-level and are named here only:
`WT_MEDIA_CLOUD_PID_FILE`, `WT_MEDIA_CLOUD_LOG_FILE`, `WT_MEDIA_CLOUD_BINARY_FILE`,
`WT_MEDIA_CLOUD_GOPATH`, and `WT_MEDIA_CLOUD_HTTP_ADDR`.

## Process entrypoints

`cmd/server` starts only the API Server (`serverResourcePlan()` has no scheduler or
worker step). The Discovery Scheduler and Discovery Worker are separate CMD
entrypoints, `cmd/discovery-scheduler` and `cmd/discovery-worker`, and start
nothing else: the Scheduler only creates pending `crawl_tasks`; the Worker
exclusively claims and executes them. Both take no flags and loop on the intervals
in `config/scheduler/scheduler.toml` (`discovery_interval`, `worker_interval`). To
run one due-strategy scan without waiting for the interval, call the
admin-controlled `POST /api/v1/discovery-scheduler/run-due`.

Starting `cmd/server` alone performs no discovery execution: pending `crawl_tasks`
stay pending until a Worker process claims them.
