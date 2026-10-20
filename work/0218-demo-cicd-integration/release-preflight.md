# Shared EKS release preflight

19 October 2026. Owner requested testing application delivery through PR #223,
then AWS CLI and Agenova CLI verification. This report records read-only checks;
no workflow activation, image publication, deployment, PR merge or IAM change.

## Sources

- Release PR #223: `9fa78384c8db3b735955fb06bcaae01b37594cf9`.
- Local application candidate: `5da048d38f2560d548896b2b60d216ab4bbc78a9`.
- Main observed during preflight: `ae24a67261dff289a7063f7435c7c3616fd36dc0`.
- Candidate remains local; no remotely runnable integration revision published.
- Owner confirmed this round excludes task-type conditional Policy integration.

## Verified

| Check | Observed result |
| --- | --- |
| AWS CLI identity / EKS describe | Account `931228356546`, profile `agenova-neo`, region `ap-southeast-2`; `agenova-demo` ACTIVE, Kubernetes 1.35 |
| Kubernetes read-only projection | `agenova-system/agenova-member-control-plane`: one desired/ready/available replica, Recreate, member auth, readiness probe present |
| Current control-plane digest | `sha256:282d9e32e73a87f562088c28a35490e7b4ddca2ae3a60192b3f3075091d6b10e`; ECR tag `portal-cleanup-20261019`, not a verified source SHA |
| Current registered Worker | Managed `agenova-template-slot`, engineer; `sha256:ec74e41c6348b4e4d10d4da3ab1ee0561d44c1e05964487adb2cc4a9c7b4ca9e`; actual #223 workerSnapshot passed |
| Agenova CLI | Authenticated `work list --server https://app.agenova.app --json` succeeded, five records; no Work submitted |
| Release tests | `node --test scripts/release/release.test.mjs`: 28 passed; subprocesses are test doubles, not live OIDC/rollout proof |
| GitHub configuration | Only CI currently listed; Shared EKS release not available on main. Environment branch policy explicitly main-only. Repository release/deploy enable switches and initial baseline variable absent |
| Actual release Deployment snapshot | #223 snapshot function against live object, without invented baseline: blocked with `A full source commit SHA is required`; source annotation absent |
| Source-only image plan | Actual #223 plan function, PR #216 source to local candidate: controlPlane=true, worker=true. This comparison is NOT claimed as the deployed-source diff |

Full Deployment/ConfigMap responses stayed in process memory; only selected
non-secret metadata was printed. A separate generated kubeconfig is under the
ignored `.tmp/cicd-verification` path; existing kubeconfig was not modified.

## Required before a real release

1. Review application/release source and authorize merge order. The workflow and
   application candidate cannot currently be released from main; deploying old
   main must not replace the public member-auth service.
2. Identify the actual running control-plane/Worker compatible source pair and
   establish a truthful initial baseline. Do not substitute PR #216 or an ECR
   human-readable tag for verified deployed-source provenance.
3. Provide a reviewed compatible Worker/template migration. #223 deliberately
   blocks control-plane rollout when Worker changes; immutable registration must
   not be deleted/overwritten just to bypass that protection.
4. Resolve the outstanding PR review concerning source-annotation revalidation
   after failed-rollout recovery, before enabling shared-cluster deployment.
5. Once these prerequisites are satisfied, enable publication first, inspect
   immutable ECR candidate evidence, then separately enable deployment. Follow
   with AWS image/rollout verification and real Agenova authenticated Work smoke.

Existing cluster readiness and authenticated querying do not certify deployment
of the integration candidate, live Tool service acceptance, OTEL, or resolution
of the previously open Tool credential finding.
