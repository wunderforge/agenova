# Captured reference-run output (18 September 2026)

This is a secret-free excerpt selected from the canonical JSON returned by the
installed Work API after the kind/Ollama run. The commands below print the
fields verbatim from that JSON, rather than reconstructing a narrative. The
request and task artifacts are synthetic examples in `deploy/reference/demo/`.

```powershell
$v = .\.tmp\agenova.exe run -f deploy/reference/demo/work.yaml --json | ConvertFrom-Json
$exitCode = $LASTEXITCODE
[pscustomobject]@{exitCode=$exitCode;requestRef=$v.requestRef;decision=$v.state.decision.result;policy="$($v.state.policyRef.id)@$($v.state.policyRef.version)";claimId=$v.state.claim.id;claimPhase=$v.state.claim.phase;workerId=$v.state.claim.backendIdentity.workerId;runtimeEvents=@($v.state.evidence.runtimeEvents | ForEach-Object kind);model=$v.outcome.model.model;inputTokens=$v.outcome.model.inputTokens;outputTokens=$v.outcome.model.outputTokens;outcome=$v.outcome.status;answer=$v.outcome.text} | ConvertTo-Json -Depth 5
```

```json
{
  "exitCode": 0,
  "requestRef": "investigate-payment-retries",
  "decision": "Allow",
  "policy": "reference-default-deny@1",
  "claimId": "claim:investigate-payment-retries:issuance:c5f3a2f26fb6125129420c7eeabc22be",
  "claimPhase": "Succeeded",
  "workerId": "agenova-pool-pool-engineer-mfb4j",
  "runtimeEvents": ["Bound", "BackendReady", "Running", "Succeeded", "TerminateSucceeded", "CleanupSucceeded"],
  "model": "llama3.1:latest",
  "inputTokens": 362,
  "outputTokens": 76,
  "outcome": "Succeeded",
  "answer": "The synthetic payment retries exceed the deadline due to a combination of factors, including a fixed retry backoff period of 2s, a total request deadline of 5 seconds that only accounts for attempts starting at time zero, and the fact that each attempt starts a fresh deadline instead of using remaining time."
}
```

```powershell
$v = .\.tmp\agenova.exe run -f deploy/reference/demo/denied-work.yaml --json | ConvertFrom-Json
$exitCode = $LASTEXITCODE
[pscustomobject]@{exitCode=$exitCode;requestRef=$v.requestRef;decision=$v.state.decision.result;reason=$v.state.decision.reason;claim=$v.state.claim;outcome=$v.outcome.status;facts=@($v.facts | ForEach-Object kind)} | ConvertTo-Json -Depth 5
```

```json
{
  "exitCode": 1,
  "requestRef": "investigate-unapproved-project",
  "decision": "Deny",
  "reason": "no exact policy rule matched the trusted principal and requested assignment",
  "claim": null,
  "outcome": "Deny",
  "facts": ["RequestReceived", "RequestResolution"]
}
```

The same installed service's private loopback evidence endpoint returned the
governed invocation facts below. This is a read-only capture after `run -f`
completed; it does not create a second Work.

```powershell
kubectl --context kind-agenova-k8s-lab -n agenova-system port-forward deployment/agenova-control-plane 18119:8081 --address 127.0.0.1
$v = Invoke-RestMethod 'http://127.0.0.1:18119/api/requests/investigate-payment-retries/evidence'
$kinds = @('ModelDecision','ToolDecision','ProviderAttempt','ProviderOutcome','Runtime','RunOutcome')
$counts = [ordered]@{}
foreach ($kind in $kinds) { $counts[$kind] = @($v.facts | Where-Object kind -EQ $kind).Count }
$attempts = [ordered]@{}
foreach ($operation in @('model.invoke','tool.invoke')) { $attempts[$operation] = @($v.facts | Where-Object {$_.kind -eq 'ProviderAttempt' -and $_.operation -eq $operation}).Count }
[pscustomobject]@{factCounts=$counts;attemptsByOperation=$attempts;toolExamples=@($v.facts | Where-Object kind -EQ 'ToolDecision' | Select-Object -First 2 kind,invocationId,reasonCode,operation);modelExample=@($v.facts | Where-Object {$_.operation -eq 'model.invoke' -and $_.kind -in @('ModelDecision','ProviderOutcome')} | Select-Object -Last 2 kind,invocationId,reasonCode,operation)} | ConvertTo-Json -Depth 6
```

