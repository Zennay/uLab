#!/bin/sh
set -eu

EVIDENCE="${1:-}"
if [ -z "$EVIDENCE" ]; then
  echo "usage: sh scripts/verify-gitea-proof.sh <evidence-session-dir>" >&2
  exit 2
fi

if [ ! -d "$EVIDENCE" ]; then
  echo "Gitea proof directory not found: $EVIDENCE" >&2
  exit 1
fi

for COMMAND in go grep find sed wc tr awk mktemp rm; do
  command -v "$COMMAND" >/dev/null 2>&1 || {
    echo "$COMMAND is required to verify Gitea proof evidence" >&2
    exit 1
  }
done

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1"
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1"
  else
    echo "sha256sum or shasum is required to verify Gitea proof evidence" >&2
    exit 1
  fi
}

json_status() {
  sed -n 's/^[[:space:]]*"status": "\([^"]*\)".*/\1/p' "$1" | sed -n '1p'
}

test -f "$EVIDENCE/SHA256SUMS"
test -f "$EVIDENCE/PROOF_COMPLETE"
test -f "$EVIDENCE/proof-summary.json"
test -f "$EVIDENCE/PROOF.txt"
test -f "$EVIDENCE/matrix-pass.json"
test -f "$EVIDENCE/deliberate-failure.json"

if command -v sha256sum >/dev/null 2>&1; then
  (cd "$EVIDENCE" && sha256sum -c SHA256SUMS >/dev/null)
else
  (cd "$EVIDENCE" && shasum -a 256 -c SHA256SUMS >/dev/null)
fi

EXPECTED_MANIFEST_SHA=$(sed -n 's/^manifest_sha256=//p' "$EVIDENCE/PROOF_COMPLETE")
ACTUAL_MANIFEST_SHA=$(sha256_file "$EVIDENCE/SHA256SUMS" | awk '{print $1}')
if [ -z "$EXPECTED_MANIFEST_SHA" ] || [ "$EXPECTED_MANIFEST_SHA" != "$ACTUAL_MANIFEST_SHA" ]; then
  echo "proof manifest digest does not match PROOF_COMPLETE" >&2
  exit 1
fi

COMMIT=$(sed -n 's/^tool_commit=//p' "$EVIDENCE/PROOF_COMPLETE")
if [ -z "$COMMIT" ]; then
  echo "tool commit missing from PROOF_COMPLETE" >&2
  exit 1
fi

grep -Fxq 'status=passed' "$EVIDENCE/PROOF_COMPLETE"
if [ "$(json_status "$EVIDENCE/proof-summary.json")" != "passed" ]; then
  echo "proof summary is not passed" >&2
  exit 1
fi

grep -Fq "\"tool_commit\": \"$COMMIT\"" "$EVIDENCE/proof-summary.json"
grep -Fq '"source_versions": ["1.26.0", "1.26.4"]' "$EVIDENCE/proof-summary.json"
grep -Fq '"target_version": "1.27.3"' "$EVIDENCE/proof-summary.json"
grep -Fq '"compatibility_gate": "detected"' "$EVIDENCE/proof-summary.json"
grep -Fq '"live_version_assertions": ["1.26.0", "1.26.4", "1.27.3"]' "$EVIDENCE/proof-summary.json"
grep -Fq '"containers": "clean"' "$EVIDENCE/proof-summary.json"
grep -Fq '"volumes": "clean"' "$EVIDENCE/proof-summary.json"
grep -Fq '"networks": "clean"' "$EVIDENCE/proof-summary.json"

WORK=$(mktemp -d "${TMPDIR:-/tmp}/ulab-verify-gitea-proof.XXXXXX")
trap 'rm -rf "$WORK"' EXIT INT TERM
cat > "$WORK/check-json.go" <<'EOF'
package main

import (
	"encoding/json"
	"os"
)

func main() {
	for _, path := range os.Args[1:] {
		data, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		var value any
		if err := json.Unmarshal(data, &value); err != nil {
			panic(err)
		}
	}
}
EOF
go run "$WORK/check-json.go" \
  "$EVIDENCE/proof-summary.json" \
  "$EVIDENCE/matrix-pass.json" \
  "$EVIDENCE/deliberate-failure.json" >/dev/null

if [ "$(json_status "$EVIDENCE/matrix-pass.json")" != "passed" ]; then
  echo "passing matrix artifact is not passed" >&2
  exit 1
fi
if [ "$(json_status "$EVIDENCE/deliberate-failure.json")" != "failed" ]; then
  echo "deliberate failure artifact is not failed" >&2
  exit 1
fi

BUNDLES=$(find "$EVIDENCE" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')
if [ "$BUNDLES" -ne 2 ]; then
  echo "expected 2 evidence bundles, found $BUNDLES" >&2
  exit 1
fi

PASSED_BUNDLES=0
FAILED_BUNDLES=0
for BUNDLE in "$EVIDENCE"/*; do
  [ -d "$BUNDLE" ] || continue
  test -f "$BUNDLE/config.json"
  test -f "$BUNDLE/result.json"
  test -f "$BUNDLE/metadata.json"
  grep -Fq "\"tool_commit\": \"$COMMIT\"" "$BUNDLE/metadata.json"

  EXPECTED_CONFIG_SHA=$(sed -n 's/.*"config_sha256": "sha256:\([^"]*\)".*/\1/p' "$BUNDLE/metadata.json")
  ACTUAL_CONFIG_SHA=$(sha256_file "$BUNDLE/config.json" | awk '{print $1}')
  if [ -z "$EXPECTED_CONFIG_SHA" ] || [ "$EXPECTED_CONFIG_SHA" != "$ACTUAL_CONFIG_SHA" ]; then
    echo "config hash mismatch in $BUNDLE" >&2
    exit 1
  fi

  STATUS=$(json_status "$BUNDLE/result.json")
  case "$STATUS" in
    passed) PASSED_BUNDLES=$((PASSED_BUNDLES + 1)) ;;
    failed) FAILED_BUNDLES=$((FAILED_BUNDLES + 1)) ;;
    *)
      echo "unexpected bundle status in $BUNDLE: $STATUS" >&2
      exit 1
      ;;
  esac
done

if [ "$PASSED_BUNDLES" -ne 1 ] || [ "$FAILED_BUNDLES" -ne 1 ]; then
  echo "expected one passed and one failed invocation bundle; got passed=$PASSED_BUNDLES failed=$FAILED_BUNDLES" >&2
  exit 1
fi

grep -Fq '"source_version": "1.26.0"' "$EVIDENCE/matrix-pass.json"
grep -Fq '"source_version": "1.26.4"' "$EVIDENCE/matrix-pass.json"
grep -Fq '"target_version": "1.27.3"' "$EVIDENCE/matrix-pass.json"
grep -Fq 'verified live Gitea version: 1.26.0' "$EVIDENCE/matrix-pass.json"
grep -Fq 'verified live Gitea version: 1.26.4' "$EVIDENCE/matrix-pass.json"
grep -Fq 'verified live Gitea version: 1.27.3' "$EVIDENCE/matrix-pass.json"
grep -Fq 'verified live Gitea version: 1.26.4' "$EVIDENCE/deliberate-failure.json"
grep -Fq 'verified live Gitea version: 1.27.3' "$EVIDENCE/deliberate-failure.json"

echo "Gitea proof evidence verified"
echo "tool commit: $COMMIT"
echo "manifest sha256: $ACTUAL_MANIFEST_SHA"
