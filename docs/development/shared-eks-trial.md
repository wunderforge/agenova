# Shared EKS trial (19 October 2026)

The tested member service is `https://app.agenova.app`. Ordinary members need only
Agenova CLI and an approved Google application account, not AWS/Kubernetes access.
This is a shared demo installation, not production security certification.

Use the CLI built from this integration (published preview installers may lag):

```powershell
go build -o .tmp/agenova.exe ./cmd/agenova
.tmp/agenova.exe login --server https://app.agenova.app
.tmp/agenova.exe run -f docs/examples/shared-cicd-smoke.yaml --server https://app.agenova.app
.tmp/agenova.exe work list --server https://app.agenova.app
.tmp/agenova.exe work show REQUEST_REF --server https://app.agenova.app
```

Use a unique Work name for a new execution. Reusing the same personal name and
identical input queries the existing result; changed input under that name conflicts.
The sample intentionally requests no tools and uses the existing Bedrock profile.
The new compatible template is `ci-engineer-44dc383c9d8c` in `payments`; do not
substitute the old immutable `engineer` image. In the connected Portal, select
this template, submit Work, then open its detail to inspect authority, provider
calls, result and cleanup. Existing history is retained across this rollout.

For admission denial, copy the sample to a new file/name and set `templateRef` to
the registered but unallowed `zz-ci-denied-44dc383c9d8c`. Expect CLI exit 1, Deny,
no Claim/Worker allocation and no model/tool call. This is policy denial, not a
pretend successful execution or an instruction to the model to refuse.

Platform admins can register new immutable documents through the same API:

```powershell
agenova agent-template apply -f TEMPLATE.yaml --server https://app.agenova.app --project payments
agenova policy apply -f POLICY.yaml --server https://app.agenova.app --project payments
```

Do not overwrite names/versions, expand another team's policy or upload secrets.
On that historical release, new image references must match the installed Worker.
The [independent image guide](agent-images.md) describes the #240 replacement;
it applies only after upgrading the control plane. These commands do not grant
AWS IAM/RBAC or deploy external MCP/Collector services.

Release source is `44dc383c9d8cb1ed5679677d78ed6a289a1944bc`; subsequent evidence
and release-provenance-only changes do not imply another image rollout. See
[real acceptance](../../work/0218-demo-cicd-integration/aws-acceptance-20261019.md)
and [delivery procedure](shared-eks-release.md). Temporary branch access has been
removed; automatic main cloud deployment remains off. Latest #216 schema-009 work
is intentionally separate. Tool remains labelled mock; real model execution is
not evidence that MCP, Memory or OTEL has been deployed.
