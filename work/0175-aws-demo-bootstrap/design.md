# Technical Design: E13 AWS demo infrastructure prerequisite

- Ticket: [#175](https://github.com/wunderforge/agenova/issues/175)
- Feature spec: [spec.md](spec.md)

## Current State and Constraints

At task start, no Terraform infrastructure or AWS CLI/Terraform existed in this checkout. They are now installed and the approved deployment is in progress; see the handoff for current state. EKS alone costs roughly USD 73 per 730-hour month under standard support, so lifetime is a key input.

## Decision

Propose isolated `infra/demo/aws/` Terraform with pinned AWS provider and lock file, a dedicated VPC, two public subnets in distinct AZs, internet gateway, restricted EKS public endpoint plus private endpoint, and a single managed worker node. Avoid NAT and load balancer recurring costs for this initial demo. Public addressing does not authorize inbound traffic: no SSH/application ingress, and required cluster/node rules only. Verify this topology against official EKS documentation before implementation.

Use explicit service-role trust policies, EKS operator access entries and node roles. Prefer a temporary non-root operator session; any initial human identity bootstrap is outside Terraform secret state. Bedrock permissions are not granted to worker nodes. Separate gateway credentials are a later E13/E14 integration step.

Use encrypted node volumes, bounded disk size, metadata-service hardening compatible with required components, tagged resources and limited log retention. Provider version and Kubernetes version are selected after verifying current support. Use explicit account allowlist and variable validation.

Local state is acceptable only for the initial single-operator bootstrap, with restricted file permissions and secure retention. A shared backend must be decided before a second operator applies. Do not commit state, plans or credentials. Cleanup must not force-delete repositories containing images without a reviewed deletion plan.

## Ownership and Contract Boundaries

Terraform owns demo substrate only. Agenova Platform retains application deployment ownership. E14 owns application identities and policy; E13 owns cloud/provider integration; E15 owns runtime anti-bypass evidence. An EKS Ready node proves none of those application guarantees.

## Alternatives Considered

- Always-on EKS: likely exceeds current budget once worker resources are included.
- NAT/private-node baseline: additional recurring costs, defer pending budget.
- kind on EC2: cheaper possible experiment but does not satisfy E13 EKS acceptance.
- Console-only creation: harder to reproduce/clean up; reserve UI for account authentication/bootstrap.

## Verification Strategy

Terraform fmt/init/validate, account-backed plan inspection, apply only after credentials and budget/lifetime are settled, then read EKS status and node readiness. Document exact versions and resource inventory. Run repository gates and record failures honestly. Review destruction plan and verify no unexpected retained chargeable resources at cleanup.

## Risks and Compatibility

- Budgets are delayed alerts, not hard limits. Node scale-to-zero does not stop control-plane charges.
- Spot may reduce cost but interruption complicates demonstrations; decide after live pricing.
- Public-node topology must be documented without claiming hostile-worker network isolation.
- Local state needs secure retention and one operator; losing it complicates teardown.
- Full E13 remains dependent on Bedrock provider and protected multi-user API implementation.
