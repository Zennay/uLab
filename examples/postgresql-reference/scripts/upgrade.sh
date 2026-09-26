#!/bin/sh
set -eu

. examples/postgresql-reference/scripts/common.sh

docker compose stop source
docker compose pull target
docker compose up -d target
wait_for_postgres target
assert_postgres_version target "$ULAB_TARGET_VERSION"

docker compose exec -T target   pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB"   --exit-on-error --no-owner --no-privileges /transfer/upgrade.dump

echo "restored PostgreSQL dump into $ULAB_TARGET_VERSION"
