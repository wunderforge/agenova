# Shared infrastructure state specification

Ticket: [#261](https://github.com/wunderforge/agenova/issues/261)

Each existing root uses its own S3 object in account 931228356546, Sydney. Terraform >=1.10 is required for native S3 locking. State is confidential, encrypted, versioned and blocked from public access. Operators use their existing authorized AWS SSO role; Agenova app roles do not grant AWS authority.

Migration copies existing state without importing/recreating resources or applying drift. Missing/deleted cloud resources remain historical state until a separately reviewed refresh/plan. Local stale states are retired from operational use after verification.

On-demand startup orders EKS, edge, member-edge, application deployment, with a separate reviewed database ingress update if the recreated cluster SG changes. Shutdown never destroys the whole EKS root and never deletes RDS/network/IAM/ECR/logs/certificates. Persistent roots are excluded from routine off operations.

Negative cases: unknown resources or destructive changes outside the allowed ephemeral resource set fail review; missing SSO authority is a real blocker. No new product/API semantics, automatic scheduling or runtime restart in this migration.
