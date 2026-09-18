# Changelog

All notable user-facing changes to this project will be documented here.

The format is based on Keep a Changelog, with release versions added only when a release is actually prepared.

## [Unreleased]

### Added

- deterministic `setup -> upgrade -> verify` orchestration;
- process and Docker Compose runners;
- multi-source compatibility matrices with bounded concurrency;
- fail-closed release gating;
- persistent evidence bundles with config hashes and reproduction metadata;
- local zero-credit validation;
- reproducible cross-platform release candidate builds with SHA-256 manifests.

### Changed

- GitHub-hosted Actions were removed from the supported validation path; validation is local-first.

### Security

- documented the trusted-code boundary for project hooks and evidence handling.

> Before the first tag, move the relevant entries into a versioned section such as `## [v0.1.0] - YYYY-MM-DD`.
