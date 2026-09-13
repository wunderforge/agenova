# Agent Sandbox v0.4.6 Mapping Experiment

This bounded E8-S1 experiment exercises the existing reduced Agent Sandbox
adapter against the disposable E8-T3 substrate. It proves one negative case:
after allocation reaches Ready, `Start` and `Terminate` still return
`runtime.ErrUnsupported`. Readiness is therefore not evidence of work start or
worker stop.

The integration fixture creates its uniquely named `SandboxTemplate`,
`SandboxWarmPool`, and `SandboxClaim` through the adapter. Static YAML is not
used as adapter evidence. The fixture confirms the assigned Sandbox is Ready,
prints both unsupported results, deletes the claim, independently confirms the
claim and Sandbox are absent, and removes its template and pool.

From the repository root, with Docker running:

```sh
bash harness/spike/agent-sandbox-substrate/reproduce.sh up --capture
bash harness/spike/agent-sandbox-mapping/reproduce.sh kind-agenova-k8s-lab
bash harness/spike/agent-sandbox-substrate/reproduce.sh down --capture
```

The context is mandatory and is passed to every `kubectl` request. The mapping
script does not create or delete a cluster. The substrate script owns those
mutations and refuses to delete a cluster whose recorded identity no longer
matches.

This single run is provisional spike evidence. It does not establish adapter
restart recovery, atomic replacement protection, application outcome storage,
filesystem enforcement, network isolation, or production support. Those gaps
remain for #48 and #51.
