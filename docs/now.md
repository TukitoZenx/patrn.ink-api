---
type: now
updated: 2026-08-17
horizon: 2026-08-17 → 2026-08-31
---

# Now

Production infrastructure is live. Application smoke tests (login, create link, follow `api.patrn.ink/{code}`) passed. Certificate renewal cron is on the EC2 host.

## Focus

1. Keep the live stack stable. Do not redeploy for red *old* GitHub Actions runs.
2. Apply this Knowledge Architecture (kernel + earned rings) so future agents have one protocol hub.
3. Optional ops hygiene: AWS billing alarm, confirm Redis Cloud plan, decide on DynamoDB PITR.

## Next

- [ ] Human: AWS billing budget/alarm
- [ ] Human: confirm Redis Cloud tier vs intended plan
- [ ] Later: DynamoDB point-in-time recovery if real data matters

## Blocked

—

## Do not do

- Do not put Redis or DynamoDB Local on EC2
- Do not change Nginx or rotate secrets without a leak
- Do not manually renew Let's Encrypt
- Do not invent ECS/EKS/Terraform for v1
