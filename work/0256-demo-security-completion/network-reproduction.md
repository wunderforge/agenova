# Isolated local network acceptance

This creates a dedicated **local** kind cluster, not the shared AWS cluster.
Docker must already be running. No credential files are read. Model responses
are deterministic protocol fixtures, explicitly not real-model acceptance.

From the repository root (PowerShell):

```powershell
New-Item -ItemType Directory -Force .tmp/demo-256 | Out-Null
kind create cluster --name agenova-256-network --config work/0256-demo-security-completion/kind.yaml --kubeconfig .tmp/demo-256/kind.kubeconfig
curl.exe --fail --location https://raw.githubusercontent.com/projectcalico/calico/v3.33.0/manifests/calico.yaml --output .tmp/demo-256/calico-v3.33.0.yaml
(Get-FileHash .tmp/demo-256/calico-v3.33.0.yaml -Algorithm SHA256).Hash
# Require 2de8f47595fb9c41b3f47d7b767a1f8e72ecf84057af834738ff12689a234da5 before applying.
kubectl --kubeconfig .tmp/demo-256/kind.kubeconfig --context kind-agenova-256-network apply -f .tmp/demo-256/calico-v3.33.0.yaml
kubectl --kubeconfig .tmp/demo-256/kind.kubeconfig --context kind-agenova-256-network -n kube-system rollout status daemonset/calico-node --timeout=180s
```

Build the allowlisted public source context containing only go.mod, go.sum,
internal/workerprotocol, cmd/agenova-demo-worker,
harness/integration/agentsandbox/testworker and the Dockerfile below. Never
include untracked files or credentials. Verify public DNS for jitsi.org before
using the pinned diagnostic IP; TLS continues to validate jitsi.org.

```powershell
docker build --platform linux/amd64 --build-arg NATIVE_FALLBACK_IP=172.64.154.22 -f harness/demo/research-mcp/worker.Dockerfile -t agenova-researcher:256-pinned <reviewed-public-context>
kind load docker-image agenova-researcher:256-pinned --name agenova-256-network
kubectl --kubeconfig .tmp/demo-256/kind.kubeconfig --context kind-agenova-256-network apply -f work/0256-demo-security-completion/network-local.yaml
kubectl --kubeconfig .tmp/demo-256/kind.kubeconfig --context kind-agenova-256-network -n agenova-256-network wait --for=condition=Ready pod --all --timeout=120s
python -B work/0256-demo-security-completion/network_local.py
```

The committed generated record in `evidence/local-network.json` retains exact
Pod UIDs, image IDs, same-node/address controls, bounded observations and checks.
The denied Pod is the same Pod throughout the protocol exchange. The standalone
control probe is not a model-generated action. Local CNI results never substitute
for separate real-model AWS evidence.

After preserving evidence, remove only this dedicated local cluster:

```powershell
kind delete cluster --name agenova-256-network
```
