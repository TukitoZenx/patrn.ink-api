---
type: decision
---

# 0002. Run production on one EC2 host with Docker Compose and Nginx

Status: accepted
Date: 2026-08-13
Deciders: TukitoZenx
Supersedes: —
Superseded-by: —

## Context

The goal is a real AWS deploy a junior engineer can explain. The app is already Dockerized.

## Options

- A. ECS/Fargate or EKS
- B. Elastic Beanstalk / App Runner
- C. One `t3.small` EC2, Compose (`api`, `ui`, `nginx`), Elastic IP

## Decision

We will use option C. No Terraform/CDK/CloudFormation in v1.

## Assumptions

- [A1] One instance is enough for portfolio traffic (revisit if the box is the outage)
- [A2] The operator can use SSM for deploys and emergency access

## Consequences

Single point of failure. Docker images and logs share one disk. Certbot and Nginx run beside the apps.

## Revisit if

We need multi-instance or immutable infra-as-code as the control plane.
