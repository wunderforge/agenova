# E13 EKS / Bedrock vertical-slice evidence

Captured 2026-09-28 in Sydney, account 931228356546, region ap-southeast-2, cluster agenova-demo. Current branch codex/e13-aws-demo-bootstrap. These are unmerged implementation results, not a claim that Epic #175 is complete.

## Observed outcome

- Final Terraform refresh reports [no changes](terraform-no-drift.txt).
- EKS ACTIVE, one t3.medium node Ready; [node group](nodegroup.json), [nodes](nodes.txt).
- Agent Sandbox v0.4.6 controller and EKS Pod Identity agent deployed.
- Platform [validate](validate.txt), [plan](plan.txt), [apply](apply.txt) and [status](status.txt) passed against actual EKS. [Identical reapply](reapply.txt) reports changed:false. Policy/template repeat reports already registered.
- [First Work](allowed.json) and [second independent Work](second-allowed.json) both Succeeded on real workers. Nova Micro returned task-dependent answers through the Agenova Gateway. The final first-Work response ID is b80c743f-5e4f-4145-97d3-52f599391497; all model turns have correlated ProviderAttempt/ProviderOutcome facts.
- [Initial counter](provider-before.json) is zero. After the first Work, [before denial](provider-before-denial.json) is four, and [after denial](provider-after-denial.json) remains four. The [denied request](denied.json) has Deny and no claim. This proves the unapproved-project admission negative; it does not replace adversarial cross-claim/model-profile/network tests.
- [Actual second-Work Pod snapshot](second-pod-boundaries.json) includes worker agenova-pool-pool-engineer-k4528, matching its claim backend identity: default service account, automount false, no env or volume mounts. The trusted control plane alone has Pod Identity injection. Node role lacks Bedrock grants. This is observed credential placement, not proof against arbitrary hostile-worker attacks.
- [Security group](security-group.json) has only self-group ingress and no public SSH/application rules. Kubernetes API public access is restricted to the configured operator /32.
- [Invalid configuration](invalid-config.txt) is rejected before mutation: invalid-region. No fallback provider.
- [Installed E2E](installed-ui.txt): 2 passed, comparing CLI/API/Portal canonical records and rendered successful/denied states. [Allowed screenshot](allowed-portal.png), [denied screenshot](denied-portal.png).
- [Full repository gate](final-all.txt) passed docs, Go checks/tests, integration compile, UI contracts/types/component/build, and 52 browser smoke tests. The initial native-select keyboard failure was fixed with keyboard type-ahead selection; no product UI behavior changed.

## Reproduction

The exact nonsecret [Platform](platform.yaml) and [AgentTemplate](engineer.yaml) reference immutable ECR digests from this session. After cleanup those images must be rebuilt/pushed and digests replaced; they are not permanent hosted artifacts. Build/registration procedure and limitations: [runbook](../../../deploy/demo/eks/README.md).

Run with AWS_PROFILE=agenova-demo and KUBECONFIG=/Users/neoliu/.kube/agenova-demo. Session CLI: .tmp/e13/agenova, state: .tmp/e13/state. Explicit --state-dir is required to avoid another installation's state. Local Portal at http://127.0.0.1:5177/?mode=connected#/work/investigate-payment-retries, with .tmp/e13/agenova api connect --state-dir .tmp/e13/state maintaining port 8088.

## Explicit remaining boundaries

- User deferred E14. API still uses reference Team A through an operator-authorized loopback Kubernetes tunnel. No two-real-user authentication, same-request organizational narrowing or public shared API acceptance is claimed. Do not close E13/#146.
- Worker-to-Gateway protocol remains the existing adapter-held controlled-worker bridge; no production worker identity claim.
- E15 egress enforcement / direct-provider bypass / cross-claim malicious-worker proof is not completed. No NetworkPolicy guarantee is claimed.
- kind/Ollama live regression was not run: no local Ollama executable/service was available. Existing kind/OpenAI-compatible Go and fixture UI regressions passed.
- Service Work history is memory-only. Do not restart the control plane before saving the evidence needed for tonight's review.
- Tool observations are synthetic git.read, not real repository tools.
- Cleanup is scheduled at Sydney 2026-09-29 01:00 (UTC 2026-09-28T15:00:00Z). Local automation requires an awake host and valid AWS session; see infrastructure handoff. ECR deletion is restricted to the recorded demo image inventory.

## Temporary HTTPS perimeter — 2026-09-28

Owner authorized shared password access while continuing to defer E14. Live URL:
https://trip-animal-hill-possibilities.trycloudflare.com/?mode=connected

- Independent EKS Deployment `agenova-demo-public`, all three containers Running.
- `deploy/demo/eks/public/verify.py`: 10 live assertions pass, TLS verification on;
  anonymous page/API and wrong password 401; authenticated page/setup/evidence
  200; cross-origin and cross-site requests 403; same-origin malformed submission
  reaches canonical validation (400); plain HTTP 403, no password challenge.
- Exact results: [https-checks.json](https-checks.json).
- `node deploy/demo/eks/public/render.cjs`: authenticated Chromium renders the
  original successful real-Bedrock Work; [rendered proof](https-portal.png).
- Live Pod socket inspection: only 127.0.0.1:8088, :8089 and :20241 listen. Proxy
  has no Kubernetes service-account token mount. No public Service or AWS ingress
  resource created. Pod-scoped forwarding Role targets only the existing control
  plane Pod. Control plane was not restarted; earlier evidence remains available.
- `pwsh -NoProfile -File ./scripts/check.ps1 -All`: exit 0, including 52/52 UI
  smoke tests. Full output `/tmp/agenova-e13-https-all.txt`.
