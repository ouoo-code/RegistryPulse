$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

docker compose config --quiet
if (-not (Test-Path -LiteralPath 'deploy/nginx/default.conf')) { throw 'Nginx configuration is missing.' }
foreach ($script in @('deploy/scripts/backup.sh', 'deploy/scripts/restore.sh')) {
    if (-not (Test-Path -LiteralPath $script)) { throw "Missing script: $script" }
}
$nginx = Get-Content -Raw -Encoding UTF8 deploy/nginx/default.conf
foreach ($needle in @('set $api_upstream api:8080', 'proxy_set_header X-Forwarded-Proto', 'proxy_connect_timeout')) {
    if ($nginx.IndexOf($needle) -lt 0) { throw "Nginx configuration is missing: $needle" }
}
Write-Output 'Static deployment validation passed.'
