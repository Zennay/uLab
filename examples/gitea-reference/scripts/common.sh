#!/bin/sh
set -eu

GITEA_BASE_URL="${GITEA_BASE_URL:-http://127.0.0.1:3300}"
GITEA_TEST_USER="${GITEA_TEST_USER:-ulab}"
GITEA_TEST_PASSWORD="${GITEA_TEST_PASSWORD:-uLab-Test-2026-Strong}"
GITEA_TEST_REPO="${GITEA_TEST_REPO:-upgrade-proof}"

wait_for_gitea() {
  attempts=0
  while [ "$attempts" -lt 90 ]; do
    if curl -fsS "$GITEA_BASE_URL/api/healthz" >/dev/null 2>&1; then
      return 0
    fi
    attempts=$((attempts + 1))
    sleep 1
  done

  echo "Gitea did not become healthy at $GITEA_BASE_URL" >&2
  docker compose logs server >&2 || true
  return 1
}

assert_gitea_version() {
  expected_version="$1"
  version_json=$(curl -fsS "$GITEA_BASE_URL/api/v1/version")
  if ! printf '%s' "$version_json" | grep -Fq "\"version\":\"$expected_version\""; then
    echo "expected live Gitea version $expected_version" >&2
    echo "version endpoint returned: $version_json" >&2
    return 1
  fi
}

api_get() {
  path="$1"
  curl -fsS -u "$GITEA_TEST_USER:$GITEA_TEST_PASSWORD" "$GITEA_BASE_URL$path"
}

api_post() {
  path="$1"
  payload="$2"
  curl -fsS     -u "$GITEA_TEST_USER:$GITEA_TEST_PASSWORD"     -H 'Content-Type: application/json'     -d "$payload"     "$GITEA_BASE_URL$path"
}
