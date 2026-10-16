# Non-production BAU/SRE deployment contract

This directory is a secret-free handoff for deploying the governed BAU/SRE
slice from a private GitOps or deployment repository. It is not a production
manifest and deliberately contains only example identities and endpoints.

## Required private inputs

Copy the examples into the private repository and replace:

- the exact non-production Kubernetes context and dedicated namespace;
- digest-pinned control-plane and controlled-worker images;
- the approved HTTPS model and MCP gateway endpoints;
- namespaced Secret references containing only the model and MCP gateway
  bearer tokens;
- the logical operations, exact resource scopes, native MCP tool mappings and
  allowlisted parameter values approved for the pilot;
- the team, project and template identities used by the non-production policy.

Secret values, kubeconfigs and private endpoints must not be committed here.
The control plane reads each named Secret key per invocation, so rotation does
not put a credential in Platform state or require a restart. Its Role grants
`get` only for Secret names referenced by the resolved Platform. The reference
OpenAI-compatible adapter sends the model token as `Authorization: Bearer`;
leave its reference empty only for a deliberately credential-free endpoint.
When private HTTPS endpoints use an internal CA, `ca-bundle-secret` mounts only
the named key and sets `SSL_CERT_FILE`; the Secret value remains external.

For a self-contained first `kubernetes.list` route, build
`mcp-kubernetes.Dockerfile` and adapt `mcp-kubernetes.yaml.example`. The server
accepts only the exact configured resource kinds and namespace, emits only
sorted resource identities plus bounded Pod readiness/restart or Deployment
replica status, uses in-cluster RBAC, requires bearer authentication and TLS,
and exposes no mutation tool. Response kind and namespace are revalidated
before any observation is returned, and every response declares the
`health-v1` observation contract so deployment verification can bind the
runtime response to its image lock. It never returns object specs, log text,
events or Secret data. Its token and TLS Secrets are external inputs. The TLS
chain must already be trusted by the control-plane image or be supplied
through the approved runtime CA mechanism. The MCP server rereads its bearer
token for every request and its certificate/key for every TLS handshake, so
atomic projected-Secret rotation does not require a Pod restart.

## Delivery sequence

1. Build the control-plane and worker images from the same reviewed revision,
   scan them, push them to the approved registry and record their digests. The
   control plane and worker can be built from the repository root with
   `docker buildx build --platform linux/amd64 -f deploy/reference/Dockerfile .`
   and `docker buildx build --platform linux/amd64 -f
   deploy/nonproduction/worker.Dockerfile .`; pin the builder and runtime `ARG`
   values to approved digests in the private pipeline.
2. Copy and edit `platform.yaml.example`; run `agenova platform validate`.
3. Run `verify.sh prerequisites` with the selected non-production context. It
   fails closed unless all four pinned Agent Sandbox API resources are
   discoverable, the controller image exactly matches
   `AGENOVA_EXPECTED_AGENT_SANDBOX_CONTROLLER_IMAGE`, and the configured
   controller Deployment has an available replica. Installing that
   cluster-level substrate is a separate platform approval; this package
   never installs it implicitly.
4. In an MR pipeline, run `verify.sh render`. Commit or promote the resulting
   multi-document YAML through the private GitOps repository; rendering is
   offline and does not require a kubeconfig.
5. Optionally run `verify.sh plan` with a read-only non-production identity to
   compare the selected target. Let Argo CD reconcile the reviewed manifest;
   do not use the imperative `apply` mode in the normal GitOps path. The script
   refuses contexts containing production tokens, requires an exact current-
   context match and confirms the resolved target matches the expected target.
6. Register `policy.yaml` and `agent-template.yaml`, then submit the bounded
   smoke Work. Keep the first smoke profile credential-free where possible.
7. Run `verify.sh verify`, inspect the Work and invocation evidence, and prove
   that a denied mutation makes zero provider calls.

`ready: true` from Platform apply proves only installation-component readiness.
Deployment acceptance also requires one successful read-only smoke Work, one
denied mutation/cross-scope Work, provider-side call counts, and cleanup of the
worker from the same run.

## Rollback

Revert the private deployment MR to the previous digest-pinned Platform and
rerun plan/apply. Do not downgrade a running Work in place: stop admissions,
wait for or terminate active non-production Works, then restore the previous
known-good revision. Secret rotation is performed in the private secret store;
no secret value belongs in the Platform file or Agenova state directory.
