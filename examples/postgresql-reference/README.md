# PostgreSQL contrasting reference integration

This M4 reference tests whether uLab's existing public hook contract can express a stateful major-version migration whose transformation is operator-owned rather than application-owned.

The matrix is PostgreSQL `16.15` and `17.11` to `18.6`, using only official `postgres` images. No PostgreSQL-specific behavior belongs in the uLab core.

## Upgrade shape

For each source path the project-owned hooks:

1. start the source release in an isolated Compose project;
2. assert the exact live source version;
3. seed relational state with keys, a foreign key, Unicode text, numeric data, an index and a view;
4. write a logical custom-format dump to a transfer volume;
5. stop the source service;
6. start a fresh PostgreSQL 18.6 target on a separate data volume;
7. restore the dump into the target;
8. assert the exact target version and relational invariants;
9. insert one post-migration row and verify identity-sequence continuity;
10. rely on the normal uLab Docker runner cleanup to remove source, target and transfer volumes.

The target mounts `/var/lib/postgresql`, matching the PostgreSQL 18 official-image storage layout, while the older source releases use `/var/lib/postgresql/data`.

## Deliberate failure

`ulab-broken.json` performs the same real migration from PostgreSQL 17.11 to 18.6. Its verifier then removes one restored relational invariant before running the ordinary verification script. The expected result is a compatibility failure after a successful migration, not a setup failure.

## Local validation

Static and unit validation:

```sh
go test ./...
go vet ./...
dash -n examples/postgresql-reference/scripts/*.sh
bash -n examples/postgresql-reference/scripts/*.sh
```

Runtime proof requires a reachable Docker daemon. Run the passing matrix with:

```sh
go run ./cmd/ulab test --jobs 1 \
  --config examples/postgresql-reference/ulab.json
```

Run the deliberate failure with:

```sh
go run ./cmd/ulab test --jobs 1 \
  --config examples/postgresql-reference/ulab-broken.json
```

This integration is a technical falsification test. It does not by itself prove maintainer adoption, onboarding quality or toil reduction. Those remain separate external-evidence questions.
