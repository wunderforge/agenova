# Production PostgreSQL on Amazon RDS

Owner-selected managed database: Sydney, PostgreSQL 17, `db.t4g.micro`, Single-AZ,
20 GiB gp3, encryption at rest and forced TLS. No public database address; ingress
is restricted to the explicitly supplied application security group. Members never
receive a database credential. Separate migration/runtime database roles must be
bootstrapped before application startup; the runtime must not use the master user.
RDS manages the master password in Secrets Manager rather than Terraform state.

The Owner explicitly excluded backups: automated backup retention is zero and final
snapshot creation is disabled. Accidental destruction remains blocked by RDS deletion
protection and Terraform `prevent_destroy`. Planned nightly stops must retain the
instance/storage. Do not use Terraform destroy as a stop/start mechanism.

AWS Price List API, queried 12 October 2026, Sydney:

- PostgreSQL `db.t4g.micro`, Single-AZ: USD 0.025/hour.
- PostgreSQL gp3: USD 0.138/GB-month; 20 GB = USD 2.76/month.
- At 14 running hours/day × 30 days: compute + storage = USD 13.26/month.

This is a component estimate, not the whole AUD100 service budget. Include Secrets
Manager, applicable tax, conversion, compute, network, DNS and model usage separately.
Stopped RDS still bills storage. RDS can automatically restart after seven days
stopped; a daily scheduler must observe actual status and failure, not assume a stop
request succeeded. Use standard-support engine versions, bounded usage and no paid
reservation commitment. See [pricing](https://aws.amazon.com/rds/postgresql/pricing/)
and [stopping RDS](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_StopInstance.html).

This root has not been applied. Supply the production VPC, subnet IDs in distinct AZs
and application security group as part of the complete network/host plan. Review all
changes before apply. Live DB, TLS, persistence, migration and safe stop/start tests
remain release gates under `work/0215-invited-team-access/task.md`.
