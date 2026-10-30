# Optional Task Access Advisor

The Advisor prepares a permission **request** before a Work exists. It is separate
from the user's Agent, and cannot issue authority or call a runtime. Existing
Policy resolution, template ceilings, Gateway checks and runtime enforcement
remain the authority. No Agent image change or database migration is required.

## Modes and independent callers

| Mode | Submission behavior |
| --- | --- |
| Manual | Existing explicit `requestedAccess`, with no planning call |
| Suggest | Prepare a reviewable request; the caller edits/confirms before starting |
| Automatic | One submission plans, adopts the request, authorizes and starts |

An operator enables the optional service and chooses a default and allowed
Advisor modes. Each submission can choose among those modes. Manual clients and
an upstream planner can continue using `POST /api/requests` without calling the
built-in Advisor; an automatic default does not rewrite their explicit requests.
Manual UI submission also remains available. Templates without a guide use Manual.

Automatic mode does not ask for a second confirmation. It stops before allocation
on planning failure, clarification, unavailable tools, or missing required grants.
An existing Policy `Deny` or `ApprovalRequired` retains its ordinary outcome;
Automatic does not auto-approve it. Suggestions are hints, never approvals.

## Author knowledge: ACCESS.md

Use bounded YAML frontmatter for machine-checkable profiles, then Markdown for
task context and exclusions. See the complete [Researcher example](../../harness/examples/access-advisor/ACCESS.md).
The first version selects one declared profile; it does not combine arbitrary
permissions or execute Skills. All permissions in a selected profile are required
for automatic execution. A task needing multiple profiles asks for clarification.

```yaml
---
version: "1"
profiles:
  - id: customer-retention
    taskType: customer-analysis
    description: Analyze internal customer retention; draft recommendations only.
    requestedAccess:
      tools: [crm.read]
      resourceScopes: [crm:customer-retention]
---
# Access guidance
Drafting a follow-up does not require permission to send messages or edit CRM.
```

Bundle it when registering an AgentTemplate:

```text
agenova agent-template apply -f researcher.yaml --access-guide ACCESS.md
```

The same flag works with existing authenticated `--server` / `--project`
registration. The CLI embeds the document into `spec.accessGuide`; it does not
store a local path or fetch referenced URLs later. A guide must fit the template
ceiling. Template identity remains immutable: publish a new identity for a changed
guide/image/ceiling. Inline guide plus `--access-guide` is rejected.

Limits: 16 profiles, 16 KiB Markdown, 32 KiB packaged file. The guide is untrusted
author knowledge; it cannot override identity or organization Policy.

## Operator configuration

Add this optional section to the Platform document, resolve/apply through the
existing operator configuration path, then restart the control-plane process with
the updated effective configuration. This feature does not add live config reload.

```yaml
spec:
  services:
    accessAdvisor:
      enabled: true
      allowedModes: [manual, suggest, automatic]
      defaultMode: automatic
      engine: model
      modelProfile: access-advisor
      timeout: 30s
      maxConcurrent: 1
      maxCalls: 32
      maxRecords: 128
```

`modelProfile` must already be installed in the Platform's model routes. It can
use a separate provider/model from the Agent's execution profile. The Advisor
reuses the provider interface with a distinct profile; its calls do not borrow a
future Work's credentials or become Worker model facts. Provider credentials stay
in existing trusted provider configuration, never ACCESS.md or a task request.

`engine: rules` needs no model profile and selects exactly one declared `taskType`.
`engine: model` selects from task objective, guide and the available tool catalog.
There is no silent fallback between engines or to synthetic inference.

Omission disables the Advisor. Platform revision/lock covers the configuration.
Timeout is bounded to one minute, concurrent planning to four, calls to 1,000 and
retained records to 512. Counts are **per process**, shared by its callers, and
reset on restart; these are not a durable monetary budget or per-tenant quota.
Failed planning consumes an attempt; an identical retry reuses its result.

## HTTP contract and evidence

- `POST /api/access-advice`: `{mode:"suggest", request:<ClaimRequest>}` with empty
  `requestedAccess`; authenticate/check project before any inference. Returns a
  private draft, explanation, questions and planner metadata; no Work allocation.
- `GET /api/access-advice/{id}`: caller's retained draft only, with current access
  checked. Drafts expire after ten minutes.
- `POST /api/requests` with `X-Agenova-Advice-ID`: accept a reviewed/edited draft.
  Changed task, project, runtime, template or guide invalidates that provenance.
  Edited requested permissions still pass normal server authorization.
- `POST /api/access-submit`: `{mode:"automatic", request:<ClaimRequest>}` with
  empty `requestedAccess`. Uses the selected profile and current authorization.
  A clarification response has `status:"clarification"` and creates no Work.

Identity, guide, registered template and capability catalog come from trusted
server context, never caller-supplied replacements. Output is restricted to a
registered profile ID plus bounded prose/questions; the model cannot emit grants.
Receipts bind owner, request name, task/context and the full template digest.

Use the same request name and body when retrying. Automatic retries first look
for an already accepted Work, so a restored stored record does not need another
inference even after process restart. A different task under the same name
conflicts. Draft/failure caches do not survive restart; deliberately choose a new
name to request a new plan. A call can fail after allocation for ordinary runtime
reasons, so retain the existing uncertain-submission recovery workflow.

Accepted `accessAdvice` is part of existing persisted Work evidence. It includes
mode, guide/profile/digests, planner model and token counts, separate from Agent
execution calls. Raw provider prompts/responses are not added to durable evidence.
Draft/failure history is bounded process state, not a durable rejected-plan audit.

## Local acceptance

See the [local test guide and exact results](../../work/0211-task-access-advisor/evidence.md).
The dedicated loopback preview uses a real existing Ollama model for the Advisor,
but synthetic Work execution/tool data and volatile Work history. It is an
explicit local test mode, not AWS or infrastructure-enforcement proof.
