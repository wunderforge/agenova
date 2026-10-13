# Authenticated EKS demo Portal

Route 53 → Sydney ALB HTTPS → NodePort 31089 → nginx → loopback API bridge.
The demo perimeter uses an HTML login, not browser Basic Auth. It still shares
reference Team A; it does not implement E14 personal identities or authorization.

The session sidecar verifies the existing high-entropy generated demo password
against its SHA-256 verifier in a Kubernetes Secret. Do not replace that generated
password with a human-chosen password without introducing a password KDF.
Random server-side sessions expire after eight hours and are revoked on logout;
restarting the sidecar invalidates all sessions. Cookies use the __Host- prefix,
Secure, HttpOnly, SameSite=Lax and Path=/. Login/logout forms require a signed,
expiring CSRF token matching an HttpOnly cookie, including when an embedded
browser sends Origin:null. Explicit foreign origins are rejected. API cross-site
checks remain in nginx. Rate limiting bounds password attempts globally; this is
an ephemeral single-replica demo, not a production identity service.

Unauthenticated pages redirect to /login. Unauthenticated APIs return 401 without
WWW-Authenticate. Successful login opens Connected mode. Visit /logout to display
the Sign out form. /healthz is public and only reports proxy health.

Build the standard-library Go sidecar for linux/amd64, put the binary named
session-server beside session/Dockerfile in a temporary build context, and push
the image to the demo ECR repository. Run deploy.py from the repository root with
AWS_PROFILE=agenova-demo, the dedicated EKS KUBECONFIG and a digest-pinned
AGENOVA_SESSION_IMAGE. Build ui/dist first. The script preserves credentials in
.tmp/e13/public/credentials.json (0600) and never prints or commits secrets.

Only the forward container mounts its projected Kubernetes token. Its RBAC allows
get and port-forward on the selected control-plane Pod. Rerun deployment after
replacing that Pod. The proxy/session containers have no AWS Pod Identity.
The control plane and its current-session Work history are not restarted.

Run verify.py for live perimeter checks and render.cjs for browser login/render
verification. Use a Python runtime with the host's trusted CA chain; do not disable
TLS verification. Session unit tests cover tampering, expiry, logout and CSRF.

Destroy infra/demo/edge before EKS. Remove this Deployment, Service, ServiceAccount,
Role, RoleBinding, two ConfigMaps and Secret with the demo. Include session image
digests in ECR teardown. Keep infra/domain, hosted zone and certificates.

The Portal has a `Sign out` link at the bottom right. Confirming signs out the
current session. Anonymous visits to `/` redirect to `/login` on the public
HTTPS origin, without exposing the proxy port.
