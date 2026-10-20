# One Agent, two incident tasks

Same engineer, template and policy. **The person's ability to access two systems
does not mean every Agent task should receive both permissions.**

| Work | Granted scope | Two actual files | Source facts |
| --- | --- | --- | --- |
| Payment timeout | `repo:demo/payments` | `payments/INC-P1042/logs.txt`, `retry.txt` | 5-second total deadline; second attempt starts at 6 seconds; per-attempt budget reset; no reused idempotency key |
| Duplicate invoices | `repo:demo/billing` | `billing/INC-B2081/logs.txt`, `job-config.txt` | 60-second schedule, 80-second runs, overlapping jobs, two invoices for one customer/cycle, no deduplication/unique constraint |

All data is explicitly synthetic. The MCP service reads files; it does not
generate diagnoses. Both profiles map the same `repo.read` operation to the same
`read_file(file)` tool, with disjoint full-path allowlists. Users/upstream supply
`requestedAccess`; task prose does not automatically choose or grant permissions.

## Local verification — no cloud or cluster required

From the repository root:

```powershell
go test ./cmd/agenova-control-plane -run '^TestIncidentDemoInputs$' -count=1
go test -tags integration ./cmd/agenova-control-plane -run '^TestIncidentDemo' -count=1 -timeout 3m -v
```

The second command builds and starts the real SDK fixture and reference Worker
executables as owned local child processes, uses the installed tool builder and
real HTTP submission/evidence API, then stops the processes and removes temporary
files. The MCP endpoint is verified HTTPS with a local test CA and loopback bridge;
neither production endpoint validation nor SDK Host protection is disabled.

Evidence checks:

- Same caller/template/policy; two distinct effective scopes; four real MCP file reads.
- Every platform tool invocation matches an independent SDK handler receipt.
- While Running, A's issued identity requesting B's scope is denied. A's own
  scope with a disguised B path is rejected before dispatch, with a bounded
  `ToolRequestRejected` activity record. Neither probe adds a downstream call.
- Replay after completion is denied; unapproved-template admission allocates no Worker.
- Valid synthetic service credential reads; missing sends no HTTP; wrong sends no
  tools/call. Credential values appear neither in receipts nor tool results.

**Limits:** allocation uses the in-memory reference backend; model responses are
scripted integration inputs, NOT real inference, buyer evidence or demo answers.
Negative requests are deterministic protocol probes, not claimed model attacks.
This does not prove Google authentication, real-model diagnosis quality, AWS
Secret/RBAC placement, direct-network isolation or hostile-container containment.
No usable local Ollama endpoint was found during initial implementation. Final
rehearsal must use the actual selected model and record all attempts (target at
least 9/10 successful business outcomes); deterministic safety probes must all pass.

## Prepared configuration — not deployment approval

- `platform.yaml`: complete isolated local Platform, two incident tool profiles,
  one explicitly synthetic legacy compatibility profile and an
  optional existing local Ollama endpoint. It creates no cluster. **Do not apply
  it to the shared install:** it is not a merge patch for existing login/model/config.
- `template.yaml`, `policy.yaml`, `work-*.yaml`: canonical registration/submission
  documents. Local image tags must be replaced with reviewed compatible immutable
  digests before cloud use. Register a new template name when changing content.
- `kustomization.yaml` / `deploy.yaml`: one internal MCP Deployment (one replica,
  10m CPU/16Mi request, 100m CPU/64Mi limit), ClusterIP Service, seven-file ConfigMap
  and ingress NetworkPolicy. No public ingress, volume, database or extra Gateway.
  The image reuses `harness/integration/mcpfixture/Dockerfile`; no new service code.

The extra three files reproduce the existing labelled `git.read` sample content
for `repo:acme/payments`. Installing a nonempty Tool backend replaces the old
synthetic catalog, so this explicit compatibility route preserves existing
engineer tasks without an implicit fallback or rewriting immutable templates.
Its transport is real MCP; its repository content is still **synthetic**, not a
live Git integration. Neither incident task is granted this legacy route.

Render only; this contacts no Kubernetes API:

```powershell
kubectl kustomize harness/demo/incident-mcp
```

The exact existing fixture HTTP service name is intentionally retained rather
than expanding the adapter's cleartext exception. The draft is for an owned local
cluster; shared namespace/service ownership, immutable image pull access and CNI
NetworkPolicy support require review. The ingress selector currently permits
only the local reference control-plane labels in `agenova-incident-demo`; it is
not preconfigured for a shared production control plane. Workers receive no data
mount and must not have a direct network path to the MCP service.

### Minimum future deployment, for Owner confirmation

1. Reconfirm live baseline and existing MCP ownership; publish/reuse the reviewed
   fixture image. Do not overwrite another contributor's namespace/service.
2. Install only the internal MCP objects, preserving their single manifest owner.
   Review exact control-plane selectors; test CNI enforcement with an allowed
   Gateway probe and a denied Worker probe in that final environment.
3. Add only the tool adapter/backend/profiles to the existing Platform through its
   validated configuration/lock/revision lifecycle, preserving login, Bedrock,
   existing policies and templates. No new control-plane image is needed for
   config-only routing, but the rejection-evidence code must be released first if
   that diagnostic is to be demonstrated.
