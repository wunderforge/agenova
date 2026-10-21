# Grafana with the existing Google login

The approved #180 extension reuses the existing Agenova Google session and the
E14 `platform.diagnose` authorization for the serving installation. It creates
no OAuth client, membership copy, Google secret or new account system.

Open `https://app.agenova.app/grafana/` after deployment. An unauthenticated browser
uses the existing `/login` flow; after Google sign-in, reopen `/grafana/`.
The existing member directory decides access on each request. A disabled user,
revoked session or removed installation permission is denied before Grafana.
Platform diagnostic access is global operational visibility, not project Work
read authority. Owner and Platform Admin users receive Editor; installation operators receive Viewer.
Editors can create/save dashboards; no user gains Grafana administration.
Provisioned dashboards are maintained from source; use Save as copy for custom
changes. The opaque Grafana login includes the derived role, preventing stale
Grafana permission caches from carrying privileges across role changes.
The bridge requires the updated member authorization endpoint (`action: grafana`);
upgrade that endpoint before enabling the new bridge. Older endpoints deny access.

## Components and trust boundary

- `agenova-grafana-proxy`, built from `grafana-proxy.Dockerfile`, runs separately
  from the member Control Plane. It calls the same public HTTPS `/auth/me` and
  `/api/platform/authorize` endpoints as the approved application boundary. Its
  environment pins `AGENOVA_MEMBER_ORIGIN`, `AGENOVA_INSTALLATION` and the private
  `AGENOVA_GRAFANA_UPSTREAM`. It does not read a database or Secret.
- The existing TLS proxy routes `/grafana` and `/grafana/` to this private service,
  pins the application Host, enforces HTTPS, clears forwarded identities, and
  preserves the original browser cookie only as far as the membership bridge.
  Use the optional [route fragment](grafana-location.conf) inside the existing server block. Keep the regular login and API routes unchanged. Validate `nginx -t` before a
  graceful reload; do not restart active Work to change this route.
- Grafana accepts only the bridge-generated opaque login and live Editor/Viewer role.
  Application cookies, bearer tokens and user-supplied identity headers never go
  to Grafana. Grafana cookies are not returned to the browser. Basic/anonymous
  login and independent Grafana login tokens are disabled.
- Websocket streaming is disabled so long-lived connections cannot retain access
  after revocation; regular dashboard HTTP queries still work. Every HTTP request
  reauthenticates and reauthorizes through E14.
- Grafana remains ClusterIP-only in a dedicated namespace. A NetworkPolicy admits
  TCP/3000 only from the bridge namespace and Pod selector. This is required before
  enabling auth proxy; no other policy may broadly allow ingress to those Pods.
  Verify actual CNI enforcement and a rejected non-bridge connection, not just the
  existence of policy objects. The current EKS VPC CNI reports strict enforcement.

## Platform adapter configuration

Add this to the deployment adapter's existing `observability` configuration:

```yaml
storageClass: gp2
grafanaAdminSecretRef: grafana-admin
grafanaAuthProxy:
  publicOrigin: https://app.agenova.app
  namespace: agenova-observability
  app: agenova-grafana-proxy
```

These Kubernetes details remain adapter-owned. The credentials reference retains
bootstrap recovery material server-side; browser/basic authentication stays off.
Deploy the gateway with the matching `app` label and source namespace. For an
in-process member composition, the optional `grafanaURL` member configuration
uses the same proxy boundary directly; it cannot replace membership checks.

The current image-only member upgrade mode does not install standalone services.
Do not apply a reference Control Plane configuration over the shared member
service just to install Grafana. Install the standalone components with the
operator-reviewed deployment changes and keep the existing member Deployment,
worker compatibility, Policy, templates and data intact.

## Observed deployment prerequisites (20 October 2026)

Read-only inspection found an existing `agenova-demo` EKS cluster, a working
Google session, five approved users (one Owner and four Platform Admins), and no
Grafana installation. No user roster or secret values are stored in this repository.
The cluster advertises `gp2` but has no EBS CSI driver; only EFS CSI is installed.
A class object alone does not prove a new persistent Grafana volume can bind.

Before a persistent install, establish an approved supported volume provisioner
and its least-privilege AWS permissions/budget, or supply an existing supported
volume. Do not silently replace the planned PVC with an ephemeral volume or
claim a live Grafana login was verified before its deployment exists.

## Acceptance

Run the console/gateway/adapters tests, full repository gate, then verify a real
Google administrator reaches Grafana and an unapproved/member/disabled account
cannot. Confirm role revocation applies to subsequent requests, spoofed trusted
headers do not bypass the bridge, direct non-bridge Pod access is denied, and
no application credential appears in Grafana requests/logs. Check restart/PVC
retention and existing application login/Work routes before claiming delivery.
