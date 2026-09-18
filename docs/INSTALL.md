# Installing uLab

uLab is pre-release software. There is no tagged stable release yet, so the supported installation path is currently a source build from a specific Git revision.

## Requirements

- Git
- Go 1.23.2 or a compatible newer Go toolchain
- Docker with Docker Compose only when the project being tested uses the Docker Compose runner

## Build from source

```sh
git clone https://github.com/Zennay/uLab.git
cd uLab
go build -trimpath -o bin/ulab ./cmd/ulab
./bin/ulab version
```

On Windows, use an `.exe` output name:

```powershell
go build -trimpath -o bin/ulab.exe ./cmd/ulab
.\bin\ulab.exe version
```

A source build reports `dev (unknown)` unless version and commit values are injected at build time. Official release artifacts, once the first release is tagged, use the release script to embed both values.

## Validate the checkout

Before relying on a checkout for release decisions, run:

```sh
sh scripts/validate-local.sh
```

This path is intentionally local. It does not require GitHub-hosted Actions or paid runner minutes.

## Docker requirement

The process runner does not require Docker. A configuration with:

```json
{
  "runner": {
    "type": "docker-compose",
    "compose_file": "compose.yaml"
  }
}
```

requires a working `docker compose` command on the host.

## Release artifacts

The repository already contains the local packaging path used to create cross-platform binaries and checksum manifests:

```sh
sh scripts/release.sh v0.1.0
sh scripts/verify-release.sh dist/SHA256SUMS
```

See [RELEASING.md](RELEASING.md) for the reproducibility boundary and release procedure. Until a tag is actually published, these commands produce local release candidates rather than an official uLab release.
