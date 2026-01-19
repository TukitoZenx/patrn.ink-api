# Environment Configuration Guide

This project uses different environment configurations for development and production.

## 📁 File Structure

```
.env.example        # Template (committed to git)
.env                # Local development (gitignored)
.env.production     # Production secrets (gitignored)
```

## 🚀 Quick Start

### First Time Setup

1. **Copy the example file:**

   ```bash
   cp .env.example .env
   ```

2. **Get Google OAuth credentials:**
   - Go to [Google Cloud Console](https://console.cloud.google.com/apis/credentials)
   - Create OAuth 2.0 credentials
   - Set authorized redirect URI: `http://localhost:8080/auth/google/callback`
   - Copy Client ID and Client Secret to `.env`

3. **Generate JWT secret:**

   ```bash
   # Using OpenSSL
   openssl rand -base64 32

   # Or using PowerShell
   -join ((65..90) + (97..122) + (48..57) | Get-Random -Count 32 | % {[char]$_})
   ```

   - Copy the output to `JWT_SECRET` in `.env`

4. **Start development environment:**
   ```bash
   docker-compose up
   ```

## 🔧 Development vs Production

### Development (`.env`)

- Uses local DynamoDB (`DYNAMODB_ENDPOINT=http://localhost:8000`)
- Uses containerized Redis
- OAuth callback: `http://localhost:8080/auth/google/callback`
- Relaxed rate limiting
- CORS allows localhost origins

### Production (`.env.production`)

- Uses AWS DynamoDB (empty `DYNAMODB_ENDPOINT`)
- Uses AWS ElastiCache or production Redis
- OAuth callback: `https://patrn.ink/auth/google/callback`
- Stricter rate limiting
- CORS allows production domains only

## 🐳 Docker Compose Commands

### Development

```bash
# Start all services
docker-compose up

# Start in background
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

### Production

```bash
# Use production compose file
docker-compose -f docker-compose.prod.yml up -d

# Or set environment variable
export COMPOSE_FILE=docker-compose.prod.yml
docker-compose up -d
```

## 🔐 Security Checklist

- [ ] Never commit `.env` or `.env.production` files
- [ ] Use strong, random JWT_SECRET (32+ characters)
- [ ] Use separate Google OAuth credentials for dev/prod
- [ ] Set Redis password in production
- [ ] Use AWS IAM roles instead of access keys when possible
- [ ] Enable HTTPS in production
- [ ] Review CORS allowed origins

## 📝 Environment Variables Reference

| Variable               | Required | Description                |
| ---------------------- | -------- | -------------------------- |
| `GOOGLE_CLIENT_ID`     | Yes      | Google OAuth Client ID     |
| `GOOGLE_CLIENT_SECRET` | Yes      | Google OAuth Client Secret |
| `JWT_SECRET`           | Yes      | Secret key for JWT signing |
| `AWS_REGION`           | Yes      | AWS region for DynamoDB    |
| `DYNAMODB_ENDPOINT`    | Dev only | Local DynamoDB endpoint    |
| `REDIS_ADDR`           | Yes      | Redis server address       |
| `REDIS_PASSWORD`       | Prod     | Redis password             |
| `BASE_URL`             | Yes      | Application base URL       |
| `ALLOWED_ORIGINS`      | Yes      | CORS allowed origins       |

## 🏗️ Production Deployment

For production, consider using:

- **AWS ECS/EKS** for container orchestration
- **AWS Secrets Manager** for storing `.env.production` values
- **AWS ElastiCache** instead of containerized Redis
- **AWS DynamoDB** (managed service)
- **Application Load Balancer** with SSL/TLS

## 🆘 Troubleshooting

**Issue:** Google OAuth not working

- Check redirect URI matches exactly in Google Console
- Verify `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` are correct

**Issue:** DynamoDB connection failed

- For local: Ensure `DYNAMODB_ENDPOINT=http://localhost:8000`
- For Docker: Ensure `DYNAMODB_ENDPOINT=http://dynamodb:8000`
- For production: Leave `DYNAMODB_ENDPOINT` empty

**Issue:** Redis connection failed

- For local: Use `REDIS_ADDR=localhost:6379`
- For Docker: Use `REDIS_ADDR=redis:6379`
