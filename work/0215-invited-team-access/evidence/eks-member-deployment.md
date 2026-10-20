# EKS member-service validation deployment

On 12 October 2026 the Owner requested deployment of the complete member feature
for hands-on testing. No claim of completed production acceptance is made.

- Operator STS verified account 931228356546 and IAM user agenova-demo-operator.
- Terraform RDS plan: five additions, zero changes/deletions. Private encrypted
  PostgreSQL 17, db.t4g.micro, no backups per Owner request, deletion protection.
- Database migration versions 1–6 present on real RDS over verify-full TLS.
- Runtime login has no superuser, create-database, create-role, public-schema CREATE,
  migration UPDATE or fact UPDATE permissions. Master credentials are not mounted
  into the service. Temporary bootstrap Pods and Kubernetes operator Secrets removed.
- Member image includes Portal assets, amd64, immutable digest
  `sha256:0394fc49a37e6e38d13d7505c5cc88101fb207aba83e04f93ad832411118b19e`.
- Separate member Deployment becomes ready with authentication enabled. Proxy
  read-only filesystem startup failure was fixed by moving all temporary paths to
  the mounted `/tmp`; no writable root filesystem was enabled.
- Existing ACM certificate covers agenova.app and *.agenova.app.
- Public HTTPS checks: `/` 303, `/login` 200, `/api/requests` 401,
  `/api/members` 401, `/members` 303, `/auth/cli-config` 200.
- Owner one-time login link saved privately and opened for the human. No password,
  session token or invitation proof is in this evidence.

The old service remains available pending user acceptance. Real Google two-account
sign-in, invited Admin CLI Work and Bedrock positive/negative acceptance still need
completion. Budget-first long-term hosting is unresolved: this validation currently
reuses the existing EKS/ALB and adds retained RDS; it is not a claim that always-on
EKS fits AUD100/month. No shutdown or destruction schedule was enabled.
