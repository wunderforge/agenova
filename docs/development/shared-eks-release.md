# Shared EKS image delivery

## Manual first-pair candidate trial

For an explicitly authorized integration test before merge, the existing **CI**
workflow can dispatch this reusable workflow from one exact candidate branch.
`SHARED_EKS_CANDIDATE_ENABLED`, `_REF`, `_SHA`, `_UID` and `_PREVIOUS_IMAGE` pin
the operation. PR events, branch pushes, other refs and changed source SHAs cannot
release. The full Linux source/race/browser gate runs before OIDC access.
Automatic main releases remain separately disabled until explicitly activated.

The first-pair path builds **both** images. It records an unknown deployed source
as null, not an invented Git baseline. The member CP image includes the Portal
at `/opt/agenova/portal` (`deploy/production/release.Dockerfile`). The pipeline
atomically updates the CP image and `AGENOVA_ALLOWED_WORKER_IMAGE`, preserving
all other configuration. It verifies Deployment UID/resource version, ready
state, registry/Platform hashes and source annotation; a failed rollout restores
both image values only while those bindings remain unchanged. No Secret reads,
Policy/template writes, database migrations or external service installs occur
inside this workflow. Register a **new** project template and policy version
after rollout through authenticated Agenova CLI; retain old immutable records.

Operator procedure (exact parameters come from a fresh read-only preflight):

```powershell
node scripts/release/candidate-access.mjs authorize refs/heads/codex/BRANCH FULL_SHA DEPLOYMENT_UID OLD_CP_DIGEST_REFERENCE
gh workflow run ci.yml --repo wunderforge/agenova --ref codex/BRANCH
# Inspect Actions artifacts; verify AWS/EKS digests and real Agenova Work.
node scripts/release/candidate-access.mjs revoke
```

The access helper changes no IAM permission policy or cluster RBAC. It saves a
non-secret receipt, temporarily adds only the exact publisher OIDC subject and
environment branch, and refuses to overwrite concurrent trust changes during
revoke. Always revoke even when a trial fails. No default release flags are
enabled by this mode. A repeated authorization needs an operator-reviewed receipt,
not deletion of the previous receipt to bypass its checks.

The trial intentionally pins the reviewed login foundation `88888a2`; newer
schema-009/proxy work requires separate migration and rollback acceptance.

## Ordinary main delivery

This is delivery tooling for **Neo's existing shared demo cluster**, not an EKS
installer or a production upgrade system. It does not create/stop a cluster,
change IAM, read Secrets, alter Policy, or install the Sandbox controller.

**Publication and deployment are disabled unless explicitly enabled.** Opening
or merging this PR does not by itself publish an image or update the cluster.
The implementation was tested offline; a release-ready commit and live validation
belong to the operator's separate integration exercise.

## Flow

```text
main push / manual run
  -> full source checks (no AWS credentials)
  -> opt-in: read current Deployment and deployed source SHA
  -> diff deployed source..candidate (not previous push)
  -> build/publish only affected images, or reuse immutable SHA tags
  -> release-candidate artifact with digests
  -> separate deployment opt-in
  -> reject Worker changes requiring template migration
  -> reject stale main / changed Deployment
  -> patch only control-plane image, wait for readiness
  -> record successful source SHA; retain failure/recovery evidence
```

Publication uses a main-ref OIDC role; deployment and the read-only snapshot use
an environment-scoped OIDC role. Do not add `environment` to the publisher job:
that changes its OIDC subject and will not match the existing publisher trust.
There are no AWS access keys in GitHub. Official Actions are pinned to commits.

## What updates automatically

| Change | Image publication | Cluster update |
| --- | --- | --- |
| Control-plane Go dependency or Dockerfile | Control plane only | Selected container only |
| Worker Go dependency or Dockerfile | Worker only | Blocked pending reviewed template migration |
| Shared protocol / Go module lock | Both | Blocked pending compatible pair deployment |
| Documentation or Go unit-test-only change | None | None |
| UI, login proxy, Collector, other standalone service | Not part of these two images | Separate deployment required |

Go dependencies are discovered with `go list -deps`. A new package imported by a
Worker is therefore included, not silently assumed to be control-plane-only.
Docker build contexts and their Dockerfiles must still include those sources.

Tags are `sha-<full 40-character commit>`. ECR repositories must remain immutable.
Existing tags are reused; only `ImageNotFoundException` permits a new build.
Authorization/network errors fail, rather than masquerading as missing images.
Candidate images use `linux/amd64`, matching this shared-cluster release target;
other node architectures require an explicitly reviewed build-platform change.

## One-time activation — by the release operator, not this PR

1. Merge/review the compatible application code first. In particular, the source
   must contain the member authentication implementation used by the public
   cluster. Do not overwrite the member service with an older reference-only
   main image. Coordinate a quiet deployment window with the team.
2. Confirm the running control plane **and registered Worker** are a compatible
   pair built from an identified source commit. Set repository variable
   `SHARED_EKS_BASE_SHA` to that full commit SHA for the first run. It must be an
   ancestor of the release candidate. Never set it to an arbitrary recent commit
   just to skip changes. An unknown baseline is a stop condition.
3. Confirm the target has one replica, `Recreate`, a digest-pinned ECR image,
   member auth/config environment, and a control-plane readiness probe. The
   managed `agenova-template-slot` must also reference a digest-pinned image in
   the expected Worker ECR repository; its contents are hashed and checked for drift.
   workflow rejects other installation shapes. Confirm GitHub-hosted runners can
   reach the EKS API and have AWS CLI v2, kubectl, Docker/buildx available.
