#!/bin/sh
set -eu

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  echo "usage: sh scripts/release-preflight.sh <vMAJOR.MINOR.PATCH>" >&2
  exit 2
fi

printf '%s\n' "$VERSION" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$' || {
  echo "release version must look like v0.1.0 or v0.1.0-rc.1" >&2
  exit 2
}

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

command -v git >/dev/null 2>&1 || { echo "git is required" >&2; exit 1; }

if ! git diff --quiet || ! git diff --cached --quiet || [ -n "$(git status --porcelain --untracked-files=normal)" ]; then
  echo "release preflight requires a clean working tree" >&2
  exit 1
fi

BRANCH=$(git branch --show-current)
if [ "$BRANCH" != "main" ]; then
  echo "release preflight must run from main; current branch: ${BRANCH:-detached}" >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/$VERSION" >/dev/null 2>&1; then
  echo "tag already exists: $VERSION" >&2
  exit 1
fi

for FILE in README.md LICENSE SECURITY.md CONTRIBUTING.md CHANGELOG.md docs/INSTALL.md docs/RELEASING.md; do
  if [ ! -f "$FILE" ]; then
    echo "release gate missing required file: $FILE" >&2
    exit 1
  fi
done

if grep -Fq '`uLab` is still a working name.' README.md; then
  echo "release gate blocked: public product name is still marked as a working name" >&2
  exit 1
fi

if grep -Fq 'project license has also not been selected yet' README.md; then
  echo "release gate blocked: README still marks the license as undecided" >&2
  exit 1
fi

if ! grep -Fq "## [$VERSION]" CHANGELOG.md; then
  echo "release gate blocked: CHANGELOG.md has no section for $VERSION" >&2
  exit 1
fi

echo "==> deterministic validation"
sh scripts/validate-local.sh

echo "release preflight passed for $VERSION"
