# #45 reference Platform apply evidence

- Date: 2026-09-16
- Existing cluster/context: `agenova-k8s-lab` / `kind-agenova-k8s-lab`
- Platform: `deploy/reference/platform.kind.yaml`
- Compiled CLI: `cmd/agenova`
- Reference image: `agenova-control-plane:0.1.0`

## First apply

The read-only plan reported three exact adapter activations and four target changes. The first compiled-CLI apply returned:

```text
applied: true
ready: true
platform revision: sha256:5bbc6195ef78669bf189b2d210718ee922ff1eb136f5af4ff4875d77b4577fcc
deployment adapter: used
runtime adapter: configured
model adapter: configured
control plane: available
initial policy: configured (reference-default-deny@1)
```

## Idempotent second apply

The same compiled command and Platform file returned:

```text
changes: []
applied: false
ready: true
managed resource count before: 4
managed resource count after: 4
```

Managed resources remained exactly:

```text
configmap/agenova-platform
configmap/agenova-policy-reference-default-deny-v1
deployment.apps/agenova-control-plane
service/agenova-control-plane
```

The internal ClusterIP status endpoint returned the same revision and policy with `state: available`.
The seeded `policy.json` is the same canonical `reference-default-deny@1` baseline used by local admission: one explicit Team A engineer `claim.create` allow, all other assignments denied. A subsequent same-revision policy-content correction appeared as a plan change; after reconciliation, the next apply again reported `changes: []`.

## Denied real-cluster apply

A compiled `agenova platform apply --yes --json` was run against the existing kind target with a probe Platform that changed the desired revision. A test-only `kubectl` wrapper used Kubernetes impersonation rather than copying kubeconfig credentials. The temporary impersonated user could `get` the managed resources but could not `watch deployments.apps`; the plan contained three changes, so the command reached the apply preflight and exited `1` without mutation:

```text
preflight target kind-agenova-k8s-lab/agenova-system:
current Kubernetes identity lacks required RBAC: watch deployments.apps --namespace agenova-system
apply_exit=1
managed_before=4 managed_after=4
```

The existing Platform revision stayed `sha256:5bbc6195ef78669bf189b2d210718ee922ff1eb136f5af4ff4875d77b4577fcc`. The temporary ClusterRole and ClusterRoleBinding named `agenova-platform-readonly-test` were deleted after the check. The probe manifest and wrapper are test harnesses only; they do not enter the installed Platform. Fake-runner tests also prove that denied `watch` stops before the first mutation.

The adapter now applies resources one at a time. A fake-target failure injected at the Deployment step records the completed namespace and ConfigMaps as available/configured, that Deployment as failed, and the unattempted Service as pending. A current kind plan after these changes still reports `changes: []` for the original revision.
