#!/usr/bin/env bash
set -euo pipefail

# Simple helper to generate language-specific stubs into ./generated
# Requirements:
# - protoc
# - protoc-gen-go, protoc-gen-go-grpc (for Go)
# - python -m grpc_tools.protoc (for Python)

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="$ROOT_DIR/proto"
OUT_GO="$ROOT_DIR/generated/go"
OUT_PY="$ROOT_DIR/generated/python"

mkdir -p "$OUT_GO" "$OUT_PY"

echo "Generating Go stubs..."
protoc \
  -I"$PROTO_DIR" \
  --go_out="$OUT_GO" --go_opt=paths=source_relative \
  --go-grpc_out="$OUT_GO" --go-grpc_opt=paths=source_relative \
  $(find "$PROTO_DIR" -name '*.proto')

echo "Generating Python stubs..."
python -m grpc_tools.protoc \
  -I"$PROTO_DIR" \
  --python_out="$OUT_PY" \
  --grpc_python_out="$OUT_PY" \
  $(find "$PROTO_DIR" -name '*.proto')

echo "Done. Artifacts are in $ROOT_DIR/generated"
