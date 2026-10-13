# Disposable ALB demo entry

Route 53 `demo.agenova.app` → Sydney internet-facing ALB HTTPS listener → EKS
NodePort 31089 → authenticated Portal proxy → private installed API.

This Terraform root is independent of both `infra/demo/aws` (EKS) and
`infra/domain` (permanent certificates/validation). It reads the existing hosted
zone and issued Sydney certificate; it does not own the domain or certificates.
Run with `AWS_PROFILE=agenova-demo`. Initialize, review a saved plan, then apply.

ALB permits public TCP 443 only. NodePort ingress on the node cluster SG permits
only the ALB SG; the ALB's egress is restricted to that port/SG. Two public
subnets provide the ALB availability zones. Its target group is attached to the
managed-node ASG, so replacement instances register automatically. Kubernetes
Service selection follows replacement proxy Pods. The private API bridge still
pins the control-plane Pod: rerun `deploy/demo/eks/public/deploy.py` after replacing
that Pod. No AWS Load Balancer Controller or node-specific target IP is required.

TLS terminates at the ALB; traffic from ALB to private node addresses uses HTTP.
Only `/healthz` bypasses the shared password, returning a static health signal.
It checks the proxy process, not full model inference availability. Cross-origin
requests remain blocked. Current reference Team A limitations still apply.

Destroy this root BEFORE destroying EKS, after saving evidence:

```sh
terraform -chdir=infra/demo/edge plan -destroy -out=destroy.tfplan
terraform -chdir=infra/demo/edge apply destroy.tfplan
```

This deletes the demo traffic alias, listener, target attachment, ALB and its SG
rules. Keep `infra/domain` and the owner-created hosted zone. Local state and
saved plans are ignored by Git and must remain available for teardown.
