#!/usr/bin/env bash
# Run hermetic CAP codegen in the pinned container: no network, read-only source
# mount, clean writable output, deterministic environment. After it runs, use
# `go run ./cmd/cap-codegen check` to hold the tree to tools/codegen/manifest.json.
#
# Usage: tools/codegen/codegen.sh [--check]
#   (default)  regenerate into ./ (Phase 4G reconciliation)
#   --check    build the image and run the manifest check only
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
image="cap-codegen:local"

echo ">> building pinned codegen image"
docker build -f "${repo_root}/tools/codegen/Dockerfile" -t "${image}" "${repo_root}"

if [[ "${1:-}" == "--check" ]]; then
  ( cd "${repo_root}" && go run ./cmd/cap-codegen check )
  exit $?
fi

echo ">> generating (network disabled, read-only source)"
docker run --rm \
  --network=none \
  -e SOURCE_DATE_EPOCH=1700000000 -e TZ=UTC -e LC_ALL=C.UTF-8 \
  -v "${repo_root}:/src" \
  "${image}"

echo ">> verifying manifest"
( cd "${repo_root}" && go run ./cmd/cap-codegen check )
