# Authenticated EKS Portal behind ALB

The E13 overlay serves the built Portal inside EKS. Route 53 `demo.agenova.app`
points directly to a Sydney ALB with an ACM certificate. There is no CloudFront
or Cloudflare dependency in the final deployment. Infrastructure is managed in
`infra/demo/edge`; permanent certificates are in `infra/domain`.

All pages and APIs require a random shared Basic-auth credential. This grants
access to the reference Team A service, not E14 individual identity/authorization.
Only `/healthz` is public and returns static health. HTTPS is required for other
requests; cross-origin requests are rejected, and Basic credentials are stripped
before the private API hop. Access logging is disabled.

The overlay has nginx on port 8089 and a loopback kubectl forwarder to the
unchanged control-plane API. Only kubectl mounts a service-account token, with
get/port-forward access to the selected control-plane Pod only. No Bedrock IAM
identity is assigned. A NodePort Service exposes 31089; node SG ingress allows
only the ALB SG. No control-plane restart is needed, preserving Work history.
Rerun deployment when the selected control-plane Pod is replaced.

```sh
export AWS_PROFILE=agenova-demo
export KUBECONFIG=/Users/neoliu/.kube/agenova-demo
npm --prefix ui run build
python3 deploy/demo/eks/public/deploy.py
```

Use Node 24 for the build. Credentials are in `.tmp/e13/public/credentials.json`
(0600, ignored by Git); only a bcrypt verifier is stored in the Kubernetes Secret.
The script needs Python 3 and macOS htpasswd. Images are pinned by digest. Portal
assets are distributed through a ConfigMap archive.

Save `https://demo.agenova.app` in `.tmp/e13/public/url.txt`, then run:

```sh
python3 deploy/demo/eks/public/verify.py
node deploy/demo/eks/public/render.cjs
```

The browser verification needs UI Playwright dependencies. ALB serves HTTPS 443
only; HTTP port 80 is closed. Basic-auth credentials are browser-session cached.
Use Chrome if the embedded preview cannot show the authentication prompt.

At the authorized cleanup deadline, destroy `infra/demo/edge` first, then remove:

```sh
kubectl -n agenova-system delete deployment,service,serviceaccount,role,rolebinding,secret agenova-demo-public
kubectl -n agenova-system delete configmap agenova-demo-public-config agenova-demo-public-site
```

Finally destroy EKS using `infra/demo/aws`. Preserve the domain, hosted zone,
certificates, validation records and `infra/domain` state. The local cleanup
automation still requires a running host and valid AWS login.

External cross-site links may GET the top-level Portal document (`/` or
`/index.html`) after Basic authentication. Cross-site API navigation, fetches,
iframes and POSTs remain denied; Origin checks are unchanged. The live verifier
covers both entry and denial cases.
