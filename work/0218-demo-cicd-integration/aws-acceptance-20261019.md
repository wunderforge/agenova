# Shared EKS CI/CD acceptance — 19 October 2026

Owner authorized the exact branch trial, shared-service deployment and merge after
AWS/Agenova CLI acceptance. Reviewed login source remains #216 `88888a2`;
migration 009/proxy changes are explicitly excluded and remain on #216.

## Actual delivery

- Source: `44dc383c9d8cb1ed5679677d78ed6a289a1944bc`.
- [Successful real Actions run](https://github.com/wunderforge/agenova/actions/runs/39244695886):
  Linux full/race/UI gate, OIDC snapshot, ECR publication and paired rollout all
  succeeded. Artifacts: release-snapshot, release-candidate, deployment-result.
- AWS account `931228356546`, region `ap-southeast-2`, existing cluster
  `agenova-demo`; no cluster/database/ALB/Secret/role or external service created.
- Deployment `agenova-system/agenova-member-control-plane`, UID
  `514f9f20-5bc5-442a-a570-465e4412c6d8`: generation/observedGeneration 8,
  one ready/updated replica and source annotation equal to the released SHA.
- CP ECR digest:
  `sha256:ad2c248279b0023b77d78c51af7a85ea13fd2f7dbe5add08f3fbee16291ecff7`.
- Worker ECR digest:
  `sha256:90e65cc2cf23ec77d21cb6a4dfbf8573aa93f798c79acab4f79f9157ab26cc09`.
  Both repositories enforce immutable tags; AWS CLI independently matched the
  `sha-44dc383c9d8cb1ed5679677d78ed6a289a1944bc` tags to these digests.

The pipeline changed only CP image and its explicit compatible-Worker digest,
then recorded successful source. Login/database/Secret references, sidecar,
service account, Platform and old immutable reference slot were preserved.
New project registry records were subsequently created through authenticated CLI,
not by rewriting the slot or deleting history.

## Real Agenova CLI acceptance

Endpoint: `https://app.agenova.app`. Existing personal Google application session,
OS credential store; no shared cloud credentials used by application CLI.

- Registered new immutable templates `ci-engineer-44dc383c9d8c` and
  `zz-ci-denied-44dc383c9d8c` in `payments`, both pointing to the new Worker.
- Activated `reference-default-deny@cicd-44dc383c9d8c`, preserving existing rules
  and adding only the allowed new template. Original policy remains retained.
- Denied Work:
  `work-3adf31c17ce4e8c1c53fcb7f424f056ec4fd21e9ad72a87ea67b4c1caaf9c286`.
  CLI exit 1, Deny outcome, two facts, no Claim and zero provider/tool invocations.
- Allowed Work:
  `work-4cca16f8dd16343c7b095232565eff9993ee852951cc479ac8d1dac5ee6e554e`.
  CLI exit 0, Allow/Succeeded, real Agent Sandbox execution and three successful
  Bedrock provider calls, 30 facts and CleanupSucceeded. Task requested no tools
  and performed no external business operation.
- `work show` succeeded after completion. Resubmitting the identical allowed YAML
  returned the same reference, 30 facts and three original provider attempts;
  inference was not replayed. `work list` returned seven records: five retained
  historical successes, the new success and new denial.
- Executed worker Pod `agenova-pool-pool-ci-engineer-44dc383c9d8c-k2hgk` was
  independently confirmed absent after cleanup. The controller replenished its
  warm pool with a different Pod (`...-cfpb5`) using the published Worker digest.
  Observed replenished Pod: automountServiceAccountToken=false, zero Secret env
  references/envFrom/volumes/projected service-account tokens. This is a bounded
  configuration observation, not a complete hostile-agent credential proof.

## Gates, failure and cleanup

- Earlier [run 39243523122](https://github.com/wunderforge/agenova/actions/runs/39243523122)
  passed Linux validation/OIDC snapshot but failed image publication; no deploy.
  Local reproduction showed missing Go in the Portal Vite builder. Added the
  locked build-only Go toolchain/internal fixture sources and public build logs;
  actual pure-Git-context member image then built locally and in CI. Portal asset
  presence and missing-member-config fail-closed smoke also passed locally.
- Local release regressions: 37 passed after the final non-runtime provenance
  correction (ordinary releases report the active Worker binding, not the retained
  legacy slot). This does not change deployed Go/Portal/Worker source.
- Native application full gate previously passed 128 UI unit/56 browser tests;
  fresh verify-full TLS PostgreSQL six-package member/Tool/durable campaign passed.
  Real Actions ran the Linux/race/browser gate before any cloud mutations.
- Temporary exact-branch publisher trust and environment branch policy were
  revoked after success; candidateEnabled=false and original main trust restored.
  No ordinary main publication/deployment switches were enabled.
- No production secret file, OAuth JSON, database URL or private AWS state read;
  no teammate role/session changed, no external MCP/OTEL deployment or DB migration.

## Explicit limits

Installed Tool capability remains labelled `mock`; real downstream MCP services,
credentials and OTEL remain separately owned. Task-type policy was not integrated.
The private Tool credential finding, latest #216 migration/release campaign,
general rolling upgrades/HA and hostile-agent/network escape acceptance remain
open. New compatible template names must be used for new Work; old immutable
`engineer` retains its old image and is not silently rewritten. The retained warm
pool is intentional application state, not a leaked second cloud cluster.

Reproduction: [shared member trial guide](../../docs/development/shared-eks-trial.md).
