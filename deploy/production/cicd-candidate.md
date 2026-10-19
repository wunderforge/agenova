# Application integration candidate: CI/CD handoff

This is a local-tested application candidate for the shared-EKS delivery trial,
not permission to deploy or approval of the upstream PRs. See the
[task packet](../../work/0218-demo-cicd-integration/task.md) and
[evidence](../../work/0218-demo-cicd-integration/evidence.md).

## Included / deliberately excluded

| Included in this source | Separately owned |
| --- | --- |
| Google member/API/CLI composition, project registry and durable Work foundation (#216) | Existing Google client, database, migrations, member configuration and public proxy operated by Neo |
| Control Plane MCP adapter/Gateway, filtered Worker catalog, result evidence and matching CLI/UI (#192) | MCP server image, data, Service, credentials, endpoint/TLS and downstream business functionality |
| Reservation/denial/outcome diagnosis fixes (#217), adapted to member/durable mode | OTEL implementation, Collector and telemetry backend; not implemented by this integration |

Task-conditional business policy is not introduced by default in this candidate.
Existing requestedAccess/template intersection remains. Do not infer per-task
business-rule narrowing from the Tool catalog alone.

## Build contract

- Control Plane: `deploy/reference/Dockerfile`, linux/amd64, entrypoint
  `/agenova-control-plane`; includes pinned-checksum kubectl 1.36.1.
- Worker: `harness/integration/agentsandbox/testworker/Dockerfile`, linux/amd64;
  entrypoint `/agenova-workerctl serve`, agent executable `/agenova-demo-worker`.
- Both builders use Go 1.27.2, matching the member branch toolchain. Do not build
  the new Worker with the historical Go 1.22 Dockerfile.
- Build matching CLI (`go build ./cmd/agenova`) and Portal (`npm --prefix ui ci`,
  `npm --prefix ui run build`). New Tool evidence has additive `resultRef` and
  `truncated` fields; old strict clients must not be assumed compatible.
- The reference Control Plane image does **not** contain Portal assets. Preserve
  the member deployment's read-only Portal asset mount and update it from this
  revision when validating UI parity. CP/Worker image delivery alone does not
  automatically update a separately mounted Portal bundle.
- Local `harness/integration/identity/build-candidate.py` builds the actual two
  Dockerfiles from allowlisted public contexts and runs no-network fail-closed
  executable smokes. It never pushes. Do not send a developer's whole worktree,
  OAuth file, secret store, kubeconfig or generated private test files as context.

## Configuration and rollout checks

1. Preserve member mode, database TLS, session-key/Google secret mounts, proxy,
   service account and current membership/project checks. No fixed-principal
   fallback or direct public private-listener exposure.
2. Match the published Worker digest across `AGENOVA_ALLOWED_WORKER_IMAGE`,
   runtime compatibility configuration and the selected AgentTemplate artifact.
   The worker protocol changes here: **do not roll out only the Control Plane
   while leaving an unverified old Worker binding**.
3. Same-identity different-content templates conflict by design. A new image is
   not an in-place update of an immutable template; use an explicitly reviewed
   new template identity/policy version and matching Work references, or the
   Owner's reviewed migration. Retain the old configuration for rollback.
4. Real tools require effective Platform `toolBackends`/`toolProfiles`, the MCP
   adapter and routes matching the template's logical tools/resources. The old
   `git.read` reference configuration will stay mock unless real routes are
   deliberately configured; publishing images alone does not activate MCP.
5. Token-required MCP references are currently adapter-specific
   `provisional-token-secret: name/key`. Control Plane needs `get secrets` only
   for those named Secrets in its namespace, plus existing runtime/registry RBAC.
   The member Deployment is a separate install surface: verify its actual Role,
   not just the reference installer's generated Role. Worker gets no such grant,
   secret mount or external token. No actual secret belongs in Platform/Git.
6. Preserve database schema checks; runtime does not apply migrations. Test
   recovery and evidence through the same member API after deployment.

## External-service Owner checklist

The Tool Owner supplies a running MCP service, immutable image, bounded operation
mapping, safe request receipts, endpoint and health evidence. The existing fixture
under `harness/integration/mcpfixture` reads mounted test files, not production
repositories. Its kind manifest uses local image `imagePullPolicy: Never`; the
Owner must adapt/publish it for EKS. This integration does not deploy it.

Cleartext MCP is accepted only for the exact fixture host
`e16-mcp.agenova-e16.svc.cluster.local:8080`, paths `/mcp` and `/mcp-token`.
Arbitrary internal HTTP service names are rejected. Real bearer credentials need
trusted HTTPS; do not broaden this synthetic-fixture exception to pass a test.

The Observability Owner supplies actual application instrumentation/exporter and
Collector/backend if external traces are required. #217 contains diagnosis fixes,
not that implementation. Canonical Work evidence can be validated independently;
do not present it as a working OTEL backend.

## CI/CD trial boundary

The separately developed `codex/neo-shared-eks-cicd` workflow inspected on
19 October is main-ref scoped and intentionally stops deployment when the Worker
changes. This candidate changes Worker/catalog behavior. A branch validation run,
candidate publication and reviewed Worker/configuration migration are distinct
steps. Do not disable those guards or merge to main merely to obtain a green
deployment. Coordinate the pre-merge candidate test mechanism with the pipeline
Owner; this branch does not edit that workflow.

After the Owners provide external services and the deployment window is approved:
sign in with the matching CLI, verify current setup/project/template, register
reviewed immutable governance versions if required, submit distinct new Work
names and compare CLI/API/Portal authority, invocation and result. Check allowed
real reads, ungranted resource rejection with independently witnessed zero calls,
and admission rejection before Worker allocation. A health/readiness success is
not a model/tool or security-boundary acceptance result.

## Release restrictions

The private R6/AC-7 credential-exclusion finding on #192 is still unresolved by
independent acceptance. Ordinary token-path tests and token-reflection regression
do not close it. Do not use this candidate to certify a real-credential production
rollout. Native source tests, real disposable SQL, local images and synthetic
browser tests are separate evidence, not public EKS/real-provider acceptance.
Worker authentication/network escape hardening and complete OTEL remain owned by
their respective lanes.
