# Task: EKS Bedrock reference-identity demo slice

- Ticket: [#200](https://github.com/wunderforge/agenova/issues/200)
- Mission: Deliver the bounded cloud demo slice without implying completion of parent E13.
- Target: deploy/demo/eks, infra/demo, internal/modelprovider, internal/adapters/bundled.
- User value: Install and exercise real governed Bedrock Work on EKS with an authenticated HTTPS demo.
- PRD outcome: [Reference installation](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap).

## Context to Read

- `AGENTS.md`, `docs/product/prd.md`, this packet.
- Existing [slice spec](../0175-shared-eks-bedrock/spec.md), [design](../0175-shared-eks-bedrock/design.md), and [Owner decisions](../0175-shared-eks-bedrock/task.md).
- [Architecture contract](../../docs/product/architecture-contract.md).
- [Cloud runbook](../../deploy/demo/eks/README.md) and [evidence](../0175-shared-eks-bedrock/evidence/summary.md).

## Scope

In scope: existing compatible remote images, native Bedrock adapter, disposable Terraform EKS/ALB, HTML demo sessions, repeatable installation and cloud positive/negative evidence.

Out of scope: real multi-user identity (#197), hostile-worker isolation, multiple models, durable Work history, production installation lifecycle. The existing shared reference identity remains explicit.

## Acceptance Criteria

- Platform installation/status and identical reapply work on EKS.
- Allowed Work returns real Nova Micro evidence; denied admission produces zero additional provider calls.
- Worker manifests have no provider credential binding; HTTPS session login/logout and anonymous denial work.
- Teardown evidence preserves permanent domain/certificates; restore checks use the same validated images.

## Negative Case

Denied Work creates no claim/provider call; invalid provider configuration never falls back; invalid sessions and cross-origin writes fail closed.

## Execution Todo

- [x] Scout the existing cloud implementation and evidence.
- [x] Assignee self-review against #200 and PRD: this separates the Owner-approved delivered slice from incomplete parent Epic acceptance.
- [x] Implement remote images, Bedrock and authenticated demo perimeter in PR #196.
- [x] Capture initial cloud positive/negative and teardown evidence.
- [x] Complete the requested fresh restore and focused checks.
- [x] Run final full gate and review the diff.

## Quality Gates

- `go test ./internal/modelprovider ./internal/adapters/bundled ./internal/platformapply ./cmd/agenova-control-plane ./internal/connectedclient ./internal/console`
- `/usr/bin/python3 deploy/demo/eks/public/verify.py`
- `pwsh -NoProfile -File ./scripts/check.ps1 -All`

## Evidence Required

Existing cloud transcripts and screenshots under `work/0175-shared-eks-bedrock/evidence`, fresh restore status/Work/perimeter checks, and exact CI outcome.

## Constraints

Preserve architecture boundaries, AUD100/month demo budget, explicit cleanup deadline, domain and certificates. Credentials stay outside Git and workers.

## Decisions and Blockers

- 2026-10-05: PR #196 failed the required PR-body contract before code tests. A separate bounded ticket prevents falsely closing #175 to satisfy the harness. No implementation scope expansion.
- E14 #197 has planning-only commits and no PR. E15 bypass proof and live kind/Ollama regression remain parent-integration gaps; no claim of their completion.
