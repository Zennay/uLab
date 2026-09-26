#!/bin/sh
set -eu

. examples/postgresql-reference/scripts/common.sh

wait_for_postgres target
assert_postgres_version target "$ULAB_TARGET_VERSION"

[ "$(psql_scalar target "SELECT COUNT(*) FROM accounts")" = "2" ]

join_count=$(psql_scalar target "SELECT COUNT(*) FROM events e JOIN accounts a ON a.id = e.account_id")
[ "$join_count" = "2" ] || {
  echo "foreign-key join count: expected 2, got $join_count" >&2
  exit 1
}

[ "$(psql_scalar target "SELECT note FROM accounts WHERE slug = 'alpha'")" = "héllo 🌍" ]
[ "$(psql_scalar target "SELECT amount::TEXT FROM accounts WHERE slug = 'beta'")" = "20.50" ]
[ "$(psql_scalar target "SELECT account_count || '|' || total_amount FROM account_totals")" = "2|30.75" ]

next_id=$(psql_scalar target "INSERT INTO accounts (slug, note, amount, created_at) VALUES ('gamma', 'created-after-upgrade', 1.00, '2026-03-04T05:06:07Z') RETURNING id")
[ "$next_id" = "3" ] || {
  echo "expected restored sequence to continue at 3, got $next_id" >&2
  exit 1
}

[ "$(psql_scalar target "SELECT COUNT(*) FROM accounts")" = "3" ]
echo "verified PostgreSQL relational state and sequence continuity"
