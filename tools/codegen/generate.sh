#!/usr/bin/env bash
# ENTRYPOINT of the pinned codegen container (tools/codegen/Dockerfile).
#
# Runs the canonical generator, tools/proto_codegen.py, in its hermetic
# --offline mode: buf resolves every plugin locally from the image instead of
# from buf.build, and the Node bundler uses the node_modules baked in at image
# build time. Nothing is downloaded, so this works under --network=none.
#
#   generate.sh            regenerate the tracked tree in place (default)
#   generate.sh --check    fail if the tracked tree differs from a fresh run
#
# Exit status is the generator's: non-zero on drift, on a missing or empty
# output, on a non-idempotent second run, or on any unusable generator. There is
# deliberately no skip path — an absent generator is a hard failure, because a
# generator that silently produces nothing is exactly the false green this
# container exists to prevent.
set -euo pipefail

mode="--write"
if [[ "${1:-}" == "--check" ]]; then
  mode="--check"
elif [[ $# -gt 0 ]]; then
  echo "usage: generate.sh [--check]" >&2
  exit 2
fi

cd /src

# Fail loudly if the image is missing a generator rather than letting
# proto_codegen.py fall back to anything on the host.
for tool in buf protoc-gen-go protoc-gen-go-grpc protoc-gen-js node python3; do
  command -v "$tool" >/dev/null || { echo "codegen image is missing $tool" >&2; exit 1; }
done
for protoc in "${CAP_CODEGEN_PROTOC_CPP:-}" "${CAP_CODEGEN_PROTOC_RUBY:-}"; do
  [[ -x "$protoc" ]] || { echo "codegen image is missing protoc: ${protoc:-<unset>}" >&2; exit 1; }
done

echo ">> cap codegen ${mode} (hermetic, $(buf --version), $(node --version))"
exec python3 tools/proto_codegen.py "${mode}" --offline
