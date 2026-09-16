# Reference Platform install on kind

This is the bounded MVP installation path for an **existing** Kubernetes test cluster. It does not create a cluster or embed cluster credentials.

Prerequisites:

- Docker, `kind`, and `kubectl`
- an existing cluster named `agenova-k8s-lab` with context `kind-agenova-k8s-lab`
- current Kubernetes identity authorized to create/patch the named reference resources

```powershell
docker build -f deploy/reference/Dockerfile -t agenova-control-plane:0.1.0 .
kind load docker-image agenova-control-plane:0.1.0 --name agenova-k8s-lab
go build -o .tmp/agenova.exe ./cmd/agenova

.\.tmp\agenova.exe platform validate -f deploy/reference/platform.kind.yaml --json
.\.tmp\agenova.exe platform plan -f deploy/reference/platform.kind.yaml --state-dir .tmp/platform-state --json
.\.tmp\agenova.exe platform apply -f deploy/reference/platform.kind.yaml --state-dir .tmp/platform-state --yes --json
.\.tmp\agenova.exe platform apply -f deploy/reference/platform.kind.yaml --state-dir .tmp/platform-state --yes --json

kubectl --context kind-agenova-k8s-lab get --raw `
  '/api/v1/namespaces/agenova-system/services/http:agenova-control-plane:8080/proxy/v1/status'
```

Expected behavior:

- `plan` is read-only and lists exact adapter activations plus target changes.
- first `apply` reports `applied: true`, `ready: true`, and `readinessScope: "installation-components"`;
- second identical `apply` reports `changes: []`, `applied: false`, and the same scoped readiness;
- the namespace contains two managed ConfigMaps, one Deployment, and one ClusterIP Service;
- runtime and model adapters are configured independently; their external prerequisites are not installed by the deployment adapter.

`ready` means the declared reference installation components reconciled. The
internal status process is not a claim-serving control plane, and this check
does not prove an end-to-end agent run or external runtime/model availability.
For a drifted install, `apply` writes only the target resources named in the
confirmed plan.

The Platform file owns Kubernetes context and namespace. There are intentionally no root `--kube-context` or `--namespace` flags.
