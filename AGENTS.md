# Agent protocol

This is the Go API for patrn.ink. The dashboard lives in the sibling repo `patrn.ink-ui`.

## Commands

- Install: `go mod download`
- Dev (recommended): `cp .env.example .env` then `docker compose up --build` — API on `:8080`, Redis and DynamoDB Local
- Dev (API only): `go run ./cmd/api` — needs `REDIS_ADDR` and `DYNAMODB_ENDPOINT`
- Test (one package): `go test ./internal/shortcode`
- Test (full): `go test ./...`
- Lint / format: `gofmt -l .` and `go vet ./...`
- Build image: `docker build -t patrn-ink-api:local .`

## Hard rules

- Minimal diffs. Touch only what the task requires.
- Do not add a dependency, edit generated Swagger under `docs/` by hand, or change DynamoDB table shape without an explicit ask.
- Never commit secrets, credentials, or `.env` values. Production secrets live only on the EC2 host at `/opt/patrn.ink/.env`.
- Do not replace local `docker-compose.yml` with production Compose. Do not add Redis or DynamoDB Local to `deploy/docker-compose.prod.yml`.
- Do not change the domain contract: `patrn.ink` is the UI; `api.patrn.ink` is this API, OAuth, and `/{code}` redirects.
- For work that will edit more than two files, write `PLAN.md` first (gitignored session file).
- Run the targeted test (`go test` / `go vet`) before calling the task done.

## Authority

- Level 0 (not facts): `PLAN.md`, chat
- Level 2 (constraints): accepted files in `docs/decisions/`, `docs/architecture.md`, `deploy/DEPLOYMENT.md`
- Level 3 (prefer over prose): `docs/swagger.json`, `docs/swagger.yaml`, `internal/models/models.go`
- Level 4 (do not edit unless asked): this file, `docs/identity.md`, `README.md` product scope

## Where to read

| Need | File |
| --- | --- |
| What this is | [README.md](README.md), [docs/identity.md](docs/identity.md) |
| What we are doing now | [docs/now.md](docs/now.md) |
| How the system fits together | [docs/architecture.md](docs/architecture.md) |
| How production is operated | [deploy/DEPLOYMENT.md](deploy/DEPLOYMENT.md) |
| Why a choice was made | [docs/decisions/_index.md](docs/decisions/_index.md) |
| Data model intent | [docs/schema.md](docs/schema.md) |
| Generated API surface | [docs/swagger.yaml](docs/swagger.yaml) |

## After you finish

Propose, do not silently apply: a `docs/now.md` patch, a decision draft if you chose something, a one-line history note if git will not explain it. Do not silently edit this file or `docs/identity.md`.
