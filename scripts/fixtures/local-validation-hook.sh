#!/bin/sh
set -eu

: "${ULAB_VALIDATION_STATE_DIR:?ULAB_VALIDATION_STATE_DIR is required}"
phase="${1:-}"
state="$ULAB_VALIDATION_STATE_DIR/$ULAB_RUN_ID"

case "$phase" in
  setup)
    mkdir -p "$ULAB_VALIDATION_STATE_DIR"
    printf '%s\n' "$ULAB_SOURCE_VERSION" > "$state"
    ;;
  upgrade)
    printf '%s\n' "$ULAB_TARGET_VERSION" >> "$state"
    ;;
  verify)
    grep -qx "$ULAB_SOURCE_VERSION" "$state"
    grep -qx "$ULAB_TARGET_VERSION" "$state"
    if [ "${ULAB_VALIDATION_FAIL_SOURCE:-}" = "$ULAB_SOURCE_VERSION" ]; then
      echo "intentional incompatibility for $ULAB_SOURCE_VERSION" >&2
      exit 9
    fi
    ;;
  *)
    echo "unknown validation phase: $phase" >&2
    exit 2
    ;;
esac
