#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

command -v go >/dev/null 2>&1 || { echo "go is required" >&2; exit 1; }
command -v git >/dev/null 2>&1 || { echo "git is required" >&2; exit 1; }

if [ -n "$(git status --porcelain --untracked-files=normal)" ]; then
  echo "prepare the usability session from a clean checkout" >&2
  exit 1
fi

SESSION_DIR=${1:-.ulab/m5-usability}
if [ -e "$SESSION_DIR" ]; then
  echo "session directory already exists: $SESSION_DIR" >&2
  echo "choose a new path so previous evidence is preserved" >&2
  exit 1
fi

mkdir -p "$SESSION_DIR/bin" "$SESSION_DIR/evidence" "$SESSION_DIR/state"
SESSION_DIR=$(CDPATH= cd -- "$SESSION_DIR" && pwd)
BIN="$SESSION_DIR/bin/ulab"
EVIDENCE="$SESSION_DIR/evidence"
STATE="$SESSION_DIR/state"
CONFIG="$SESSION_DIR/config.json"
COMMIT=$(git rev-parse --verify HEAD)

cat > "$CONFIG" <<'JSON'
{
  "runner": {"type": "process"},
  "versions": {"from": ["v1.8.0", "v1.9.0", "v2.0.0"], "to": "v3.0.0"},
  "setup": {"command": "sh scripts/fixtures/local-validation-hook.sh setup"},
  "upgrade": {"command": "sh scripts/fixtures/local-validation-hook.sh upgrade"},
  "verify": {"command": "sh scripts/fixtures/local-validation-hook.sh verify"},
  "policy": {"require_all_paths": true}
}
JSON

go build -trimpath -buildvcs=false \
  -ldflags="-s -w -X main.version=m5-usability -X main.commit=$COMMIT" \
  -o "$BIN" ./cmd/ulab

set +e
ULAB_VALIDATION_FAIL_SOURCE=v1.9.0 \
ULAB_VALIDATION_STATE_DIR="$STATE" \
"$BIN" test --config "$CONFIG" --jobs 2 --evidence-root "$EVIDENCE" \
  > "$SESSION_DIR/test.out" 2> "$SESSION_DIR/test.err"
RC=$?
set -e

if [ "$RC" -ne 1 ]; then
  echo "expected the prepared matrix to fail closed with exit 1, got $RC" >&2
  exit 1
fi

grep -Eq '^v1\.8\.0[[:space:]]+v3\.0\.0[[:space:]]+passed$' "$SESSION_DIR/test.out"
grep -Eq '^v1\.9\.0[[:space:]]+v3\.0\.0[[:space:]]+failed$' "$SESSION_DIR/test.out"
grep -Eq '^v2\.0\.0[[:space:]]+v3\.0\.0[[:space:]]+passed$' "$SESSION_DIR/test.out"
grep -Fq 'compatibility policy failed' "$SESSION_DIR/test.err"

BUNDLES=$(find "$EVIDENCE" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')
if [ "$BUNDLES" -ne 1 ]; then
  echo "expected exactly one prepared evidence bundle, found $BUNDLES" >&2
  exit 1
fi

for bundle in "$EVIDENCE"/*; do
  test -f "$bundle/config.json"
  test -f "$bundle/result.json"
  test -f "$bundle/metadata.json"
  grep -Fq "\"tool_commit\": \"$COMMIT\"" "$bundle/metadata.json"
done

echo "M5 usability evidence prepared."
echo "Exact uLab commit: $COMMIT"
echo "Evidence root: $EVIDENCE"
echo "Start the read-only UI with:"
printf "  '%s' view --evidence-root '%s'\n" "$BIN" "$EVIDENCE"
echo "Then open http://127.0.0.1:8080/"
