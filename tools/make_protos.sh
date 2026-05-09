#!/usr/bin/env bash
set -euo pipefail

# Simple helper to generate language-specific stubs into ./cordum (Go), ./python, ./cpp, and ./node
# (configurable via CAP_OUT_GO/CAP_OUT_PY/CAP_OUT_CPP/CAP_OUT_JS)
# Requirements:
# - protoc
# - protoc-gen-go, protoc-gen-go-grpc (for Go)
# - python -m grpc_tools.protoc (for Python)

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROTO_DIR="$ROOT_DIR/proto"
# Override output paths with env vars if desired.
OUT_GO="${CAP_OUT_GO:-$ROOT_DIR}"
OUT_PY="${CAP_OUT_PY:-$ROOT_DIR/python}"
OUT_CPP="${CAP_OUT_CPP:-$ROOT_DIR/cpp}"
OUT_JS="${CAP_OUT_JS:-$ROOT_DIR/node}"

# Ensure writable outputs (fallback to /tmp when mounted paths are read-only).
ensure_out_dir() {
  local target="$1"
  local fallback="$2"
  mkdir -p "$target" 2>/dev/null || true
  if [ -w "$target" ]; then
    echo "$target"
  else
    echo "WARN: $target not writable; using fallback $fallback" >&2
    mkdir -p "$fallback"
    echo "$fallback"
  fi
}

OUT_GO="$(ensure_out_dir "$OUT_GO" /tmp/capgen/go)"
OUT_PY="$(ensure_out_dir "$OUT_PY" /tmp/capgen/python)"
OUT_CPP="$(ensure_out_dir "$OUT_CPP" /tmp/capgen/cpp)"
OUT_JS="$(ensure_out_dir "$OUT_JS" /tmp/capgen/node)"

# Prefer a working python; fall back to python3 if python is missing.
PYTHON_BIN="${PYTHON_BIN:-python}"
if ! command -v "$PYTHON_BIN" >/dev/null 2>&1 && command -v python3 >/dev/null 2>&1; then
  PYTHON_BIN="python3"
fi

# Ensure protoc plugins installed via `go install` are on PATH.
export PATH="$(go env GOPATH)/bin:$PATH"

# Optional supplementary protoc include path. Linux apt installs of
# `protobuf-compiler` carry the well-known protos (google/protobuf/*.proto)
# at `/usr/include` automatically; macOS Homebrew puts them at
# `$(brew --prefix protobuf)/include`; Windows binary releases often ship
# without them. Set EXTRA_PROTO_INCLUDE to the directory containing
# `google/protobuf/{timestamp,struct,empty,...}.proto` if your protoc
# install lacks them. Example for Windows hosts using grpc.tools NuGet:
#   EXTRA_PROTO_INCLUDE=/c/Users/$USER/.nuget/packages/grpc.tools/<v>/build/native/include
EXTRA_INCLUDE_FLAG=""
if [ -n "${EXTRA_PROTO_INCLUDE:-}" ]; then
  if [ -d "$EXTRA_PROTO_INCLUDE/google/protobuf" ]; then
    EXTRA_INCLUDE_FLAG="-I$EXTRA_PROTO_INCLUDE"
    echo "Using EXTRA_PROTO_INCLUDE=$EXTRA_PROTO_INCLUDE for well-known proto resolution"
  else
    echo "WARN: EXTRA_PROTO_INCLUDE=$EXTRA_PROTO_INCLUDE does not contain google/protobuf/; ignoring" >&2
  fi
fi

mkdir -p "$OUT_GO" "$OUT_PY" "$OUT_CPP" "$OUT_JS"

echo "Generating Go stubs..."
protoc \
  -I"$PROTO_DIR" $EXTRA_INCLUDE_FLAG \
  --go_out="$OUT_GO" --go_opt=paths=source_relative \
  --go-grpc_out="$OUT_GO" --go-grpc_opt=paths=source_relative \
  $(find "$PROTO_DIR" -name '*.proto')

# C++ (static) stubs for consumers that want vendored headers/sources.
if [ "${CAP_RUN_CPP:-1}" = "1" ]; then
  echo "Generating C++ stubs..."
  protoc \
    -I"$PROTO_DIR" $EXTRA_INCLUDE_FLAG \
    --cpp_out="$OUT_CPP" \
    $(find "$PROTO_DIR" -name '*.proto')
