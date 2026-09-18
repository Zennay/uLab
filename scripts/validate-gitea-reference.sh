#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

for COMMAND in git go docker curl grep find; do
  command -v "$COMMAND" >/dev/null 2>&1 || {
    echo "$COMMAND is required for the Gitea runtime proof" >&2
    exit 1
  }
done

docker compose version >/dev/null

WORK=$(mktemp -d "${TMPDIR:-/tmp}/ulab-gitea-proof.XXXXXX")
BIN="$WORK/ulab"
SESSION=$(date -u +%Y%m%dT%H%M%SZ)-$$
EVIDENCE="${ULAB_GITEA_EVIDENCE_ROOT:-.ulab/gitea-reference-runs/$SESSION}"

cleanup() {
  for PROJECT in \
    ulab-1-26-0-to-1-27-3 \
    ulab-1-26-4-to-1-27-3
  do
    docker compose \
      -f examples/gitea-reference/compose.yaml \
      -p "$PROJECT" \
      down --volumes --remove-orphans >/dev/null 2>&1 || true
  done
  rm -rf "$WORK"
}
trap cleanup EXIT INT TERM

COMMIT=$(git rev-parse --verify HEAD)

echo "==> build revision-stamped uLab binary"
go build -trimpath -buildvcs=false \
  -ldflags="-s -w -X main.version=m3-gitea-validation -X main.commit=$COMMIT" \
  -o "$BIN" ./cmd/ulab

echo "==> run real Gitea upgrade matrix"
"$BIN" test \
  --jobs 1 \
  --config examples/gitea-reference/ulab.json \
  --json-out "$WORK/pass.json" \
  --evidence-root "$EVIDENCE" > "$WORK/pass.out"

grep -Eq '^1\.26\.0[[:space:]]+1\.27\.3[[:space:]]+passed$' "$WORK/pass.out"
grep -Eq '^1\.26\.4[[:space:]]+1\.27\.3[[:space:]]+passed$' "$WORK/pass.out"
grep -Fq '"status": "passed"' "$WORK/pass.json"

echo "==> run deliberate Gitea assertion failure"
set +e
"$BIN" test \
  --jobs 1 \
  --config examples/gitea-reference/ulab-broken.json \
  --json-out "$WORK/fail.json" \
  --evidence-root "$EVIDENCE" > "$WORK/fail.out" 2> "$WORK/fail.err"
FAIL_RC=$?
set -e

if [ "$FAIL_RC" -ne 1 ]; then
  echo "expected deliberate Gitea failure to exit 1, got $FAIL_RC" >&2
  exit 1
fi

grep -Eq '^1\.26\.4[[:space:]]+1\.27\.3[[:space:]]+failed$' "$WORK/fail.out"
grep -Fq 'compatibility policy failed' "$WORK/fail.err"
grep -Fq '"status": "failed"' "$WORK/fail.json"

BUNDLES=$(find "$EVIDENCE" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')
if [ "$BUNDLES" -ne 2 ]; then
  echo "expected 2 Gitea evidence bundles, found $BUNDLES" >&2
  exit 1
fi

for BUNDLE in "$EVIDENCE"/*; do
  test -f "$BUNDLE/config.json"
  test -f "$BUNDLE/result.json"
  test -f "$BUNDLE/metadata.json"
  grep -Fq '"config_sha256": "sha256:' "$BUNDLE/metadata.json"
  grep -Fq "\"tool_commit\": \"$COMMIT\"" "$BUNDLE/metadata.json"
done

echo "==> verify Compose cleanup"
for PROJECT in \
  ulab-1-26-0-to-1-27-3 \
  ulab-1-26-4-to-1-27-3
do
  if docker ps -aq --filter "label=com.docker.compose.project=$PROJECT" | grep -q .; then
    echo "leftover container for Compose project $PROJECT" >&2
    exit 1
  fi
  if docker volume ls -q --filter "label=com.docker.compose.project=$PROJECT" | grep -q .; then
    echo "leftover volume for Compose project $PROJECT" >&2
    exit 1
  fi
  if docker network ls -q --filter "label=com.docker.compose.project=$PROJECT" | grep -q .; then
    echo "leftover network for Compose project $PROJECT" >&2
    exit 1
  fi
done

echo "Gitea runtime proof passed"
echo "evidence preserved at: $EVIDENCE"
