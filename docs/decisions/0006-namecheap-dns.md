---
type: decision
---

# 0006. Keep DNS on Namecheap BasicDNS

Status: accepted
Date: 2026-08-13
Deciders: TukitoZenx, AndysTMC
Supersedes: —
Superseded-by: —

## Context

The domain was already at Namecheap (apex on Vercel, `api` on Render). Route 53 was the first written plan.

## Options

- A. Move the hosted zone to Route 53
- B. Leave Namecheap BasicDNS; A-record apex and `api` at the Elastic IP; leave `www` on Vercel as fallback

## Decision

We will use option B.

## Assumptions

- [A1] Namecheap BasicDNS stays authoritative (revisit if nameservers change)
- [A2] `www` is not required for the product

## Consequences

Docs must not claim Route 53. Certbot covers `patrn.ink` and `api.patrn.ink`.

## Revisit if

We need AWS-side DNS failover or `www` must terminate on this EC2.
