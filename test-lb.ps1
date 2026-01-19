# Load Balancer Test Script (PowerShell)
# Tests the API with multiple instances to verify load balancing works correctly

param(
    [string]$BaseUrl = "http://localhost",
    [int]$Iterations = 20
)

Write-Host "🧪 Testing Load Balancer Setup" -ForegroundColor Cyan
Write-Host "================================" -ForegroundColor Cyan
Write-Host "Base URL: $BaseUrl"
Write-Host "Iterations: $Iterations"
Write-Host ""

# Test 1: Health Check
Write-Host "📊 Test 1: Health Check" -ForegroundColor Yellow
Write-Host "------------------------"
try {
    $response = Invoke-RestMethod -Uri "$BaseUrl/health" -Method Get
    if ($response.status -eq "healthy") {
        Write-Host "✓ Health check passed" -ForegroundColor Green
        $response | ConvertTo-Json
    } else {
        Write-Host "✗ Health check failed" -ForegroundColor Red
        $response | ConvertTo-Json
        exit 1
    }
} catch {
    Write-Host "✗ Health check failed: $_" -ForegroundColor Red
    exit 1
}
Write-Host ""

# Test 2: Load Distribution
Write-Host "📊 Test 2: Load Distribution Across Instances" -ForegroundColor Yellow
Write-Host "----------------------------------------------"
Write-Host "Making $Iterations requests to check distribution..."

$hostnameCounts = @{}

for ($i = 1; $i -le $Iterations; $i++) {
    try {
        $response = Invoke-RestMethod -Uri "$BaseUrl/health" -Method Get
        $hostname = $response.hostname
        if ($hostname) {
            if ($hostnameCounts.ContainsKey($hostname)) {
                $hostnameCounts[$hostname]++
            } else {
                $hostnameCounts[$hostname] = 1
            }
        }
    } catch {
        Write-Host "Request $i failed: $_" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "Distribution:"
$total = 0
foreach ($hostname in $hostnameCounts.Keys) {
    $count = $hostnameCounts[$hostname]
    $total += $count
    $percentage = [math]::Round(($count * 100 / $Iterations), 1)
    Write-Host "  $hostname: $count requests ($percentage%)" -ForegroundColor Green
}

$numInstances = $hostnameCounts.Count
if ($numInstances -ge 2) {
    Write-Host "✓ Load is distributed across $numInstances instances" -ForegroundColor Green
} else {
    Write-Host "⚠ Only $numInstances instance(s) detected" -ForegroundColor Yellow
}
Write-Host ""

# Test 3: Request ID Propagation
Write-Host "📊 Test 3: Request ID Propagation" -ForegroundColor Yellow
Write-Host "----------------------------------"
try {
    $response = Invoke-WebRequest -Uri "$BaseUrl/health" -Method Get
    $requestId = $response.Headers["X-Request-ID"]
    if ($requestId) {
        Write-Host "✓ Request ID header present" -ForegroundColor Green
        Write-Host "  X-Request-ID: $requestId"
    } else {
        Write-Host "✗ Request ID header missing" -ForegroundColor Red
    }
} catch {
    Write-Host "✗ Failed to check request ID: $_" -ForegroundColor Red
}
Write-Host ""

# Test 4: CORS Headers
Write-Host "📊 Test 4: CORS Headers" -ForegroundColor Yellow
Write-Host "-----------------------"
try {
    $headers = @{
        "Origin" = "http://localhost:3000"
    }
    $response = Invoke-WebRequest -Uri "$BaseUrl/health" -Method Get -Headers $headers
    $corsHeader = $response.Headers["Access-Control-Allow-Origin"]
    if ($corsHeader) {
        Write-Host "✓ CORS headers present" -ForegroundColor Green
        Write-Host "  Access-Control-Allow-Origin: $corsHeader"
    } else {
        Write-Host "⚠ CORS headers not found" -ForegroundColor Yellow
    }
} catch {
    Write-Host "⚠ CORS check failed: $_" -ForegroundColor Yellow
}
Write-Host ""

# Test 5: Metrics Endpoint
Write-Host "📊 Test 5: Metrics Endpoint" -ForegroundColor Yellow
Write-Host "---------------------------"
try {
    $metrics = Invoke-RestMethod -Uri "$BaseUrl/metrics" -Method Get
    if ($metrics -match "http_requests_total") {
        Write-Host "✓ Metrics endpoint working" -ForegroundColor Green
        $sample = ($metrics -split "`n" | Select-String "http_requests_total" | Select-Object -First 1)
        Write-Host "  Sample: $sample"
    } else {
        Write-Host "✗ Metrics endpoint not working" -ForegroundColor Red
    }
} catch {
    Write-Host "✗ Metrics endpoint failed: $_" -ForegroundColor Red
}
Write-Host ""

# Test 6: Response Time
Write-Host "📊 Test 6: Response Time" -ForegroundColor Yellow
Write-Host "------------------------"
$totalTime = 0
for ($i = 1; $i -le 10; $i++) {
    $stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    try {
        Invoke-RestMethod -Uri "$BaseUrl/health" -Method Get | Out-Null
    } catch {
        Write-Host "Request failed: $_" -ForegroundColor Red
    }
    $stopwatch.Stop()
    $totalTime += $stopwatch.Elapsed.TotalSeconds
}
$avgTime = [math]::Round($totalTime / 10, 3)
Write-Host "Average response time: $avgTime`s" -ForegroundColor Green

if ($avgTime -lt 0.5) {
    Write-Host "✓ Response time is good (<0.5s)" -ForegroundColor Green
} elseif ($avgTime -lt 1.0) {
    Write-Host "⚠ Response time is acceptable (<1.0s)" -ForegroundColor Yellow
} else {
    Write-Host "✗ Response time is slow (>1.0s)" -ForegroundColor Red
}
Write-Host ""

# Summary
Write-Host "================================" -ForegroundColor Cyan
Write-Host "🎉 Load Balancer Tests Complete!" -ForegroundColor Cyan
Write-Host "================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Summary:"
Write-Host "  - Instances detected: $numInstances"
Write-Host "  - Total requests: $total"
Write-Host "  - Average response time: $avgTime`s"
Write-Host ""
Write-Host "Next steps:"
Write-Host "  1. If testing locally, verify all instances are running"
Write-Host "  2. Check logs: docker-compose -f docker-compose.lb.yml logs"
Write-Host "  3. Monitor metrics: curl $BaseUrl/metrics"
Write-Host ""
