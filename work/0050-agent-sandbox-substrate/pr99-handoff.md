# PR #99 repair handoff

Leo remains the #50 Owner; Frank's repair is based on PR #99 head `b031f6a`.
The requested changes are from [the independent review](https://github.com/wunderforge/agenova/pull/99#discussion_r3910327615)
and the [8 September ownership plan](https://github.com/wunderforge/agenova/issues/107#issuecomment-5581030030).

## Delivered repair

- LF checkout rules for shell scripts.
- Refusal to adopt/delete unowned or replaced clusters, using a checkout-local
  Docker container ID plus kube-system UID receipt; namespace ownership checked
  before fixture changes and cleanup.
- Correct Windows kind asset URL, with version/platform-qualified local cache.
- Read-only diagnostics; explicit transient evidence capture without rewriting
  committed evidence; commit, dirty state and script hash recorded.
- CRD served/storage versions recorded, not just CRD names.
- API lookup errors and cleanup timeouts fail rather than passing.
- Comparison script/report removed from this ticket per review; recoverable from
  Git history for #48 or a follow-up.
- Stale real-run evidence replaced by an explicit Docker environment blocker.

## Remaining Owner steps

1. Review and integrate the repair into `neo/e8-t3-kind-agent-sandbox`.
2. Inspect any old `agenova-k8s-lab` cluster manually. An old cluster has no
   ownership receipt and must not be adopted. After confirming it is disposable,
   remove it manually before creating a new lab from the repaired script.
3. From the committed repair, run twice:

   ```sh
   bash harness/spike/agent-sandbox-substrate/reproduce.sh all --capture
   bash harness/spike/agent-sandbox-substrate/reproduce.sh all --capture
   ```

4. Verify the raw captures show v0.4.6, actual tools/server versions, each CRD's
   served/storage versions, controller and claim readiness, and scoped cleanup.
   Replace the evidence blocker with one current accepted capture and record
   that both runs passed. Fix any real-runtime discrepancies before acceptance.
5. Update **the original #99 PR description**: it still describes v1.0.0,
   v1beta1/sandbox.yaml, old commits and obsolete verification. Use v0.4.6,
   v1alpha1, manifest.yaml + extensions.yaml, the final source commit and actual
   validation. Explain that compare/report are deferred and diagnostics do not
   capture evidence.
6. Request independent re-review of #99. Only then can #66 use the substrate as
   its accepted basis. This repair does not close #50 or prove the #66 mapping.

## Local verification limits

`test-reproduce.sh` passes 15 isolated command-double scenarios, including two
mock lifecycle runs. The repository `-All` baseline passes. The integration
package was compiled only. Docker was unavailable; there is no new real-cluster
or race evidence. A green unit/baseline result cannot remove that blocker.
