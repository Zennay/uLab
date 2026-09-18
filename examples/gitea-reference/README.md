# Gitea external reference integration

This directory is the first external-OSS integration candidate for uLab. It is deliberately implemented as project-owned hooks and Compose configuration; no Gitea-specific behavior is added to the uLab core.

## Why Gitea

Gitea matches the M3 selection criteria:

- public OSS with a long release history;
- official Docker and Docker Compose installation path;
- stateful application with a persistent data volume;
- SQLite can be used for a self-contained local test;
- Gitea documents that startup checks the database schema and automatically performs required migrations;
- the REST API supports Basic authentication, repository creation and issue operations, so the fixture can seed and verify meaningful application state without production credentials.

The reference currently checks two historical source releases, `1.26.0` and `1.26.4`, upgrading each to `1.27.3`.

## What the hooks do

For each source path:

1. start the real source Gitea container with a fresh isolated uLab Compose project;
2. wait for `/api/healthz`;
3. create a local admin user as Gitea's container `git` user;
4. create a repository and an issue through the Gitea REST API;
5. replace the running container with the target image while keeping the same `/data` volume;
6. allow Gitea's real startup migration path to run;
7. verify the target reports version `1.27.3`;
8. verify the user, repository and issue still exist;
9. let the uLab Docker runner remove the isolated Compose project and volume during cleanup.

This is intentionally application-specific code. uLab only supplies orchestration, version variables, isolation, evidence and cleanup.

## Requirements

- Docker with Docker Compose v2
- `curl`
- access to `docker.gitea.com`

The integration binds Gitea to `127.0.0.1:3300`, so it runs sequentially with `--jobs 1`.

## One-command runtime proof

On a Docker-capable host, run:

```sh
sh scripts/validate-gitea-reference.sh
```

That command builds the exact current uLab revision, runs both real source upgrades, verifies the resulting evidence, runs the deliberate failure variant and checks that no uLab-owned Compose containers, volumes or networks remain.

For manual inspection, the passing matrix is:

```sh
go run ./cmd/ulab test \
  --jobs 1 \
  --config examples/gitea-reference/ulab.json
```

The deliberate failure variant first performs the real verification and then fails the project assertion:

```sh
go run ./cmd/ulab test \
  --jobs 1 \
  --config examples/gitea-reference/ulab-broken.json
```

## Evidence status

The integration contract, runtime-proof harness and hook scripts are covered by local static validation. A successful run of `sh scripts/validate-gitea-reference.sh` on a real Docker host is still required before M3 can be called VERIFIED.

## Upstream references

- Gitea Docker install: https://docs.gitea.com/installation/install-with-docker/
- Gitea upgrades/migrations: https://docs.gitea.com/installation/upgrade-from-gitea/
- Gitea CLI admin user creation: https://docs.gitea.com/administration/command-line/
- Gitea API create repository: https://docs.gitea.com/api/operations/create-current-user-repo/
