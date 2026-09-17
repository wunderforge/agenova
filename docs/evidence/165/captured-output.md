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
