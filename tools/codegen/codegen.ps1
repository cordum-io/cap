# Run hermetic CAP codegen in the pinned container (Windows/PowerShell wrapper).
# Mirrors codegen.sh: no network, read-only source mount, deterministic env.
#
# Usage: pwsh tools/codegen/codegen.ps1 [-Check]
[CmdletBinding()]
param([switch]$Check)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$image = 'cap-codegen:local'

Write-Host '>> building pinned codegen image'
docker build -f (Join-Path $repoRoot 'tools/codegen/Dockerfile') -t $image $repoRoot
if ($LASTEXITCODE -ne 0) { throw 'image build failed' }

if ($Check) {
    Push-Location $repoRoot
    try { go run ./cmd/cap-codegen check } finally { Pop-Location }
    exit $LASTEXITCODE
}

Write-Host '>> generating (network disabled, read-only source)'
docker run --rm `
    --network=none `
    -e SOURCE_DATE_EPOCH=1700000000 -e TZ=UTC -e LC_ALL=C.UTF-8 `
    -v "${repoRoot}:/src" `
    $image
if ($LASTEXITCODE -ne 0) { throw 'generation failed' }

Write-Host '>> verifying manifest'
Push-Location $repoRoot
try { go run ./cmd/cap-codegen check } finally { Pop-Location }
