$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

docker compose config --quiet
docker compose up -d
try {
    $timeout = 120
    if ($env:COMPOSE_SMOKE_TIMEOUT_SECONDS) { $timeout = [int]$env:COMPOSE_SMOKE_TIMEOUT_SECONDS }
    $deadline = (Get-Date).AddSeconds($timeout)
    do {
        $health = try { Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1/health' -TimeoutSec 5 } catch { $null }
        if ($health.StatusCode -eq 200) { break }
        if ((Get-Date) -ge $deadline) {
            docker compose ps
            docker compose logs --no-color api nginx
            throw 'Compose smoke test timed out.'
        }
        Start-Sleep -Seconds 3
    } while ($true)

    foreach ($uri in @('http://127.0.0.1/health', 'http://127.0.0.1/api/v1/health', 'http://127.0.0.1/')) {
        $response = Invoke-WebRequest -UseBasicParsing -Uri $uri -TimeoutSec 10
        if ($response.StatusCode -ne 200) { throw ("Unexpected HTTP status for {0}: {1}" -f $uri, $response.StatusCode) }
    }
    Write-Output 'Compose smoke test passed.'
}
finally {
    docker compose down
}
