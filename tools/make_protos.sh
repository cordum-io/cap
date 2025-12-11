#!/usr/bin/env bash
set -euo pipefail

# Simple helper to generate language-specific stubs into ./generated
# Requirements:
# - protoc
# - protoc-gen-go, protoc-gen-go-grpc (for Go)
# - python -m grpc_tools.protoc (for Python)

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="$ROOT_DIR/proto"
# Override output paths with env vars if desired.
OUT_GO="${CAP_OUT_GO:-$ROOT_DIR/gen/go}"
OUT_PY="${CAP_OUT_PY:-$ROOT_DIR/generated/python}"

# Prefer a working python; fall back to python3 if python is missing.
PYTHON_BIN="${PYTHON_BIN:-python}"
if ! command -v "$PYTHON_BIN" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  PYTHON_BIN="python3"
fi

# Ensure protoc plugins installed via `go install` are on PATH.
export PATH="$(go env GOPATH)/bin:$PATH"

mkdir -p "$OUT_GO" "$OUT_PY"

echo "Generating Go stubs..."
protoc \
  -I"$PROTO_DIR" \
  --go_out="$OUT_GO" --go_opt=paths=source_relative \
  --go-grpc_out="$OUT_GO" --go-grpc_opt=paths=source_relative \
  $(find "$PROTO_DIR" -name '*.proto')

echo "Generating Python stubs..."
"$PYTHON_BIN" -m grpc_tools.protoc \
  -I"$PROTO_DIR" \
  --python_out="$OUT_PY" \
  --grpc_python_out="$OUT_PY" \
  $(find "$PROTO_DIR" -name '*.proto')

echo "Done. Artifacts are in $OUT_GO and $OUT_PY"
