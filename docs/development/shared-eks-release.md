# Shared EKS image delivery

## Automatic main delivery

After green Linux source/race/browser checks, a main push automatically publishes
affected artifacts and deploys the control-plane image to **Neo's existing demo
cluster**. Unset switches default on. Explicit repository variables
`SHARED_EKS_RELEASE_DISABLED=true` or `SHARED_EKS_DEPLOY_DISABLED=true` stop future
publication/deployment; unset/`false` permits automatic delivery. Obsolete opt-in
`SHARED_EKS_RELEASE_ENABLED` / `SHARED_EKS_DEPLOY_ENABLED` variables no longer gate
delivery, including retained `false` values from old installations. This is the
Owner-approved switch to automatic delivery on merge, not an instruction to
change live variables or deploy this unmerged PR. Invalid new switch values fail.

```text
main push -> full checks + offline event diff
  -> no artifact changes: stop before AWS authentication
  -> snapshot deployed control-plane source/configuration
  -> diff deployed source..candidate; publish affected immutable images
  -> control plane changed: quiet-window check, guarded rollout, readiness
  -> Worker only: publish reference artifact; no deployment/template changes
```

| Change | Publication | Cluster update |
| --- | --- | --- |
| Control-plane dependency / Dockerfile / Portal | Control plane | Named container image only |
| Reference Worker dependency / Dockerfile | Worker | None; authors select new images in new templates |
| Shared protocol / Go module lock | Both | Control plane only; existing templates retain images |
| Documentation / unit-test-only push | None | No cloud access |
| Gateway service / Collector / login proxy | Not managed by these two images | Separate service delivery |

Components use `go list -deps`, not a hardcoded shared-package map. Artifact
pushes diff from the installed source, so skipped/superseded changes are included.
A docs-only push intentionally does not flush pending artifact changes; the next
artifact push or explicit manual publish does. Manual dispatch defaults to
validation only; select publish/deploy explicitly for cumulative delivery.
PR events cannot access cloud roles. Official release Actions are commit-pinned.

Worker publication is **not** Worker deployment: users register immutable
AgentTemplates through authenticated Agenova CLI/API. Existing templates/pools
are never rewritten, and Worker-only publication does not advance the platform's
deployed source annotation. See the [image authoring guide](agent-images.md).
Protocol compatibility remains a review/test obligation: an incompatible shared
protocol change requires a versioned migration, not automatic user-image updates.

## Existing target and prerequisites

Target: account `931228356546`, region `ap-southeast-2`, cluster `agenova-demo`,
namespace `agenova-system`, Deployment `agenova-member-control-plane`, container
`control-plane`. No cluster, IAM role, Secret, volume or other cloud resource is
created. This workflow does not install services, run migrations or reconcile
Platform configuration.

| Location | Required configuration |
| --- | --- |
| Repository | `AWS_REGION`, `AWS_PUBLISH_ROLE_ARN`, `ECR_CONTROL_PLANE_REPOSITORY`, `ECR_WORKER_REPOSITORY` |
| `shared-eks` environment | `AWS_DEPLOY_ROLE_ARN`; main-only deployment policy |
| First identified deployment only | `SHARED_EKS_BASE_SHA` if no Deployment source annotation exists |
| Repository, optional emergency stops | `SHARED_EKS_RELEASE_DISABLED=true`, `SHARED_EKS_DEPLOY_DISABLED=true` |

Publisher OIDC trust is main-ref-scoped; snapshot/deployment use the environment
role. Do not add an environment to the publisher job: it changes the OIDC subject.
No AWS access keys belong in GitHub. Existing environment approval rules apply;
this PR does not bypass them.

The target needs one replica, `Recreate`, a digest-pinned control-plane image,
member auth/configuration, readiness probe and identified ancestral source SHA.
Unknown baselines stop delivery: never invent one. The deploy identity needs list
access to SandboxClaims for the quiet-window check. Runners need AWS CLI, kubectl,
Docker/buildx and EKS connectivity.

Images use immutable `sha-<full source SHA>` tags and repository-bound digests on
`linux/amd64`. Only ECR `ImageNotFoundException` permits building; access/network
errors stop. Reference Worker publication needs its ECR repository; user images
need not use that registry.

## Guardrails and limits

- Releases serialize without cancelling a running rollout. Main must still match
  the tested SHA immediately before deployment; this is not a distributed lock.
- Refuse rollout while any upstream SandboxClaim exists in the target namespace;
  retry in a coordinated quiet window. This is not admission locking/draining:
  incoming Work can race the check. Avoid submitting Work during deployment.
- Patch only the named control-plane image. Check UID, resource version, old
  image, source annotation and the rest of the Pod configuration. Preserve
  authentication, sidecars, service accounts and credential references.
- Record source only after readiness. Readiness does not prove Google login,
  schema compatibility or successful Work; those require application smokes.
- Failure restores the old image only while configuration, identity, source and
  attempted image still match. A recovered release remains failed. Concurrent
  changes require an operator and are never overwritten.
- No template slot, project registry or Policy writes occur. Deprecated
  `AGENOVA_ALLOWED_WORKER_IMAGE` is preserved but ignored by the new control plane.
- `agenova-platform` is not updated. A later `platform apply` needs reviewed image
  configuration or may restore an older control plane.
- Migrations, external services, runner loss, malicious approved main code, zero
  downtime and schema rollback are not solved by image delivery. Kubernetes
  patch rights are object-scoped, not field-scoped. Budget policy remains separate.

## Legacy pre-merge first-pair trial

`candidate.mjs` and `candidate-access.mjs` retain the explicitly authorized legacy
first-pair migration path, not ordinary Worker delivery. It pins an exact manual
ref/SHA/Deployment UID/previous image, builds both images and changes the old image
environment binding atomically. Registry/Platform hashes and pair rollback are
checked; templates are not rewritten. It requires separate operator authorization,
temporary exact OIDC trust and revocation after the trial. It cannot run from PR
events or ordinary branch/main pushes. Do not use it to force user-image updates.
