#!/usr/bin/env bash
# Run hermetic CAP codegen in the pinned container: no network, clean writable
# output, deterministic environment.
#
# Usage: tools/codegen/codegen.sh [--check]
#   (default)  regenerate the tracked tree in place, then verify the manifest
#   --check    prove a fresh hermetic run reproduces the tracked tree, then
#              verify the manifest. Read-only source mount; writes nothing.
#
# Both modes run the generator. `cap-codegen check` alone only compares the
# tracked files to their recorded hashes, which stays green even if the
# generator is broken, absent, or emits nothing -- so it verifies the tree, not
# the pipeline. The container run is what proves the artifacts are reproducible.
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
image="cap-codegen:local"
mode="${1:-}"

if [[ -n "${mode}" && "${mode}" != "--check" ]]; then
  echo "usage: codegen.sh [--check]" >&2
  exit 2
fi

echo ">> building pinned codegen image"
docker build -f "${repo_root}/tools/codegen/Dockerfile" -t "${image}" "${repo_root}"

if [[ "${mode}" == "--check" ]]; then
  echo ">> verifying the pinned generator reproduces the tree (network disabled, read-only source)"
  docker run --rm \
    --network=none \
    -e SOURCE_DATE_EPOCH=1700000000 -e TZ=UTC -e LC_ALL=C.UTF-8 \
    -v "${repo_root}:/src:ro" \
    "${image}" --check
else
  echo ">> generating (network disabled)"
  docker run --rm \
    --network=none \
    -e SOURCE_DATE_EPOCH=1700000000 -e TZ=UTC -e LC_ALL=C.UTF-8 \
    -v "${repo_root}:/src" \
    "${image}"
fi

echo ">> verifying manifest"
( cd "${repo_root}" && go run ./cmd/cap-codegen check )
