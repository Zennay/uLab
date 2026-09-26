#!/bin/sh
set -eu

. examples/postgresql-reference/scripts/common.sh

docker compose pull source
docker compose up -d source
wait_for_postgres source
assert_postgres_version source "$ULAB_SOURCE_VERSION"

docker compose exec -T source psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<'SQL'
CREATE TABLE accounts (
  id BIGSERIAL PRIMARY KEY,
  slug TEXT UNIQUE NOT NULL,
  note TEXT NOT NULL,
  amount NUMERIC(10,2) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE events (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL REFERENCES accounts(id),
  label TEXT NOT NULL
);
CREATE INDEX events_account_id_idx ON events(account_id);

INSERT INTO accounts (slug, note, amount, created_at) VALUES
  ('alpha', 'héllo 🌍', 10.25, '2026-01-02T03:04:05Z'),
  ('beta', 'state survives', 20.50, '2026-02-03T04:05:06Z');

INSERT INTO events (account_id, label) VALUES
  (1, 'created-before-upgrade'),
  (2, 'foreign-key-survives');

CREATE VIEW account_totals AS
SELECT COUNT(*)::BIGINT AS account_count, SUM(amount)::NUMERIC(10,2) AS total_amount
FROM accounts;
SQL

docker compose exec -T source   pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB"   --format=custom --file=/transfer/upgrade.dump

docker compose exec -T source test -s /transfer/upgrade.dump
echo "created PostgreSQL logical dump for $ULAB_SOURCE_VERSION"
