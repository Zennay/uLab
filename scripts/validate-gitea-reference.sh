#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

for COMMAND in git go docker curl grep find cp sed uname date wc tr cat mkdir mktemp rm sort awk dirname basename; do
  command -v "$COMMAND" >/dev/null 2>&1 || {
    echo "$COMMAND is required for the Gitea runtime proof" >&2
    exit 1
  }
done

docker compose version >/dev/null
if ! docker info >/dev/null 2>&1; then
  echo "a reachable Docker daemon is required for the Gitea runtime proof" >&2
  exit 1
fi

WORK=$(mktemp -d "${TMPDIR:-/tmp}/ulab-gitea-proof.XXXXXX")
BIN="$WORK/ulab"
SESSION=$(date -u +%Y%m%dT%H%M%SZ)-$$
EVIDENCE="${ULAB_GITEA_EVIDENCE_ROOT:-.ulab/gitea-reference-runs/$SESSION}"

preserve_work_artifacts() {
  FOUND=0
  for FILE in pass.json pass.out fail.json fail.out fail.err; do
    if [ -f "$WORK/$FILE" ]; then
      FOUND=1
      break
    fi
  done
  [ "$FOUND" -eq 1 ] || return 0

  mkdir -p "$EVIDENCE" || return 0
  [ ! -f "$WORK/pass.json" ] || cp "$WORK/pass.json" "$EVIDENCE/matrix-pass.json" || true
  [ ! -f "$WORK/pass.out" ] || cp "$WORK/pass.out" "$EVIDENCE/matrix-pass.txt" || true
  [ ! -f "$WORK/fail.json" ] || cp "$WORK/fail.json" "$EVIDENCE/deliberate-failure.json" || true
  [ ! -f "$WORK/fail.out" ] || cp "$WORK/fail.out" "$EVIDENCE/deliberate-failure.txt" || true
  [ ! -f "$WORK/fail.err" ] || cp "$WORK/fail.err" "$EVIDENCE/deliberate-failure.stderr.txt" || true
}

gitea_project_prefixes() {
  printf '%s\n' \
    ulab-1-26-0-to-1-27-3 \
    ulab-1-26-4-to-1-27-3
}

cleanup_gitea_projects() {
  gitea_project_prefixes | while IFS= read -r PREFIX; do
    docker ps -a --format '{{.Label "com.docker.compose.project"}}' 2>/dev/null |
      grep -E "^$PREFIX-" |
      sort -u |
      while IFS= read -r PROJECT; do
        [ -n "$PROJECT" ] || continue
        docker compose \
          -f examples/gitea-reference/compose.yaml \
          -p "$PROJECT" \
          down --volumes --remove-orphans >/dev/null 2>&1 || true
      done

    docker volume ls -q 2>/dev/null |
      grep -E "^$PREFIX-.*_gitea-data$" |
      while IFS= read -r VOLUME; do
        [ -n "$VOLUME" ] || continue
        docker volume rm -f "$VOLUME" >/dev/null 2>&1 || true
      done

    docker network ls --format '{{.Name}}' 2>/dev/null |
      grep -E "^$PREFIX-.*_default$" |
      while IFS= read -r NETWORK; do
        [ -n "$NETWORK" ] || continue
        docker network rm "$NETWORK" >/dev/null 2>&1 || true
      done
  done
}

cleanup() {
  preserve_work_artifacts
  cleanup_gitea_projects
  rm -rf "$WORK"
}
trap cleanup EXIT INT TERM

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1"
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1"
  else
    echo "sha256sum or shasum is required for proof integrity" >&2
    exit 1
  fi
}

verify_sha256_manifest() {
  manifest="$1"
  manifest_dir=$(dirname -- "$manifest")
  manifest_base=$(basename -- "$manifest")
  if command -v sha256sum >/dev/null 2>&1; then
    (cd "$manifest_dir" && sha256sum -c "$manifest_base")
  elif command -v shasum >/dev/null 2>&1; then
    (cd "$manifest_dir" && shasum -a 256 -c "$manifest_base")
  else
    echo "sha256sum or shasum is required for proof integrity" >&2
    exit 1
  fi
}

if ! git diff --quiet || ! git diff --cached --quiet || [ -n "$(git status --porcelain --untracked-files=normal)" ]; then
  echo "Gitea runtime proof requires a clean working tree" >&2
  exit 1
fi

BRANCH=$(git branch --show-current)
if [ "$BRANCH" != "main" ]; then
  echo "Gitea runtime proof must run from canonical main; current branch: ${BRANCH:-detached}" >&2
  exit 1
