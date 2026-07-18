#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PYTHON_BIN="${PYTHON_BIN:-python}"

if ! command -v "$PYTHON_BIN" >/dev/null 2>&1; then
  echo "Python is required (tried PYTHON_BIN=$PYTHON_BIN)" >&2
  exit 1
fi

exec "$PYTHON_BIN" "$ROOT_DIR/tools/proto_codegen.py" "$@"
