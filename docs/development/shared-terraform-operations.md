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
not restore workload secrets or the UI. An absent immutable main tag must be
built and published before referencing it; a branch update does not prove that
publication succeeded. Use `deploy/production/release.Dockerfile` with the exact
main source tree, `linux/amd64`, revision label and immutable SHA tag. On managed
networks, provide the trusted enterprise CA to build stages; never disable TLS
verification or copy build-only trust material into the final runtime image.

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
python3 scripts/infra/check_backend.py --root production/eks
# Set a current RFC3339 reminder in the private tfvars before planning.
terraform -chdir=infra/production/eks plan -input=false -out=../../../.tmp/terraform-plans/eks.tfplan
terraform -chdir=infra/production/eks show -json ../../../.tmp/terraform-plans/eks.tfplan > .tmp/terraform-plans/eks.json
python3 scripts/infra/check_backend.py --root production/eks
python3 scripts/infra/review_plan.py --root production/eks --operation on .tmp/terraform-plans/eks.json
```

Private recovery tfvars may still contain an expired `expires_at` and old API
CIDRs. Update those explicitly for the current test window and operator egress.
The reminder is not a shutdown scheduler. First recovery can also propose
tag-only changes on retained resources; review these separately, without
expanding the routine ephemeral allowlist.

Read the human plan as well. The scope guard rejects persistent or unknown
resource changes and replacements. It is a review aid, not authorization to apply.
Do not broaden the allowlist to make unexpected drift pass. Persistent repairs
require a separate reviewed plan. After any apply, replan to inspect remaining
changes; targeted plans do not represent the whole configuration.

## Restore order and current limits

1. Review EKS root drift and restore the cluster/node group, access entries and
   addons while preserving its VPC, subnets, IAM, ECR and logs. Verify access for
   the existing SSO administrator principal: the current Terraform operator
   entry alone does not restore the separately provisioned team SSO entry.
   Restore its existing cluster-admin policy association if absent; do not
   create a new IAM role or grant application users cluster access.
2. Review the new cluster security group wiring to RDS. A recreated cluster can
   change its security group ID. Review a separate narrowly scoped database
   ingress repair; never apply unrelated RDS engine or data changes.
3. Restore `production/edge`, then `member-edge`, using the current certificate
   and node group inputs. Stale security-group rules may need a separate reviewed
   repair before routine allowlisted plans pass.
4. Restore the pinned Agent Sandbox controller/CRDs compatible with the saved
   templates. Create namespaces and apply
   `deploy/production/eks/network-policies.yaml` before starting workloads and
   `deploy/production/eks/worker-egress-deny.yaml` after the CNI addon is ready: the
   EKS CNI uses strict network-policy enforcement.
5. Restore workload secrets, service and main's artifact; verify endpoints and UI.
   Exclude old `kube-root-ca.crt`, object UIDs/resource versions/owner references
   and Service cluster IPs from backups. The new cluster supplies its own CA.
   Preserve the original application session key and runtime DB credentials;
   do not rerun Owner bootstrap.

The migration and lifecycle verification are separate operations. The current
configuration mixes persistent network resources with ephemeral compute, so
restoration requires the above drift review. See the ticket evidence for the
actual cycle results and remaining manual-login limitation. Native EKS Pod Identity addon
and model-gateway association are cluster-scoped restore targets; their IAM role
and policy remain persistent. Do not promise a one-click
restart until that recovery has been exercised.

## Stop order (only when explicitly authorized)

Use the reviewed AWS API procedure for this mixed persistent/ephemeral layout.
The real EKS targeted-destroy preview included the retained model-gateway IAM role
and policy because of dependencies; it was rejected and was **not applied**.
Terraform also marks targeted plans incomplete. Do not disable the guard or
assume `-target` limits deletion to the addresses typed on the command line.

If a person is performing a manual login check, wait for their explicit result
or instruction to shut down before removing the entry point.

1. Confirm account/backend, no running apply, no active SandboxClaims/Work, and
   privately save state, current deployment/configuration and exact AWS IDs.
2. Remove the warm pool and allow its Sandbox workers to finish deletion. Keep
   the controller running until that cleanup finishes; never remove finalizers
   or bypass a node drain to hurry the test.
3. Inspect the current Route53 records. Submit DELETE only for app/demo traffic
   aliases that point to the reviewed ALB. Preserve all other records and zone.
4. Delete that exact ALB ARN; wait until absent. Delete only the reviewed
   `agenova-members` and `agenova-demo-portal` target-group ARNs. Listener/rule
   resources disappear with their ALB. Do not delete security groups.
5. Delete node group `demo` in `agenova-demo`; wait until absent and verify its
   recorded EC2 nodes and root EBS volumes are gone. Then delete `agenova-demo`
   and wait until absent. Preserve IAM roles/policies, including model-gateway.
6. Verify RDS is still `available`, deletion protection is enabled, and its
   resource identity is unchanged. Check the exact ALB/TG/DNS/node/EBS/EKS IDs.
7. After confirmed absence, reconcile only the absent ephemeral addresses with
   locked `terraform state rm` in the three affected roots. Pull private state
   backups first; never remove persistent resource addresses. This removes stale
   bindings, not cloud resources, and the next start plan will recreate them.
   Keep the audit of removed addresses and final S3 lineage/serial.

The exact ephemeral address sets are in `scripts/infra/review_plan.py`. Use them
for reviewed state reconciliation, not as an instruction to bypass plan review.
Re-run init/backend checks from other checkouts; shared state remains authoritative.

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
