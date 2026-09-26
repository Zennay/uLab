#!/bin/sh
set -eu

POSTGRES_USER="postgres"
POSTGRES_DB="ulab"

wait_for_postgres() {
  service="$1"
  attempts=0
  while [ "$attempts" -lt 90 ]; do
    if docker compose exec -T "$service" pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; then
      return 0
    fi
    attempts=$((attempts + 1))
    sleep 1
  done
  echo "PostgreSQL service $service did not become ready" >&2
  docker compose logs "$service" >&2 || true
  return 1
}

psql_scalar() {
  service="$1"
  sql="$2"
  docker compose exec -T "$service" psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "$sql"
}

version_num() {
  version="$1"
  major=${version%%.*}
  minor=${version#*.}
  printf '%s%04d\n' "$major" "$minor"
}

assert_postgres_version() {
  service="$1"
  expected="$2"
  expected_num=$(version_num "$expected")
  actual=$(psql_scalar "$service" "SHOW server_version_num")
  [ "$actual" = "$expected_num" ] || {
    echo "expected PostgreSQL $expected ($expected_num), got $actual" >&2
    return 1
  }
  echo "verified live PostgreSQL version: $expected"
}
