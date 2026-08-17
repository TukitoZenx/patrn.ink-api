---
type: decision
---

# 0001. Serve the UI on `patrn.ink` and the API plus short links on `api.patrn.ink`

Status: accepted
Date: 2026-08-13
Deciders: TukitoZenx, AndysTMC
Supersedes: —
Superseded-by: —

## Context

The product needs a branded dashboard and short URLs. Putting both on one hostname makes `/{code}` collide with Next.js routes (`/dashboard`, `/auth`).

## Options

- A. Same host, path-prefix the API (`/api`) and hope short codes never clash
- B. `app.` for UI and apex for short links
- C. Apex UI (`patrn.ink`) and `api.patrn.ink` for API, OAuth, and `/{code}`

## Decision

We will use option C. OAuth callbacks stay on the API host. The UI talks to `https://api.patrn.ink` via CORS.

## Assumptions

- [A1] Short-link users can tolerate `api.patrn.ink/{code}` instead of apex codes (revisit if branding requires apex shorts)
- [A2] Browsers will send CORS + Bearer from `patrn.ink` to `api.patrn.ink`

## Consequences

Nginx must route by `Host`. The UI `app/[code]` page is a fallback, not the production redirector.

## Revisit if

Apex short links become a product requirement, or cookies/CORS block the split-domain auth flow.
