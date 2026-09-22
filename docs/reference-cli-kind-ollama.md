# Reproduce the kind + Ollama Demo from the CLI (#165)

This guide covers the reference CLI flow and the optional local React view. The same installed service stores and returns Work; it does not start a separate `agenova-console`.

## One-command macOS path

On an Intel or Apple Silicon Mac, run these commands from the repository root:

```bash
bash harness/local/macos-bootstrap.sh doctor
bash harness/local/macos-bootstrap.sh dry-run
bash harness/local/macos-bootstrap.sh setup
```

`setup` reuses the pinned substrate harness, creates or reuses the checkout-owned `agenova-k8s-lab` cluster, installs Agent Sandbox v0.4.6, builds and loads the worker and control-plane images, builds the local CLI, installs the Platform through the existing `platform validate/plan/apply` path, and registers the reference Policy and AgentTemplate. Repeating the command converges on the same state; it does not create a second installation configuration.

To also check Ollama and run a real Work, use:

```bash
bash harness/local/macos-bootstrap.sh verify
```

This command requires a reachable local Ollama service with `llama3.1:latest` installed. Base `setup` deliberately keeps installation readiness separate from model readiness. Cleanup is explicit: `bash harness/local/macos-bootstrap.sh down` deletes only a kind cluster whose ownership receipt matches this checkout. The script does not install or start Docker Desktop, Homebrew, Ollama, or other host software.

The manual path below remains available for step-by-step diagnosis and non-macOS environments.

## Prerequisites

- Docker Desktop, kind, kubectl, Go 1.22+, and Ollama are available.
- The existing kind cluster is named `agenova-k8s-lab`, its context is `kind-agenova-k8s-lab`, and Agent Sandbox v0.4.6 is installed.
- Local Ollama contains `llama3.1:latest` and is reachable from kind containers at `host.docker.internal:11434`.
- Both `agenova-testworker:kind` and the branch-built `agenova-control-plane:0.1.0` image are loaded into the kind cluster. Building and loading these images prepares the underlying test environment; it is not a hidden step in the Agenova control flow. For a new environment, run from the repository root:

```powershell
docker build -f harness/integration/agentsandbox/testworker/Dockerfile -t agenova-testworker:kind .
docker build -f deploy/reference/Dockerfile -t agenova-control-plane:0.1.0 .
kind load docker-image agenova-testworker:kind --name agenova-k8s-lab
kind load docker-image agenova-control-plane:0.1.0 --name agenova-k8s-lab
```

Installing Agent Sandbox into an empty cluster remains substrate preparation. Follow the [pinned v0.4.6 runbook](../harness/spike/agent-sandbox-substrate/RUNBOOK.md). This flow does not pretend that `platform apply` installs kind or the upstream controller.

Build the CLI from the repository root and add it to the current PowerShell session:

```powershell
New-Item -ItemType Directory -Force .tmp | Out-Null
go build -o .tmp/agenova.exe ./cmd/agenova
$env:PATH = "$(Resolve-Path .tmp)$([IO.Path]::PathSeparator)$env:PATH"
```

## Seven commands

Run these commands in order from the repository root. The first `platform apply` prompts for confirmation; enter `y`.

```powershell
agenova platform validate -f deploy/reference/platform.kind.yaml
agenova platform plan -f deploy/reference/platform.kind.yaml
agenova platform apply -f deploy/reference/platform.kind.yaml
agenova platform status
agenova policy apply -f deploy/reference/demo/policy.yaml
agenova agent-template apply -f deploy/reference/demo/engineer.yaml
agenova run -f deploy/reference/demo/work.yaml
```

The final command waits for Work to finish and then displays the request, decision, worker claim, terminal phase, answer, and model token count. A successful run includes `decision: Allow` and `phase: Succeeded`. Do not treat installation readiness from `platform status` as proof that Ollama or an individual Work succeeded; only the seventh command verifies real execution.

