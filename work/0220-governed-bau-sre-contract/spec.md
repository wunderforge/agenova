# Feature Specification: Governed BAU/SRE investigation contract

- Ticket: [#220](https://github.com/wunderforge/agenova/issues/220)
- PRD outcome: [Claim-scoped authority](../../docs/product/prd.md#4-claim-scoped-authority) and [Facts and accountability](../../docs/product/prd.md#5-facts-and-accountability)

## Intent

An enterprise troubleshooting worker can name a small set of read-only logical operations and exact logical resources without exposing provider credentials or leaking provider SDK types into Agenova's shared contract. Agenova validates the request structurally, resolves the authoritative Running claim, enforces its exact effective authority, records one correlated decision, and only then dispatches to a configured adapter.

This contract is a sanitized product slice. Company-specific systems, topology, policy values, identities and knowledge remain private integration inputs.

## In Scope

- The existing tool operation representation `tool.action`, using the following E21 read-only vocabulary. These operations receive operation/resource structural checks; legacy custom operations remain governed by exact issued authority so E21 does not break unrelated experimental contracts:

  | Category | Operations |
  | --- | --- |
  | Work tracking | `workitem.read` |
  | Source control | `repository.read`, `file.read`, `commit.read`, `mergerequest.read` |
  | Kubernetes | `kubernetes.get`, `kubernetes.list`, `kubernetes.logs` |
  | Observability | `metrics.query`, `dashboard.read` |
  | GitOps | `gitopsapplication.read` |

- Exact, backend-neutral resource scopes:

  | Kind | Canonical form | Example |
  | --- | --- | --- |
  | Work item | `workitem:<system>/<project>/<item>` | `workitem:tracker/example/INC-123` |
  | Repository | `repo:<owner>/<repository>` | `repo:example/payments` |
  | Repository object | `repo-object:<owner>/<repository>/<object-kind>/<object-ref>` | `repo-object:example/payments/path/docs/runbook.md` |
  | Kubernetes namespace | `kubernetes:<cluster>/namespace/<namespace>` | `kubernetes:lab/namespace/monitoring` |
  | Kubernetes object | `kubernetes:<cluster>/namespace/<namespace>/<kind>/<name>` | `kubernetes:lab/namespace/monitoring/pod/collector-0` |
  | Metrics target | `metrics:<backend>/<tenant>` | `metrics:prometheus/lab` |
  | Dashboard | `dashboard:<backend>/<dashboard-id>` | `dashboard:grafana/sre-overview` |
  | GitOps application | `gitops:<control-plane>/<project>/<application>` | `gitops:argo/platform/monitoring` |

- ASCII identifiers with segments separated by `/`. Segments must be non-empty; `.` and `..` segments, whitespace, backslashes, control characters, query/fragment delimiters, and wildcard characters are forbidden.
- `repo-object` treats everything after `<object-kind>` as one object reference composed of safe non-empty segments. Supported object kinds are `path`, `commit`, and `merge-request`.
- Kubernetes scopes contain either the namespace boundary or one exact namespaced object. Cluster-scoped resources require a later contract decision and are not inferred from a namespace form.
- Stable structural rejection categories before claim attribution:
  - existing `ambiguous-resource-scope` for blank or wildcard input;
  - `invalid-resource-scope` for unsupported kinds, wrong segment shape, forbidden characters, or non-canonical encoding.
  - `unsupported-tool-operation` for the E21 read-only slice's explicit mutation denylist (`git.write`, `kubernetes.delete`, `kubernetes.patch`, `gitopsapplication.sync`, and `terraform.apply`).
  - `operation-resource-mismatch` when a known E21 read operation is combined with the wrong canonical resource kind.

## Out of Scope

- Provider endpoint, organization URL, project numeric ID, Kubernetes API object, PromQL, dashboard JSON, GitOps CRD, credential or token in the resource scope.
- Tool arguments beyond the existing bounded string parameters.
- Natural-language-to-authority translation. Agent/model output may request an operation but cannot define or widen its grant.
- Mutation operations, draft merge-request creation, approval persistence, provider transport, production identity, durable history, or deployment packaging.

## Requirements

- Given a supported scope, when it is parsed, then its kind and segments are returned without provider-specific interpretation and re-format to exactly the original canonical string.
- Given an unsupported kind or malformed/non-canonical scope, when it reaches the Tool Gateway, then it is denied as `invalid-resource-scope` before claim resolution, policy evaluation, fact attribution, or adapter dispatch.
- Given an empty or wildcard scope, when it reaches the Tool Gateway, then the existing `ambiguous-resource-scope` behavior remains unchanged.
- Given an operation outside the claim's exact effective `tools`, when the claim is Running, then it is denied as `tool-not-granted`, records one correlated claim fact, and makes zero adapter calls.
- Given an explicitly forbidden E21 mutation operation, when it reaches the Tool Gateway, then it is rejected before claim attribution, policy evaluation, or adapter dispatch even if an invalid authority snapshot contains that string.
- Given a known E21 read operation with the wrong resource kind, when it reaches the Tool Gateway, then it is rejected as `operation-resource-mismatch` before claim attribution, policy evaluation, or adapter dispatch.
- Given a canonical resource outside the claim's exact effective `resourceScopes`, when the claim is Running, then it is denied as `resource-scope-not-granted`, records one correlated claim fact, and makes zero adapter calls.
- Given an allowed operation and exact resource, when the claim is Running and policy allows, then exactly one correlated Allow fact is recorded before exactly one adapter call.
- Given caller/model/tool parameters containing a resource-looking string, when the granted `ResourceScope` does not match, then parameters do not change the authority decision.
- Given a terminal or cross-claim request, existing authoritative lifecycle and claim-bound identity rules remain the enforcement owners; this contract does not treat the bare ClaimID as authentication.

## Negative Cases

- Reject `kubernetes:lab/namespace/*`, `kubernetes:lab/namespace/monitoring/`, `kubernetes:lab/namespace/../pod/x`, `repo:example//payments`, `repo:example/payments?ref=main`, an unknown `company:` kind, whitespace, and backslash variants before adapter dispatch.
- Deny `kubernetes.delete` even when its resource is canonical and an invalid authority snapshot contains the operation.
- Deny a canonical resource in another namespace/repository/project with zero adapter calls.
- Deny governed calls for Pending, Bound, Succeeded, Failed and Expired claims.
- Never accept a provider credential or kubeconfig through resource scopes or parameters.

## Compatibility

- Existing `repo:acme/payments` requests and fixtures remain valid.
- Existing experimental/custom operations remain subject to exact issued authority and additional policy. E21 does not claim their provider safety or add them to the supported BAU/SRE vocabulary.
- Legacy `file.read` with a repository-level scope remains valid; new adapters should issue the more precise `repo-object:.../path/...` form when the exact path is known.
- Existing capability-ceiling terminal wildcards remain an authority-resolution concern. A concrete Tool Gateway invocation still forbids every wildcard.
- `ClaimRequest`, `AgentTemplate`, `EffectiveAuthority`, `SandboxClaim`, `ToolInvocation`, `RuntimeEvent` and evidence-view public types do not change in M1.
- E16 owns real governed provider transport/catalog behavior; E21 M1 defines the logical contract consumed by that boundary.
- E14 owns authenticated worker-to-Gateway claim binding. Until it is present, the bare `ClaimID` is correlation/lifecycle gating only.
- E18 owns durable evidence and observability. M1 uses current correlated in-process evidence and states that limitation.

## Open Decisions

- Whether a later Kubernetes contract should support cluster-scoped resources and collection selectors requires a separate accepted use case; M1 fails closed rather than guessing.
- Whether provider-specific query text belongs in bounded parameters or a typed operation payload is deferred to the first real adapter ticket.