fi

COMMIT=$(git rev-parse --verify HEAD)
STARTED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)
START_EPOCH=$(date +%s)

echo "==> build revision-stamped uLab binary"
go build -trimpath -buildvcs=false \
  -ldflags="-s -w -X main.version=m3-gitea-validation -X main.commit=$COMMIT" \
  -o "$BIN" ./cmd/ulab

echo "==> run real Gitea upgrade matrix"
PASS_START=$(date +%s)
"$BIN" test \
  --jobs 1 \
  --config examples/gitea-reference/ulab.json \
  --json-out "$WORK/pass.json" \
  --evidence-root "$EVIDENCE" > "$WORK/pass.out"
PASS_SECONDS=$(( $(date +%s) - PASS_START ))

grep -Eq '^1\.26\.0[[:space:]]+1\.27\.3[[:space:]]+passed$' "$WORK/pass.out"
grep -Eq '^1\.26\.4[[:space:]]+1\.27\.3[[:space:]]+passed$' "$WORK/pass.out"
grep -Fq '"status": "passed"' "$WORK/pass.json"
grep -Fq 'verified live Gitea version: 1.26.0' "$WORK/pass.json"
grep -Fq 'verified live Gitea version: 1.26.4' "$WORK/pass.json"
grep -Fq 'verified live Gitea version: 1.27.3' "$WORK/pass.json"

echo "==> run deliberate Gitea assertion failure"
FAIL_START=$(date +%s)
set +e
"$BIN" test \
  --jobs 1 \
  --config examples/gitea-reference/ulab-broken.json \
  --json-out "$WORK/fail.json" \
  --evidence-root "$EVIDENCE" > "$WORK/fail.out" 2> "$WORK/fail.err"
FAIL_RC=$?
set -e
FAIL_SECONDS=$(( $(date +%s) - FAIL_START ))

if [ "$FAIL_RC" -ne 1 ]; then
  echo "expected deliberate Gitea failure to exit 1, got $FAIL_RC" >&2
  exit 1
fi

grep -Eq '^1\.26\.4[[:space:]]+1\.27\.3[[:space:]]+failed$' "$WORK/fail.out"
grep -Fq 'compatibility policy failed' "$WORK/fail.err"
grep -Fq '"status": "failed"' "$WORK/fail.json"
grep -Fq 'verified live Gitea version: 1.26.4' "$WORK/fail.json"
grep -Fq 'verified live Gitea version: 1.27.3' "$WORK/fail.json"

