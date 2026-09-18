# uLab

uLab is an experiment in reusable upgrade-path verification for stateful software.

The project keeps application-specific upgrade logic in the application. uLab is responsible for the surrounding execution: planning a run, invoking setup/upgrade/verify hooks, stopping on failure, and recording evidence.

The current milestone is intentionally small. It proves the execution contract before Docker orchestration, version matrices, or a web UI are added.

## Current commands

```sh
go run ./cmd/ulab init
go run ./cmd/ulab test
```

`ulab init` creates a starter `ulab.json`. The config format is temporary while the core execution contract is being validated; the intended public config format is YAML once the repository is wired to its normal dependency toolchain.

A run exposes the versions to project hooks as:

```text
ULAB_SOURCE_VERSION
ULAB_TARGET_VERSION
```

The runner executes phases in this order:

```text
setup -> upgrade -> verify
```

It stops at the first failed phase and writes a structured result to `ulab-result.json`.

## Project boundary

uLab owns orchestration. Projects own their upgrade logic.

That means uLab should eventually handle version matrices, isolated environments, evidence, retries, cleanup, CI integration and reporting. A project remains responsible for what valid setup, upgrade and verification mean for that application.
