#!/bin/sh
set -eu
APP_VERSION="$ULAB_TARGET_VERSION" docker compose build app
APP_VERSION="$ULAB_TARGET_VERSION" docker compose run --rm app upgrade
