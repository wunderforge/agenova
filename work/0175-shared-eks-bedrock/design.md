# Technical Design: E13 shared EKS service with governed Bedrock inference

- Ticket: [#175](https://github.com/wunderforge/agenova/issues/175).
- Feature spec: [spec.md](spec.md).

## Current State and Constraints

Bundled Kubernetes adapter hardcodes control-plane image and only accepts the kind controlled worker image. Model provider is bounded OpenAI-compatible HTTP. Installed server selects fixed Team A; connected client opens a loopback Kubernetes tunnel. Existing Terraform deployment is a separate prerequisite in progress.

## Decision

Execute reviewable slices. First make the existing compatible worker/control-plane build deployable from ECR with operator-owned image and pull settings; retain exact compatibility checks rather than treating an arbitrary image as trusted. Then add a Bedrock implementation of modelprovider.Client using AWS SDK v2 Converse, bounded prompt/output/timeouts, logical-profile allowlist and sanitized errors. Validate model/region support and access in Sydney before selecting the model. SDK credentials remain on the trusted control-plane side via a dedicated workload role; node/worker role never receives Bedrock grants.

Register Bedrock as a model capability and wire installed service construction by the resolved adapter ID, rejecting unsupported bindings. Preserve OpenAI-compatible local behavior. Do not accept worker-selected model IDs/endpoints.

For shared API, reuse the same installed service and canonical evidence. A tunnel can provide encrypted reachability without LB cost but cannot supply application authorization. The Owner has explicitly deferred E14 for this slice. Keep the fixed reference principal and loopback tunnel visibly reference-only; two-user protected API acceptance stays pending. Do not expose this service publicly. Do not generate a parallel API or label test principals as authenticated users.

Provider invocation counting must happen at the actual trusted provider boundary after governance, with a positive control and correlated request IDs. Cloud records supplement application boundary evidence; absence of a CloudWatch event alone cannot prove zero calls.

## Ownership and Contract Boundaries

E13 owns deployment/provider integration and cloud E2E. E14 owns user identity, action/read authorization and organizational narrowing. E15 owns adversarial runtime/bypass evidence; E13 supplies actual cloud topology and re-runs applicable checks. AWS configuration remains adapter-side. Product authority/architecture changes need explicit review, not incidental edits.

## Alternatives Considered

- NAT Gateway: not an authentication or workload egress policy; unnecessary for this economical topology.
- LiteLLM: optional later; native minimal provider avoids extra routing scope.
- Two fixed-principal instances: fails the same-service/real-users requirement.
- Broad worker AWS permissions: violates credential boundary.

## Verification Strategy

Behavioral tests for unknown profiles, bounded calls and sanitized failures; adapter validation before Kubernetes mutations; remote-image deployment and identical reapply; real Bedrock positive and zero-call negative; authenticated independent clients plus unauthorized read/management; EKS worker credential and direct-connect controls; rendered Portal evidence. Preserve existing local reference gate and report any failing gate.

## Risks and Compatibility

New AWS account may lack model access; treat as explicit blocker. CNI configuration must enforce NetworkPolicy before isolation claims. Ten-hour retention needs an exact deadline and functioning local cleanup automation. A cloud cluster alone proves neither E14 identity nor E15 isolation. Main PRD's narrower local reference path remains supported; cloud integration is the accepted Epic-specific extension.

## Owner-authorized temporary HTTPS demo (2026-09-28)

Owner requested provider-issued HTTPS hostname and minimum access control, while E14 stays deferred. Supersedes the earlier prohibition on any public endpoint only for this authenticated demo perimeter. Deploy a separate EKS Pod with static Portal, password-protected loopback nginx and outbound Cloudflare Quick Tunnel. An explicit Pod-name-scoped Kubernetes port-forward connects to the unchanged private reference API; no control-plane restart or public Service. Only the forwarding container mounts its projected service-account token. Store a random shared demo password outside Git and its verifier in a Secret. This is shared reference-Team-A access, not two-user authorization. Check anonymous/wrong-password rejection, authenticated UI/API, cross-origin rejection and lack of public origin listeners. Keep the original cleanup deadline; remove the demo Deployment, RBAC, ConfigMaps and Secret with the cluster. Quick Tunnel hostname is temporary and has no availability guarantee.