Add `--json` to `run` for the complete representation. The same evidence contains `RequestResolution`, `AuthorityResolved`, `Runtime`, `ModelDecision`, `ProviderOutcome`, `ToolDecision`, and `RunOutcome`. After submission exits, the current service session can still be queried:

```powershell
agenova work list
agenova work show investigate-payment-retries
agenova work show investigate-payment-retries --json
```

The reference service currently stores Work and evidence in process memory, so restarting the Pod loses that history. Policy and AgentTemplate registrations are stored in Kubernetes ConfigMaps and survive a service restart.

## Optional: inspect the same record in the local UI

`agenova platform status` prints the local connection command and connected API address. It verifies installation state but does **not** mean the local connection is open. In a second terminal, from the repository root, run and leave this process open:

```powershell
agenova api connect
```

After it prints `Forwarding from 127.0.0.1:8088 -> 8081`, use a third terminal:

```powershell
npm --prefix ui ci
npm --prefix ui run browsers:install
npm --prefix ui run dev -- --port 5177 --strictPort
```

Open the local address printed by Vite and select Connected, or visit `/?mode=connected#/work`. The Work list and detail must show the same request ID, decision, claim ID, and result returned by the CLI. The UI is not installed by `platform apply`; `npm dev` runs only a local client. If port 8088 is busy, the connection command fails. Connected mode also verifies the installed Platform identity and revision instead of accepting an old demo service. When the connection closes, the page reports unavailability and never falls back to mock data.

To automatically check that the CLI, API, and UI use the same real record, leave the preceding two terminals open and run the following after the allowed Work above and the denied Work below:

```powershell
$env:AGENOVA_CLI_PATH = (Resolve-Path .tmp/agenova.exe).Path
$env:AGENOVA_LIVE_REQUEST_REF = 'investigate-payment-retries'
$env:AGENOVA_LIVE_DENIED_REF = 'investigate-unapproved-project'
npm --prefix ui run test:installed
```

This test requires those request IDs to exist in the installed service. A service restart clears in-memory Work records, so rerun the requests first.

## Governance checks

```powershell
agenova policy apply -f deploy/reference/demo/policy.yaml
agenova agent-template apply -f deploy/reference/demo/engineer.yaml
agenova run -f deploy/reference/demo/denied-work.yaml --json
agenova policy apply -f deploy/reference/demo/policy-conflict.yaml
```

The first two commands must report `already registered`. The third must finish with `Deny` and produce no Claim, worker, model, or tool invocation. The fourth deliberately submits different policy content under the same ID/version and must report a conflict without replacing the active policy. Fixture request names are fixed; change `metadata.name` before repeating the same Work, or restart the reference service. Do not treat restart as a production history deletion mechanism.

## Current boundaries

- The worker is a real Agent Sandbox Pod and the model call uses real Ollama. The example `git.read` result is explicitly marked as a mock and does not prove a real Git integration.
- The reference service supplies the trusted Team A identity. Work YAML and CLI arguments cannot forge it. Production identity providers, user switching, and multi-tenant authentication are not implemented.
- The CLI uses the current Kubernetes identity/RBAC to register configuration and a temporary local port-forward to call the installed service. `api connect` opens a persistent loopback tunnel for the UI. The in-container Work entry point listens only on loopback. These tunnels provide transport isolation, not per-local-user authentication, and are intended only for a trusted local demo.
- Do not run a new `platform apply` or restart the Control Plane while Work is active. Work/evidence is held in one Pod's memory; rolling updates do not yet drain or transfer it and may interrupt execution and cleanup. Production upgrade behavior remains future work.
- Only this Kubernetes, Agent Sandbox, and OpenAI-compatible configuration is verified. Memory, Observability, other gateways/adapters, and arbitrary agent images are not claimed as available.
- This is a verified reference demo path, not proof of production identity, replicated history, or general adapter readiness.
