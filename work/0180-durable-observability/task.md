# Task: Durable evidence and OpenTelemetry observability

- Ticket: [#180](https://github.com/wunderforge/agenova/issues/180)
- Mission: Preserve attributable Work evidence across restart and correlate safe operational telemetry in a reproducible installed reference stack.
- Target: facts/evidence, console service/API, connected CLI/Portal, Platform schema/registry/deployment adapter, control-plane composition and integration harness.
- User value: Authorized viewers can explain historical Work and distinguish policy, provider and runtime failures across the same evidence surfaces.
- PRD outcome: [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability), [installation](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap), [console](../../docs/product/prd.md#7-read-only-claim-console).

## Context to Read

Always:

- `AGENTS.md`
- `docs/product/prd.md`
- this task packet

Additional task-specific context:

- [Google/Grafana deployment boundary](../../deploy/production/grafana-google.md), [standalone gateway](../../cmd/agenova-grafana-proxy/main.go), [proxy contract](../../internal/console/grafana_proxy.go).
- [Spec](spec.md), [design](design.md), [verification report](evidence.md), [architecture contract](../../docs/product/architecture-contract.md), especially Backend Neutrality, Authority and Credentials, Evidence Surfaces and Reference Installation and Bootstrap.
- [AIDLC approval/source ownership](../../docs/development/AIDLC.md#from-existing-ticket-to-task-packet), [playbooks](../../docs/harness/playbooks.md), [existing shared API packet](../0167-shared-installed-api/task.md).
- [Journal](../../internal/facts/journal.go), [evidence](../../internal/evidence/view.go), [service](../../internal/console/service.go), [HTTP](../../internal/console/http.go) and adjacent tests.
- [Platform types](../../api/v1alpha1/platform.go), [resolver](../../internal/platform/resolve.go), [registry](../../internal/adapters/bundled/registry.go), [deployment](../../internal/adapters/bundled/kubernetes.go), [composition root](../../cmd/agenova-control-plane/main.go) and adjacent tests.
- [Connected client](../../internal/connectedclient/run.go), [CLI](../../internal/cli), [Portal](../../ui), [checks](../../scripts/check.ps1).
- E14 [#197](https://github.com/wunderforge/agenova/issues/197)/[#203](https://github.com/wunderforge/agenova/issues/203) own identity/read authorization; [#169](https://github.com/wunderforge/agenova/issues/169) owns safe revision rollout.

## Scope

In scope:

- Transactional SQLite evidence repository/PVC, conservative restart recovery, bounded authorized history and shared diagnostics.
- OTEL traces/metrics/logs through OTLP and safe correlation.
- Explicit Platform observability instance and separate Collector, Grafana, Tempo, Loki, Prometheus components on existing kind.
- Provisioned dashboards/datasources, loopback access, seven-day telemetry retention, idempotent installation and data-preserving cleanup.

Out of scope:

- Identity/SSO/policy implementation, production HA/cloud, new cluster creation, automatic Work replay, safe active-Work cutover and general secret management.
- Worker-host telemetry agents, payload collection, telemetry as governance authority and automatic evidence pruning.

## Acceptance Criteria

- Success, denial and failure survive Pod replacement with original identities, policy/authority snapshots and facts. Pre-claim denial never acquires a fabricated claim.
- Required writes commit before associated external side effects; storage failure blocks dispatch. Incomplete attempts remain unknown after restart and are never replayed.
- Bounded authorized history and single-item evidence have CLI/API/Portal parity; E14 supplies trusted viewer identity and read policy with default deny.
- Real Collector and queryable backends receive all three correlated signals. Grafana shows distinguishable Provider/Runtime failures and Work-to-trace correlation.
- Collector outage does not alter Work outcome/evidence; export is bounded and excludes credentials, full prompts, tool and Memory bodies.
- Platform validate/plan are read-only; first apply installs configured components, repeat apply is idempotent, and status separates readiness from ingestion evidence.
- Full gates, real kind/Ollama restart/fault verification, rendered proof and real E14 two-user integration pass before #180 closes.

## Negative Case

- Storage unavailable/full/rollback, unsupported schema, crash between attempt/outcome, missing viewer, cross-team access, invalid cursor, Collector outage, payload canaries, missing storage/RBAC and unavailable components have safe explicit behavior without false success or fallback.

## Execution Todo

- [x] Scout relevant implementation, tests, risks and dependencies.
- [x] Draft task/spec/design from canonical templates and the selected plan.
- [x] User explicitly waived the packet approval pause and requested self-review plus continued implementation in this chat on 20 October 2026.
- [x] Implement repository, SQLite transactions, snapshots and recovery with fault/concurrency tests.
- [x] Wire durable admission/lifecycle/Gateway writes and propagate finalization failures while preserving cleanup.
- [x] Add authorized history/diagnosis and connected CLI/Portal compatibility.
- [x] Add safe OTEL and Platform-managed independent stack.
- [x] Verify real restart, outage, component faults, install idempotency and rendered parity; see evidence report.
- [x] Retain explicit E14 real-user integration blocker; #180 is not closed.
- [x] Run focused/full gates, review diff and record task-local evidence/residual risk. No remote PR/Ticket publication was requested.

## Quality Gates

- `go test ./internal/facts ./internal/evidence ./internal/console ./internal/app ./internal/modelgateway ./internal/toolgateway ./internal/connectedclient ./internal/cli ./internal/platform ./internal/platformapply ./internal/adapters/bundled ./cmd/agenova-control-plane`
- Include new storage/telemetry packages in the focused gate; race-test concurrent repository/service behavior.
- `npm --prefix ui run test:smoke`
- `pwsh -NoProfile -File ./scripts/check.ps1 -Docs` for packet; `pwsh -NoProfile -File ./scripts/check.ps1 -All` for implementation.
- Real E2E explicitly selects its cluster/context and records sanitized commands, versions and output.

## Evidence Required

- Before/after-restart success, denied, provider-failed and runtime-failed records with matching CLI/API/Portal identities and facts; crash/unknown-attempt recovery.
- Write-fault zero-dispatch counters, positive controls, unauthorized item/page/cursor tests and real E14 two-user results.
- Tempo/Loki/Prometheus queries, Grafana/Portal screenshots, Collector-outage results and safe-field scans.
- First/repeated apply, component restarts, storage/RBAC negatives, cleanup, focused/full gates and residual blockers.

## Constraints

- Preserve architecture boundaries. SQLite/provider/Kubernetes shapes remain adapter-owned; recovered snapshots never restore worker authority.
- Preserve unrelated untracked work and the current #169 branch. Use an isolated implementation lane after approval.
- Do not read or copy secret values. Public diagnostics expose safe categories and references only.

## Decisions and Blockers

- User selected full #180, independent components, Platform-managed deployment and E14 integration on 20 October 2026; SQLite/single-instance/kind are accepted plan defaults.
- The implementation request approves the plan, which itself includes packet approval as step one. The user subsequently explicitly waived this pause and authorized self-review and implementation in this chat; this direct instruction supersedes the packet pause for this delivery. Independent external review is not claimed.
- GitHub remains the assignment/status authority: name Owner/Reviewer and record packet approval there rather than mirroring board fields here.
- Inspected checkout uses in-memory journal/records and fixed Team A identity. Real E14 integration remains required; contract fixtures cannot satisfy it.

- Self-review added atomic initial admission, defensive committer copies, rejection of request/claim identity aliases, and read-only handling of admission-only interruption. Reference per-process capacity remains 32; durable history spans sessions.
- Original workspace `-All` is blocked by the pre-existing `tmp/chatterbox-venv/.../empty_template_main.go` SPDX failure. A clean source snapshot under `/private/tmp/agenova180-verify` passed the full gate, including the original 52 browser smoke checks; the final suite adds history pagination and inactive-interruption cases; focused race checks passed separately. The snapshot has its own temporary Git baseline because the gate checks dependency metadata against Git.
- An earlier PRD/public-document update was rejected by automatic approval review for lacking a clearly recorded human decision. On 20 October 2026 the user explicitly reaffirmed authorization (“不会缺少授权的 你看下”), following the full #180 implementation request and approval-pause waiver. The PRD now records the accepted #180 scope and README links branch evidence; merged status and E14 acceptance are not claimed.
- Real Ollama returned one failed provider call during final repetitions; it remains queryable and is preserved as a failure artifact. Later allowed runs succeeded. No automatic retry or synthetic success was introduced.

- Final privacy review replaced reversible global-sequence cursors with authenticated encrypted cursors. SQLite v1-to-v2 migration retains existing evidence; cursor replay still reauthorizes every viewer. Migration, tamper and reopen tests cover this boundary.

- Owned-component cleanup/reinstallation retained all five PVC UIDs and the original successful Work. Missing storage class rejected apply; the existing impersonation wrapper proved the real namespace default ServiceAccount cannot plan installation. No kubeconfig credentials were copied.

- Final clean-snapshot gate passed all 54 browser smoke cases, including explicit history pagination and inactive authority/progress after interruption. Final screenshots were visually inspected and selected synthetic artifacts copied under `evidence/`.

- Publication verification: `-Profile PR` passed the exact staged source in a clean snapshot, including full Go race and 54 browser checks; see [PR gate log](evidence/pr-gate.log). Refreshing remote main found 135 newer commits, including E14 #207. This branch has not integrated that newer baseline; its E14-absent findings apply to the tested base. Publish as draft pending main reconciliation and real-user integration.

## Main reconciliation for PR #235

- Integrated `origin/main` at `3c3556321`; preserve E14 trusted sessions, live project authorization, workload identity, configured tool backends, PostgreSQL writer fencing and existing rollout behavior.
- SQLite remains the optional single-instance reference adapter; authenticated mode continues using E14 PostgreSQL durability. Configuring both stores or using SQLite as an authenticated replacement fails closed.
- Authenticated history uses the same SQL membership/project predicate as item reads before pagination. Cursors are authenticated/encrypted ordering hints, reauthorized on every page; PostgreSQL writer restart invalidates its cursors and clients restart from the first page. SQLite cursor keys retain their existing persisted lifetime.
- Admission snapshots are stored privately with PostgreSQL Work and retained at finalization; raw policy/template snapshots are not added to the public evidence response. The immutable reference evaluator exposes an optional descriptive bundle snapshot without changing the generic Policy evaluator contract.
- Focused actual PostgreSQL/race tests cover paging, new admissions during traversal, own/project visibility, cursor reuse under another principal, tampering and live role revocation. Browser regression now contains 63 cases. These are database and injected identity tests, not real external OIDC/two-user acceptance.
- The pre-merge kind/Ollama images and screenshots remain historical evidence for the earlier source manifest. No deployment of the reconciled branch is implied by those artifacts.

- Reconciled `-Profile PR` passed with lockfile-installed UI dependencies, full Go/race and 63 browser cases; real PostgreSQL tests were enabled. Evidence: [merge gate](evidence/merge-pr-gate.log), [focused database race](evidence/merge-postgres-race.log), [source hashes](evidence/merge-source.json).

## Google/Grafana access extension

- Owner explicitly requested Grafana Google login integration and reuse of existing system users in this chat. Read-only EKS/admin inspection found five approved users (one Owner, four Platform Admins); no roster or credentials are copied into source.
- Reuse the existing Google session BFF at `/grafana/`, enforcing E14's installation-pinned `platform.diagnose` permission on every request. Grafana remains a private auth-proxy backend with Viewer access, no independent bearer/cookie session, and no anonymous/basic access.
- Deploy behind the existing HTTPS application origin; no new OAuth client, external identity store or public unauthenticated Grafana endpoint. Explicit target configuration and enforced ingress isolation are required before enabling the proxy. Existing EKS has no Grafana stack yet; deployment and real Google positive/negative checks must be distinguished from local test evidence.

- EKS preflight found only the EFS CSI driver; the advertised gp2 class has no installed EBS CSI provisioner. A persistent Grafana install is blocked pending a supported volume/provisioner and its explicitly approved budget/permissions, as required by the PRD production extension. No ephemeral substitute, IAM change or unverified EKS login is claimed.

- Google extension validation: full PR profile passed all Go/race and 63 browser tests; actual Grafana 11.4 auth-proxy integration passed. The standalone linux/amd64 image built successfully; no EKS rollout has occurred. The user was asked whether to install EBS CSI with required AWS permissions and a 2 GiB Grafana volume under the PRD explicit-budget requirement. That infrastructure choice is pending; no elapsed-time approval is inferred.
