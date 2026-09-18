# Contributing to uLab

uLab is still validating its core release-compatibility model. Contributions are welcome when they strengthen that model without turning project-specific upgrade semantics into uLab-owned logic.

## Development setup

Requirements:

- Git
- Go 1.23.2 or a compatible newer Go toolchain
- Docker with Docker Compose only for Docker-backed fixture work

Run the complete zero-credit validation path before proposing a change:

```sh
sh scripts/validate-local.sh
```

That command runs the unit tests, `go vet`, a three-source compatibility matrix, an intentional failure case, the release gate and evidence checks.

## Architecture boundary

Keep this rule intact:

> uLab owns orchestration. Projects own their upgrade logic.

A change should not teach the core how a particular database, framework or application performs its upgrade unless repeated external evidence shows that a reusable abstraction is needed.

The current core responsibilities are:

- execution planning;
- runner lifecycle;
- source-to-target matrices;
- bounded concurrency and run isolation;
- evidence capture;
- cleanup;
- release-gate reporting.

## Pull requests

Keep changes focused and explain:

- the problem being solved;
- the observable behavior before and after;
- the success path tested;
- the failure path tested;
- any change to the config or evidence contract.

Update documentation when behavior changes. Do not add GitHub-hosted runner workflows; this repository uses local-first validation and must not consume hosted Actions minutes.

For larger architecture changes, open an issue first so the contract can be discussed before implementation.

## Security reports

Do not use a normal pull request or public issue to disclose an exploitable vulnerability. Follow [SECURITY.md](SECURITY.md).