```json
{
  "factCounts": {"ModelDecision": 4, "ToolDecision": 3, "ProviderAttempt": 7, "ProviderOutcome": 7, "Runtime": 7, "RunOutcome": 1},
  "attemptsByOperation": {"model.invoke": 4, "tool.invoke": 3},
  "toolExamples": [
    {"kind": "ToolDecision", "invocationId": "inv-7f73aa627e5114e973708a24dd1d0331", "reasonCode": "within-effective-authority", "operation": "tool.invoke"},
    {"kind": "ToolDecision", "invocationId": "inv-88309d519983b56197078721c0c1af79", "reasonCode": "within-effective-authority", "operation": "tool.invoke"}
  ],
  "modelExample": [
    {"kind": "ModelDecision", "invocationId": "inv-c770492d2a20b37f62aaa06651ac4b6d", "reasonCode": "within-effective-authority", "operation": "model.invoke"},
    {"kind": "ProviderOutcome", "invocationId": "inv-c770492d2a20b37f62aaa06651ac4b6d", "reasonCode": null, "operation": "model.invoke"}
  ]
}
```

The JSON block groups selected exact fields from `$v.facts`; its counts and
invocation IDs are from this run. The seven `ProviderAttempt` facts are **four
`model.invoke` calls plus three `tool.invoke` calls**, each following its own
matching decision; they are not seven model requests. Tool provider outcomes
remain explicitly mock; model outcomes came from local Ollama.

The seven `Runtime` facts are also intentional: `Pending` is emitted when the
request is admitted, before `RunService` starts recording issued-state runtime
events. The remaining six correspond one-for-one to the six
`state.evidence.runtimeEvents`. A read-only query of the same installed service
returned:

```text
Runtime facts: Pending, Bound, BackendReady, Running, Succeeded, TerminateSucceeded, CleanupSucceeded
Issued-state runtime events: Bound, BackendReady, Running, Succeeded, TerminateSucceeded, CleanupSucceeded
```

```powershell
kubectl --context kind-agenova-k8s-lab -n agenova-system get sandboxclaims.extensions.agents.x-k8s.io
```

```text
No resources found in agenova-system namespace.
```

These captures came from the #165 CLI and installed service after the review
corrections. They do not depend on the later #167 `work show` extension and do
not imply durable history or production authentication. An identical policy
and AgentTemplate reapply printed `already registered` after the run.

## Unavailable-model negative on the installed kind path

We temporarily applied [the test-only Platform](unavailable-model.platform.kind.yaml) with only the model endpoint changed
from `http://host.docker.internal:11434/v1` to the deliberately unreachable
`http://host.docker.internal:19999/v1`. The trusted policy and registered
template both remained active (`already registered`). We submitted a new
synthetic [Work](unavailable-model.work.yaml) name to avoid duplicate-request
behavior:

```powershell
.\.tmp\agenova.exe platform apply -f docs/evidence/165/unavailable-model.platform.kind.yaml --yes --json
$v = .\.tmp\agenova.exe run -f docs/evidence/165/unavailable-model.work.yaml --json | ConvertFrom-Json
$exitCode = $LASTEXITCODE
[pscustomobject]@{exitCode=$exitCode;requestRef=$v.requestRef;decision=$v.state.decision.result;phase=$v.state.claim.phase;outcome=$v.outcome.status;failure=$v.outcome.failure;facts=@($v.facts | Where-Object {$_.kind -in @('ModelDecision','ProviderAttempt','ProviderOutcome','RunOutcome')} | Select-Object kind,operation,reasonCode,result)} | ConvertTo-Json -Depth 8
```

```json
{
  "exitCode": 1,
  "requestRef": "investigate-payment-retries-unavailable-model",
  "decision": "Allow",
  "phase": "Failed",
  "outcome": "Failed",
  "failure": "The last governed provider call failed. Open the failed call record for context.",
  "facts": [
    {"kind": "ModelDecision", "operation": "model.invoke", "reasonCode": "within-effective-authority", "result": "Allow"},
    {"kind": "ProviderAttempt", "operation": "model.invoke", "reasonCode": null, "result": null},
    {"kind": "ProviderOutcome", "operation": "model.invoke", "reasonCode": "model-provider-failed", "result": null},
    {"kind": "RunOutcome", "operation": "Failed", "reasonCode": "provider-failed", "result": null}
  ]
}
```

