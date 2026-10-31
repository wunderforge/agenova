# Local Researcher acceptance

Owner scope: local implementation, tests and feature-branch backup only. No PR,
merge, deployment, migrations, new model downloads or paid inference.

## Which preview to use

Use **http://127.0.0.1:5224/login** for the repaired acceptance build. The `/login`
route supplies a synthetic local identity, not Google authentication. The existing
5221 preview and its old in-memory Work records are deliberately preserved.

5224 uses the real Researcher executable, real existing local Ollama inference,
the real Tool Gateway and a dedicated real MCP process. Customer records are
synthetic. Registration, identity, lifecycle and network state are local fixtures;
they do not prove Pod/CNI isolation. Work history is volatile and resets on restart.
The preview banner and Platform page state these limits.

## Walkthrough

Project: **research**. Agent: **researcher-advisor-v2**. No Task type or caller URL
is required. Leave Task preset at Custom task. The local template has a five-minute
limit; inference performance depends on other workloads. The Advisor uses the
existing Qwen code model; Agent execution defaults to the existing general-purpose
`llama3.1:latest`, configured independently with `--execution-model`.

1. Public: `Explain what Python is used for, based on its official website.`
   Choose Manual, request only `web.read` and `web:public-pages`.
   Inspect requested/granted permissions, the actual model and tool attempts,
   the source citation and the final answer. The Agent chooses its own URL.
2. Internal: `Identify three customers with the largest usage declines and draft
   follow-up recommendations.` Choose Automatic. The Advisor should request
   `crm.read` / `crm:customer-retention`. Inspect its separate planning receipt,
   the Work's model/MCP evidence, synthetic label, customer IDs and recommendations.
3. Missing capability: copy the Public objective, choose Manual and grant only
   `crm.read` / `crm:customer-retention`. The Agent should explain that web access
   is unavailable and remain incomplete, without reading unrelated customers.
   This tests Agent behavior, not a Gateway denial if no call was attempted.
4. The independent Gateway regression deliberately submits an installed but
   ungranted `web.read` operation with the Work's actual scoped identity. It must
   record Deny and zero provider attempts. This is a local Gateway test with a
   provider counter; it is not an AWS or packet-blocking result.

The public tool reads URLs, not a search engine. Unsupported, redirected,
inaccessible or uninformative pages may leave a goal incomplete. For example,
the Zoom pricing objective can now execute without a supplied URL, but its
current fact quality depends on the pages actually obtainable. Do not present
an execution Succeeded marker as proof of the business goal.

## Reproduce

From the repository root, use the installed Go/Node runtimes. All generated
artifacts stay under the repository's ignored `.tmp` directory on D:.

```powershell
$repo = (Get-Location).Path
$env:GOCACHE = Join-Path $repo '.tmp/gocache'
$env:GOTMPDIR = Join-Path $repo '.tmp'
$env:TEMP = $env:GOTMPDIR
$env:TMP = $env:TEMP
$env:GOFLAGS = '-p=2'
go build -o .tmp/local-access-live.exe ./tools/local-product-preview
go build -ldflags '-X main.agentRole=researcher' -o .tmp/researcher-local.exe ./cmd/agenova-demo-worker
go -C harness/integration/mcpfixture build -o "$repo/.tmp/local-mcp.exe" .
./.tmp/local-access-live.exe --volatile --listen 127.0.0.1:5224 --advisor-config harness/examples/access-advisor/advisor-model.json --live-worker .tmp/researcher-local.exe --live-mcp .tmp/local-mcp.exe
```

The helper owns its MCP on literal loopback port 5216. It refuses occupied ports
and verifies a random instance marker before using that service. It does not
take over an existing MCP. Child processes receive an allowlisted environment,
not inherited provider credentials. Gateway-only mode disables this reference
Agent's native HTTPS path, but is an application guard, not OS isolation.

Run browser acceptance only after starting this explicit local service:

```powershell
Set-Location ui
$env:AGENOVA_TEST_LIVE_RESEARCHER = '1'
npx playwright test --config playwright.live-researcher.config.ts
```

Evidence and screenshots: `.tmp/live-researcher-browser`. The owned MCP writes
separate correlation receipts at the path printed during startup. Never copy
tokens, cookies, raw provider errors or personal credentials into a report.

## Compatibility

- Omitted/empty `task.type` canonicalizes to omission. No model/classifier is
  introduced for Manual or explicit upstream requests.
- Existing nonempty types and their serialized bytes remain unchanged; copied
  coding tasks retain their explicit type, repository and base branch.
- Model planning uses the objective. Rules planning remains a declared exact-type
  contract: presets may select one unambiguous registered rule; otherwise clarify.
- The controlled-v1/research-v1 execution messages and authorization remain intact.
  The local provider optionally constrains the same tool/finish shapes with a
  discriminated schema. Other providers retain the flat format. Default cloud
  Researcher behavior is untouched by the explicit gateway-only flag.

The formatting approach follows [Ollama structured outputs](https://docs.ollama.com/capabilities/structured-outputs)
and the documented [llama.cpp schema limitations](https://github.com/ggml-org/llama.cpp/blob/master/grammars/README.md).
Provider support is verified locally; format validity alone cannot prove answer
correctness, relevance, authorization or infrastructure enforcement.
