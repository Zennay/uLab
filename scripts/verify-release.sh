#!/bin/sh
set -eu

MANIFEST="${1:-dist/SHA256SUMS}"
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

if [ ! -f "$MANIFEST" ]; then
  echo "checksum manifest not found: $MANIFEST" >&2
  exit 1
fi

DIR=$(dirname -- "$MANIFEST")
BASE=$(basename -- "$MANIFEST")

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$DIR" && sha256sum -c "$BASE")
elif command -v shasum >/dev/null 2>&1; then
  (cd "$DIR" && shasum -a 256 -c "$BASE")
else
  echo "sha256sum or shasum is required" >&2
  exit 1
fi

BINARY_COUNT=$(grep -Ec '  ulab_.*_(linux|darwin|windows)_(amd64|arm64)(\.exe)?$' "$MANIFEST" || true)
if [ "$BINARY_COUNT" -ne 6 ]; then
  echo "expected 6 platform binaries in $MANIFEST, found $BINARY_COUNT" >&2
  exit 1
fi

if ! grep -Eq '  RELEASE-METADATA\.txt$' "$MANIFEST"; then
  echo "release metadata is missing from $MANIFEST" >&2
  exit 1
fi

echo "release manifest verified"
