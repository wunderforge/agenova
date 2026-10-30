# Shared Terraform state and on-demand EKS

Infrastructure operators use AWS SSO and the private backend below. Application
members must not receive state access: state can contain database credentials.
Automatic retirement was cancelled on 30 October 2026. There is no active shutdown
schedule. Do not use an old cleanup deadline as authorization.

## State and authentication

Use Terraform >= 1.10, < 2.0 and AWS CLI v2. The bucket is
`agenova-tfstate-931228356546-ap-southeast-2`, region `ap-southeast-2`, account
`931228356546`. Each root has its own `<root>/terraform.tfstate` key:
`production/eks`, `production/edge`, `member-edge`, `database`, `domain`.
Native S3 lock files are enabled. Never use `-lock=false`, copy another checkout's
`.terraform` directory, or apply an old saved plan.

Configure an AWS SSO profile for the existing `AgenovaAdministrator` permission
set in this account using your assigned Identity Center access. No new users or
permissions are granted by this change. Then, from the repository root:

```sh
umask 077
export AWS_PROFILE=agenova-sso
aws sso login --profile "$AWS_PROFILE"
aws sts get-caller-identity
terraform -chdir=infra/production/eks init
python3 scripts/infra/check_backend.py --root production/eks
```

Repeat init and the backend check for each root you operate. Obtain private
`terraform.tfvars` and workload secrets through the team's private channel; they
are deliberately not in Git. Do not reconstruct unknown inputs from defaults.
The check accepts the existing SSO administrator role or designated demo operator,
rejects root and other accounts, and verifies the initialized bucket/key/locking.

The bucket has versioning, AES256 encryption, all public-access blocks,
BucketOwnerEnforced ownership, and a deny for non-TLS requests. Preserve it.
See [HashiCorp's S3 backend documentation](https://developer.hashicorp.com/terraform/language/backend/s3)
for lock permissions: state requires GetObject/PutObject; its `.tflock` also needs
DeleteObject. Do not grant application workloads state access.

## Version to deploy

Use the latest reviewed `origin/main` commit as the UI deployment baseline. Record
its full SHA when restoring, and deploy the artifact built from that SHA. As of the
initial inspection, main was `32b59bf`; this is a source baseline, not proof of a
running UI. EKS and app/demo DNS were absent. Restoring infrastructure alone does
not restore workload secrets or the UI.

Follow [shared EKS release](shared-eks-release.md) and
[member service bootstrap](../../deploy/production/member-service.md). The existing
release workflow deploys to an existing cluster; it does not bootstrap a missing
cluster. Verify the deployed image digest and revision, HTTPS page rendering,
authentication and a real service smoke test before announcing availability.

## Plan and review workflow

Always use a fresh plan and run both checks before applying the same saved file.
Plans and JSON can contain secrets; keep them outside Git with private permissions.
For example, after setting the correct root and reviewed inputs:

```sh
mkdir -p .tmp/terraform-plans
chmod 700 .tmp/terraform-plans
terraform -chdir=infra/production/eks plan -out=../../../.tmp/terraform-plans/eks.tfplan
terraform -chdir=infra/production/eks show -json ../../../.tmp/terraform-plans/eks.tfplan > .tmp/terraform-plans/eks.json
python3 scripts/infra/check_backend.py --root production/eks
python3 scripts/infra/review_plan.py --root production/eks --operation on .tmp/terraform-plans/eks.json
```

Read the human plan as well. The scope guard rejects persistent or unknown
resource changes and replacements. It is a review aid, not authorization to apply.
Do not broaden the allowlist to make unexpected drift pass. Persistent repairs
require a separate reviewed plan. After any apply, replan to inspect remaining
changes; targeted plans do not represent the whole configuration.

## Restore order and current limits

1. Review EKS root drift and restore the cluster/node group, access entries and
   addons while preserving its VPC, subnets, IAM, ECR and logs.
2. Review the new cluster security group wiring to RDS. A recreated cluster can
   change its security group ID. Review a separate narrowly scoped database
   ingress repair; never apply unrelated RDS engine or data changes.
3. Restore `production/edge`, then `member-edge`, using the current certificate
   and node group inputs. Stale security-group rules may need a separate reviewed
   repair before routine allowlisted plans pass.
4. Restore workload secrets, service and main's artifact; verify endpoints and UI.

This migration does not execute or certify a complete off/on cycle. The current
configuration mixes persistent network resources with ephemeral compute, so
first restoration requires the above drift review. Do not promise a one-click
restart until that recovery has been exercised.

## Stop order (only when explicitly authorized)

Plan exact ephemeral targets in `member-edge`, then `production/edge`, then
`production/eks`. The allowed addresses are maintained in
`scripts/infra/review_plan.py`. Use repeated `-target=ADDRESS` arguments with
`terraform plan -destroy -out=...`; inspect the JSON with `--operation off` and
run the backend check. Terraform may include dependencies; the guard must reject
any resulting persistent-resource deletion. Apply only a reviewed saved plan.

**Never run whole-root destroy for EKS, database, domain or state storage.**
Preserve RDS/data, VPC/subnets/routes/security dependencies, permanent certificates
and validation records, hosted zone, IAM/SSO, ECR and logs. Delete only approved
application traffic DNS alongside its retired ingress. Confirm node termination
and associated root EBS removal, ALB/target groups and EKS absence, and RDS
`available`; investigate unknown volumes instead of deleting by name alone.

Node scale-to-zero leaves EKS and ingress charges. Removing EKS/ALB saves those
charges but requires recreation. RDS, retained storage/network and logs can still
cost money. Neither operation stops every AWS charge.

## Recovery and handoff

Before a backend migration, privately back up every state and tfvars file, stop
concurrent writers, and compare resources/outputs and metadata after migration.
All checkouts must initialize the committed backend before use. Old local state
copies are recovery material only; never apply them as a second authority.
If a state restore is needed, stop all writers and inspect S3 version history,
lineage and resource IDs with an operator before restoring a selected version.
Do not force-unlock unless the owning operation is confirmed stopped.

Share the merged commit, this guide, SSO assignment and private-input location
with the team. Each teammate must verify their own SSO login and backend access;
an operator's successful test does not prove another person's access.
