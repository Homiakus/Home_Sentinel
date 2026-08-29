$ErrorActionPreference = "Stop"
$Root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Push-Location $Root
try {
    & go run ./cmd/sentinelctl @args
    exit $LASTEXITCODE
}
finally {
    Pop-Location
}
