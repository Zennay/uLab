#!/bin/sh
set -eu

. examples/postgresql-reference/scripts/common.sh

wait_for_postgres target
docker compose exec -T target psql -X -v ON_ERROR_STOP=1   -U "$POSTGRES_USER" -d "$POSTGRES_DB"   -c "DELETE FROM events WHERE label = 'foreign-key-survives'" >/dev/null

echo "intentionally removed one migrated invariant" >&2
sh examples/postgresql-reference/scripts/verify.sh
