---
type: decision
---

# 0004. Use Amazon DynamoDB via the EC2 instance role

Status: accepted
Date: 2026-08-13
Deciders: TukitoZenx, AndysTMC
Supersedes: —
Superseded-by: —

## Context

The code already treats empty `DYNAMODB_ENDPOINT` as “real AWS DynamoDB.” Local Compose uses DynamoDB Local plus dummy keys.

## Options

- A. DynamoDB Local in production Compose
- B. Amazon DynamoDB, credentials from the EC2 instance profile
- C. Another hosted database

## Decision

We will use option B. Do not set `DYNAMODB_ENDPOINT` in production. Do not put `AWS_ACCESS_KEY_ID` on the instance.

## Assumptions

- [A1] On-demand / Free Tier usage stays cheap at portfolio traffic (revisit if spend grows)
- [A2] Boot-time `CreateTable` plus table-level CRUD is enough IAM

## Consequences

Tables `Users`, `Links`, `Analytics`, `APITokens` are the contract. Dashboard lists still Scan by `UserID`.

## Revisit if

We outgrow scans, or we stop trusting instance metadata (IMDS hop limit must stay 2 for Docker).