else
  echo "CAP_RUN_CPP not set to 1; skipping C++ stubs"
fi

# JavaScript stubs (CommonJS, binary wire) for Node consumers.
if [ "${CAP_RUN_JS:-1}" = "1" ]; then
  PROTOC_JS=""
  if command -v protoc-gen-js >/dev/null 2>&1; then
    PROTOC_JS="protoc"
  elif command -v grpc_tools_node_protoc >/dev/null 2>&1; then
    PROTOC_JS="grpc_tools_node_protoc"
  fi

  if [ -n "$PROTOC_JS" ]; then
    echo "Generating Node JS stubs ($PROTOC_JS)..."
    "$PROTOC_JS" \
      -I"$PROTO_DIR" $EXTRA_INCLUDE_FLAG \
      --js_out=import_style=commonjs,binary:"$OUT_JS" \
      $(find "$PROTO_DIR" -name '*.proto')
  fi

  if command -v pbjs >/dev/null 2>&1; then
    echo "Generating Node JS bundle (pbjs)..."
    pbjs -p "$PROTO_DIR" -t static-module -w commonjs -r cordum.agent.v1 -o "$OUT_JS/cap_pb.js" $(find "$PROTO_DIR" -name '*.proto')
  fi

  if [ -z "$PROTOC_JS" ] && ! command -v pbjs >/dev/null 2>&1; then
    echo "No JS generator found; skipping Node JS stubs (install protoc-gen-js or grpc-tools, or pbjs)"
  fi

  # Generate TypeScript definitions when protoc-gen-ts or pbts is available.
  if command -v protoc-gen-ts >/dev/null 2>&1; then
    if [ -n "$PROTOC_JS" ]; then
      "$PROTOC_JS" \
        -I"$PROTO_DIR" $EXTRA_INCLUDE_FLAG \
        --ts_out="$OUT_JS" \
        $(find "$PROTO_DIR" -name '*.proto')
    else
      echo "No protoc JS generator found; skipping TypeScript typings (install protoc-gen-js or grpc-tools)"
    fi
  elif command -v pbts >/dev/null 2>&1 && [ -f "$OUT_JS/cap_pb.js" ]; then
    echo "Generating TypeScript typings (pbts)..."
    pbts -o "$OUT_JS/cap_pb.d.ts" "$OUT_JS/cap_pb.js"
  else
    echo "No TS generator found; skipping TypeScript typings (install protoc-gen-ts or pbjs/pbts)"
  fi
else
  echo "CAP_RUN_JS not set to 1; skipping Node JS stubs"
fi

# Default: skip Python stubs unless explicitly enabled.
if [ "${CAP_RUN_PY:-0}" = "1" ]; then
  echo "Generating Python stubs..."
  if "$PYTHON_BIN" - <<'PY' >/dev/null 2>&1
import importlib.util, sys
sys.exit(0 if importlib.util.find_spec("grpc_tools") else 1)
PY
  then
    "$PYTHON_BIN" -m grpc_tools.protoc \
      -I"$PROTO_DIR" $EXTRA_INCLUDE_FLAG \
      --python_out="$OUT_PY" \
      --grpc_python_out="$OUT_PY" \
      $(find "$PROTO_DIR" -name '*.proto')
  else
    echo "grpc_tools not found; skipping Python stubs (install with: pip install grpcio-tools) or set CAP_RUN_PY=0"
  fi
else
  echo "CAP_RUN_PY not set to 1; skipping Python stubs"
fi

# Java stubs (requires protoc-gen-grpc-java for gRPC services).
if [ "${CAP_RUN_JAVA:-0}" = "1" ]; then
  OUT_JAVA="${CAP_OUT_JAVA:-$ROOT_DIR/sdk/java/src/main/java}"
  OUT_JAVA="$(ensure_out_dir "$OUT_JAVA" /tmp/capgen/java)"
  echo "Generating Java stubs..."
  JAVA_GRPC_OPT=""
  if command -v protoc-gen-grpc-java >/dev/null 2>&1; then
    JAVA_GRPC_OPT="--grpc-java_out=$OUT_JAVA"
  else
    echo "protoc-gen-grpc-java not found; generating protobuf stubs only (no gRPC services)"
  fi
  protoc \
    -I"$PROTO_DIR" $EXTRA_INCLUDE_FLAG \
    --java_out="$OUT_JAVA" \
    $JAVA_GRPC_OPT \
    $(find "$PROTO_DIR" -name '*.proto')
