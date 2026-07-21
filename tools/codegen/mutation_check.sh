#!/usr/bin/env bash
# Non-vacuity check for the hermetic codegen pipeline.
#
# A green `codegen.sh --check` proves the tracked tree matches a fresh run. It
# does NOT prove the run derives from the .proto sources: a generator that
# copied the tracked files, or a language that silently emitted nothing, would
# look identical. This drives mutation_probe.sh, which changes one proto inside
# the container and requires every language to change with it.
#
# The source mount is read-only and the probe works on a copy, so the real
# repository is never mutated. Run from anywhere:
#
#   tools/codegen/mutation_check.sh
set -euo pipefail

# On Windows/MSYS this POSIX wrapper cannot work: without MSYS_NO_PATHCONV the
# container --entrypoint /src/... is rewritten to a host path, and with it Docker
# Desktop rejects the /d/... build context. Redirect to the PowerShell wrapper
# rather than fail obscurely.
case "$(uname -s)" in
  MINGW* | MSYS* | CYGWIN*)
    echo "On Windows, run the PowerShell wrapper instead:" >&2
    echo "  pwsh tools/codegen/mutation_check.ps1" >&2
    exit 2
    ;;
esac

repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
image="cap-codegen:local"

echo ">> building pinned codegen image"
docker build -q -f "${repo_root}/tools/codegen/Dockerfile" -t "${image}" "${repo_root}" >/dev/null

exec docker run --rm --network=none \
  -e SOURCE_DATE_EPOCH=1700000000 -e TZ=UTC -e LC_ALL=C.UTF-8 \
  -v "${repo_root}:/src:ro" \
  --entrypoint /src/tools/codegen/mutation_probe.sh \
  "${image}"
