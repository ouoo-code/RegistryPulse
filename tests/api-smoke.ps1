$ErrorActionPreference = 'Stop'
Set-Location (Join-Path $PSScriptRoot '..')

$baseUrl = $env:API_BASE_URL
if ([string]::IsNullOrWhiteSpace($baseUrl)) { $baseUrl = 'http://127.0.0.1' }
$baseUrl = $baseUrl.TrimEnd('/')
$timeoutValue = $env:API_SMOKE_TIMEOUT_SECONDS
if ([string]::IsNullOrWhiteSpace($timeoutValue)) { $timeoutValue = '10' }
$timeout = [int]$timeoutValue

function Invoke-Api([string]$Path, [hashtable]$Headers = @{}) {
    $response = Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl$Path" -Headers $Headers -TimeoutSec $timeout
    $json = $response.Content | ConvertFrom-Json
    if ($json.success -ne $true) { throw "API response did not contain success=true: $Path" }
    return $json
}

Invoke-Api '/api/v1/health' | Out-Null
Invoke-Api '/api/v1/public/summary' | Out-Null
Invoke-Api '/api/v1/public/categories' | Out-Null
Invoke-Api '/api/v1/public/sources' | Out-Null
$metrics = Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl/metrics" -TimeoutSec $timeout
if ($metrics.StatusCode -ne 200) { throw "Metrics returned $($metrics.StatusCode)" }

if ($env:ADMIN_API_TOKEN) {
    try { Invoke-WebRequest -UseBasicParsing -Uri "$baseUrl/api/v1/admin/sources" -TimeoutSec $timeout | Out-Null; throw 'Unauthenticated admin request unexpectedly succeeded.' }
    catch { if ($_.Exception.Response.StatusCode.value__ -ne 401) { throw } }
    Invoke-Api '/api/v1/admin/sources' @{ Authorization = "Bearer $($env:ADMIN_API_TOKEN)" } | Out-Null
}

Write-Output "API smoke test passed: $baseUrl"
