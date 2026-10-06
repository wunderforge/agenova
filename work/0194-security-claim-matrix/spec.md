# Feature Specification: Security claim matrix and threat model

- Ticket: [#194](https://github.com/wunderforge/agenova/issues/194)
- Parent Epic: [#177](https://github.com/wunderforge/agenova/issues/177) (E15)
- PRD outcome: [4. Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority), [5. Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability), and the out-of-scope rule "Claims of hostile-agent isolation without runtime and network evidence"
- Baseline: `main` @ `c56ba3a` (2026-09-28)

## Intent

Every security-relevant sentence in the [architecture contract](../../docs/product/architecture-contract.md) gets one row that states which configuration it applies to, how it would be verified, what independent oracle decides the result, and which ticket owns the evidence. E15 is done when every row either links to accepted evidence or to a recorded, narrowed limitation.

This spec routes readers to tickets. It does not track mutable status: the "Baseline" column is a one-time classification at `c56ba3a`; live state stays in GitHub and [`docs/project-status.md`](../../docs/project-status.md).

## In Scope

- Threat model: attackers, protected assets, trust boundaries, attack paths, and explicit non-goals.
- Claim matrix for the contract sections Submission and Resolution, Authority and Credentials, Evidence Surfaces, and Reference Installation and Bootstrap, plus the Facts rows that protect evidence integrity (a named asset).
- Gaps the threat model finds that the contract does not cover.

## Out of Scope

- Running attacks or collecting runtime evidence (#52 and later E15 tickets).
- Fixes, new NetworkPolicies, or CNI changes.
- A second authoritative security document or per-row mutable status.
- Parent/child claim inheritance, hostile-agent isolation certification, broad scanning.

## Threat Model

### Configurations

| ID | Configuration | Notes |
|---|---|---|
| K | Reference kind install (`agenova-k8s-lab`, kindnet, Agent Sandbox, local Ollama at `host.docker.internal:11434`) | The only configuration with merged runtime evidence. No external secret exists: Ollama needs no credential. |
| E | Future shared EKS install (E13 [#175](https://github.com/wunderforge/agenova/issues/175), Bedrock) | Not built. VPC CNI, IMDS, and IRSA change the network and credential surface. A result on K is never extended to E. |

### Attackers

| ID | Attacker | Capability assumed |
|---|---|---|
| A1 | Uncooperative worker/agent code | Arbitrary code inside a claim's worker: network probes, reading env/mounts, sending forged worker-protocol operations. Cannot change the image admitted by `AGENOVA_ALLOWED_WORKER_IMAGE`. |
| A2 | Manipulated model output | Prompt-injected task input or tool content steers the model to request ungranted tools, resources, profiles, or another claim. |
| A3 | Unauthenticated or spoofed caller | Can reach the private API (for example through a port-forward) and send arbitrary JSON, including principal or authority fields. |
| A4 | Misconfigured operator | Holds Kubernetes credentials; may use over-broad RBAC, expose a Service, or edit registry ConfigMaps directly. |

### Protected Assets

| ID | Asset |
|---|---|
| P1 | Provider credentials (model/tool/memory backends; on E also node/IRSA credentials) |
| P2 | Authority of other claims (their grants, their governed calls) |
| P3 | Evidence integrity (decisions and facts attributable to the right claim, not forged or lost silently) |
| P4 | Control-plane API and registry (admission, PolicyBundle/AgentTemplate registration) |

### Trust Boundaries

| ID | Boundary | Current reference mechanism at baseline |
|---|---|---|
| B1 | Human → API | Private API listens only on Pod-local `127.0.0.1:8081` (`cmd/agenova-control-plane/main.go:65`), reached through `kubectl port-forward`. No HTTP authentication; the principal is fixed to Team A (`main.go:167`). |
| B2 | Worker ↔ Control plane / Gateway | The worker does not call a Gateway over the network. The control plane drives it over the exec channel and authorizes each operation in-process with a session binding (`internal/console/service.go:288`, `:463`) and `RequireRunningClaim`. Only `git.read` is wired; any other tool is rejected at the demo edge as "unsupported demo tool" (`:296`), which is **not** a Gateway decision. |
| B3 | Gateway → Provider | The in-process Model/Tool Gateway calls Ollama. The control plane holds the provider endpoint; there is no provider secret on K. |
| B4 | Worker → Network | Agent Sandbox generates `agenova-tmpl-engineer-network-policy` (egress to public internet except RFC1918, 169.254/16 and kube-dns; ingress only from `sandbox-router`). **Unverified lead:** whether kindnet enforces it. Worker Pod uses SA `default` with `automountServiceAccountToken=false` (**unverified lead**, observed once, not evidence). |

### Attack Paths

| ID | Path | Assets | Matrix rows |
|---|---|---|---|
| T1 | A1 calls the provider (or other services) directly and bypasses B2/B3 | P1, P3 | AC-7, AC-9, TM-1, TM-3 |
| T2 | A1 reads a credential from env, mounts, SA token, or metadata endpoint | P1 | AC-6, AC-8, AC-9, TM-2 |
| T3 | A1/A2 sends an operation that names another Running claim | P2, P3 | AC-5, FL-1 |
| T4 | A2 escalates to an ungranted tool, resource, or profile through model output | P2 | SR-4, AC-1 |
| T5 | A3 self-declares principal, status, or effective authority | P2, P4 | SR-1, SR-2, SR-5, ES-4 |
| T6 | A3/A4 reaches a management or evidence surface without authentication | P3, P4 | ES-3, ES-5, SR-8, RI-1 |
| T7 | A component fails (provider, runtime, restart) and the system fails open or loses evidence silently | P3 | RI-3, TM-4 |

### Explicit Non-Goals

- Container or kernel escape, node compromise, side channels, and hostile-agent isolation certification.
- Worker image supply chain. The image allowlist is a product check that tests must not weaken, not something E15 attacks.
- Defending the agent's own reasoning against prompt injection. Agent frameworks own reasoning; the claim is only that model/tool output cannot expand authority (T4).
- Denial of service and resource exhaustion beyond the existing turn limit and claim timeouts.
- Parent/child claim inheritance.

## Claim Matrix

### Reading the matrix

- **Contract** quotes the sentence verbatim. It is not paraphrased.
- **Config**: K = reference kind, E = future EKS. A row is only ever claimed for the configurations listed.
- **Baseline @ `c56ba3a`** (one-time classification, not maintained):
  - `R+`: runtime evidence on K with an independent oracle.
  - `R`: runtime evidence on K from Agenova's own evidence only. This is not enough for a demo claim.
  - `U`: unit, static, or in-memory only. **Unverified at runtime.**
  - `G`: known gap. The current reference does not satisfy the sentence.
  - `V`: vacuous on K. The protected object does not exist there (for example no external secret), so a canary is needed.
- **Verify** gives the method, `+` positive control, `−` negative control, and the **oracle** that decides the result outside Agenova's own evidence.
- **Demo**: `D` = the row shapes the final demo narration on 5 November 2026; `F` = full delivery only. The maintainer confirmed on [#177](https://github.com/wunderforge/agenova/issues/177) (2 October 2026) that demo evidence is collected on K first and E is revalidated only if time allows, so `D` always means evidence on K. A row whose `D` evidence is not accepted by the demo wording freeze is narrated as an unverified limit, not as a guarantee.
- Owners named "E15-T2" / "E15-T3" / "E15-ID" / "E15-EKS" refer to planned E15 tickets that are not yet created; the matrix row is updated with the issue link when they exist.

### Submission and Resolution

| ID | Contract | Config | Baseline | Verify | Owner | Demo |
|---|---|---|---|---|---|---|
| SR-1 | "Caller identity comes from a trusted upstream authentication boundary and must not be self-declared as authoritative data in `ClaimRequest`." | K, E | `G`: the principal is fixed to Team A by service configuration. Whoever reaches the API is Team A. This is a current gap, **not** an identity control. | Two trusted identities on one installed API. + Team A allowed; − Team B denied; − a `ClaimRequest` carrying a principal field is rejected. Oracle: Kubernetes object count (no SandboxClaim/Pod for denied) and provider log (zero calls). | [#91](https://github.com/wunderforge/agenova/issues/91), [#172](https://github.com/wunderforge/agenova/issues/172) (E14); E15-ID negatives after #172 | D |
| SR-2 | "Possessing or invoking the Agenova CLI grants no authority; Agenova authorizes the caller's action, project, and template at the trusted application boundary." | K, E | `G` for identity (see SR-1). `U`+`R` for action/project/template evaluation (`internal/authorization` tests; Team B denial in [#165 evidence](../../docs/evidence/165/)). | Same as SR-1, plus a raw HTTP client (no CLI) gets the same decision as the CLI. Oracle: identical decision ID/policy version in evidence and zero Kubernetes/provider side effects on deny. | [#146](https://github.com/wunderforge/agenova/issues/146), [#172](https://github.com/wunderforge/agenova/issues/172) (E14); E15-ID | D |
| SR-3 | "A denied submission must not create a claim or allocate a runtime backend." | K | `R`: `denied-work.yaml` → Deny and no worker ([#165](../../docs/evidence/165/), [#167](../../docs/evidence/167/)), based on Agenova's evidence. `U`: `TestIssueCanonicalTeamBDenialProducesNoClaim`, `TestHTTPTeamBDeniedBeforeAllExternalWork`. | Rerun with an external oracle. + allowed Work creates exactly one SandboxClaim; − denied Work creates none. Oracle: `kubectl get sandboxclaims,pods -w` in the runtime namespace across the attempt, plus Ollama request log. | E15-T2 | D |
| SR-4 | "Task input does not grant resource access; requested resource scopes must be resolved through the same authority rules as tools, models, and memory." | K | `U`: `TestResolveResourceScopeContainmentAndWildcardFailures`. No runtime injection test. Memory is not implemented (E17 [#179](https://github.com/wunderforge/agenova/issues/179)). | Task text instructs the agent to read an out-of-scope file. + in-scope `git.read` allowed; − out-of-scope read reaches the Gateway and is denied with policy reference. Oracle: tool-backend/provider-side counter shows no out-of-scope access. | E15-T2; memory part → E17 | D |
| SR-5 | "`SandboxClaim` is the system-managed record of one resolved worker run. Callers do not self-issue claim status or effective authority." | K | `U`: strict request schema (`TestSubmitRejectsInvalidFixturesBeforeAllocation`, frontend `unknownField` negatives in [#59](../../docs/evidence/59/)); `Handler` never accepts authority (`internal/console/http.go:42`). | POST canonical request plus injected `status`/`effectiveAuthority`/`claim` fields to the private API. + canonical request accepted; − injected fields rejected before admission. Oracle: no SandboxClaim created for the rejected request. | E15-ID | F |
| SR-6 | "The MVP may use a static, versioned, default-deny reference policy bundle." | K | `U`: `TestPolicyBundleDeniesUnmatchedRules`, `TestAuthorizerDefaultsToDenyWithoutPolicyOrExactMatch`. `R`: out-of-policy request denied on K. | Apply an empty rules bundle, then submit. + a matching rule allows; − no rule denies. Oracle: Kubernetes object count and provider log. | E15-T3 (fail-closed) | F |
| SR-7 | "Owner-approved reference extension (#165, 17 September 2026): a trusted operator may create a versioned PolicyBundle and compatible AgentTemplate from declarative files, with identical reapply idempotency and same-identity conflict. This is not general policy/template CRUD or a task-submitter capability." | K | `U`: `TestRegistrationCreateIdempotentConflictAndSnapshot`, `TestKubernetesStoreRejectsIdenticalReapplyWithoutMutationAuthority` (`auth can-i` before write, fake kubectl); the private API has no registration route (`http.go:56-109`). `G` on K: the default kind kubeconfig is cluster-admin, so on K "operator" and "any local user" coincide. | Register with a restricted ServiceAccount kubeconfig that can reach the API but has no ConfigMap write access. + operator kubeconfig registers; − restricted one fails with no ConfigMap change. Oracle: `kubectl get configmap -o yaml` diff and `kubectl auth can-i`. | E15-ID (with [#146](https://github.com/wunderforge/agenova/issues/146)) | F |

Excluded from Submission and Resolution, with reasons:

- "`ClaimRequest` is the application-facing declaration…", "The task remains embedded…": definitions, not security claims.
- "YAML, API JSON, and a future `agenova run -f <file>` command must use the same request schema.": schema parity. Its security effect (no side door around validation) is covered by SR-5.
- "Request resolution precedes claim creation and must remain backend-neutral.": ordering is covered by SR-3; neutrality is not a security claim.
- "Identity-provider integration and self-service policy administration are separate concerns.": a scope statement, covered by SR-1 ownership.

### Authority and Credentials

| ID | Contract | Config | Baseline | Verify | Owner | Demo |
|---|---|---|---|---|---|---|
| AC-1 | "Requested access is intent, not granted authority." / "A request may narrow authority but cannot create authority." | K | `U`: `TestResolveNarrowsListsInRequestOrder`, `TestResolveDoesNotApplyTemplateDefaults`. `R`: effective authority on K excludes out-of-limit requests (#165 evidence). | Request a tool/profile outside template limits, then have model output ask for it. + granted profile call succeeds; − ungranted one is absent from effective authority **and** denied at the Gateway. Oracle: provider log shows zero calls for the denied profile. | E15-T2 | D |
| AC-2 | "Effective claim authority is the intersection of Agent Template limits, applicable caller/project/platform policy, requested access, and runtime restrictions." | K | `U` for template ∩ request (`internal/authority`). `G`: policy governs admission only and does not narrow grants (`internal/policy/bundle.go`, `internal/authority/resolver.go`). "Runtime restrictions" has no runtime-enforced term. | Contract-level: a policy term that narrows a template grant must appear narrowed in effective authority. Oracle: Gateway denial plus provider log zero. | [#176](https://github.com/wunderforge/agenova/issues/176) (E14) for policy narrowing; E15-T2 re-tests | F |
| AC-3 | "Requests contain scopes and references, never external secret values." | K, E | `U`: `TestRunServiceRejectsCredentialBearingLaunchInputBeforeAllocation`, `TestFindReservedCredentialKey`. | Submit a request with a canary under a credential-like key through the private API. − rejected before admission. Oracle: canary absent from Kubernetes objects, Pod env, and control-plane logs (`kubectl get -o yaml` / `kubectl logs` grep). | E15-T2 | F |
| AC-4 | "Authority is anchored to an active claim, not an idle sandbox or network location." | K | `U`: `TestGatewayUnknownAndInactiveClaimsFailClosed`, `RequireRunningClaim` in the operation path. Warm-pool/network part: `G`/unverified (see AC-6, AC-9). | − operation after the claim reached `Succeeded`/`Failed`/`Expired` is denied and recorded; − warm-pool worker holds nothing usable. + the same operation while `Running` is allowed. Oracle: provider log. | E15-T2 (terminal); [#52](https://github.com/wunderforge/agenova/issues/52) (warm/network) | D |
| AC-5 | "A governed invocation must carry system-established context bound to its worker claim; a caller-supplied claim ID alone is never proof of that binding. A target claim that differs from the bound claim is denied." | K | `U`: session binding `op.ClaimID != claimID` (`service.go:288`, `:463`); `TestJournalRejectsCrossClaimAndInvocation`. The reference binding is in-process, which the PRD permits; it is **not** workload identity. | Two Running claims A and B; A's worker channel sends an operation naming B. − denied, no fact on B. + A's own operation allowed. Oracle: provider log zero for the forged attempt. | E15-T2; network-level worker auth → [#121](https://github.com/wunderforge/agenova/issues/121) (E14) | D |
| AC-6 | "Warm workers must not hold standing external authority." | K, E | Unverified lead: pool worker SA `default`, `automountServiceAccountToken=false`, observed once. | Inspect a warm-pool worker before binding: env, mounts, SA token, reachable endpoints. + a synthetic canary placed on purpose is found by the scan; − no real credential is found. Oracle: the scan itself plus Kubernetes Pod spec. | [#52](https://github.com/wunderforge/agenova/issues/52) | F |
| AC-7 | "External system credentials remain behind Tool and Model Gateways or the future Memory Interface." | K: `V`; E | `V` on K: Ollama has no credential. `G` for the general case: no credential resolver ([#155](https://github.com/wunderforge/agenova/issues/155)). | On K use a canary credential configured only on the control-plane side; − canary not visible in worker. On E: Bedrock via IRSA, − worker cannot obtain node/IRSA credentials. Oracle: canary scan; AWS CloudTrail on E. | [#155](https://github.com/wunderforge/agenova/issues/155) (E14); [#52](https://github.com/wunderforge/agenova/issues/52) (K scan); E15-EKS | D (only if narrated) |
| AC-8 | "A sandbox may receive only scoped identity material required to authenticate to Agenova components." | K, E | Unverified lead: the worker currently receives no identity material; the exec channel needs none. | Enumerate identity material in the worker (tokens, certs, env). − nothing beyond what a future #121 design issues. Oracle: Pod spec plus in-worker scan. | [#52](https://github.com/wunderforge/agenova/issues/52); design → [#121](https://github.com/wunderforge/agenova/issues/121) | F |
| AC-9 | "Gateway policy and tests do not replace network controls, workload identity, or backend isolation evidence." | K, E | `G` by definition until runtime evidence exists. Network: generated NetworkPolicy, enforcement by kindnet **unverified**. Workload identity: none (#121). Isolation: filesystem not probed ([#51 evidence](../../docs/evidence/51/kind-run.md)). | Network: from a claim worker, attempt Ollama host port, control-plane `:8080`, Kubernetes API, 169.254.169.254, kube-dns, public egress. + a reachable control target proves the probe works; − protected targets unreachable. Oracle: provider log and packet/connection result from outside the worker. | Network → [#52](https://github.com/wunderforge/agenova/issues/52); identity → [#121](https://github.com/wunderforge/agenova/issues/121); isolation → [#51](https://github.com/wunderforge/agenova/issues/51) | D (network part, K only): the demo intends to say "the worker cannot bypass the Gateway" (#177, 2 October 2026) |

No sentence in Authority and Credentials is excluded.

### Facts (evidence integrity)

Included beyond the four sections named in the Ticket because evidence integrity (P3) is a named asset.

| ID | Contract | Config | Baseline | Verify | Owner | Demo |
|---|---|---|---|---|---|---|
| FL-1 | "Facts must be attributable to the correct claim and must not be cross-assigned between workers." | K | `U`: `TestStore_IsolatesByClaimID`, `TestJournalAuthorityAndBackendCannotBeReassigned`. | Two concurrent claims; each `work show --json` contains only its own facts, including after the forged attempt in AC-5. Oracle: provider log request count per claim matches each claim's `ProviderAttempt` count. | E15-T2 | D |
| FL-2 | "`ToolInvocation`, `ModelInvocation`, and `RuntimeEvent` are append-only facts below a claim." | K | `U`: `TestJournalDefensiveAppendAndQuery`. `G` for durability: evidence is process-local and lost on restart ([project status](../../docs/project-status.md#limits-and-unverified-claims)). | Restart the control plane after a run. The current expected result is loss, recorded as a limit, not as tampering. | [#180](https://github.com/wunderforge/agenova/issues/180) (E18); E15-T3 records behaviour | F |

### Evidence Surfaces

| ID | Contract | Config | Baseline | Verify | Owner | Demo |
|---|---|---|---|---|---|---|
| ES-1 | "CLI JSON, the read-only evidence API, and the React console must consume the same backend-neutral evidence representation." / "An API or UI may transport, validate, and render governance evidence; it must not create a second claim, policy, decision, or evidence model." | K | `R`: CLI/UI parity for allowed and denied Work on K ([#167](../../docs/evidence/167/summary.md)). | Covered. E15 re-checks parity only for the adversarial runs in E15-T2. | [#167](https://github.com/wunderforge/agenova/issues/167) (done); E15-T2 | D |
| ES-2 | "The MVP console is read-only. Claim mutation, policy editing, workflow control, and broad administration require separately approved scope." / "The #165 operator CLI registration path is a narrow management exception and does not authorize a policy/template editor in the React console." | K | `U`: the private API routes are `GET /api/setup`, `GET/POST /api/requests`, `GET …/evidence` (`http.go:56-109`). | Send PUT/PATCH/DELETE and POST to evidence/claim paths. − all return 404/405 and change nothing. Oracle: evidence snapshot and Kubernetes objects unchanged. | E15-ID | F |
| ES-3 | "A local tunnel or loopback listener is transport, not authentication; management writes require an explicit trusted operator boundary." | K | `G`: loopback plus port-forward is today's only access control for Work submission (see SR-1). Management writes go through Kubernetes RBAC, not the API (SR-7). | Same as SR-7 and SR-1. | [#146](https://github.com/wunderforge/agenova/issues/146), [#172](https://github.com/wunderforge/agenova/issues/172) (E14); E15-ID | F |
| ES-4 | "Approved exception, 15 September 2026 (#143): a loopback/internal demo submission endpoint may accept canonical ClaimRequest input and invoke the existing trusted admission, resolution and issuance boundary. It must not trust caller-provided principal or granted authority; it permits new request submission only, not editing existing claims, policy or identity. Evidence queries remain read-only." | K | `U`: `Handler` never accepts identity or authority (`http.go:42`); `TestHTTPBoundaryFailuresDoNotLeakOrExecute`, `TestHTTPSameOriginDoesNotTrustForwardedOrMultipleOrigins`. | Same as SR-5 and ES-2 through the port-forward. | E15-ID | F |
| ES-5 | "A reference evidence endpoint must bind locally or internally by default and must not be exposed publicly without an upstream authentication boundary." | K, E | `U`: private API binds `127.0.0.1:8081` (`main.go:65`); only the `:8080` status endpoint has a ClusterIP Service. | On K: list Services/Ingress for NodePort/LoadBalancer; from another Pod (including a worker) connect to the control-plane Pod IP on `:8081` and `:8080`. − `:8081` unreachable; `:8080` reveals no evidence. On E: repeat against the shared deployment. Oracle: connection results from outside the control-plane Pod. | [#52](https://github.com/wunderforge/agenova/issues/52) (worker side); [#146](https://github.com/wunderforge/agenova/issues/146) (E); E15-EKS | F |

### Reference Installation and Bootstrap

| ID | Contract | Config | Baseline | Verify | Owner | Demo |
|---|---|---|---|---|---|---|
| RI-1 | "The caller's existing Kubernetes authentication and RBAC authorize installation mutations. An installation file or `ClaimRequest` cannot grant that privilege." / "That bundle governs later Agenova control-plane actions; it does not retroactively authorize the bootstrap operation that installed it." | K | `U`: `kubectl auth can-i` preflight before mutation (`internal/adapters/bundled/kubernetes.go:681-719`); `TestKubernetesApplyDeniesBeforeMutationWhenRBACMissing`, `TestKubernetesPreflightChecksRoleGrantAuthorityBeforeMutation` (fake kubectl). On K the default kubeconfig is cluster-admin, so only a restricted identity can show the negative. | `platform apply` with a restricted ServiceAccount kubeconfig. − fails before any mutation. + cluster-admin succeeds and is idempotent. Oracle: `kubectl get` diff of the namespace before and after. | E15-ID | F |
| RI-2 | "The supported reference install must be idempotent and must not require embedding administrative credentials in the Agenova CLI or configuration." | K | `U`: `TestKubernetesApplyUsesSecretFreeResourceStepsAndWaitsReady`, `TestInitRejectsAdapterEmittedCredentialConfiguration`. | Inspect the installed Deployment, ConfigMaps, Secrets, and CLI config for admin credentials. − none found. Oracle: `kubectl get -o yaml` scan and local config file scan. | [#52](https://github.com/wunderforge/agenova/issues/52) (exposure audit) | F |
| RI-3 | "The installed reference service must consume the applied effective Platform revision for subsequent Work. A status-only installation cannot be presented as a connected application service, and an implicit memory/demo fallback cannot satisfy the reference operator journey." | K | `U`: `TestHTTPSetupProviderFailsClosed`, `TestHTTPProviderFailureIsSanitizedEvidenceNotPermissionDenial`. | Fault injection: remove/corrupt the active policy pointer; stop Ollama; stop the Agent Sandbox controller. − Work fails closed with a distinguishable reason and no fallback to memory. + restored state succeeds. Oracle: provider log and Kubernetes objects. | E15-T3 | D |

Excluded from Reference Installation, with reasons:

- "The MVP install path targets an existing, explicitly selected test cluster; Agenova does not create the cluster.": scope statement; the install always passes an explicit `--context` (`kubernetes.go:198`).
- "Production upgrades, rollback, high availability, multi-cluster administration, and general policy management require separately approved scope.": scope statement.

### Threat-model gaps not covered by the contract

| ID | Gap | Config | Treatment | Owner |
|---|---|---|---|---|
| TM-1 | The generated NetworkPolicy allows worker egress to the public internet, so task data or model output can be exfiltrated. | K, E | Record the observed behaviour. E15 reports and does not remediate (#177, 2 October 2026): if open, file a follow-up issue for the owning Epic. This is an exfiltration risk, not a Gateway bypass on K (see the meaning of the demo sentence below), so it does not decide the demo sentence. | [#52](https://github.com/wunderforge/agenova/issues/52) records; follow-up issue if open |
| TM-2 | Instance metadata (169.254.169.254) and IRSA/node credentials reachable from a worker on EKS | E | Unverifiable on K. Re-test on E once E13 provides it; VPC CNI enforces NetworkPolicy only with the network policy agent enabled. | E15-EKS (after [#175](https://github.com/wunderforge/agenova/issues/175)) |
| TM-3 | A worker reaches the Kubernetes API or control-plane `:8080` | K, E | Probe as part of AC-9. | [#52](https://github.com/wunderforge/agenova/issues/52) |
| TM-4 | Restart, provider outage, or runtime failure fails open or silently drops evidence. | K | Fault matrix with fail-closed expectations; evidence loss on restart is a known limit, not a failure of E15. | E15-T3; durability → [#180](https://github.com/wunderforge/agenova/issues/180) |
| TM-5 | Anyone with ConfigMap write access in the Agenova namespace can change the active PolicyBundle, bypassing `policy apply` idempotency/conflict checks. | K, E | `U`: `TestKubernetesStoreRejectsUnmanagedActivePointer` rejects unmanaged pointers, but a managed-looking record written directly is not tested. Record as an operator-boundary limit. | E15-ID; RBAC design → [#146](https://github.com/wunderforge/agenova/issues/146) |

## Requirements

- Given the contract sections Submission and Resolution, Authority and Credentials, Evidence Surfaces, and Reference Installation and Bootstrap, when a reviewer reads this matrix, then every security-relevant sentence appears verbatim in one row or in an exclusion list with a reason.
- Given any row, when a reviewer reads it, then the row states the configurations it applies to, and no K result is claimed for E.
- Given a row marked `D`, when it is verified, then the evidence includes a positive control, a negative control, and an oracle outside Agenova's own evidence.
- Given any row, when its owner ticket closes, then that ticket's evidence, not this spec, records the result. This spec only changes when a row's owner or wording changes.

## Negative Cases

- A row supported only by unit tests, static manifests, or in-memory behaviour is classified `U` (unverified at runtime), never proven.
- The fixed reference identity ("reaching the API means Team A") is classified `G` and co-owned with E14. It is never described as an identity control.
- The demo-edge "unsupported demo tool" rejection is never counted as a Gateway denial.
- A zero provider count without a matching positive control is inconclusive, not a pass.
- Observed reference facts (generated NetworkPolicy, `automountServiceAccountToken=false`) are recorded as unverified leads, not conclusions.

## Compatibility

- No product code, contract, or test behaviour changes.
- The architecture contract, PRD, and `docs/project-status.md` wording stay unchanged unless merged evidence or known gaps change.
- The worker image allowlist (`AGENOVA_ALLOWED_WORKER_IMAGE`) must not be weakened by any E15 test harness.

## Maintainer Decisions

Recorded on [#177](https://github.com/wunderforge/agenova/issues/177) on 2 October 2026:

1. The final demo is on 5 November 2026 and intends to say "the worker cannot bypass the Gateway". Evidence is collected on K first; E is revalidated if time allows. Rows marked `D` are scoped to K accordingly.
2. E15 reports and does not remediate. Each open gap becomes a follow-up issue for the owning Epic; E15 re-tests after the fix.
3. High-severity findings are disclosed through a GitHub private security advisory, not a public issue.

## Disclosure

- High severity means a finding that lets a worker, model output or caller obtain a credential, act for another claim, or reach a protected backend without a Gateway decision, on a configuration a reader could reproduce.
- For a high-severity finding, public issues, PRs and `docs/evidence/` state only that a finding exists, its matrix row and its status. Reproduction steps stay in the advisory until the fix is merged or the maintainer approves publication.
- A gap already listed in [`docs/project-status.md`](../../docs/project-status.md) as an unverified claim (for example kindnet enforcement of the generated NetworkPolicy) is not new information and may be recorded publicly; the maintainer decides borderline cases.
- Private vulnerability reporting is disabled on the repository at the time of writing, and E15 contributors cannot open an advisory without it. Until the maintainer enables it, a high-severity finding is sent to the maintainer privately and nothing is published.

## Meaning of "the worker cannot bypass the Gateway"

Following the maintainer's calibration on [#52](https://github.com/wunderforge/agenova/issues/52) (16 September 2026) and the E15 Epic text, the sentence means: a worker cannot reach the configured model provider, or another endpoint the Gateway or control plane protects (control-plane Service, Kubernetes API, metadata range), without going through Agenova. On K the provider is Ollama at `host.docker.internal:11434`. Public internet egress (TM-1) is a separate exfiltration risk, not a Gateway bypass on K, because the worker holds no provider credential. On E the provider (Bedrock) is a public AWS endpoint, so the two overlap and are re-tested under TM-2.

## Open Decisions

None blocks merging this spec.

- Whether the caller-side negatives (SR-5, SR-7, ES-2, ES-4, RI-1, TM-5) become one E15-ID ticket after #172, or join E14 tickets.