4. Register a digest-pinned demo template and a reviewed policy version that
   **preserves existing rules** (or use an isolated project). Never replace a
   shared active policy with this minimal standalone `policy.yaml` unchanged.
5. Run both actual model tasks and the negative probes; correlate Work -> trusted
   invocation -> SDK receipt. Keep real-model success rate separate from safety tests.
6. If pitching credential isolation, separately approve token provisioning and
   exact Secret/RBAC references, switch to `/mcp-token`, and prove valid/missing/
   wrong cases plus absence from Worker configuration in the actual environment.
   The credential-free draft is not credential-isolation evidence.

Before any deployment, present the final resource/config diff, estimated cost,
namespace ownership, rollout window, recovery and cleanup to the Owner.

### Approved integrated release scope — 20 October 2026

The Owner subsequently authorized parallel local completion with [#243](https://github.com/wunderforge/agenova/issues/243),
one integrated release after local joint acceptance, and AWS verification/repair.
This does not authorize extra infrastructure, shared RBAC expansion or bypassing
branch protection. MCP delivery must not be mistaken for an outbound-network fix.

The separate [AWS overlay](../incident-mcp-aws/kustomization.yaml) imports this
unchanged local base. It selects only `app: agenova-member-control-plane` in
`agenova-system`, uses `IfNotPresent`, and denies MCP-initiated egress. It has a
deliberately invalid image placeholder: render it into an owned temporary overlay
with the reviewed ECR image digest before deployment. Recheck the live selector
and namespace/service ownership; do not expand ingress to all namespace Pods.

```powershell
kubectl kustomize harness/demo/incident-mcp-aws
```

This render makes no API calls and must not be piped directly into apply with the
placeholder image. No LoadBalancer, Ingress, PVC, database, service credentials,
service-account token or additional node is included. The credential-free service
is suitable only for these synthetic files behind verified ingress controls.
Configuring `FIXTURE_TOKEN_FILE` alone would not disable its open `/mcp` route.

### Real-model rehearsal driver

After the installed configuration and joint security probes have passed, use the
reviewed CLI and its existing personal login; the driver reads no token/key file.
Run from the repository root with explicit reviewed values:

```powershell
go run ./harness/demo/incident-mcp/acceptance `
  -cli '<absolute path to reviewed agenova.exe>' `
  -server 'https://app.agenova.app' `
  -context '<explicit reviewed Kubernetes context>' `
  -template '<registered immutable incident template>' `
  -model 'amazon.nova-micro-v1:0' `
  -out '<new private output directory with existing parent>' `
  -execute
```

- Without `-execute`, no tasks are submitted and no output directory is created.
  The command never deploys resources or changes Policy/Platform/template state.
- The campaign runs payments/billing in alternating order, ten each (20 total).
  It commits a start record before each submission and retains every result; it
  never hides failures by retrying. Unknown submissions stop further dispatch,
  since the remote task may still be running after a local client timeout.
- Each Work remains bounded to three minutes. The reviewed Worker permits six
  model turns, and installed providers cap output at 512 tokens per call. Confirm
  those runtime/provider settings on the released version before starting. The
  driver checks observed model call counts and final-call output metadata; it is
  not a separate provider billing or token-limit enforcement layer.
- The only Kubernetes operation is bounded read-only log collection from this
  MCP Deployment. Successful reads must have both `tools/call` and tool-handler
  receipts matching platform-issued invocation IDs; each task must read both
  own-scope files. A missing observer is failure, not evidence of no calls.
- Evidence must name the selected real model and a provider response ID. Same
  caller, template and Policy plus exact per-task effective scopes are checked;
  these incident tasks must have no public network grant.
- `attempts.jsonl` is a private projection: synthetic final answers, request/claim
  refs, hashed caller, Policy/model metadata, invocation IDs, files and bounded
  failure categories. It excludes raw CLI stderr, full evidence, credentials and
  full provider/tool payloads. Do not publish it without review.
- Target: at least 9/10 **mechanical screens** per scenario, followed by human
  inspection of each answer against the source facts and proposed fix. Keyword
  checks cannot establish factual correctness; the report always marks human
  review required. A plausible unsupported diagnosis must not be counted passed.
- This campaign does **not** establish network isolation or attack resistance.
  Deterministic Running cross-scope/path probes, terminal replay, admission deny,
  native network attempts and independent positive controls belong to joint
  acceptance. A model choosing not to attack is never an enforcement result.

The driver cleans only its fresh local temporary request directory. It does not
delete remote Work, immutable registrations, evidence, shared configuration,
namespaces or warm pools. Use the reviewed ownership-aware release/cleanup steps
after checking active Work; killing the CLI is not proof of remote cleanup.

### Targeted cleanup

First restore only the changed Platform/policy configuration, with drift checks,
and confirm no active Work. Remove only this demo's owned MCP Deployment, Service,
ConfigMap and NetworkPolicy after verifying names/UIDs and ownership labels.
Do not delete a shared namespace, cluster, database, login/model configuration or
other warm pools. Retain immutable registrations/audit evidence. If an isolated
namespace/cluster was created separately, destroy it only with its owner's explicit
scope/ownership verification. No cleanup command is executed by local tests against
a cluster.
