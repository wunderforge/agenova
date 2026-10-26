# E15 main integration re-test — 26 October 2026

## Version and scope

Source: `6571f9c0c4fd338d5cca418b48abff795e10aec9`, merging main
`33193da3aaf3b97850c5e0ff8d2653996697d6a4` into the #198 evidence branch.
Verification/reporting only; no product or acceptance changes. Local candidate
rehearsal, not final deployed AWS acceptance.

## Dependency assessment

GitHub #204, #248 and #251 remain open at this inspection. No new maintainer
answer resolves the reported cap semantics or installed late-identity replay path.
The main changes since `58d951c` leave `internal/authority`, both Gateways and
workload identity unchanged. #256 adds Researcher completion/native observations;
it does not repair these authority/evidence gaps.

- #248: `InvokeAuthenticated` still returns an unrecorded identity Deny for an
  inactive claim; the console handler also exits at `RequireRunningClaim`.
  The fresh-context retained-handler test reproduces all three terminal states:
  model/tool refused, downstream delta 0, decision-fact delta 0. This is an
  in-memory composition seam, **not installed old-token replay**.
- #204: `authority.Resolve` still rejects a legacy Policy cap containing any
  unrequested value, before intersection. Policy semantics require the owning
  team's decision; E15 does not change them.
- #251: `PrepareFor` still wraps resolution errors in HTTP 422
  `assignment_unavailable` before a Work record is created. Installed observations
  must be classified separately from this source assessment.

## Gates

- Focused race gate: `go test -race -count=1 ./harness/security/...
  ./internal/modelgateway/... ./internal/toolgateway/... ./internal/authority/...`
  passed; see [log](checks/focused-race.log).
- Three-phase terminal probe: passed its existing missing-record assertions;
  see [log](checks/terminal-replay.log). A passing gap-reproduction test does
  not mean #248 is fixed.
- Historical candidate offline verifier: passed; see [log](checks/offline-verifier.log).
  It verifies historical material, not this version's runtime acceptance.
- `pwsh -NoProfile -File ./scripts/check.ps1 -All`: exit 0, all Go tests,
  frontend checks/build and **90 browser smoke tests** passed;
  see [log](checks/check-all.log).
- Shell syntax and pre-change `git diff --check`: passed.

Initial focused/full gate attempts failed because the sandbox disallowed the
user Go cache/local test sockets; normal-permission reruns above passed without
changing product code. Initial logs remain local in `/tmp/agenova-e15-20261026`.

## Runtime preparation

Docker was initially stopped. After starting it, the saved independent kind
cluster was checked against its Docker container ID and kube-system UID receipt.
Existing services recovered. An initial digest alias omission caused a local
image pull failure; aliases were registered to already loaded images.
The original test installation was restored after its single immutable-template
registration rule rejected a second template. Current-source testing uses a
fresh `agenova-e15-ready-20261026` namespace and private CLI state instead.
No template record was rewritten and no EKS/member deployment was changed.

Current built image digests (loaded only into the owned local kind node):

- Control plane: `sha256:34eb49ce7abf4fe587edaa25768cc04011c93e3c6e41e649d2a962bc293487bc`.
- Scripted/reference Worker: `sha256:0779d5ee5b6f54fa6fd9661912ddfb8c59f906fbe97fe314288aa9baba64dba0`.

Fresh credential-free CNI startup probes use separate positive/denied Pods and
an independently logged same-path target. See the final runtime observations
below; no timeout alone is promoted to a CNI verdict.

## Group 1 observations

[Raw observations](group1/summary.json), [driver log](group1.log). A and B
completed under the same principal/template/Policy v2 with distinct immutable
resource scopes: A model/MCP deltas 3/2, B 2/1. MCP receipt IDs match platform
invocation IDs. The scripted foreign-scope attempt failed before a ToolDecision,
with one allowed model control and zero MCP calls; B facts remained identical.
This is an edge refusal, not a recorded Gateway Deny. Both bounded model
inductions executed only available tools; an actual model-originated
out-of-scope Gateway attempt was not observed.

Cap [A,B] with request [A] again returned HTTP 422; `work show` returned 404,
model/MCP deltas and all Pod/SandboxClaim watch events were zero. Policy v2
was restored. This reproduces #204/#251 without changing their semantics.

## Group 2 observations

[Raw observations](group2/summary.json), [driver log](group2.log). The installed
service produced Succeeded, Failed and Expired with independently matched
model/MCP counts; each terminal Work was followed by an 8-second quiet window,
zero new model/MCP calls and identical facts. Runtime resources were released.
These are quiet windows only: no installed late-token request was sent or
observed. The separate component replay above continues to reproduce #248.
The required supported installed replay path remains unresolved.

## Reproduction and CNI qualification

Follow the [ownership/substrate procedure](../candidate-20261022/reproduction.md),
using this packet's [current Platform](setup/platform.yaml),
[immutable template](setup/engineer.yaml), [baseline Policy](setup/policy.yaml)
and [cap Policy](setup/policy-cap.yaml). Replace target names and use fresh refs;
[reproduction note](setup/reproduction.txt) describes the sequence. Private state,
kubeconfig and receipt are intentionally excluded. Commands use the maintained
`harness/security/run-demo-groups.py` at the source commit named above.

The recovered enforcing Cilium substrate was independently rechecked before
the driver: 44 HTTP 200 responses from the allowed Pod with 44 matching target
receipts; zero successful responses or target receipts for the default-denied
Pod. This is same-path local CNI evidence, not a native Researcher/AWS test.
See [positive](checks/cni-positive.log), [denied](checks/cni-denied.log),
[target receipts](checks/cni-target.log) and [Pod identities](checks/cni-pods.json).

## Group 3 observations and remaining action

[Raw observations](group3/summary.json), [driver log](group3.log). Allowed
control: model/MCP 3/2, correlated receipts. Unapproved project: inspectable
admission Deny and zero allocation/calls. Out-of-template scope, unsupported-only
tool and Policy cap [A,B]/request [A]: HTTP 422, `show` 404, zero Pod/Claim watch
events and zero model/MCP calls. Mixed read+write was narrowed to read and
Succeeded (3/2); no runtime write was attempted, so this is not Gateway write
refusal evidence. Baseline Policy v2 was restored.

All three groups remain partial. #204 needs the owning team's semantics/repair
decision; #248/#251 need auditable rejection fixes, and terminal replay needs a
supported installed boundary. Concurrent Running-claim acceptance remains open.
No current result establishes final deployed AWS or complete Demo acceptance.

The driver cross-checked backend deltas against ProviderAttempt facts and MCP
correlation IDs. Additional archive checks recomputed provider counts and watch
event summaries from raw files and checked expected 422/404/zero-allocation
observations. A key/pattern scan found no token or kubeconfig contents; this is
a supplementary archive check, not a universal secret-scanner guarantee.

## Cleanup and delivery checks

All submitted Work runtime resources were gone before cleanup. This campaign's
three namespace UIDs were captured and rechecked before deleting those namespaces;
see [ownership receipts](checks/cleanup-receipts.json) and
[pre-cleanup resources](checks/resources-before-cleanup.json). The earlier
20261022 installation was restored through Platform apply; the original reference
cluster and default kube context were not reconfigured. Current test CLI state
and kubeconfig remain private. Image build caches remain local.

Final docs gate passed ([log](checks/docs-final.log)); the full gate above ran
on the same product source before these documentation-only additions. No tests
were skipped, and failed sandbox/preparation attempts are retained separately.
