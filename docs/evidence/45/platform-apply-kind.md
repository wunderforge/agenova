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

## Denied RBAC

A temporary service account with no Agenova namespace permissions used a temporary kubeconfig to run the compiled, read-only plan. It exited `1` before mutation:

```text
read Kubernetes configmap/agenova-platform: Error from server (Forbidden):
User "system:serviceaccount:default:agenova-platform-denied" cannot get resource
"configmaps" in namespace "agenova-system"
```

The temporary kubeconfig and service account were removed after the check. Fake-runner tests separately prove `apply` performs all `auth can-i` checks before issuing its first mutation.
