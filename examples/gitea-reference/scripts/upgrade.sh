#!/bin/sh
set -eu

. examples/gitea-reference/scripts/common.sh

export GITEA_IMAGE_TAG="$ULAB_TARGET_VERSION"

docker compose pull server
docker compose up -d --force-recreate server
wait_for_gitea