BUNDLES=$(find "$EVIDENCE" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')
if [ "$BUNDLES" -ne 2 ]; then
  echo "expected 2 Gitea evidence bundles, found $BUNDLES" >&2
  exit 1
fi

for BUNDLE in "$EVIDENCE"/*; do
  [ -d "$BUNDLE" ] || continue
  test -f "$BUNDLE/config.json"
  test -f "$BUNDLE/result.json"
  test -f "$BUNDLE/metadata.json"
  grep -Fq '"config_sha256": "sha256:' "$BUNDLE/metadata.json"
  grep -Fq "\"tool_commit\": \"$COMMIT\"" "$BUNDLE/metadata.json"
done

echo "==> verify Compose cleanup"
gitea_project_prefixes | while IFS= read -r PREFIX; do
  if docker ps -a --format '{{.Label "com.docker.compose.project"}}' | grep -Eq "^$PREFIX-"; then
    echo "leftover container for run-scoped Compose project prefix $PREFIX" >&2
    exit 1
  fi
  if docker volume ls -q | grep -Eq "^$PREFIX-.*_gitea-data$"; then
    echo "leftover volume for run-scoped Compose project prefix $PREFIX" >&2
    exit 1
  fi
  if docker network ls --format '{{.Name}}' | grep -Eq "^$PREFIX-.*_default$"; then
    echo "leftover network for run-scoped Compose project prefix $PREFIX" >&2
    exit 1
  fi
done

echo "==> capture auditable proof metadata"
preserve_work_artifacts

GITEA_1260_DIGESTS=$(docker image inspect --format '{{json .RepoDigests}}' docker.gitea.com/gitea:1.26.0)
GITEA_1264_DIGESTS=$(docker image inspect --format '{{json .RepoDigests}}' docker.gitea.com/gitea:1.26.4)
GITEA_1273_DIGESTS=$(docker image inspect --format '{{json .RepoDigests}}' docker.gitea.com/gitea:1.27.3)

for VERSION_AND_DIGESTS in \
  "1.26.0|$GITEA_1260_DIGESTS" \
  "1.26.4|$GITEA_1264_DIGESTS" \
  "1.27.3|$GITEA_1273_DIGESTS"
do
  VERSION=${VERSION_AND_DIGESTS%%|*}
  DIGESTS=${VERSION_AND_DIGESTS#*|}
  case "$DIGESTS" in
    ""|"[]"|"null")
      echo "no immutable repo digest recorded for Gitea $VERSION" >&2
      exit 1
      ;;
  esac
done

GO_VERSION=$(go version)
DOCKER_CLIENT_VERSION=$(docker version --format '{{.Client.Version}}')
DOCKER_SERVER_VERSION=$(docker version --format '{{.Server.Version}}')
COMPOSE_VERSION=$(docker compose version --short)
HOST_UNAME=$(uname -a)
FINISHED_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)
TOTAL_SECONDS=$(( $(date +%s) - START_EPOCH ))

cat > "$EVIDENCE/proof-summary.json" <<EOF
{
  "schema_version": 1,
  "subject": "gitea-reference",
  "status": "passed",
  "started_at": "$STARTED_AT",
  "finished_at": "$FINISHED_AT",
  "tool_commit": "$COMMIT",
  "matrix": {
    "source_versions": ["1.26.0", "1.26.4"],
    "target_version": "1.27.3",
    "jobs": 1,
    "result": "passed"
  },
  "deliberate_failure": {
    "source_version": "1.26.4",
    "target_version": "1.27.3",
    "process_exit_code": $FAIL_RC,
    "compatibility_gate": "detected"
  },
  "durations_seconds": {
    "passing_matrix": $PASS_SECONDS,
    "deliberate_failure": $FAIL_SECONDS,
    "total_proof": $TOTAL_SECONDS
  },
  "cleanup": {
    "containers": "clean",
    "volumes": "clean",
    "networks": "clean"
  },
  "evidence_bundle_count": $BUNDLES,
  "live_version_assertions": ["1.26.0", "1.26.4", "1.27.3"],
  "images": {
    "1.26.0": $GITEA_1260_DIGESTS,
    "1.26.4": $GITEA_1264_DIGESTS,
    "1.27.3": $GITEA_1273_DIGESTS
  },
  "environment": {
    "go": "$(json_escape "$GO_VERSION")",
    "docker_client": "$(json_escape "$DOCKER_CLIENT_VERSION")",
    "docker_server": "$(json_escape "$DOCKER_SERVER_VERSION")",
    "docker_compose": "$(json_escape "$COMPOSE_VERSION")",
    "host": "$(json_escape "$HOST_UNAME")"
  }
}
EOF

cat > "$WORK/check-proof-json.go" <<'EOF'
package main

import (
	"encoding/json"
	"os"
)

func main() {
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		panic(err)
	}
}
EOF
go run "$WORK/check-proof-json.go" "$EVIDENCE/proof-summary.json" >/dev/null

cat > "$EVIDENCE/PROOF.txt" <<EOF
Gitea runtime proof

Status: passed
uLab commit: $COMMIT
Started: $STARTED_AT
Finished: $FINISHED_AT
Passing matrix wall time: ${PASS_SECONDS}s
Deliberate failure wall time: ${FAIL_SECONDS}s
Total proof wall time: ${TOTAL_SECONDS}s
Evidence bundles: $BUNDLES
Cleanup: containers, volumes and networks clean

See proof-summary.json for machine-readable proof metadata.
See the individual bundle directories for exact configs, results and reproduction metadata.
Verify this archived proof later with:
  sh scripts/verify-gitea-proof.sh $EVIDENCE
EOF

echo "==> seal proof artifacts with SHA-256"
(
  cd "$EVIDENCE"
  find . -type f ! -name SHA256SUMS ! -name PROOF_COMPLETE | LC_ALL=C sort |
    while IFS= read -r FILE; do
      if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$FILE"
      else
        shasum -a 256 "$FILE"
      fi
    done
) > "$EVIDENCE/SHA256SUMS"

verify_sha256_manifest "$EVIDENCE/SHA256SUMS" >/dev/null
MANIFEST_SHA=$(sha256_file "$EVIDENCE/SHA256SUMS" | awk '{print $1}')
cat > "$EVIDENCE/PROOF_COMPLETE" <<EOF
status=passed
tool_commit=$COMMIT
manifest_sha256=$MANIFEST_SHA
EOF

sh scripts/verify-gitea-proof.sh "$EVIDENCE" >/dev/null

echo "Gitea runtime proof passed"
echo "evidence preserved at: $EVIDENCE"
