#!/bin/sh
set -eu

sh examples/gitea-reference/scripts/verify.sh

echo "intentional reference-integration failure after real Gitea verification" >&2
exit 9
