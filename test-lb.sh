#!/bin/bash

# Load Balancer Test Script
# Tests the API with multiple instances to verify load balancing works correctly

set -e

BASE_URL="${1:-http://localhost}"
ITERATIONS="${2:-20}"

echo "🧪 Testing Load Balancer Setup"
echo "================================"
echo "Base URL: $BASE_URL"
echo "Iterations: $ITERATIONS"
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test 1: Health Check
echo "📊 Test 1: Health Check"
echo "------------------------"
response=$(curl -s "$BASE_URL/health")
status=$(echo $response | jq -r '.status')

if [ "$status" == "healthy" ]; then
    echo -e "${GREEN}✓ Health check passed${NC}"
    echo $response | jq .
else
    echo -e "${RED}✗ Health check failed${NC}"
    echo $response | jq .
    exit 1
fi
echo ""

# Test 2: Load Distribution
echo "📊 Test 2: Load Distribution Across Instances"
echo "----------------------------------------------"
echo "Making $ITERATIONS requests to check distribution..."

declare -A hostname_counts

for i in $(seq 1 $ITERATIONS); do
    hostname=$(curl -s "$BASE_URL/health" | jq -r '.hostname')
    if [ -n "$hostname" ]; then
        hostname_counts[$hostname]=$((${hostname_counts[$hostname]:-0} + 1))
    fi
done

echo ""
echo "Distribution:"
total=0
for hostname in "${!hostname_counts[@]}"; do
    count=${hostname_counts[$hostname]}
    total=$((total + count))
    percentage=$((count * 100 / ITERATIONS))
    echo -e "  $hostname: ${GREEN}$count requests ($percentage%)${NC}"
done

# Check if load is distributed (at least 2 different instances)
num_instances=${#hostname_counts[@]}
if [ $num_instances -ge 2 ]; then
    echo -e "${GREEN}✓ Load is distributed across $num_instances instances${NC}"
else
    echo -e "${YELLOW}⚠ Only $num_instances instance(s) detected${NC}"
fi
echo ""

# Test 3: Request ID Propagation
echo "📊 Test 3: Request ID Propagation"
echo "----------------------------------"
response=$(curl -s -i "$BASE_URL/health" | grep -i "X-Request-ID")
if [ -n "$response" ]; then
    echo -e "${GREEN}✓ Request ID header present${NC}"
    echo "  $response"
else
    echo -e "${RED}✗ Request ID header missing${NC}"
fi
echo ""

# Test 4: CORS Headers
echo "📊 Test 4: CORS Headers"
echo "-----------------------"
response=$(curl -s -i -H "Origin: http://localhost:3000" "$BASE_URL/health" | grep -i "Access-Control")
if [ -n "$response" ]; then
    echo -e "${GREEN}✓ CORS headers present${NC}"
    echo "$response"
else
    echo -e "${YELLOW}⚠ CORS headers not found (may need Origin header)${NC}"
fi
echo ""

# Test 5: Metrics Endpoint
echo "📊 Test 5: Metrics Endpoint"
echo "---------------------------"
metrics=$(curl -s "$BASE_URL/metrics" | grep "http_requests_total" | head -n 1)
if [ -n "$metrics" ]; then
    echo -e "${GREEN}✓ Metrics endpoint working${NC}"
    echo "  Sample: $metrics"
else
    echo -e "${RED}✗ Metrics endpoint not working${NC}"
fi
echo ""

# Test 6: Response Time
echo "📊 Test 6: Response Time"
echo "------------------------"
total_time=0
for i in $(seq 1 10); do
    time=$(curl -s -o /dev/null -w "%{time_total}" "$BASE_URL/health")
    total_time=$(echo "$total_time + $time" | bc)
done
avg_time=$(echo "scale=3; $total_time / 10" | bc)
echo -e "Average response time: ${GREEN}${avg_time}s${NC}"

if (( $(echo "$avg_time < 0.5" | bc -l) )); then
    echo -e "${GREEN}✓ Response time is good (<0.5s)${NC}"
elif (( $(echo "$avg_time < 1.0" | bc -l) )); then
    echo -e "${YELLOW}⚠ Response time is acceptable (<1.0s)${NC}"
else
    echo -e "${RED}✗ Response time is slow (>1.0s)${NC}"
fi
echo ""

# Summary
echo "================================"
echo "🎉 Load Balancer Tests Complete!"
echo "================================"
echo ""
echo "Summary:"
echo "  - Instances detected: $num_instances"
echo "  - Total requests: $total"
echo "  - Average response time: ${avg_time}s"
echo ""
echo "Next steps:"
echo "  1. If testing locally, verify all instances are running"
echo "  2. Check logs for any errors: docker-compose logs"
echo "  3. Monitor metrics: curl $BASE_URL/metrics"
echo ""
