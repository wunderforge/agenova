# 1 November local follow-up acceptance

Status: implemented and tested locally; owner acceptance pending. Feature branch
`codex/task-access-advisor`. Initial backup commit `e42a0d5`; the final follow-up
commit records the remaining compatibility/evidence refinements. No PR, merge,
deployment, AWS access, infrastructure recovery, paid model calls or downloads.
The existing 5224 and 5221 processes/history and untracked debug.log are preserved.

## Owner entry points

- [New Work](http://127.0.0.1:5225/login), project research, researcher-advisor-v3.
- [Public result](http://127.0.0.1:5225/?mode=connected&project=research&template=researcher-advisor-v3#/work/work-1c3fa883-8ec6-4485-9277-a90e1c5d463d).
- [Internal customers](http://127.0.0.1:5225/?mode=connected&project=research&template=researcher-advisor-v3#/work/work-aaba31a6-73cc-4a2c-8f24-b5f9e46c7db9).
- [Missing permission / quota evidence](http://127.0.0.1:5225/?mode=connected&project=research&template=researcher-advisor-v3#/work/work-bd72eac3-3a9b-4301-b363-f33a9bb19567).

The login route supplies a synthetic local owner identity. Work history is volatile.
The model/Agent process, actual Gateway decisions and MCP transport are real;
identity, lifecycle/network state and CRM records are explicitly synthetic. This
is not a new Kubernetes, AWS, PostgreSQL-restart or CNI packet-isolation result.
See [the walkthrough and reproduction commands](local-acceptance.md).

## What changed

- Removed Task preset and its mutations; ordinary tasks still need no category.
  Manual and explicit upstream clients remain independent of the Advisor.
- New Work places Max model calls beside Time limit. Defaults to six; the new
  Template allows twenty. An internal image safety bound is separate from the
  trusted Gateway budget. Explicit over-ceiling/invalid requests fail before allocation.
- Template-supported installed descriptors are discoverable independently of
  Work grants through an explicit compatible-image protocol. Legacy envelopes
  and grant-filtered discovery remain valid. Actual calls still require the
  current Work identity, operation and concrete resource authority.
- Every governed execution-model request is atomically reserved before dispatch
  in the existing journal. Concurrent attempts cannot exceed the allowance;
  failed dispatches consume their reservation. Failed recording prevents dispatch.
  Existing recovery interrupts rather than replays Work; no quota reset/resume or
  distributed monetary spending cap is claimed. Advisor planning has its own limit.
- Public selected actions, turns and actual Gateway decisions are inspectable
  together. A refusal has a real correlation ID and no fabricated provider call.
  No prompts, private chain-of-thought or credentials are recorded as action facts.
- Compact recorded progress is above unchanged requested/granted permissions.
  Activity is open. Buttons open actual evidence; unknown stages remain unknown.
  Repeated refusals share one top warning, with every individual fact preserved.
- Platform shows the configured Advisor model. New Researcher registration uses
  approved-general-model; old names/registrations/history remain valid.
- UI recognizes model-budget support before sending the new optional field, so
  older preview APIs retain their existing request shape. Platform editing/toggles
  remain deferred. Reference/copy buttons and keyboard/pointer focus share June styling.

## Real local model / MCP observations

Execution model: llama3.1:latest. Advisor: qwen2.5-coder:7b, configured separately.

| Case | Model dispatches | Tool attempts / independent MCP tool receipts | Observed outcome |
| --- | ---: | ---: | --- |
| Public, Manual, Agent chooses URL | 5 | 2 / 2 | Completed; actual official Python source cited |
| Internal customers, Automatic | 4 | 1 / 1 | Completed; synthetic C001/C002/C003 and follow-up recommendations |
| Same public goal, only CRM granted | 6 | 0 / 0 | Two real web.read denials; zero backend requests for both denied IDs; then actual model-budget denial |

All three recorded cleanup success. Actual Deny facts link to matching selected
model actions/turns. Independent MCP receipts were checked by invocation ID, not
only by the platform's own status. No network/Pod claim is inferred from those receipts.

The negative model did not return an accepted final answer within six calls.
The platform stopped the seventh dispatch before inference, and recorded an
explicit **platform-reported incomplete execution summary**. It did not fabricate
an Agent answer or count lifecycle success as business completion. Both web.read
blocked and Model-call limit reached are visible. Improving that local model's
refusal quality is a remaining quality opportunity, not an authorization bypass.

Suggest planning was also exercised with the actual Qwen model: ready CRM profile,
zero Work allocation before confirmation, model identity shown, and changing the
runtime budget invalidated the old suggestion. This is separate from Worker calls.

Worker executable SHA-256: `17963eb2a89dcd2c9c0bee978f980c1c175bd97b6dd2caf904c2624d073a4476`.

## Regression gates and exact scope

- Focused Go contracts, authority, journal, Gateway, stdio protocol, console/CLI,
  Advisor, MCP adapter and reference-Agent packages passed. Follow-up tests also
  cover no tool grants, wildcard Template discovery, hidden unrelated routes,
  missing/cross-scope access, quota limits 2/8/20, parallel Gateway instances,
  reservation replay/commit failures, CLI tampering and legacy protocol defaults.
- 180 frontend unit tests, TypeScript/contracts and production build passed.
- 95 browser smoke cases passed, including older-service compatibility, API-only
  typed workflows, copy/retry, warning states, progress-to-fact navigation,
  dark-system/light-page behavior, Administration and desktop/mobile layouts.
- Six separate rules/fixture cases passed without real inference. Suggest and
  automatic rules-positive cases use explicit API types; generic UI does not
  restore hidden category/preset routing.
- Five read-only browser checks passed against the already completed real tasks,
  including incomplete execution summary, selected-action link and mobile refusal
  evidence. They made no additional model/tool calls. Screenshots were inspected.
- The original four-case live run reported three tests passed and one UI assertion
  failed: its helper expected a Result heading, while quota exhaustion correctly
  displayed Execution summary. The helper was corrected and the same real negative
  was validated read-only; it was not rerun or relabeled as a model-written answer.
- `scripts/check.ps1 -All` ran. Documentation/boundary checks, formatting, tidy and
  vet passed. The full Go run passed 55 packages and failed only the unchanged
  Windows `cmd/agenova-control-plane: TestOwnerBootstrapDeliveryGuards` owner-file
  directory-sync behavior. The global gate is **not green** and was not skipped.
  Fresh focused console/Advisor/CLI checks passed after the compatibility change.

## Reproduction evidence on D:

Ignored local files remain available in `.tmp/`:

- followups-acceptance-summary.json: sanitized observation/receipt counts.
- followups-final-refs.json: current Work IDs.
- followups-check-all-2.log: full-check failure and other package results.
- followups-final-go.log and followups-compat-go.log: final focused Go checks.
- followups-unit-release.log, followups-build-release.log,
  followups-browser-release.log: final frontend gates.
- followups-rules-browser.log: six explicit-rules/fixture checks.
- followups-live-final.log and live-researcher-browser/: actual model run,
  including its original expectation failure and captures.
- followups-render-review.log and followups-render-review/: five read-only checks
  of actual terminal records and evidence links, including mobile.
- followups-first-live-evidence.json and followups-second-live-evidence.json:
  earlier unsuccessful local runs, preserved rather than overwritten as success.

These ignored artifacts are not uploaded with credentials or raw provider errors.
No remote recovery, merge or deployment is implied by owner local acceptance.
