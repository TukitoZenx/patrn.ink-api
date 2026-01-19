# Load Balancer Setup Guide

## Quick Start - Test Load Balancing Locally

### 1. Start the Load Balanced Setup

```bash
# Start 3 API instances behind Nginx load balancer
docker-compose -f docker-compose.lb.yml up --build
```

### 2. Run Tests

```powershell
# Windows (PowerShell)
.\test-lb.ps1

# Linux/Mac (Bash)
chmod +x test-lb.sh
./test-lb.sh
```

### 3. Verify Load Distribution

```bash
# Should see different hostnames (api-1, api-2, api-3)
for ($i=1; $i -le 20; $i++) {
    (Invoke-RestMethod http://localhost/health).hostname
}
```

## What's Been Fixed for Load Balancer Readiness

### ✅ Distributed Rate Limiting

- **Before**: Rate limits were per-instance (could bypass by hitting different instances)
- **After**: Rate limits stored in Redis, shared across all instances
- **File**: `ratelimit.go`

### ✅ Request ID Tracking

- **Purpose**: Trace requests across multiple instances
- **Header**: `X-Request-ID`
- **File**: `request_id.go`

### ✅ Enhanced Health Check

- **Added**: `hostname` field to identify which instance responded
- **Added**: `timestamp` for debugging
- **Endpoint**: `/health`

### ✅ Load Balancer Test Environment

- **File**: `docker-compose.lb.yml`
- **Setup**: 3 API instances + Nginx load balancer
- **Purpose**: Test load balancing locally before AWS deployment

## Architecture

```
Client → Nginx (Port 80) → api-1 (Port 8080)
                         → api-2 (Port 8080)
                         → api-3 (Port 8080)
                              ↓
                         Redis (Shared)
```

## Files Created

1. **`ratelimit.go`** - Redis-based distributed rate limiting
2. **`request_id.go`** - Request ID middleware for tracing
3. **`docker-compose.lb.yml`** - Load balancer test setup
4. **`nginx.conf`** - Nginx load balancer configuration
5. **`test-lb.ps1`** - PowerShell test script
6. **`test-lb.sh`** - Bash test script
7. **`LOAD_BALANCER_READINESS.md`** - Detailed readiness report
8. **`DEPLOYMENT_CHECKLIST.md`** - AWS deployment guide

## Files Modified

1. **`main.go`** - Added RequestIDMiddleware, switched to RedisRateLimitMiddleware
2. **`logic.go`** - Enhanced health check with hostname
3. **`Dockerfile`** - Added wget for health checks

## Testing Checklist

- [ ] Start load balanced setup: `docker-compose -f docker-compose.lb.yml up`
- [ ] Run test script: `.\test-lb.ps1`
- [ ] Verify load distribution across 3 instances
- [ ] Test rate limiting (should limit at 100 requests total, not per instance)
- [ ] Check request IDs are present
- [ ] Verify health checks return different hostnames

## Next Steps

1. **Test Locally** (you are here!)
2. **Review** `LOAD_BALANCER_READINESS.md` for detailed analysis
3. **Deploy to AWS** following `DEPLOYMENT_CHECKLIST.md`
4. **Monitor** using `/metrics` endpoint and CloudWatch

## Common Commands

```powershell
# Start load balanced setup
docker-compose -f docker-compose.lb.yml up --build

# Stop and clean up
docker-compose -f docker-compose.lb.yml down

# View logs
docker-compose -f docker-compose.lb.yml logs -f

# Check health of specific instance
docker exec patrn-api-api-1-1 wget -qO- http://localhost:8080/health

# Test rate limiting
for ($i=1; $i -le 150; $i++) {
    Invoke-RestMethod http://localhost/health | Select-Object status
}
```

## Troubleshooting

### Instances not starting

```bash
# Check logs
docker-compose -f docker-compose.lb.yml logs

# Check if ports are in use
netstat -ano | findstr :80
netstat -ano | findstr :6379
```

### Load not distributing

```bash
# Verify all instances are healthy
docker-compose -f docker-compose.lb.yml ps

# Check Nginx logs
docker-compose -f docker-compose.lb.yml logs nginx
```

### Rate limiting not working

```bash
# Check Redis connection
docker exec patrn-api-redis-1 redis-cli KEYS "ratelimit:*"

# Check rate limit keys
docker exec patrn-api-redis-1 redis-cli GET "ratelimit:<your-ip>"
```

## Production Deployment

See `DEPLOYMENT_CHECKLIST.md` for complete AWS deployment guide.

Key AWS services needed:

- Application Load Balancer (ALB)
- EC2 instances (or ECS/Fargate)
- ElastiCache Redis
- DynamoDB
- (Optional) Auto Scaling Groups

---

**Status**: ✅ Ready for load balancer deployment!
