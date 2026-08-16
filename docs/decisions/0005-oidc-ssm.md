---
type: decision
---

# 0005. Deploy with GitHub OIDC and SSM, not SSH or static AWS keys

Status: accepted
Date: 2026-08-13
Deciders: TukitoZenx
Supersedes: —
Superseded-by: —

## Context

Images already built to ECR. We needed a deploy path that does not store an EC2 key or long-lived AWS keys in GitHub.

## Options

- A. `AWS_ACCESS_KEY_ID` in GitHub Secrets + SSH
- B. GitHub OIDC assume role → ECR push → SSM `AWS-RunShellScript` → `/opt/patrn.ink/scripts/deploy.sh`
- C. Manual `docker compose` on the box only

## Decision

We will use option B. CD triggers on `v*` tags and optional `workflow_dispatch`. PR CI does not deploy.

## Assumptions

- [A1] `AWS-RunShellScript` runs under `/bin/sh`; SSM command lines must not use bash-only `set -o pipefail`
- [A2] The instance stays an SSM managed node

## Consequences

GitHub needs `AWS_ROLE_ARN`, `AWS_REGION`, `ECR_*_REPOSITORY`, `EC2_INSTANCE_ID`. The AWS-owned SSM document ARN has an empty account id.

## Revisit if

SSM is unavailable or we move off a single EC2 host.
