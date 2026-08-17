---
type: decision
---

# 0003. Use Redis Cloud for production cache, not Redis on EC2

Status: accepted
Date: 2026-08-13
Deciders: TukitoZenx, AndysTMC
Supersedes: —
Superseded-by: —

## Context

The API requires Redis at boot (`InitRedis` fatals on ping failure). A Redis container on EC2 was the first Compose design. The operator already had Redis Cloud (`redis.io`).

## Options

- A. `redis:7-alpine` on the same Compose network
- B. ElastiCache
- C. Existing Redis Cloud database (host:port, username `default`, database password)

## Decision

We will use option C. Production Compose has no Redis service. `REDIS_ADDR` / `REDIS_USERNAME` / `REDIS_PASSWORD` come from `/opt/patrn.ink/.env`. Local Compose still runs Redis.

## Assumptions

- [A1] The Cloud connection is non-TLS (`redis://` style), matching the operator’s client snippet (revisit if Redis Cloud requires `rediss://`)
- [A2] Cross-region latency (Cloud in `asia-south1`, EC2 in `us-east-1`) is acceptable for v1

## Consequences

No Redis volume or healthcheck on EC2. `/health` reports Redis Cloud, not a sidecar. Use the **database password**, not the Redis Cloud account API key.

## Revisit if

TLS is required, latency is too high, or the Cloud instance is retired.
