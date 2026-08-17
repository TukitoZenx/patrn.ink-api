---
type: identity
---

# Identity

**patrn.ink** is a portfolio URL shortener: OAuth login, scoped API tokens, analytics, QR codes, and gated public redirects.

## Intent

Show a complete, explainable product: Go API, Next.js dashboard, and a small AWS production path (EC2, Docker, Nginx, ECR, IAM, SSM) without a container orchestrator.

## Scope

- Short-link create / update / delete, custom codes, expiry, schedule, password and age gates
- Click analytics, QR, bulk import/export, destination health
- Google and GitHub OAuth; JWT for the dashboard; hashed API tokens with scopes
- Production on one EC2 host; Redis Cloud; Amazon DynamoDB

## Non-goals

- Multi-region, Kubernetes, ECS/Fargate, Lambda, or API Gateway
- Terraform / CDK / CloudFormation as the v1 control plane
- Running Redis or DynamoDB Local in production
- Treating this as a multi-tenant SaaS with SLAs

## Success

A stranger can run locally with Compose, and the live site at `patrn.ink` / `api.patrn.ink` can create and follow a short link.

## Ownership

Portfolio project by TukitoZenx with contributions from AndysTMC. Public GitHub: `TukitoZenx/patrn.ink-api` and `TukitoZenx/patrn.ink-ui`.
