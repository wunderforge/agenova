# Production infrastructure layout

| Root | Ownership |
| --- | --- |
| `eks` | Existing EKS, worker nodes, workload IAM, VPC/subnets, ECR, logs and budget |
| `edge` | Existing shared ALB, HTTPS listener, legacy default target and DNS |
| `../member-edge` | `app.agenova.app` host rule, member target group and DNS |
| `../database` | Persistent RDS and database security configuration |
| `../domain` | Permanent certificates and validation records |

These are independent state roots. The EKS VPC also supports production RDS.
**Never destroy the entire EKS root to stop compute.** Preserve the database,
network dependencies, domain, certificates, IAM/SSO, ECR and logs. Use a reviewed,
resource-specific lifecycle operation under the latest authorized deadline.
Renaming this directory does not certify high availability or budget compliance.

## Existing installation migration

The former `infra/demo/aws` and `infra/demo/edge` roots moved to `eks` and `edge`.
Terraform resource labels, cloud names (including `agenova-demo`), provider aliases,
and existing tags intentionally stay unchanged. Renaming them can replace resources.
The legacy default ALB target remains managed; the active member service uses the
separate host rule. Removing legacy cloud resources needs its own reviewed plan.

For an existing local backend, stop concurrent Terraform operations, privately back
up the complete old root, then move its `terraform.tfstate`, backups, `.terraform`,
lock file and private tfvars into the corresponding new root. Do not create an
empty state, import duplicates, or reuse old saved plans. Only one checkout owns
the live state. Other checkouts must not apply without the operator's state/backend.
Keep state/vars/plans private (`umask 077`). Reinitialize if provider/backend metadata
requires it, preserving the backend; do not change backends as part of this move.

Compare state hashes, lineage, serial and resource IDs before/after. Run `validate`
and compare saved `plan -refresh=false` results using the same state and inputs.
Then review a refreshed plan separately for pre-existing drift. A directory move
must add no resource changes; it does not authorize applying unrelated drift.
For team-operated Terraform, establish an encrypted shared backend and locking as
a separate migration. Agenova application users never receive Terraform state.

See [member deployment](../../deploy/production/member-service.md),
[member ingress](../member-edge/README.md) and
[migration evidence](../../work/0215-invited-team-access/evidence/infra-directory-migration.md).
