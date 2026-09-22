# Two roles, one agent template: live incident rehearsal

This is a **local, opt-in reference demonstration**, not an installed production identity system. The incident, repository and Deployment are synthetic. The agent's Ollama inference, kind worker, Tool Gateway decisions, GitHub PR and Kubernetes rollback are real. Both roles use `real-engineer.yaml`; the operator starts separate loopback services with different fixed principal presets. A Work request or Portal user cannot select its own identity. Do not describe this preset as SSO or authentication.

## Record the already completed run

The local APIs retain evidence only while their processes remain running. Open these Connected Portal links on the same computer:

| Actor and requested action | Work | What to inspect |
| --- | --- | --- |
| Payments Developer: create a fix PR | [Developer allowed](http://127.0.0.1:5178/?mode=connected#/work/incident-dev-pr-allowed-r9) | Effective tools contain `github.pr.create`, not `kubernetes.rollback`; Agent activity and records show a failed first test attempt, a revised model-authored source, then a succeeded provider outcome and [real PR #1](https://github.com/wunderforge/agenova-payment-incident-demo/pull/1). |
| Payments Developer: attempt rollback | [Developer denied](http://127.0.0.1:5178/?mode=connected#/work/incident-dev-rollback-denied-r2) | Open the blocked-attempt callout. `ToolDecision` is `Deny` (`tool-not-granted`), with no provider attempt for that invocation. |
| Payments SRE: remediate by rollback | [SRE allowed](http://127.0.0.1:5179/?mode=connected#/work/incident-sre-rollback-allowed-r2) | Effective tools contain `kubernetes.rollback`, not `github.pr.create`; the provider outcome names `payment-api` revision 3 (`v2.6`). |
| Payments SRE: attempt PR creation | [SRE denied](http://127.0.0.1:5180/?mode=connected#/work/incident-sre-pr-denied-r2) | `ToolDecision` is `Deny` (`tool-not-granted`), with no provider attempt for that invocation. |

The SRE denial lives on a third loopback service so the completed SRE rollback evidence can stay available in the second service. These are **not** four accounts in one installed multi-user API. A denied-attempt Work can end `Succeeded`: its task was to investigate and report the denial, not to perform the forbidden operation.

For an independent check, run:

```powershell
gh pr view 1 --repo wunderforge/agenova-payment-incident-demo --json url,state,headRefOid,files
kubectl --context kind-agenova-k8s-lab -n agenova-payment-demo get deployment payment-api -o jsonpath='{.metadata.annotations.deployment\.kubernetes\.io/revision} {.spec.template.spec.containers[0].env[?(@.name=="PAYMENT_VERSION")].value} {.status.readyReplicas}'
```

Expected at this verified checkpoint: PR #1 is open, its sole changed file is `src/retry.go`, head commit `d514b17c2d0e04fa3f0da668b894e272e9dbd370`; the Deployment reports `3 v2.6 1`. The PR is deliberately **not merged**.

## Re-run on a prepared local kind cluster

Prerequisites: Docker Desktop, kind cluster `agenova-k8s-lab` with the Agent Sandbox controller (see the [kind setup guide](../../../../docs/reference-cli-kind-ollama.md)), Ollama with `qwen2.5-coder:7b` and `llama3.1:latest`, authenticated `gh` and `kubectl` on the **host**, and the dedicated public demo repo. Run from the Agenova repository root. Pull the pinned Go test container image before trying PR creation:

```powershell
docker pull golang:1.26-alpine@sha256:8ac98ca534ac3f51e1f420a1dd2c15e74c75cfa0f23f3ad27eb5d7236c349a0c
git clone https://github.com/wunderforge/agenova-payment-incident-demo.git .tmp/payment-incident-repo
docker build -f harness/integration/agentsandbox/testworker/Dockerfile -t agenova-testworker:kind .
kind load docker-image agenova-testworker:kind --name agenova-k8s-lab
go build -buildvcs=false -o .tmp/agenova-console.exe ./cmd/agenova-console
```

Prepare the disposable payment target once on a fresh namespace/Deployment. The v2.6 Deployment is revision 1; changing its environment creates the intentionally bad v2.7 revision 2. These commands touch only `agenova-payment-demo`:

```powershell
kubectl --context kind-agenova-k8s-lab create namespace agenova-payment-demo
docker build -t agenova-payment-demo:local deploy/reference/demo/role-rehearsal/payment-api
kind load docker-image agenova-payment-demo:local --name agenova-k8s-lab
kubectl --context kind-agenova-k8s-lab apply -f deploy/reference/demo/role-rehearsal/payment-api/deployment.yaml
kubectl --context kind-agenova-k8s-lab -n agenova-payment-demo rollout status deployment/payment-api
kubectl --context kind-agenova-k8s-lab -n agenova-payment-demo set env deployment/payment-api PAYMENT_VERSION=v2.7
kubectl --context kind-agenova-k8s-lab -n agenova-payment-demo rollout status deployment/payment-api
kubectl --context kind-agenova-k8s-lab create namespace agenova-role-demo-dev
kubectl --context kind-agenova-k8s-lab create namespace agenova-role-demo-sre
kubectl --context kind-agenova-k8s-lab create namespace agenova-role-demo-sre-denied
```

The local `real-policy.yaml` narrows the shared template to Developer (`git.read`, `github.pr.create`) and SRE (`git.read`, `kubernetes.rollback`). Start these services in three separate terminals, with distinct fixed principals and namespaces. The third retains the successful SRE evidence while the denied attempt runs separately:

```powershell
.\.tmp\agenova-console.exe --kube-context kind-agenova-k8s-lab --namespace agenova-role-demo-dev --listen 127.0.0.1:8091 --principal payments-developer --policy-file deploy/reference/demo/role-rehearsal/real-policy.yaml --template-file deploy/reference/demo/role-rehearsal/real-engineer.yaml --tool-repo .tmp/payment-incident-repo --provider-model qwen2.5-coder:7b
.\.tmp\agenova-console.exe --kube-context kind-agenova-k8s-lab --namespace agenova-role-demo-sre --listen 127.0.0.1:8092 --principal payments-sre --policy-file deploy/reference/demo/role-rehearsal/real-policy.yaml --template-file deploy/reference/demo/role-rehearsal/real-engineer.yaml --tool-repo .tmp/payment-incident-repo --provider-model llama3.1:latest
.\.tmp\agenova-console.exe --kube-context kind-agenova-k8s-lab --namespace agenova-role-demo-sre-denied --listen 127.0.0.1:8093 --principal payments-sre --policy-file deploy/reference/demo/role-rehearsal/real-policy.yaml --template-file deploy/reference/demo/role-rehearsal/real-engineer.yaml --tool-repo .tmp/payment-incident-repo --provider-model qwen2.5-coder:7b
```

Start one Vite proxy per API in three more terminals, from the repository root. Each command uses only the endpoint named in that terminal:

```powershell
$env:AGENOVA_API_URL='http://127.0.0.1:8091'; npm --prefix ui run dev -- --port 5178 --strictPort
$env:AGENOVA_API_URL='http://127.0.0.1:8092'; npm --prefix ui run dev -- --port 5179 --strictPort
$env:AGENOVA_API_URL='http://127.0.0.1:8093'; npm --prefix ui run dev -- --port 5180 --strictPort
```

Run one submission per terminal after its API starts. The helper waits for the actual final evidence; it does not fabricate the agent or provider result:

```powershell
go run ./harness/e2e/role-demo-submit --endpoint http://127.0.0.1:8091 --file deploy/reference/demo/role-rehearsal/real-developer-allowed.yaml
go run ./harness/e2e/role-demo-submit --endpoint http://127.0.0.1:8091 --file deploy/reference/demo/role-rehearsal/real-developer-denied.yaml
go run ./harness/e2e/role-demo-submit --endpoint http://127.0.0.1:8092 --file deploy/reference/demo/role-rehearsal/real-sre-allowed.yaml
go run ./harness/e2e/role-demo-submit --endpoint http://127.0.0.1:8093 --file deploy/reference/demo/role-rehearsal/real-sre-denied.yaml
```

A successful rollback consumes the bad revision-2 starting state; it is not idempotently repeatable. Every new Work in a service session needs a new `metadata.name`. PR creation also needs a new branch and may create a new PR. Do not replay the supplied request files unchanged against the same live service session or Deployment after rollback.

The host-side provider accepts only four named synthetic repo files, one source replacement, the dedicated GitHub repository and the exact kind Deployment. The model-authored replacement is tested inside a no-network, read-only, unprivileged Docker container before any push. Credentials remain with host `gh` and `kubectl`, not in Work YAML, the browser, or the agent Pod. This reference provider is not a general GitHub/Kubernetes adapter or a production secret-management solution.

Run `npx playwright test --config playwright.role-demo.config.ts` from `ui/` only while all three loopback APIs and Vite proxies for ports 5178, 5179 and 5180 are active. Its four cases assert the saved live evidence; normal CI does not depend on those local external resources.
