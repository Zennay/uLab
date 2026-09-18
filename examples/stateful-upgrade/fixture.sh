#!/bin/sh
set -eu

action="${1:-}"
state=/data/state.txt

case "$action" in
  seed)
    printf 'user=alice\nschema=1\n' > "$state"
    ;;
  upgrade)
    case "$APP_VERSION" in
      v2)
        grep '^user=' "$state" > "$state.tmp"
        printf 'schema=2\n' >> "$state.tmp"
        mv "$state.tmp" "$state"
        ;;
      broken)
        printf 'schema=2\n' > "$state"
        ;;
      *)
        echo "unsupported target version: $APP_VERSION" >&2
        exit 2
        ;;
    esac
    ;;
  verify)
    grep -qx 'user=alice' "$state"
    grep -qx 'schema=2' "$state"
    ;;
  *)
    echo "usage: fixture <seed|upgrade|verify>" >&2
    exit 2
    ;;
esac
