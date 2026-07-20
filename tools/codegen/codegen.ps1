# Run hermetic CAP codegen in the pinned container (Windows/PowerShell wrapper).
# Mirrors codegen.sh: no network, deterministic env, generator always runs.
#
# Usage: pwsh tools/codegen/codegen.ps1 [-Check]
#   (default)  regenerate the tracked tree in place, then verify the manifest
#   -Check     prove a fresh hermetic run reproduces the tracked tree, then
#              verify the manifest. Read-only source mount; writes nothing.
[CmdletBinding()]
param([switch]$Check)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$image = 'cap-codegen:local'

Write-Host '>> building pinned codegen image'
docker build -f (Join-Path $repoRoot 'tools/codegen/Dockerfile') -t $image $repoRoot
if ($LASTEXITCODE -ne 0) { throw 'image build failed' }

# `cap-codegen check` alone only compares tracked files to recorded hashes and
# stays green even if the generator is broken or absent, so both modes run the
# container: that is what proves the artifacts are actually reproducible.
if ($Check) {
    Write-Host '>> verifying the pinned generator reproduces the tree (network disabled, read-only source)'
    docker run --rm `
        --network=none `
        -e SOURCE_DATE_EPOCH=1700000000 -e TZ=UTC -e LC_ALL=C.UTF-8 `
        -v "${repoRoot}:/src:ro" `
        $image --check
} else {
    Write-Host '>> generating (network disabled)'
    docker run --rm `
        --network=none `
        -e SOURCE_DATE_EPOCH=1700000000 -e TZ=UTC -e LC_ALL=C.UTF-8 `
        -v "${repoRoot}:/src" `
        $image
}
if ($LASTEXITCODE -ne 0) { throw 'hermetic codegen failed' }

Write-Host '>> verifying manifest'
Push-Location $repoRoot
try {
    go run ./cmd/cap-codegen check
    if ($LASTEXITCODE -ne 0) { throw 'manifest check failed' }
} finally { Pop-Location }
