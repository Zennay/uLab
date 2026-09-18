#!/bin/sh
set -eu

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  echo "usage: sh scripts/release.sh <version>" >&2
  exit 2
fi

case "$VERSION" in
  *[!A-Za-z0-9._-]*)
    echo "version may only contain letters, numbers, dot, underscore and dash" >&2
    exit 2
    ;;
esac

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

if ! git diff --quiet || ! git diff --cached --quiet || [ -n "$(git status --porcelain --untracked-files=normal)" ]; then
  echo "refusing release from a dirty working tree" >&2
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "go is required" >&2
  exit 1
fi

COMMIT=$(git rev-parse --verify HEAD)
SOURCE_DATE_EPOCH=$(git show -s --format=%ct HEAD)
export SOURCE_DATE_EPOCH
export TZ=UTC
export CGO_ENABLED=0

rm -rf dist
mkdir -p dist

echo "==> testing"
go test ./...

echo "==> building $VERSION from $COMMIT"
TARGETS="
linux amd64
linux arm64
darwin amd64
darwin arm64
windows amd64
windows arm64
"

echo "$TARGETS" | while read -r GOOS_VALUE GOARCH_VALUE; do
  [ -n "$GOOS_VALUE" ] || continue

  EXT=""
  if [ "$GOOS_VALUE" = "windows" ]; then
    EXT=".exe"
  fi

  OUTPUT="dist/ulab_${VERSION}_${GOOS_VALUE}_${GOARCH_VALUE}${EXT}"
  echo "    $GOOS_VALUE/$GOARCH_VALUE -> $OUTPUT"

  GOOS="$GOOS_VALUE" GOARCH="$GOARCH_VALUE" \
    go build \
      -trimpath \
      -buildvcs=false \
      -ldflags="-s -w" \
      -o "$OUTPUT" \
      ./cmd/ulab
done

cat > dist/RELEASE-METADATA.txt <<EOF
version=$VERSION
commit=$COMMIT
source_date_epoch=$SOURCE_DATE_EPOCH
go_version=$(go version | awk '{print $3}')
cgo_enabled=0
build_flags=-trimpath -buildvcs=false -ldflags=-s -w
targets=linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64,windows/arm64
EOF

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1"
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1"
  else
    echo "sha256sum or shasum is required" >&2
    exit 1
  fi
}

: > dist/SHA256SUMS
for FILE in \
  "dist/ulab_${VERSION}_linux_amd64" \
  "dist/ulab_${VERSION}_linux_arm64" \
  "dist/ulab_${VERSION}_darwin_amd64" \
  "dist/ulab_${VERSION}_darwin_arm64" \
  "dist/ulab_${VERSION}_windows_amd64.exe" \
  "dist/ulab_${VERSION}_windows_arm64.exe" \
  "dist/RELEASE-METADATA.txt"
do
  sha256_file "$FILE" | sed 's#  dist/#  #; s# \*dist/# *#' >> dist/SHA256SUMS
done

echo "==> verifying checksums"
sh scripts/verify-release.sh dist/SHA256SUMS

echo "==> release artifacts ready in dist/"
