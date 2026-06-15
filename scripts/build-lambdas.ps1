# Cross-compile the reservation Lambdas to infrastructure/build/<fn>/bootstrap
# (provided.al2023 runtime). Run this before `terraform apply`.
# Usage:
#   ./scripts/build-lambdas.ps1

param(
    [string[]]$Functions = @("create-reservation", "list-reservations", "get-reservation")
)

$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path "$PSScriptRoot/..").Path
$buildDir = Join-Path $repoRoot "infrastructure/build"

Push-Location "$repoRoot/backend"
try {
    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"
    foreach ($fn in $Functions) {
        $outDir = Join-Path $buildDir $fn
        New-Item -ItemType Directory -Force -Path $outDir | Out-Null
        $out = Join-Path $outDir "bootstrap"
        Write-Host "==> Building lambda $fn -> $out" -ForegroundColor Yellow
        go build -trimpath -ldflags="-s -w" -o $out "./cmd/lambda/$fn"
    }
}
finally {
    Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}

Write-Host "Done. Now run: cd infrastructure; terraform apply" -ForegroundColor Green
