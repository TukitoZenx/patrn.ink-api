---
type: belief
---

# Data model intent

Source of fields: [`internal/models/models.go`](../internal/models/models.go). Persistence: [`internal/storage/storage.go`](../internal/storage/storage.go). Do not transcribe attributes here.

## Why these stores

- **DynamoDB** is the system of record so production can use an IAM role and on-demand tables without running a database process on EC2.
- **Redis** is a cache in front of redirect lookups, not the source of truth.

## Tables (names are part of the contract)

| Table | Key | Why |
| --- | --- | --- |
| `Users` | `ID` | OAuth subject (`google_…` / `github_…`) |
| `Links` | `ShortCode` | Hot path for `GET /{code}` |
| `Analytics` | `ShortCode` + `Timestamp` | Per-click events |
| `APITokens` | `ID` | Hashed tokens; listing still scans by `UserID` |

`ListTables` / `CreateTable` run at boot so a fresh account can start. There is no `UserID` GSI yet; dashboard lists scan and filter. Revisit if more than demo data.

## Redis

Keys are cache entries for short codes (`url:{code}`). A miss falls back to DynamoDB. Production uses Redis Cloud with `REDIS_USERNAME=default`. Local Compose uses a Redis container and may have an empty password.
