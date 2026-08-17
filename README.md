# 🔗 patrn.ink — URL shortener API

A feature-rich URL shortener API in Go (Gin): OAuth 2.0 (Google & GitHub), scoped API tokens, click analytics, QR codes, and gated public redirects. Portfolio project — the dashboard lives in the sibling `patrn.ink-ui` repo.

**Start here:** [Quick Start](#-quick-start) · [AGENTS.md](AGENTS.md) (how agents work in this repo) · [docs/identity.md](docs/identity.md) · [docs/architecture.md](docs/architecture.md) · [deploy/DEPLOYMENT.md](deploy/DEPLOYMENT.md) (production)

## ✨ Features

- OAuth 2.0 (Google & GitHub) + JWT; hashed API tokens with scopes and per-token rate limits
- Custom short codes, expiry, scheduled links, password and age gates, tags
- Click analytics (referrers, devices, countries), bulk import/export, link previews
- QR code per short URL; Redis-cached redirects; DynamoDB storage
- Interactive Swagger docs; complete Docker workflow

## 🚀 Quick Start

### Prerequisites

- Go 1.24+, Docker & Docker Compose
- Google Cloud Console account (OAuth); GitHub OAuth App (optional)

### 1. OAuth setup

**Google:** create an OAuth 2.0 Web client in the [Google Cloud Console](https://console.cloud.google.com/) with authorized redirect URI `http://localhost:8080/auth/google/callback`.

**GitHub (optional):** create an OAuth App in [Developer Settings](https://github.com/settings/developers) with callback URL `http://localhost:8080/auth/github/callback`.

### 2. Configure and run

```bash
cp .env.example .env    # add OAuth credentials
docker-compose up --build
```

The API is at `http://localhost:8080`; the public redirect path is `GET /{code}`.

## 📚 API docs

The endpoint list is **not** maintained here — it is generated:

- Interactive: `http://localhost:8080/swagger/index.html` while the server runs
- Source of truth: [`docs/swagger.yaml`](docs/swagger.yaml) (generated — do not hand-edit)
- Health: `GET /health`

```bash
# Regenerate after changing handler annotations
go install github.com/swaggo/swag/cmd/swag@latest
swag init -g cmd/api/main.go -o docs --parseDependency
```

## 💡 First calls

```bash
# Create a short link (JWT from OAuth login, or an API token)
curl -X POST http://localhost:8080/api/shorten \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"long_url":"https://example.com/very/long/url","custom_code":"my-link"}'

# Follow it
curl -L http://localhost:8080/my-link

# Create and use a scoped API token
curl -X POST http://localhost:8080/api/tokens -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"cli","scopes":["links:read","links:write"]}'
curl http://localhost:8080/api/links -H "X-API-Key: ptk_…"
```

## 🧪 Testing

```bash
go test ./...        # CI runs vet + this; add -race locally
```

## 🔧 Configuration

All configuration is environment variables. The authoritative list with defaults: [`.env.example`](.env.example). Production values (Redis Cloud, DynamoDB via instance role): [`deploy/env.prod.example`](deploy/env.prod.example) and [deploy/DEPLOYMENT.md](deploy/DEPLOYMENT.md).

## 🏭 Production

Single EC2 host with Docker Compose + Nginx: `https://patrn.ink` (UI) and `https://api.patrn.ink` (this API, OAuth, `/{code}` redirects). Redis is Redis Cloud, not a container; the database is Amazon DynamoDB via the EC2 instance role. Full runbook: **[deploy/DEPLOYMENT.md](deploy/DEPLOYMENT.md)**.

## 🛠️ Stack

Go 1.24 · Gin · DynamoDB (Local in dev) · Redis · Zap · swaggo/swag · Docker

## 🤝 Contributing · 📝 License

Portfolio project — suggestions welcome. MIT License.
