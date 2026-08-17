---
type: belief
---

# Current shape

## Boundaries

| Piece | Where | Job |
| --- | --- | --- |
| Next.js UI | `patrn.ink` (sibling repo) | Dashboard, OAuth callback page, JWT in `localStorage` |
| Go/Gin API | `api.patrn.ink` | Auth, CRUD, analytics, QR, HTML gates, `GET /{code}` redirects |
| Nginx | same EC2 as both containers | TLS, host routing, hide `/metrics` |
| Redis Cloud | managed, not on EC2 | Redirect cache |
| Amazon DynamoDB | `us-east-1` | Users, Links, Analytics, APITokens |

Local Compose is a different shape: Redis + DynamoDB Local on the laptop. Production Compose runs only `api`, `ui`, and `nginx`.

## Data flow

1. Browser hits `patrn.ink` → UI. Login starts at `api.patrn.ink/auth/{google,github}/login`.
2. OAuth callback stays on the API host. API issues a JWT and redirects to `{FRONTEND_URL}/auth/callback?token=`.
3. UI calls `/api/*` with `Authorization: Bearer`. CORS allow-list is `https://patrn.ink`.
4. Public short links are `https://api.patrn.ink/{code}` only. They never go through Next.js.
5. Unset `DYNAMODB_ENDPOINT` → AWS SDK default endpoint + EC2 instance role.

## Invariants

- `NEXT_PUBLIC_*` is baked at UI image build time.
- No AWS access keys on the instance or in GitHub (OIDC + instance role).
- Production secrets only in `/opt/patrn.ink/.env` (mode `600`).

Ops detail (TLS, SSM, Compose, certs) lives in [deploy/DEPLOYMENT.md](../deploy/DEPLOYMENT.md), not here.
