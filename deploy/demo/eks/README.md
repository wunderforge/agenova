# E13 EKS and Bedrock vertical slice

This extends the existing installed reference service to EKS and Bedrock. It is
not the completed E13 Epic: the Owner deferred E14 shared identity integration.
The current service still uses reference Team A and is accessed through a local
Kubernetes tunnel. Do not publish its private API or present two tunnel clients
as two authenticated application users.

Use the [task packet](../../../work/0175-shared-eks-bedrock/task.md) and
[Terraform runbook](../../../infra/demo/aws/README.md). Select an existing EKS
context, install the repository-pinned Agent Sandbox v0.4.6 substrate, and build
both repository images for linux/amd64. Push them to operator-owned ECR and use
immutable sha256 references. Setting `compatible-worker-protocol: controlled-v1`
is an operator attestation that the image is the bundled compatible worker build,
not a proof that arbitrary images implement or respect that protocol.

The Kubernetes deployment config accepts `control-plane-image` and
`image-pull-policy` (Always or IfNotPresent); omitted fields preserve kind defaults.
The runtime config accepts a remote digest-pinned `compatible-worker-image` with
the explicit protocol attestation. Registered AgentTemplate image must exactly
match that installed binding.

Use model adapter `agenova.io/model/bedrock@0.1.0`, backend config
`region: ap-southeast-2`, and a logical model profile whose backend config is
`model: amazon.nova-micro-v1:0`. Work still requests only the logical profile.
Bedrock Converse uses the host AWS SDK chain with one attempt and a bounded
request deadline. Structured worker responses use the documented Converse tool
schema path; the worker still validates its action protocol. No provider fallback.

Terraform associates only `agenova-system/agenova-control-plane` with a role
allowed to invoke that one regional model. It installs the EKS Pod Identity agent.
Worker templates explicitly disable service-account token automount, receive no
AWS identity association, and the node role has no Bedrock permission. Live
worker inspection remains required; this is not proof of egress isolation.

Build behind an already-trusted corporate TLS proxy with the optional BuildKit
`--secret id=build_ca_bundle,src=/path/to/trusted-ca-bundle.pem`. The build stage
adds that CA for downloads; the runtime image does not copy it. Never disable TLS
verification or put credentials in build arguments.

`GET /api/provider-observations` on the private tunneled API reports Bedrock SDK
invocation attempts for the current service process. Capture before/after values
in an otherwise quiet, serial test and a successful positive control. This is not
billing data or durable history; a process restart resets it. Correlate the normal
ProviderAttempt/ProviderOutcome facts and Bedrock response IDs. No raw credential
or prompt is included in this diagnostic endpoint.

Reuse the local Portal installed E2E with `AGENOVA_LIVE_MODEL=amazon.nova-micro-v1:0`
and the same allowed/denied Work references. Keep the existing kind/Ollama default.
At the authorized deadline, retain sanitized evidence, remove app-created cloud
resources, then follow the Terraform cleanup runbook. ECR images need explicit
review/deletion before non-force-delete repositories can be destroyed.

## Commands after images and substrate are ready

From the repository root, set AWS_PROFILE and KUBECONFIG to your explicit non-root
operator and EKS kubeconfig. Prepare `.tmp/e13/platform.yaml` from
`platform.yaml.example`, replacing the registry/digests and context. Prepare
`.tmp/e13/engineer.yaml` from `deploy/reference/demo/engineer.yaml`, replacing its
image with the exact same worker digest.

```sh
go build -o .tmp/e13/agenova ./cmd/agenova
.tmp/e13/agenova adapters install agenova.io/deployment/kubernetes@0.1.0 --state-dir .tmp/e13/state
.tmp/e13/agenova adapters install agenova.io/runtime/agent-sandbox@0.1.0 --state-dir .tmp/e13/state
.tmp/e13/agenova adapters install agenova.io/model/bedrock@0.1.0 --state-dir .tmp/e13/state
.tmp/e13/agenova platform validate -f .tmp/e13/platform.yaml --state-dir .tmp/e13/state
.tmp/e13/agenova platform plan -f .tmp/e13/platform.yaml --state-dir .tmp/e13/state
.tmp/e13/agenova platform apply -f .tmp/e13/platform.yaml --state-dir .tmp/e13/state
.tmp/e13/agenova platform status --state-dir .tmp/e13/state
.tmp/e13/agenova policy apply -f deploy/reference/demo/policy.yaml --state-dir .tmp/e13/state
.tmp/e13/agenova agent-template apply -f .tmp/e13/engineer.yaml --state-dir .tmp/e13/state
.tmp/e13/agenova run -f deploy/reference/demo/work.yaml --state-dir .tmp/e13/state --json
```

Then repeat `platform apply` and both registrations to prove idempotence. Start
`api connect --state-dir .tmp/e13/state` in a separate terminal before querying
`http://127.0.0.1:8088/api/provider-observations`; run
`deploy/reference/demo/denied-work.yaml` and verify the count does not increase.
The denied command intentionally exits nonzero. Preserve evidence before restarting
anything, and use new metadata names for subsequent Work in the same service.
