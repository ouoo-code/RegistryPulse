$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..\backend')
if (Get-Command go -ErrorAction SilentlyContinue) {
    go test ./...
    go vet ./...
    go build ./...
} else {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { throw 'go or docker is required for Go verification.' }
    $image = if ($env:GO_VERIFY_IMAGE) { $env:GO_VERIFY_IMAGE } else { 'golang:1.22-alpine' }
    docker run --rm -v "$(Get-Location):/src" -w /src $image sh -lc 'go_bin=go; command -v go >/dev/null 2>&1 || go_bin=/usr/local/go/bin/go; "$go_bin" test ./... && "$go_bin" vet ./... && "$go_bin" build ./...'
    if ($LASTEXITCODE -ne 0) { throw 'Go verification container failed.' }
}
