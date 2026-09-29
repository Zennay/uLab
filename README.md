# uLab

uLab is a compatibility lab for stateful upgrades. A project describes how to set up a source-version dataset, upgrade it to a target version, and verify that the upgraded state remains correct. uLab orchestrates those steps across every requested source version and preserves machine-readable evidence for each run.

## Status

This repository contains the first runnable uLab prototype:

- process and Docker Compose runners;
- setup, upgrade, and verify hooks;
- multiple source versions against one target version;
- bounded concurrent upgrade paths;
- RAM-aware automatic concurrency for VPS runners;
- JSON result output and persistent evidence bundles;
- a small evidence viewer.

The project is intentionally runner-oriented: uLab owns the lifecycle and evidence, while the project under test owns the setup, upgrade, and verification commands.

## Quick start

Create a starter configuration:

```sh
go run ./cmd/ulab init --config ulab.json
```

Run a process-backed matrix:

```sh
go run ./cmd/ulab test \
  --jobs 2 \
  --config examples/stateful-upgrade/ulab.json
```

For a long-running VPS runner, let uLab choose the number of concurrent paths from the live RAM headroom:

```sh
go run ./cmd/ulab test \
  --jobs auto \
  --config examples/stateful-upgrade/ulab.json
```

Auto mode uses the number of CPUs minus one as its upper bound, then admits each new path only when Linux `MemAvailable` has room for the configured safety reserve and estimated per-test memory. The defaults keep 2 GiB available for the rest of the VPS and budget 1 GiB per active path. If another service (for example FTMO, HaxLab, or zCloud) uses more RAM, uLab automatically starts fewer new paths; it never stops a path that is already running. On systems without `/proc/meminfo`, auto mode falls back to its CPU-based upper bound.

The RAM assumptions can be tuned without code changes:

```sh
go run ./cmd/ulab test \
  --jobs auto \
  --memory-reserve-mb 3072 \
  --memory-per-job-mb 1536 \
  --config ulab.json
```

The same behavior can be enabled in a configuration used by an autonomous service:

```json
{
  "policy": {
    "require_all_paths": true,
    "auto_concurrency": true,
    "memory_reserve_mb": 2048,
    "memory_per_job_mb": 1024
  }
}
```

An explicit `--jobs N` always overrides `auto_concurrency` and keeps deterministic fixed parallelism.

Each Docker Compose run scopes its project name to the source/target pair and a unique run identifier, so parallel paths do not share containers or volumes. Cleanup runs even after a failed hook.

View stored evidence:

```sh
go run ./cmd/ulab view --evidence-root .ulab/runs
```

Then open `http://127.0.0.1:8080`.

## Configuration

```json
{
  "runner": {
    "type": "docker-compose",
    "compose_file": "examples/stateful-upgrade/compose.yaml"
  },
  "versions": {
    "from": ["v1", "v1.1", "v1.2"],
    "to": "v2"
  },
  "setup": {
    "command": "./scripts/setup.sh"
  },
  "upgrade": {
    "command": "./scripts/upgrade.sh"
  },
  "verify": {
    "command": "./scripts/verify.sh"
  },
  "policy": {
    "require_all_paths": true
  }
}
```

`versions.from` accepts either one string or an array. The runner type defaults to `process`. Docker Compose requires `runner.compose_file`.

## Evidence

Every test invocation writes a bundle under `.ulab/runs/<invocation-id>/` containing:

- the exact configuration snapshot;
- the full result snapshot;
- metadata including source/target versions, tool version, and reproduction command;
- the configured concurrency mode and RAM budget.

The bundle is published atomically so interrupted runs do not leave a partially published result.

## Local validation

Run:

```sh
scripts/validate-local.sh
```

The validation script runs shell checks, Go tests, `go vet`, a build, and a three-path acceptance matrix.
