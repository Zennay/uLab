#!/bin/sh
set -eu

. examples/gitea-reference/scripts/common.sh

command -v curl >/dev/null 2>&1 || {
  echo "curl is required for the Gitea reference integration" >&2
  exit 1
}

export GITEA_IMAGE_TAG="$ULAB_SOURCE_VERSION"

docker compose pull server
docker compose up -d server
wait_for_gitea

docker compose exec -T --user git server \
  gitea admin user create \
    --username "$GITEA_TEST_USER" \
    --password "$GITEA_TEST_PASSWORD" \
    --email ulab@example.invalid \
    --admin \
    --must-change-password=false

api_post "/api/v1/user/repos" \
  "{\"name\":\"$GITEA_TEST_REPO\",\"auto_init\":true,\"private\":false}" >/dev/null

api_post "/api/v1/repos/$GITEA_TEST_USER/$GITEA_TEST_REPO/issues" \
  '{"title":"survives-upgrade","body":"seeded by the uLab Gitea reference integration"}' >/dev/null

api_get "/api/v1/repos/$GITEA_TEST_USER/$GITEA_TEST_REPO" |
  grep -Fq "\"name\":\"$GITEA_TEST_REPO\""

api_get "/api/v1/repos/$GITEA_TEST_USER/$GITEA_TEST_REPO/issues/1" |
  grep -Fq '"title":"survives-upgrade"'
