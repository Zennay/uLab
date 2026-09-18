# uLab

uLab tests software upgrade paths without taking ownership of an application's upgrade logic.

A project defines how to set up old state, perform its upgrade and verify correctness. uLab handles the surrounding execution: isolated runner lifecycle, multiple source versions, failure handling and machine-readable evidence.

> uLab owns orchestration. Projects own their upgrade logic.

## Current state

The current prototype supports:

- process and Docker Compose runners;
- `setup -> upgrade -> verify` hooks;
- cleanup on success and failure;
- multiple source versions against one target;
- bounded concurrent paths with `--jobs`;
- JSON evidence and a non-zero compatibility gate;
- persistent evidence bundles with config hashes and reproduction metadata;
- a stateful Docker fixture with both passing and destructive upgrade cases.

## Project status

uLab is an early prototype. The public configuration and evidence formats may change while external integrations are being validated.

`uLab` is still a working name. A project license has also not been selected yet, so do not assume reuse rights beyond applicable law until a license file is added.

Project guides:

- [Installation](docs/INSTALL.md)
- [Contributing](CONTRIBUTING.md)
- [Security](SECURITY.md)
- [Releasing](docs/RELEASING.md)
- [First release checklist](docs/FIRST_RELEASE_CHECKLIST.md)
- [Changelog](CHANGELOG.md)
- [External validation protocol](docs/EXTERNAL_VALIDATION.md)

## Try the included fixture

Docker is required for the example.

```sh
go run ./cmd/ulab test \
  --jobs 2 \
  --config examples/stateful-upgrade/ulab.json
```

The fixture checks three source versions against `v2`. A second config deliberately removes required state and should fail:

```sh
go run ./cmd/ulab test \
  --jobs 2 \
  --config examples/stateful-upgrade/ulab-broken.json
```

## Evidence and reproduction

Each invocation of `ulab test` writes a bundle under `.ulab/runs/<invocation-id>/` containing:

- the exact config bytes used for the run;
- `result.json`;
- `metadata.json` with a SHA-256 config hash, working directory, source/target matrix, concurrency, tool build identity and a reproduction command.

The normal `--json-out` file remains available for integrations that only need the machine-readable release gate. Use `--evidence-root` to place persistent bundles elsewhere.

Release builds expose their embedded identity with:

```sh
ulab version
```

## Configuration

`versions.from` accepts a single version or a list:

```json
{
  "runner": {
    "type": "docker-compose",
    "compose_file": "compose.yaml"
  },
  "versions": {
    "from": ["v1.8.0", "v1.9.0", "v2.0.0"],
    "to": "v3.0.0"
  },
  "setup": { "command": "./ulab/setup.sh" },
  "upgrade": { "command": "./ulab/upgrade.sh" },
  "verify": { "command": "./ulab/verify.sh" },
  "policy": { "require_all_paths": true }
}
```

Each hook receives:

```text
ULAB_RUN_ID
ULAB_SOURCE_VERSION
ULAB_TARGET_VERSION
```

For Docker Compose runs, uLab scopes the Compose project name to the source/target path **and the unique run id**, then performs cleanup after each path. This prevents identical upgrade paths in separate or concurrent runs from sharing Compose resources.

## Validate locally

The canonical validation path consumes no GitHub-hosted runner minutes or credits:

```sh
sh scripts/validate-local.sh
```

It runs the Go test suite and `go vet`, builds a revision-stamped binary, exercises three source versions with `--jobs 2`, verifies the terminal compatibility matrix, then deliberately fails one path and checks the non-zero release gate plus persistent evidence output. Docker is not required for this deterministic core validation.

## Local release artifacts

uLab can be packaged without GitHub-hosted CI or paid runners:

```sh
sh scripts/release.sh v0.1.0
sh scripts/verify-release.sh dist/SHA256SUMS
```

The release flow runs the Go test suite first, cross-compiles six platform binaries, embeds the release version and source commit, and writes SHA-256 checksums plus release provenance metadata. See [docs/RELEASING.md](docs/RELEASING.md).

A separate public-release preflight intentionally blocks tagging while required release decisions such as the project license remain unresolved.

## What uLab does not decide

uLab does not infer whether application data is correct after an upgrade. The project owns those assertions. uLab's job is to run them consistently across supported paths and preserve the resulting evidence.
