# Captured reference-run output (17 September 2026)

This is a secret-free excerpt selected from the canonical JSON returned by the
installed Work API after the kind/Ollama run. The commands below print the
fields verbatim from that JSON, rather than reconstructing a narrative. The
request and task artifacts are synthetic examples in `deploy/reference/demo/`.

```powershell
$v = .\.tmp\agenova.exe run -f deploy/reference/demo/work.yaml --json | ConvertFrom-Json
[pscustomobject]@{requestRef=$v.requestRef;decision=$v.state.decision.result;policy="$($v.state.policyRef.id)@$($v.state.policyRef.version)";claimId=$v.state.claim.id;claimPhase=$v.state.claim.phase;workerId=$v.state.claim.backendIdentity.workerId;runtimeEvents=@($v.state.evidence.runtimeEvents | ForEach-Object kind);model=$v.outcome.model.model;inputTokens=$v.outcome.model.inputTokens;outputTokens=$v.outcome.model.outputTokens;outcome=$v.outcome.status;answer=$v.outcome.text} | ConvertTo-Json -Depth 5
```

```json
{
  "requestRef": "investigate-payment-retries",
  "decision": "Allow",
  "policy": "reference-default-deny@1",
  "claimId": "claim:investigate-payment-retries:issuance:c5f3a2f26fb6125129420c7eeabc22be",
  "claimPhase": "Succeeded",
  "workerId": "agenova-pool-pool-engineer-t5r5p",
  "runtimeEvents": ["Bound", "BackendReady", "Running", "Succeeded", "TerminateSucceeded", "CleanupSucceeded"],
  "model": "llama3.1:latest",
  "inputTokens": 362,
  "outputTokens": 63,
  "outcome": "Succeeded",
  "answer": "The synthetic payment retries exceed the deadline due to a fixed backoff strategy with a 2s delay between attempts, and each attempt starting with a fresh 5s deadline. This exceeds the total request deadline of 5 seconds mentioned in README.md."
}
```

```powershell
$v = .\.tmp\agenova.exe run -f deploy/reference/demo/denied-work.yaml --json | ConvertFrom-Json
[pscustomobject]@{requestRef=$v.requestRef;decision=$v.state.decision.result;reason=$v.state.decision.reason;claim=$v.state.claim;outcome=$v.outcome.status;facts=@($v.facts | ForEach-Object kind)} | ConvertTo-Json -Depth 5
```

```json
{
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
kubectl --context kind-agenova-k8s-lab -n agenova-system port-forward deployment/agenova-control-plane 8088:8081 --address 127.0.0.1
$v = Invoke-RestMethod 'http://127.0.0.1:8088/api/requests/investigate-payment-retries/evidence'
$kinds = @('ModelDecision','ToolDecision','ProviderAttempt','ProviderOutcome','Runtime','RunOutcome')
$counts = [ordered]@{}
foreach ($kind in $kinds) { $counts[$kind] = @($v.facts | Where-Object kind -EQ $kind).Count }
[pscustomobject]@{factCounts=$counts;toolExamples=@($v.facts | Where-Object kind -EQ 'ToolDecision' | Select-Object -First 2 kind,invocationId,reasonCode,operation);modelExample=@($v.facts | Where-Object {$_.operation -eq 'model.invoke' -and $_.kind -in @('ModelDecision','ProviderOutcome')} | Select-Object -Last 2 kind,invocationId,reasonCode,operation)} | ConvertTo-Json -Depth 6
```

```json
{
  "factCounts": {"ModelDecision": 4, "ToolDecision": 3, "ProviderAttempt": 7, "ProviderOutcome": 7, "Runtime": 7, "RunOutcome": 1},
  "toolExamples": [
    {"kind": "ToolDecision", "invocationId": "inv-f985ba625be57a0581fa000e572ae664", "reasonCode": "within-effective-authority", "operation": "tool.invoke"},
    {"kind": "ToolDecision", "invocationId": "inv-c37119daefb6e36872904206bb7013e4", "reasonCode": "within-effective-authority", "operation": "tool.invoke"}
  ],
  "modelExample": [
    {"kind": "ModelDecision", "invocationId": "inv-aac2770d3d1dfe68f6b4b93188021e55", "reasonCode": "within-effective-authority", "operation": "model.invoke"},
    {"kind": "ProviderOutcome", "invocationId": "inv-aac2770d3d1dfe68f6b4b93188021e55", "reasonCode": null, "operation": "model.invoke"}
  ]
}
```

The JSON block groups selected exact fields from `$v.facts`; its counts and
invocation IDs are from this run. Tool provider outcomes remain explicitly
mock; model outcomes came from local Ollama.

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
