# Temporary authenticated EKS Portal

This optional E13 demo overlay is separate from `platform apply`. It serves the
built Portal inside EKS through a Cloudflare Quick Tunnel. No public Kubernetes
Service, NAT Gateway, load balancer or owned domain is required. Cloudflare
terminates browser TLS; the connector uses an encrypted outbound tunnel.

All pages and API routes require a random shared Basic-auth credential. This is
access to the existing **reference Team A** service, not E14 individual identity
or per-user authorization. Share the password only with trusted demo teammates.
HTTP is rejected before an authentication challenge. Cross-origin browser
requests are rejected before forwarding. The proxy strips the Basic credential
and the externally validated Origin before the private HTTP hop. Access logging is disabled; credentials are not logged. Do not expose the underlying API directly.

The Pod has three containers: nginx on loopback, cloudflared, and kubectl
port-forward on loopback. Only kubectl mounts a Kubernetes token; its Role permits
get/port-forward only to the selected control-plane Pod. No Bedrock IAM identity
is assigned to this Pod. Control-plane restart requires rerunning deployment to
select the new Pod; it does not grant namespace-wide forwarding. The overlay
never restarts the control plane and therefore preserves current Work records.

From the repository root, with an authorized temporary operator login:

```sh
export AWS_PROFILE=agenova-demo
export KUBECONFIG=/Users/neoliu/.kube/agenova-demo
npm --prefix ui run build
python3 deploy/demo/eks/public/deploy.py
kubectl -n agenova-system logs deployment/agenova-demo-public -c tunnel
```

Copy the generated HTTPS URL into `.tmp/e13/public/url.txt`, then run
`python3 deploy/demo/eks/public/verify.py`. Rendered verification uses
`node deploy/demo/eks/public/render.cjs` with the UI Playwright dependencies installed. The password is generated once in
`.tmp/e13/public/credentials.json` (0600, ignored by Git); Kubernetes receives only
a bcrypt verifier. The script requires macOS htpasswd and Python 3. Use Node 24
for the UI build. Public images are pinned by digest. UI assets are a small
ConfigMap archive, not an additional ECR image.

Quick Tunnels have no uptime guarantee and change hostname after connector
recreation. They support at most 200 in-flight requests and no SSE; this Portal
uses polling. Browser Basic-auth credentials remain cached until the browser
session is closed. For a stable shared service, replace this temporary overlay
with a managed hostname/authentication solution and complete E14.

To close access immediately, before the planned cluster destruction:

```sh
kubectl -n agenova-system delete deployment,serviceaccount,role,rolebinding,secret agenova-demo-public
kubectl -n agenova-system delete configmap agenova-demo-public-config agenova-demo-public-site
```

This removes the connector and invalidates reachability without changing the
control plane. No additional AWS cloud resources or ECR digests were created.
The original teardown deadline remains 2026-09-28T15:00:00Z (Sydney 01:00 Sep 29).

Reference: [Cloudflare Quick Tunnels](https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/do-more-with-tunnels/trycloudflare/).