This verifies that authorization success is distinct from provider execution
success and that the installed path did not silently fall back to a fixture or
another model. To verify that failure still tore down the allocated worker, we
submitted [a second tracked Work](unavailable-model-cleanup.work.yaml) with a
distinct request reference on the #165 image and captured that **same failed
request's** full lifecycle. The first terminal request remains in the service
record map; this is not a duplicate submission:

```powershell
$v = .\.tmp\agenova.exe run -f docs/evidence/165/unavailable-model-cleanup.work.yaml --json | ConvertFrom-Json
$exitCode = $LASTEXITCODE
[pscustomobject]@{exitCode=$exitCode;requestRef=$v.requestRef;claimId=$v.state.claim.id;workerId=$v.state.claim.backendIdentity.workerId;phase=$v.state.claim.phase;outcome=$v.outcome.status;runtimeFacts=@($v.facts | Where-Object kind -EQ 'Runtime' | ForEach-Object operation);runtimeEvents=@($v.state.evidence.runtimeEvents | ForEach-Object kind);providerOutcomes=@($v.facts | Where-Object kind -EQ 'ProviderOutcome' | Select-Object operation,reasonCode);runOutcomes=@($v.facts | Where-Object kind -EQ 'RunOutcome' | Select-Object operation,reasonCode)} | ConvertTo-Json -Depth 7
```

```json
{
  "exitCode": 1,
  "requestRef": "investigate-payment-retries-unavailable-model-cleanup",
  "claimId": "claim:investigate-payment-retries-unavailable-model-cleanup:issuance:0e3894b8f647939dba783d1ae11268ac",
  "workerId": "agenova-pool-pool-engineer-hj424",
  "phase": "Failed",
  "outcome": "Failed",
  "runtimeFacts": ["Pending", "Bound", "BackendReady", "Running", "Failed", "TerminateSucceeded", "CleanupSucceeded"],
  "runtimeEvents": ["Bound", "BackendReady", "Running", "Failed", "TerminateSucceeded", "CleanupSucceeded"],
  "providerOutcomes": [{"operation": "model.invoke", "reasonCode": "model-provider-failed"}],
  "runOutcomes": [{"operation": "Failed", "reasonCode": "provider-failed"}]
}
```

Immediately after that CLI response, before restoring the Platform, the
cluster check returned:

```powershell
kubectl --context kind-agenova-k8s-lab -n agenova-system get sandboxclaims.extensions.agents.x-k8s.io
```

```text
No resources found in agenova-system namespace.
```

Thus the failed model call produced a failed outcome and the bound worker's
terminate/cleanup evidence, with no SandboxClaim remaining in this dedicated
test namespace. We immediately restored the original endpoint. The exact
commands and raw CLI responses are below; they preserve the Platform revision
and zero-drift result, rather than relying on a prose status summary:

```powershell
.\.tmp\agenova.exe platform apply -f deploy/reference/platform.kind.yaml --yes --json
.\.tmp\agenova.exe platform status --json
```

