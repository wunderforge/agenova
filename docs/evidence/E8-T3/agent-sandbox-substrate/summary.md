# Evidence Summary

- Ticket: E8-T3 (#50)
- Gate: agent-sandbox-substrate
- Date: 2026-09-10
- Branch / commit: neo/e8-t3-kind-agent-sandbox / `0072e38`
- Script SHA256: `6a307daf9efad06ad6e7ec1ef897fe35dac9b3caaebde4c3e62783cd4ae743e5` (clean working tree)
- Command: `bash harness/spike/agent-sandbox-substrate/reproduce.sh all --capture` (run twice)
- Host: Darwin arm64, Docker Desktop, existing Homebrew `kind v0.32.0` + `kubectl v1.36.2` (both resolved as `existing`; nothing downloaded)
- Cluster / context: `agenova-k8s-lab` / `kind-agenova-k8s-lab`, node `kindest/node:v1.36.1`, server `v1.36.1`
- Upstream: Agent Sandbox **v0.4.6** / `extensions.agents.x-k8s.io/v1alpha1`, controller `registry.k8s.io/agent-sandbox/agent-sandbox-controller:v0.4.6`
- Result: **pass** (two consecutive `all` runs)

## Runs

| # | Command | Cluster | Result |
| --- | --- | --- | --- |
| 1 | `up --capture` then `all --capture` | reused (created by the `up`) | pass |
| 2 | `up --capture` then `all --capture` | fresh (new cluster) | pass |

`all` = `up -> smoke -> teardown -> down`. In each `all` the `up` phase re-used the
just-created owned cluster (`reusing cluster with matching ownership fingerprint`)
and re-applied the manifests idempotently (`unchanged` / `configured`).

## What each run proved (`output.txt` is run 2, raw)

- `kind` / `kubectl` resolved from `PATH` as `existing` and recorded; no downloads.
- Disposable `kind` cluster created, control-plane Ready in ~18-19s; kube context verified as `kind-agenova-k8s-lab` before every mutation.
- Ownership receipt (Docker control-plane container ID + `kube-system` UID) written to `.tmp/agenova-k8s-lab-owner/identity` and re-verified on every reuse / mutation / delete.
- Pinned Agent Sandbox **v0.4.6** `manifest.yaml` + `extensions.yaml` downloaded and applied; controller Deployment rolled out Ready on image `registry.k8s.io/agent-sandbox/agent-sandbox-controller:v0.4.6`.
- CRDs recorded: `sandboxes` / `sandboxtemplates` / `sandboxwarmpools` / `sandboxclaims` — all `v1alpha1 served=true storage=true`. Server `v1.36.1`.
- `SandboxTemplate` -> `SandboxWarmPool(replicas:1)` -> `SandboxClaim` created; `SandboxClaim/smoke-claim` reached `Ready=True` with `status.sandbox={"name":"smoke-claim","podIPs":["10.244.0.10"]}`; sandbox pods `smoke-claim` (+ `smoke-pool-*`) `Running`.
- Teardown deleted claim + pool + template, asserted sandbox pods -> 0, deleted the `agent-sandbox-smoke` namespace.
- `down` re-verified context + receipt and deleted **only** `agenova-k8s-lab` (unrelated local `kind` clusters untouched), removing the receipt.

## Environment note — TLS interception

This machine's network runs TLS inspection (Zscaler). The `kind` node's containerd
ships its own CA bundle and otherwise rejects
`registry.k8s.io` / `docker.io` pulls with
`x509: certificate signed by unknown authority` (the host, which trusts the
inspection CA, pulls the same images fine; `reproduce.sh` reports this as a
non-zero failure, never a silent pass).

For these runs the disposable node was made to trust the inspection root once per
cluster, before the controller rollout completed:

```
docker cp <zscaler-root>.crt agenova-k8s-lab-control-plane:/usr/local/share/ca-certificates/
docker exec agenova-k8s-lab-control-plane update-ca-certificates
docker exec agenova-k8s-lab-control-plane systemctl restart containerd
```

This is the standard kind workaround for a TLS-inspecting corporate network and
touches only the throwaway node — `reproduce.sh` itself is unmodified. A run from
a network without interception needs no such step; a reviewer wanting that pristine
form can re-run `all --capture` off the corporate network.

No Agenova API type, `RuntimeBackend`, claim / authority / gateway path is
exercised here (that is E8-T4 / #51).

Raw output: [output.txt](output.txt).
