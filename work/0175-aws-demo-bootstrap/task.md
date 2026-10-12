# Task: E13 AWS demo infrastructure prerequisite

- Ticket: [#175](https://github.com/wunderforge/agenova/issues/175)
- Mission: Prepare reproducible, disposable Sydney AWS infrastructure for the team E13 demonstration.
- Target: Proposed `infra/demo/aws/` Terraform configuration and operator runbook.
- User value: A new team account can host the later Agenova EKS/Bedrock acceptance path.
- PRD outcome: [Reference installation](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap); infrastructure is an external prerequisite, not a new Agenova cluster-provisioning feature.

## Context to Read

- `AGENTS.md`
- `docs/product/prd.md`
- This task packet, [spec](spec.md), [design](design.md).
- [Architecture: Reference Installation](../../docs/product/architecture-contract.md#reference-installation-and-bootstrap).
- [Start a GitHub Ticket](../../docs/harness/playbooks.md#start-a-github-ticket).
- [AIDLC](../../docs/development/AIDLC.md).
- AWS official EKS pricing, current standard-support versions, IAM temporary-credential and EKS networking documentation before implementation.

## Scope

In scope:

- AWS account/bootstrap identity discovery without collecting credentials in repository or chat.
- Terraform for a disposable VPC, two availability-zone subnets, EKS, one bounded managed node group, ECR, required service roles and budget alerts.
- Explicit account, region, operator principal, API CIDR and supported Kubernetes-version checks.
- Credential-free configuration, cost worksheet, apply verification and resource cleanup runbook.

Out of scope:

- Agenova product changes, Bedrock adapter, shared API authentication, and full E13 acceptance.
- Account creation, production infrastructure, HA, NAT gateways, load balancers, public application endpoints or blanket Bedrock permissions.
- Persisting root keys, IAM passwords or access keys in Terraform state.

## Acceptance Criteria

- Formatting and Terraform validation pass with a committed provider lock file.
- An authenticated plan identifies only the intended team account, Sydney resources, explicit identities and bounded node capacity.
- Live apply, when prerequisites are satisfied, yields an ACTIVE cluster and Ready node with no inbound SSH/public application access.
- Budget currency and planned resource lifetime have been confirmed; pricing inputs and exclusions are recorded before apply.
- Destruction steps cover cluster, nodes, volumes, addresses, network and repositories; retained resources are explicit.
- E13 remains open: infrastructure creation alone does not prove governed Bedrock inference.

## Negative Case

- Wrong AWS account, root execution, unrestricted API CIDR, unsupported Kubernetes version, or missing bootstrap authentication blocks deployment.
- Invalid permissions fail visibly without adding blanket administrative grants automatically.
- A budget alert must never be described as a hard spending cap or automatic teardown.

## Execution Todo

- [x] Scout repository and installed tools; check AWS session and EKS pricing.
- [x] Draft task/spec/design from canonical templates.
- [x] Owner approved this packet in the conversation on 2026-09-28.
- [x] Resolve currency, operating lifetime, authenticated AWS account and bootstrap identity.
- [x] Implement bounded Terraform and credential/state exclusions.
- [x] Install required official tooling and run fmt/init/validate.
- [x] Produce account-backed plan and cost estimate; resolve changes exceeding budget or authorization.
- [x] Apply authorized plan and capture cluster/node evidence.
- [x] Run focused gates and repository gate; document blockers exactly.
- [x] Review diff and deliver reproducible setup/teardown instructions.

## Quality Gates

- `terraform fmt -check -recursive infra/demo/aws`
- `terraform -chdir=infra/demo/aws init -backend=false`
- `terraform -chdir=infra/demo/aws validate`
- Authenticated saved plan and post-apply EKS/node checks; no secrets in logs.
- `pwsh -NoProfile -File ./scripts/check.ps1 -All`

## Evidence Required

- Versioned dependency lock file, exact validation outputs, sanitized plan summary, verified account/region.
- EKS status and Kubernetes node readiness or explicit authentication/environment blocker.
- Cost inputs, planned active hours, cleanup procedure and residual resource list.

## Constraints

- Preserve architecture contract and existing kind/Ollama behavior.
- Do not provision unrelated resources or modify pre-existing shared infrastructure.
- Human bootstrap credentials remain outside Terraform; prefer temporary sessions.

## Decisions and Blockers

- Approved by Owner; AUD100/month, Sydney, disposable deployment. Latest retention is ten hours after readiness, superseding four hours.
- Account 931228356546; CLI profile agenova-demo verified as IAM user agenova-demo-operator. User supplied admin-group permissions; no additional bootstrap policy created. Temporary browser-based CLI login works with Terraform.
- Real plan: 23 add / 0 change / 0 destroy. Apply in progress; paid resources already exist. See [handoff](HANDOFF.md).
- One t3.medium node, 20GB gp3, one shared Internet Gateway, no NAT Gateway or Load Balancer. User explicitly requested economical networking. No public SSH/application ingress.
- Ten-hour core estimate approximately USD1.60: EKS USD0.10/hour, Sydney t3.medium USD0.0528/hour, public IPv4 USD0.005/hour, gp3 USD0.096/GB-month; excludes logs, transfer, ECR, taxes, FX and creation/deletion time. Budget alert USD50 is not a hard cap.
- Terraform 1.16.4 / AWS provider 6.66.0: init, fmt, validate passed. EKS1.35 STANDARD_SUPPORT and EC2 quotas/availability verified live.
- Full gate passed docs/Go checks/integration compilation; UI smoke 51 passed / 1 failed (`ui/smoke/console.spec.ts:99`, keyboard selection did not show Loading evidence). No UI source changes; baseline/flakiness not established. Full gate is not green.
- Codex local cleanup automation agenova-eks must be aligned with exact readiness + ten-hour deadline. It requires an awake host and valid credentials; ExpiresAt tags do not trigger AWS deletion.
- Infrastructure completion does not prove Agent Sandbox, Agenova, protected API or Bedrock functionality; full E13 remains open.

- Final override: cluster/node Ready, initial apply and retention update succeeded, full gate now passes (52 browser smoke tests). See current handoff and E13 live evidence; historical failure above is resolved.
