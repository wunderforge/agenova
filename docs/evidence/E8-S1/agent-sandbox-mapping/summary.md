# E8-S1 Agent Sandbox Mapping Evidence

- Ticket: [#66](https://github.com/wunderforge/agenova/issues/66)
- Pull request: [#105](https://github.com/wunderforge/agenova/pull/105)
- Date: 2026-09-13
- Experiment commit: `3ce542bf951a2cbab1533cda3befabbcf922415b`
- Upstream substrate: Agent Sandbox `v0.4.6`
- Docker Engine: `29.7.2`
- kind: `v0.33.0`
- kubectl client / Kubernetes server: `v1.36.1` / `v1.37.0`
- Cluster / explicit context: `agenova-k8s-lab` / `kind-agenova-k8s-lab`
- Raw-output SHA256: `f3d67945b04265ed7909816dd9ee19afaa5b276d36dff1c91de0849a2479a664`
- Result: pass

## Exact Commands

From the repository root:

```sh
bash harness/spike/agent-sandbox-substrate/reproduce.sh up --capture
bash harness/spike/agent-sandbox-mapping/reproduce.sh kind-agenova-k8s-lab
bash harness/spike/agent-sandbox-substrate/reproduce.sh down --capture
```

The mapping script records all four installed CRD schemas through
`kubectl explain --recursive`, then runs only:

```sh
go test -count=1 -v -tags integration -timeout 5m \
  ./harness/integration/agentsandbox/ \
  -run '^TestRuntimeBackend_AllocateObserveCleanup$' \
  -args -kube-context kind-agenova-k8s-lab
```

## Demonstrated Result

The adapter allocated a unique claim, preserved the assigned backend identity,
and observed the upstream Ready condition. On that same valid, unreleased Ready
identity:

- `Start` returned `runtime.ErrUnsupported` because upstream readiness is not a
  work-start acknowledgement and the adapter has no worker-start channel.
- `Terminate` returned `runtime.ErrUnsupported` because the adapter has no
  worker-stop evidence channel distinct from resource deletion.
- `Cleanup` then confirmed both the claim and assigned Sandbox absent and
  returned `Released=true`, `Replaced=false`.

The full command transcript and CRD schemas are in [output.txt](output.txt).

## Classification Effect

This real-cluster negative closes the E8-S1 reproduced-unsupported requirement.
It confirms the existing `unsupported` classifications for Start and
Termination. The same bounded run provides real-cluster observations for the
translated allocation, readiness, backend-identity, and cleanup paths, while
leaving their documented recovery and atomicity gaps intact.

Durability remains `unsupported`: this run does not restart the adapter or add
persistence. General isolation remains `unknown`, and the observed filesystem
evidence remains `Unsupported`; no filesystem, process, credential, or network
enforcement claim follows from this experiment. Those proofs belong to #48 and
#51.