- Limits: one shared credential and reference Team A, not E14 users; temporary
  hostname changes on connector recreation; no service uptime guarantee. Existing
  control-plane memory-only history and E15 gaps remain. Local host is not in the
  request path; teardown automation still depends on it and AWS session validity.
- Cleanup automation updated to remove the overlay before Terraform destruction;
  original Sydney Sep 29 01:00 deadline unchanged. No new ECR images.

## Permanent-domain Terraform adoption

Owner requested Terraform for domain HTTPS resources except the existing zone.
`infra/domain` now owns the imported ACM certificate (us-east-1) and its validation
CNAME; the zone is a read-only data source. No new domain/zone or duplicate cert.
`terraform validate` and `fmt -check` pass. Reviewed apply: 0 add, 1 tag update,
0 destroy; refreshed plan exits 0 with no drift. See [apply](domain/apply.txt)
and [no-drift plan](domain/no-drift.txt). Full repository All gate exits 0,
including 52 UI smoke tests (`/tmp/agenova-domain-all.txt`). Latest certificate
status is PENDING_VALIDATION; no custom-domain reachability or issuance is claimed.
Local state must be preserved separately from disposable EKS state; certificate
and CNAME have prevent_destroy and are excluded from scheduled demo cleanup.

### Apex/wildcard adoption

Console-requested certificate 146c07de-d961-43f3-a291-d27bd552c410 is ISSUED with
agenova.app and *.agenova.app, both DNS validations SUCCESS. Imported into the
independent domain state after Owner deleted the previous demo-only certificate.
One shared verification CNAME imported; historical demo CNAME retained. Apply
only added tags (0 add, 1 change, 0 destroy); refreshed plan no changes:
[proof](domain/wildcard-no-drift.txt). Terraform validate passed and repository
All gate passed including 52 UI smoke tests (`/tmp/agenova-wildcard-all.txt`).
Certificate readiness does not imply website routing has been configured.

## Direct Route 53 → ALB → EKS ingress

Owner explicitly replaced both CloudFront and Cloudflare with ALB. Permanent
Sydney ACM certificate 05298630-af13-42c2-8ffa-4bed2401d2ea is ISSUED for apex and
wildcard. Disposable `infra/demo/edge` owns ALB agenova-demo, HTTPS 443 listener,
instance/NodePort target group, ASG attachment, restricted SG rules and the
`demo.agenova.app` A alias. No AAAA alias is published for this IPv4 ALB.

- ALB target healthy: [target health](alb/target-health.json).
- DNS points directly to ALB: [record](alb/dns.json); [HTTPS listener](alb/listeners.json).
- 11 live HTTP assertions pass: [checks](alb/https-checks.json), including anonymous
  and wrong-password 401, authenticated real evidence 200, cross-origin 403,
  same-origin malformed POST reaching canonical validation (400), no credential
  cache leak after authenticated access. Only static `/healthz` is unauthenticated.
- TLS validation enabled throughout. After the Mac's stale DNS cache cleared,
  all 11 assertions and [rendered Portal](alb-portal.png) passed using normal DNS
  (`dnsOverride: null`). No TLS bypass, resolver override or hosts-file edit.
  Independently, EKS `wget https://demo.agenova.app/healthz` returned `ok`.
- CloudFront E326A8GTQMZBIR deletion completed; AWS GetDistribution now returns
  NoSuchDistribution. Refreshed Terraform plans for edge and domain both exit 0
  (no changes). Cloudflare is absent from the running overlay.
- EKS overlay now has only `proxy` and `forward`; cloudflared was removed. The
  existing control plane was not restarted. ALB SG is the sole new allowed source
  for node port 31089. This is not proof of E15 hostile-worker network isolation.
- Terraform roots validate; full All gate exits 0, 52 UI smoke tests passed
  (`/tmp/agenova-alb-all.txt`). Cleanup automation destroys edge before EKS and
  preserves domain/hosted zone/both regional certificates and validation records.

## External-link navigation fix

The blanket Sec-Fetch-Site cross-site denial also rejected ordinary links.
The proxy now permits only GET + navigate + document for Portal entry paths,
with Basic Auth still required. All 18 live HTTPS assertions passed using normal
DNS: [navigation checks](alb/navigation-checks.json). Cross-site API navigation,
POST, iframe and fetch remain 403; anonymous entry is 401, authenticated entry
200. A fresh Chrome tab opened the real Succeeded Work on the first navigation
without refresh. Only the public proxy was rolled; existing Work history remains.

Full `./scripts/check.ps1 -All` passed after the navigation fix, including
52/52 browser smoke checks (log: /tmp/agenova-navigation-all.txt).

## HTML login and demo sessions

Owner requested ordinary web login for the Codex embedded browser, which could
not complete Basic Auth. The standalone demo session sidecar now authenticates
HTML forms and issues eight-hour opaque server-side sessions. TLS cookies are
Secure/HttpOnly/SameSite=Lax; logout and restart revoke sessions. Signed expiring
CSRF form tokens protect login/logout even when the embedded browser sends
Origin:null. Explicit foreign origins and cross-site APIs remain rejected.

21 live perimeter checks passed: [session checks](alb/session-checks.json).
Session unit tests cover successful opaque-origin login, wrong password, CSRF,
tampering, expiry and logout. Codex IAB rendered the login form, submitted the
existing demo credentials and displayed all four current-session Work records.
One request during rollout returned 504; the complete post-rollout rerun passed.
No control-plane restart, Work loss, provider change or E14 identity claim.
Credentials and session values are omitted from evidence and Git.

Full All gate passed for session login, including 52/52 browser smoke tests
(/tmp/agenova-session-final-all.txt); added CSRF edge-case tests also pass.
