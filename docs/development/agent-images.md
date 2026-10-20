# Agent images: publish independently from the platform

## Separate release paths

The operator releases the control plane. An authorized Agent publisher builds and
pushes an application image, then registers a **new immutable AgentTemplate** with
`agenova agent-template apply -f agent.yaml --server https://app.agenova.app --project payments`
(authenticated connected mode; select your own endpoint/project as appropriate). Work
selects that template. No platform rebuild, restart or CI/CD run is required
merely to select another Agent image.

`spec.artifact.image` and `spec.entrypoint.command` feed the template's own runtime
template/pool. Use a distinct template name for each version: same-name changed
content conflicts. Registration validates structure, name and timeout bounds,
**not** compatibility, provenance, trust or permission grants. Requested access
still cannot grant authority. Prefer immutable digests; mutable tags make future
pulls non-reproducible. Registry/node pull access must exist separately; publishing
an image or template does not grant it.
Applicable policy must also authorize the new template reference; registration
never automatically edits policy or grants permission to run it.

Public template names are project-local. Internally, the runtime keys template
and warm-pool resources by immutable resolved content (including image, command
and ceiling), not the public name alone. Two projects can both use `engineer`
without repointing each other's image or entrypoint. Equal-content templates may
share standing-authority-free warm capacity, but every Work still has its own
worker binding, temporary identity, authority and cleanup evidence.

Upgrading does not adopt/repoint old name-based warm pools. Their retirement is
a separate operator cleanup after confirming no active Work; this change never
deletes existing shared resources during rollout.

## Current execution contract: controlled-v1

Select your own image/entrypoint, but this adapter is not a generic executor for
any program. A successful image must provide:

- A long-running control service, normally `/agenova-workerctl serve`.
- `/agenova-workerctl status|start|stop <claim-token>` with exact claim-bound
  acknowledgements and stable result tokens. Idle is verified before start;
  infrastructure readiness never means execution success.
- `/agenova-demo-worker` implementing the existing bounded newline-JSON stdio
  Task/Operation/Reply exchange in `internal/workerprotocol`, including claim and
  workload-token checks. This historical path remains the contract; your app may
  implement it without using the example's agent logic.
- A bounded task compatible with the five-minute stdio execution limit and a
  node-compatible image architecture (the shared demo uses linux/amd64). Reference
  operator registration also retains its existing five-minute template ceiling;
  broader existing member-template ceilings are not retroactively rejected.

Reuse/extend the [reference Worker](../../harness/integration/agentsandbox/testworker/README.md)
if useful. A base image is optional, not an enforced allowlist. Never put provider
credentials in image layers, environment or Worker files. Use governed Tool/Model
interfaces; warm workers have no standing external authority.

Image choice does not exempt infrastructure policy. Registration is not proof of
hostile-container isolation: admission, network, filesystem and workload-identity
boundaries require their own enforcement/evidence.

## Failures and migration

| Evidence / symptom | Check |
| --- | --- |
| `runtime-image-unavailable` | Image reference, registry/node pull access, architecture |
| `runtime-worker-control-unavailable` | Control executable/service, exec transport/RBAC |
| `runtime-worker-protocol` | Exact idle/start/status acknowledgements and claim binding |
| `agent-no-final-result` / execution failure | Stdio executable, newline JSON and final governed result |
| Allocation/readiness timeout | Pool/controller readiness on the explicitly selected backend |
| Cleanup unknown | Inspect retained resources; never label deletion as confirmed stop |

Diagnostics use safe categories, not raw worker/provider/registry output. Image
pull diagnosis is available after binding when structured Pod state is observed;
failure before binding may remain an allocation timeout. Control readiness retries
have a 10-second polling window plus a bounded kubectl call; Work retains its
deadline. Invalid pre-start responses issue no start/provider call. Ambiguity
**after** start requires confirmed stop before cleanup and never implies success.
Runtime command capture is capped at 1 MiB as well as time-bounded, so arbitrary
worker stdout/stderr cannot grow the control plane's capture buffer indefinitely.

Old Platform `compatible-worker-image` values remain parseable deprecated hints,
optional and non-authoritative. Explicit unknown `compatible-worker-protocol`
values are rejected. `AGENOVA_ALLOWED_WORKER_IMAGE` is ignored by the new control
plane, so no live config change is needed solely for independent images. Old
binary rollbacks retain the former image restriction; newly registered images may
stop working until the new control plane is restored.

Independent artifacts are not automatic protocol upgrades. Retain controlled-v1
compatibility when releasing the platform. Breaking protocols need an explicit
version/migration path and regression tests for old templates.

## Reproduce the local image regression

Use a disposable, explicitly selected cluster with the pinned Agent Sandbox
v0.4.6 controller, never an implicit/default or shared production context.

```powershell
docker build -f harness/integration/agentsandbox/testworker/Dockerfile -t agenova-worker-240:a .
docker build -f harness/integration/agentsandbox/testworker/variants.Dockerfile --target variant -t agenova-worker-240:b .
docker build -f harness/integration/agentsandbox/testworker/variants.Dockerfile --target incompatible -t agenova-worker-240:incompatible .
kind load docker-image agenova-worker-240:a agenova-worker-240:b agenova-worker-240:incompatible --name YOUR_OWNED_TEST_CLUSTER
go test -v -count=1 -tags 'integration controlled' -run '^TestIndependentAgentImages_Kind$' -timeout 5m ./harness/integration/agentsandbox -args -kube-context kind-YOUR_OWNED_TEST_CLUSTER -namespace NEW_DISPOSABLE_NAMESPACE -controlled-worker-image agenova-worker-240:a -second-worker-image agenova-worker-240:b -incompatible-worker-image agenova-worker-240:incompatible
```

This registers the same public name in separate project registries, prewarms
all variants, runs A -> B -> A through one unchanged service and checks each
actual Pod's image/entrypoint. It also proves separate Worker identities,
incompatible-image failure with zero provider calls and confirmed release.
Model replies are explicitly synthetic;
this is not real-LLM, production-authentication or hostile-container proof. The
test refuses namespace reuse and deletes only its newly created namespace once
Work cleanup is confirmed. Failure retains diagnostic resources for inspection.
