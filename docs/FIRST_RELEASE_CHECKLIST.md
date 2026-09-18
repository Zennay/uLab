# First release checklist

This checklist is for the first public, tagged uLab release. It intentionally separates automated proof from decisions that require the project owner's authority.

## Human decisions

These must be complete before tagging:

- [ ] public product name is final enough for the first release;
- [ ] collision/confusion risk has been reviewed;
- [ ] an open-source license has been selected and committed as `LICENSE`;
- [ ] the README no longer describes the name or license as undecided;
- [ ] the project owner approves creation of the public tag/release.

## Repository proof

- [ ] `main` is the intended release revision;
- [ ] working tree is clean;
- [ ] `sh scripts/validate-local.sh` passes;
- [ ] `CHANGELOG.md` contains a section for the intended version;
- [ ] installation, security, contribution and release docs match current behavior;
- [ ] no GitHub-hosted Actions/minutes are required.

The automated subset is checked with:

```sh
sh scripts/release-preflight.sh v0.1.0
```

The preflight deliberately fails while the name/license gates remain unresolved.

## Package candidate

After preflight is green:

```sh
sh scripts/release.sh v0.1.0
sh scripts/verify-release.sh dist/SHA256SUMS
```

Inspect `dist/RELEASE-METADATA.txt` and `dist/SHA256SUMS` before creating a tag.

## Tag/release boundary

Creating a Git tag or GitHub Release is a publication action. Do not do it merely because the build is green; it requires explicit owner approval.

After the tag exists, verify that the published artifacts correspond to the same commit and checksums produced by the local release candidate.
