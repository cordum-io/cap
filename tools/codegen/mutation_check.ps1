# Non-vacuity check for the hermetic codegen pipeline (Windows/PowerShell).
#
# Mirrors mutation_check.sh: build the pinned image, then run mutation_probe.sh
# inside it against a read-only source mount, requiring every language's output
# to change when one proto changes. The source mount is read-only and the probe
# works on a copy, so the real repository is never mutated.
#
# PowerShell is used on Windows because MSYS/Git-Bash rewrites the bash wrapper's
# POSIX build context and container paths, which Docker Desktop then rejects.
#
# Usage: pwsh tools/codegen/mutation_check.ps1
[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$image = 'cap-codegen:local'

Write-Host '>> building pinned codegen image'
docker build -q -f (Join-Path $repoRoot 'tools/codegen/Dockerfile') -t $image $repoRoot | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'image build failed' }

Write-Host '>> mutation probe (network disabled, read-only source)'
docker run --rm `
    --network=none `
    -e SOURCE_DATE_EPOCH=1700000000 -e TZ=UTC -e LC_ALL=C.UTF-8 `
    -v "${repoRoot}:/src:ro" `
    --entrypoint /src/tools/codegen/mutation_probe.sh `
    $image
if ($LASTEXITCODE -ne 0) { throw 'mutation probe failed' }