4. Set repository variable `SHARED_EKS_RELEASE_ENABLED=true`. Leave
   `SHARED_EKS_DEPLOY_ENABLED` unset/false. Manually run **Shared EKS release** on
   **main**, selecting **publish=true**, **deploy=false**. Inspect the candidate
   artifact and validate real OIDC/ECR behavior. No cluster mutation is made in
   this mode, although ECR images are published.
5. Once reviewed, set repository variable `SHARED_EKS_DEPLOY_ENABLED=true` and
   rerun on main with both inputs true. The immutable tag is reused. These two
   enabled variables also activate subsequent automatic main-push releases.

Both switches must be **repository variables**, not only environment variables,
because job admission is evaluated before environment variables become available.
Manual dispatch with default inputs runs only local validation. It is not a
deployment dry run and does not claim to verify EKS access.

Existing non-secret configuration:

| Location | Variables |
| --- | --- |
| Repository | `AWS_REGION`, `AWS_PUBLISH_ROLE_ARN`, `ECR_CONTROL_PLANE_REPOSITORY`, `ECR_WORKER_REPOSITORY` |
| `shared-eks` environment | `AWS_DEPLOY_ROLE_ARN` |
| Repository, operator activation | `SHARED_EKS_RELEASE_ENABLED`, `SHARED_EKS_DEPLOY_ENABLED`, initial `SHARED_EKS_BASE_SHA` |

The workflow explicitly targets account `931228356546`, region `ap-southeast-2`,
cluster `agenova-demo`, namespace `agenova-system`, Deployment
`agenova-member-control-plane`, container `control-plane`. Environment branch
policy must remain `main` only. Existing target-name environment variables are
not used to redirect this workflow; changing target requires reviewing the file.

## Safety, compatibility and rollback limits

- A workflow-wide concurrency group serializes snapshot, publication and rollout.
  Running rollouts are not cancelled by new pushes. GitHub may supersede pending
  runs; diffing from the last deployed SHA preserves cumulative changes.
- Deployment rechecks current main immediately before cluster access. A newer
  push causes a stale candidate to fail; there is still no distributed lock
  against changes arriving after this check. Kubernetes resource-version tests
  protect the actual patch from concurrent object mutations.
- Only the named container's image is replaced. UID, resource version, current
  image and a hash of the rest of the Pod template/strategy/replicas are checked.
  Sidecars, authentication, credential references and service account remain
  unchanged. Source annotation is updated only after readiness succeeds.
- Readiness proves rollout convergence, **not** Google login, Work execution or
  application-schema compatibility. Those smoke checks remain the operator's
  first-release acceptance. Do not present this as full product E2E validation.
- On rollout failure, restore the previous image only if UID, image, source and
  configuration still match the attempted release; otherwise stop for the
  operator. A failed release remains failed even when recovery succeeds.
  Cancellation, runner loss, migrations and in-memory Work loss are not fixed by
  image rollback. This is not zero-downtime/draining or transactional recovery.
- **Worker template migration is deliberately not implemented.** The current
  registry uses an immutable slot. Publishing a Worker image does not change
  that slot or any existing worker Pod. A candidate with Worker changes cannot
  automatically replace the control plane. Establish a compatible pair through
  a separately reviewed registration/upgrade path, then explicitly re-baseline
  the `delivery.agenova.io/source-sha` Deployment annotation to the verified
  commit. Never delete/recreate the slot merely to bypass this guard.
- **This workflow updates an image, not the declarative Platform revision.** It
  leaves `agenova-platform`, Policy and template records untouched. Deployment
  source annotation and release artifacts identify the binary version. A later
  `platform apply` must use a reviewed configuration with the chosen image;
  stale installation configuration can otherwise restore an older image or
  report drift. Full Platform reconciliation remains separate work.
- Kubernetes patch rights are object-scoped, not field-scoped. The script checks
  constrain normal releases but do not protect against malicious code approved
  on main. Main review, OIDC trust and environment restrictions remain essential.

Set both enable variables to false to stop future cloud jobs. That does not undo
an already running release; coordinate with its operator before cancelling.
No cleanup command in this workflow destroys the shared cluster or ECR repos.

## Evidence and local verification

Artifacts retained for 14 days:

- `release-snapshot`: UID, resource version, image, source SHA, configuration hash,
  and registered Worker image/template hash;
- `release-candidate`: source diff plan, changed-image digests and both images
  in the proposed pair (unchanged images retain the installed digest);
- `deployment-result`: previous/desired image and ready/failed/recovery result.

No kubeconfig, OIDC token, Docker login password, complete Deployment or Secret
is uploaded. AWS/EKS permissions must be reviewed independently; offline mocks
are not proof that live OIDC exchange or cluster RBAC succeeds.

```powershell
node --test scripts/release/release.test.mjs
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7 -shellcheck= .github/workflows/shared-eks-release.yml .github/workflows/ci.yml
./scripts/check.ps1 -All
```

The first two commands do not use AWS, Docker or Kubernetes: subprocess calls in
the behavioral tests are fakes. CI runs these checks without `id-token: write`.
The full gate includes existing local Playwright tests, not the shared cluster.

References: [GitHub OIDC for AWS](https://docs.github.com/en/actions/how-tos/secure-your-work/security-harden-deployments/oidc-in-aws),
[GitHub concurrency](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency).
