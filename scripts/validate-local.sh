#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

command -v go >/dev/null 2>&1 || { echo "go is required" >&2; exit 1; }
command -v git >/dev/null 2>&1 || { echo "git is required" >&2; exit 1; }

WORK=$(mktemp -d "${TMPDIR:-/tmp}/ulab-validation.XXXXXX")
trap 'rm -rf "$WORK"' EXIT INT TERM

COMMIT=$(git rev-parse --verify HEAD)
BIN="$WORK/ulab"
EVIDENCE="$WORK/evidence"
export ULAB_VALIDATION_STATE_DIR="$WORK/state"

echo "==> unit tests"
go test ./...

echo "==> static analysis"
go vet ./...

echo "==> build acceptance binary"
go build -trimpath -buildvcs=false \
  -ldflags="-s -w -X main.version=local-validation -X main.commit=$COMMIT" \
  -o "$BIN" ./cmd/ulab

cat > "$WORK/config.json" <<'JSON'
{
  "runner": {"type": "process"},
  "versions": {"from": ["v1.8.0", "v1.9.0", "v2.0.0"], "to": "v3.0.0"},
  "setup": {"command": "sh scripts/fixtures/local-validation-hook.sh setup"},
  "upgrade": {"command": "sh scripts/fixtures/local-validation-hook.sh upgrade"},
  "verify": {"command": "sh scripts/fixtures/local-validation-hook.sh verify"},
  "policy": {"require_all_paths": true}
}
JSON

echo "==> passing 3-path matrix"
"$BIN" test --config "$WORK/config.json" --jobs 2 \
  --json-out "$WORK/pass.json" --evidence-root "$EVIDENCE" > "$WORK/pass.out"
grep -Eq '^v1\.8\.0[[:space:]]+v3\.0\.0[[:space:]]+passed$' "$WORK/pass.out"
grep -Eq '^v1\.9\.0[[:space:]]+v3\.0\.0[[:space:]]+passed$' "$WORK/pass.out"
grep -Eq '^v2\.0\.0[[:space:]]+v3\.0\.0[[:space:]]+passed$' "$WORK/pass.out"

echo "==> deliberately failing one path"
set +e
ULAB_VALIDATION_FAIL_SOURCE=v1.9.0 "$BIN" test --config "$WORK/config.json" --jobs 2 \
  --json-out "$WORK/fail.json" --evidence-root "$EVIDENCE" > "$WORK/fail.out" 2> "$WORK/fail.err"
FAIL_RC=$?
set -e
if [ "$FAIL_RC" -ne 1 ]; then
  echo "expected compatibility failure exit 1, got $FAIL_RC" >&2
  exit 1
fi
grep -Eq '^v1\.8\.0[[:space:]]+v3\.0\.0[[:space:]]+passed$' "$WORK/fail.out"
grep -Eq '^v1\.9\.0[[:space:]]+v3\.0\.0[[:space:]]+failed$' "$WORK/fail.out"
grep -Eq '^v2\.0\.0[[:space:]]+v3\.0\.0[[:space:]]+passed$' "$WORK/fail.out"
grep -Fq 'compatibility policy failed' "$WORK/fail.err"

BUNDLES=$(find "$EVIDENCE" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')
if [ "$BUNDLES" -ne 2 ]; then
  echo "expected 2 evidence bundles, found $BUNDLES" >&2
  exit 1
fi
for bundle in "$EVIDENCE"/*; do
  test -f "$bundle/config.json"
  test -f "$bundle/result.json"
  test -f "$bundle/metadata.json"
  grep -Fq '"config_sha256": "sha256:' "$bundle/metadata.json"
  grep -Fq "\"tool_commit\": \"$COMMIT\"" "$bundle/metadata.json"
done

echo "local validation passed"
