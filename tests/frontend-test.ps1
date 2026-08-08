$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..\frontend')
if (-not (Get-Command node -ErrorAction SilentlyContinue)) { throw 'node is required for frontend verification.' }
if (-not (Test-Path node_modules)) { npm.cmd install --no-audit --no-fund }
npx.cmd tsc --noEmit
npm.cmd test -- --run
npm.cmd run build