else
  echo "CAP_RUN_JAVA not set to 1; skipping Java stubs"
fi

# C# stubs (Grpc.Tools NuGet handles this automatically during dotnet build,
# but this block can be used for standalone generation).
if [ "${CAP_RUN_CSHARP:-0}" = "1" ]; then
  OUT_CSHARP="${CAP_OUT_CSHARP:-$ROOT_DIR/sdk/dotnet/src/Cordum.Cap/Proto}"
  OUT_CSHARP="$(ensure_out_dir "$OUT_CSHARP" /tmp/capgen/csharp)"
  echo "Generating C# stubs..."
  CSHARP_GRPC_OPT=""
  if command -v grpc_csharp_plugin >/dev/null 2>&1; then
    CSHARP_GRPC_OPT="--grpc_out=$OUT_CSHARP --plugin=protoc-gen-grpc=$(which grpc_csharp_plugin)"
  else
    echo "grpc_csharp_plugin not found; generating protobuf stubs only (no gRPC services)"
  fi
  protoc \
    -I"$PROTO_DIR" $EXTRA_INCLUDE_FLAG \
    --csharp_out="$OUT_CSHARP" \
    $CSHARP_GRPC_OPT \
    $(find "$PROTO_DIR" -name '*.proto')
else
  echo "CAP_RUN_CSHARP not set to 1; skipping C# stubs"
fi

# PHP stubs (google/protobuf Composer package provides the runtime).
if [ "${CAP_RUN_PHP:-0}" = "1" ]; then
  OUT_PHP="${CAP_OUT_PHP:-$ROOT_DIR/sdk/php/proto}"
  OUT_PHP="$(ensure_out_dir "$OUT_PHP" /tmp/capgen/php)"
  echo "Generating PHP stubs..."
  PHP_GRPC_OPT=""
  if command -v grpc_php_plugin >/dev/null 2>&1; then
    PHP_GRPC_OPT="--grpc_out=$OUT_PHP --plugin=protoc-gen-grpc=$(which grpc_php_plugin)"
  else
    echo "grpc_php_plugin not found; generating protobuf stubs only (no gRPC services)"
  fi
  protoc \
    -I"$PROTO_DIR" $EXTRA_INCLUDE_FLAG \
    --php_out="$OUT_PHP" \
    $PHP_GRPC_OPT \
    $(find "$PROTO_DIR" -name '*.proto')
else
  echo "CAP_RUN_PHP not set to 1; skipping PHP stubs"
fi

# Ruby stubs (google-protobuf gem provides the runtime).
if [ "${CAP_RUN_RUBY:-0}" = "1" ]; then
  OUT_RUBY="${CAP_OUT_RUBY:-$ROOT_DIR/sdk/ruby/proto}"
  OUT_RUBY="$(ensure_out_dir "$OUT_RUBY" /tmp/capgen/ruby)"
  echo "Generating Ruby stubs..."
  RUBY_GRPC_OPT=""
  if command -v grpc_ruby_plugin >/dev/null 2>&1; then
    RUBY_GRPC_OPT="--grpc_out=$OUT_RUBY --plugin=protoc-gen-grpc=$(which grpc_ruby_plugin)"
  else
    echo "grpc_ruby_plugin not found; generating protobuf stubs only (no gRPC services)"
  fi
  protoc \
    -I"$PROTO_DIR" $EXTRA_INCLUDE_FLAG \
    --ruby_out="$OUT_RUBY" \
    $RUBY_GRPC_OPT \
    $(find "$PROTO_DIR" -name '*.proto')
else
  echo "CAP_RUN_RUBY not set to 1; skipping Ruby stubs"
fi

echo "Done. Artifacts are in $OUT_GO and $OUT_PY"
