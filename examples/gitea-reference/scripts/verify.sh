#!/bin/sh
set -eu

. examples/gitea-reference/scripts/common.sh

wait_for_gitea
assert_gitea_version "$ULAB_TARGET_VERSION"

api_get "/api/v1/user" |
  grep -Fq "\"login\":\"$GITEA_TEST_USER\""

api_get "/api/v1/repos/$GITEA_TEST_USER/$GITEA_TEST_REPO" |
  grep -Fq "\"name\":\"$GITEA_TEST_REPO\""

api_get "/api/v1/repos/$GITEA_TEST_USER/$GITEA_TEST_REPO/issues/1" |
  grep -Fq '"title":"survives-upgrade"'
