# Load Balancer Deployment Checklist

## ✅ Pre-Deployment Checklist

### Infrastructure Setup

- [ ] **AWS Account Setup**
  - [ ] VPC configured with public and private subnets
  - [ ] Security groups configured
  - [ ] IAM roles for EC2 instances (DynamoDB access)

- [ ] **Application Load Balancer (ALB)**
  - [ ] ALB created in public subnets
  - [ ] Target group created (port 8080)
  - [ ] Health check configured: `/health` endpoint
  - [ ] Health check interval: 30s
  - [ ] Healthy threshold: 2
  - [ ] Unhealthy threshold: 3
  - [ ] Timeout: 5s

- [ ] **EC2 Instances**
  - [ ] Launch 2-3 instances in private subnets
  - [ ] Install Docker and Docker Compose
  - [ ] Configure security group (allow 8080 from ALB only)
  - [ ] Register instances with target group

- [ ] **ElastiCache Redis**
  - [ ] Redis cluster created
  - [ ] Security group allows access from EC2 instances
  - [ ] Connection string noted for environment variables

- [ ] **DynamoDB**
  - [ ] Tables created (or auto-create enabled)
  - [ ] IAM role attached to EC2 instances
  - [ ] Region configured correctly

### Environment Variables

- [ ] `.env.production` file created with:
  - [ ] `BASE_URL` (your domain)
  - [ ] `GOOGLE_CLIENT_ID`
  - [ ] `GOOGLE_CLIENT_SECRET`
  - [ ] `GOOGLE_REDIRECT_URL`
  - [ ] `JWT_SECRET` (strong random string)
  - [ ] `AWS_REGION`
  - [ ] `REDIS_ADDR` (ElastiCache endpoint)
  - [ ] `REDIS_PASSWORD` (if enabled)
  - [ ] `ALLOWED_ORIGINS` (your frontend URLs)

### Code Changes

- [x] **Rate Limiting** - Migrated to Redis (distributed)
- [x] **Request ID Tracking** - Added for distributed tracing
- [x] **Health Check** - Enhanced with hostname/timestamp
- [x] **Graceful Shutdown** - Already implemented
- [x] **Stateless Design** - No local state

## 🚀 Deployment Steps

### 1. Test Locally with Load Balancer

```bash
# Test with 3 instances behind Nginx
docker-compose -f docker-compose.lb.yml up --build

# Test health check
curl http://localhost/health

# Test load balancing (should see different hostnames)
for i in {1..10}; do curl http://localhost/health | jq .hostname; done

# Test rate limiting across instances
for i in {1..150}; do curl -H "Authorization: Bearer YOUR_JWT" http://localhost/api/shorten; done
```

### 2. Build and Push Docker Image

```bash
# Build for production
docker build -t patrn-api:latest .

# Tag for ECR (replace with your ECR URL)
docker tag patrn-api:latest <aws-account-id>.dkr.ecr.<region>.amazonaws.com/patrn-api:latest

# Push to ECR
aws ecr get-login-password --region <region> | docker login --username AWS --password-stdin <aws-account-id>.dkr.ecr.<region>.amazonaws.com
docker push <aws-account-id>.dkr.ecr.<region>.amazonaws.com/patrn-api:latest
```

### 3. Deploy to EC2 Instances

On each EC2 instance:

```bash
# Pull latest image
docker pull <aws-account-id>.dkr.ecr.<region>.amazonaws.com/patrn-api:latest

# Run container
docker run -d \
  --name patrn-api \
  --restart unless-stopped \
  -p 8080:8080 \
  --env-file .env.production \
  <aws-account-id>.dkr.ecr.<region>.amazonaws.com/patrn-api:latest

# Check health
curl http://localhost:8080/health
```

### 4. Verify ALB Health Checks

```bash
# Check target group health
aws elbv2 describe-target-health \
  --target-group-arn <your-target-group-arn>

# Should show all targets as "healthy"
```

### 5. Configure DNS

- [ ] Point your domain to ALB DNS name
- [ ] Update `BASE_URL` environment variable
- [ ] Update `GOOGLE_REDIRECT_URL` in Google Console

## 🔍 Post-Deployment Verification

### Functional Tests

- [ ] Health check returns 200 OK
- [ ] Can create short URLs
- [ ] Can redirect to long URLs
- [ ] Rate limiting works across instances
- [ ] Analytics are recorded correctly

### Load Balancing Tests

```bash
# Test distribution across instances
for i in {1..20}; do
  curl -s https://your-domain.com/health | jq -r .hostname
done | sort | uniq -c

# Should show requests distributed across all instances
```

### Performance Tests

```bash
# Use Apache Bench or similar
ab -n 1000 -c 10 https://your-domain.com/health

# Monitor metrics
curl https://your-domain.com/metrics
```

### Monitoring Setup

- [ ] CloudWatch alarms configured
  - [ ] Target 5XX errors
  - [ ] Target response time
  - [ ] Unhealthy host count
- [ ] Prometheus scraping `/metrics` endpoint
- [ ] Log aggregation (CloudWatch Logs or ELK)

## 🔧 Troubleshooting

### Instance Not Healthy

1. Check EC2 instance logs: `docker logs patrn-api`
2. Check health endpoint: `curl http://localhost:8080/health`
3. Verify Redis connectivity
4. Verify DynamoDB IAM permissions

### Rate Limiting Not Working

1. Check Redis connectivity from all instances
2. Verify Redis keys: `redis-cli KEYS "ratelimit:*"`
3. Check TTL: `redis-cli TTL "ratelimit:<key>"`

### Uneven Load Distribution

1. Check ALB algorithm (should be round-robin or least connections)
2. Verify all instances are healthy
3. Check connection draining settings

## 📊 Monitoring Metrics

### Key Metrics to Watch

- **ALB Metrics**
  - Request count
  - Target response time
  - HTTP 4xx/5xx errors
  - Healthy/unhealthy host count

- **Application Metrics** (via `/metrics`)
  - `http_requests_total`
  - `http_request_duration_seconds`
  - `redirects_total`

- **Redis Metrics**
  - Memory usage
  - Connected clients
  - Commands per second

- **DynamoDB Metrics**
  - Read/write capacity
  - Throttled requests
  - Latency

## 🔐 Security Checklist

- [ ] Security groups properly configured (least privilege)
- [ ] JWT secret is strong and rotated regularly
- [ ] HTTPS only (ALB with SSL certificate)
- [ ] Rate limiting enabled
- [ ] CORS properly configured
- [ ] No sensitive data in logs
- [ ] Environment variables secured (AWS Secrets Manager)

## 🎯 Scaling Considerations

### Horizontal Scaling

- ALB can handle up to 10+ instances
- Add/remove instances based on CPU/memory metrics
- Consider Auto Scaling Groups

### Vertical Scaling

- Start with t3.small or t3.medium
- Monitor CPU/memory usage
- Scale up if consistently >70% utilization

### Database Scaling

- DynamoDB auto-scales (on-demand mode)
- Redis: Consider cluster mode for >100GB data
- Monitor read/write patterns

## 📝 Notes

- Test everything in staging first
- Have rollback plan ready
- Monitor closely for first 24 hours
- Keep old instances running until new ones are proven stable