```json
{"plan":{"platformName":"reference-kind","revision":"sha256:9f11889f78dd5e0d2c5042804d53417c06997be8910bdf1fa2f78151cb5b847f","target":"kind-agenova-k8s-lab/agenova-system","changes":[{"component":"agenova-control-plane","action":"reconcile","detail":"run the internal reference control plane at the effective revision"},{"component":"agenova-platform","action":"update","detail":"reconcile effective Platform revision and adapter lock"},{"component":"agenova-policy-3c894142a815b4b87b6a8c70","action":"reconcile","detail":"seed the versioned reference default-deny policy"}],"components":[{"name":"agent-sandbox-runtime","category":"adapter","state":"configured","reference":"agenova.io/runtime/agent-sandbox@0.1.0"},{"name":"kubernetes-deployment","category":"adapter","state":"used","reference":"agenova.io/deployment/kubernetes@0.1.0"},{"name":"openai-compatible-backend","category":"adapter","state":"configured","reference":"agenova.io/model/openai-compatible@0.1.0"},{"name":"agenova-control-plane","category":"deployment","state":"unavailable"},{"name":"agenova-control-plane-account","category":"deployment","state":"configured"},{"name":"agenova-control-plane-runtime","category":"deployment","state":"configured"},{"name":"agenova-control-plane-runtime-binding","category":"deployment","state":"configured"},{"name":"agenova-control-plane-service","category":"deployment","state":"available"},{"name":"agenova-platform","category":"deployment","state":"unavailable","reference":"sha256:9f11889f78dd5e0d2c5042804d53417c06997be8910bdf1fa2f78151cb5b847f"},{"name":"agenova-system","category":"deployment","state":"available"},{"name":"agenova-active-policy","category":"policy","state":"configured","reference":"reference-default-deny@1"},{"name":"agenova-policy-3c894142a815b4b87b6a8c70","category":"policy","state":"unavailable","reference":"reference-default-deny@1"}]},"applied":true,"ready":true,"readinessScope":"installation-components","components":[{"name":"agent-sandbox-runtime","category":"adapter","state":"configured","reference":"agenova.io/runtime/agent-sandbox@0.1.0"},{"name":"kubernetes-deployment","category":"adapter","state":"used","reference":"agenova.io/deployment/kubernetes@0.1.0"},{"name":"openai-compatible-backend","category":"adapter","state":"configured","reference":"agenova.io/model/openai-compatible@0.1.0"},{"name":"agenova-control-plane","category":"deployment","state":"available"},{"name":"agenova-control-plane-account","category":"deployment","state":"configured"},{"name":"agenova-control-plane-runtime","category":"deployment","state":"configured"},{"name":"agenova-control-plane-runtime-binding","category":"deployment","state":"configured"},{"name":"agenova-control-plane-service","category":"deployment","state":"available"},{"name":"agenova-platform","category":"deployment","state":"configured","reference":"sha256:9f11889f78dd5e0d2c5042804d53417c06997be8910bdf1fa2f78151cb5b847f"},{"name":"agenova-system","category":"deployment","state":"available"},{"name":"agenova-active-policy","category":"policy","state":"configured","reference":"reference-default-deny@1"},{"name":"agenova-policy-3c894142a815b4b87b6a8c70","category":"policy","state":"configured","reference":"reference-default-deny@1"}]}
{"platformName":"reference-kind","revision":"sha256:9f11889f78dd5e0d2c5042804d53417c06997be8910bdf1fa2f78151cb5b847f","target":"kind-agenova-k8s-lab/agenova-system","changes":[],"components":[{"name":"agent-sandbox-runtime","category":"adapter","state":"configured","reference":"agenova.io/runtime/agent-sandbox@0.1.0"},{"name":"kubernetes-deployment","category":"adapter","state":"used","reference":"agenova.io/deployment/kubernetes@0.1.0"},{"name":"openai-compatible-backend","category":"adapter","state":"configured","reference":"agenova.io/model/openai-compatible@0.1.0"},{"name":"agenova-control-plane","category":"deployment","state":"available"},{"name":"agenova-control-plane-account","category":"deployment","state":"configured"},{"name":"agenova-control-plane-runtime","category":"deployment","state":"configured"},{"name":"agenova-control-plane-runtime-binding","category":"deployment","state":"configured"},{"name":"agenova-control-plane-service","category":"deployment","state":"available"},{"name":"agenova-platform","category":"deployment","state":"configured","reference":"sha256:9f11889f78dd5e0d2c5042804d53417c06997be8910bdf1fa2f78151cb5b847f"},{"name":"agenova-system","category":"deployment","state":"available"},{"name":"agenova-active-policy","category":"policy","state":"configured","reference":"reference-default-deny@1"},{"name":"agenova-policy-3c894142a815b4b87b6a8c70","category":"policy","state":"configured","reference":"reference-default-deny@1"}],"installationReady":true,"readinessScope":"installation-components","providerHealth":"not-checked"}
```

`providerHealth: not-checked` is explicit: installation readiness alone is not
an Ollama health probe.
