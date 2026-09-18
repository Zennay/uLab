# Releasing uLab locally

uLab's release flow is intentionally local and zero-cost. It does not require GitHub-hosted Actions, cloud runners or paid CI minutes.

## Requirements

- Git
- Go 1.23.2 or the exact Go version used for the release you want to reproduce
- `sha256sum` or `shasum`

Docker is not required to package the CLI. Docker is only needed for the Docker Compose integration fixture.

## Validate before packaging

Run the local acceptance path from the exact revision you intend to package:

```sh
sh scripts/validate-local.sh
```

This validates unit tests, static analysis, bounded matrix execution, a passing three-source release candidate, an intentionally failing path, the non-zero compatibility gate and persistent evidence output without using GitHub-hosted runners.

## First public release preflight

Before creating a public tag, complete [the first release checklist](FIRST_RELEASE_CHECKLIST.md) and run:

```sh
sh scripts/release-preflight.sh v0.1.0
```

The preflight requires a clean `main`, no existing tag with the same version, the required OSS documentation, a committed `LICENSE`, a versioned changelog section and a resolved name/license status in the README. It then runs the deterministic local validation path.

This means the current repository is expected to fail public-release preflight until the human-owned name and license choices are resolved.

## Create a release candidate

Start from the exact commit you want to publish and make sure the working tree is clean.

```sh
git checkout <commit-or-tag>
sh scripts/release.sh v0.1.0
```

The script first runs `go test ./...`. If the tests fail, no release is produced.

A successful run creates six binaries in `dist/`:

- Linux: amd64, arm64
- macOS: amd64, arm64
- Windows: amd64, arm64

It also writes:

- `dist/RELEASE-METADATA.txt` with source commit, source timestamp, Go version and build flags;
- `dist/SHA256SUMS` covering all six binaries and the metadata file.

## Verify a release

```sh
sh scripts/verify-release.sh dist/SHA256SUMS
```

The verifier checks every SHA-256 digest and rejects an incomplete platform matrix.

## Reproducibility boundary

The build uses:

- `CGO_ENABLED=0`
- `-trimpath`
- `-buildvcs=false`
- fixed target tuples
- the source commit timestamp as `SOURCE_DATE_EPOCH`

For byte-for-byte comparison, rebuild the same commit with the same Go patch version. The Go toolchain version is recorded in `RELEASE-METADATA.txt` so a release can be independently reconstructed and compared.

The release script refuses to run from a dirty working tree. This keeps the published checksum set tied to a specific Git commit instead of uncommitted local source.
