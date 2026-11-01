# Accepted Researcher / Advisor cloud release

Owner authorization: 1 November 2026, Australia/Sydney. Deploy the accepted
feature, verify against real cloud model/MCP services, then close the cluster.
Do not leave newly started compute running overnight. This document records
release intent; it is not evidence that deployment or cleanup has completed.

## Scope and preservation

- Keep feature branch `codex/task-access-advisor` and the original local 5225/5226
  histories. No preview server, fixture identity or local Qwen provider is deployed.
- Use the existing Neo account/region, existing ECR, Bedrock permissions, RDS and
  domain/edge design. No new model IAM, paid search key, Grafana, PVC or database.
- Recover only allowlisted runtime resources from the previously verified locked
  Terraform backend. Preserve original Neo principal and CIDRs. Replan against
  current state; never reuse an old binary plan or expired cleanup window.
- Prior one-use state and migration-secret permissions do not silently expand.
  Refresh normal AWS/CLI login and obtain any specifically needed sensitive access
  before reading it. Never print, commit or upload raw state/credentials.
- After acceptance, remove owned application/controller resources before EKS and
  node deletion, verify owned EC2/EBS/ENIs gone, database available, retained
  resources unchanged and shared state locks released. Preserve recovery inputs.

## Minimal production wiring

1. Production `AGENOVA_ACTIVITY_CAPTURE=memory` explicitly enables the same bounded
   16 MiB volatile content store used in local acceptance, only with authenticated
   member access. Default/disabled remains off; invalid values fail. Current Work
   read authorization gates raw content retrieval. No database migration required.
2. Raw bodies expire after 24 hours or disappear on restart/cluster shutdown.
   Durable facts and their references remain; UI honestly reports unavailable
   bodies. Save sanitized acceptance evidence before shutdown. Durable content
   storage is a separate follow-up, not silently added to the business tables.
3. Bedrock exposes its configured model name and actual stop reason to existing
   projections; no model, credentials or permissions are changed.
4. Publish immutable control-plane, Researcher and MCP artifacts. Append a
   `web.search` route alongside existing read/CRM/git routes. Register a new
   project-qualified immutable template with the packaged access guide, general
   model profile, discovery-v3 and empty direct-public-network authority. Existing
   templates and policy rules are preserved.
5. Configure the independent Advisor with the installed cloud model profile and
   bounded planning quota, enabling Manual/Suggest/Automatic. Model proposals
   still pass ordinary current Policy/ceiling checks.

## Ordered gates

- [ ] Refresh live metadata, current main/source, AWS login and recovery baseline.
- [ ] Production wiring tests, source/browser checks and Linux CI/review.
- [ ] Exact artifacts and restoration/rollback/configuration plan prepared.
- [ ] Restore allowlisted infrastructure; verify admin access, CNI and DNS.
- [ ] Restore authenticated application prerequisites through controlled existing
      provisioning paths; no ad hoc identity grants or secret dumps.
- [ ] Deploy control plane and MCP, register template/guide/policy/configuration.
- [ ] Cloud Manual/Suggest/Automatic, public calendar/weather/Python, synthetic
      customers, real denied call and independent MCP zero receipts.
- [ ] Inspect real provider outputs/summary, tool arguments/results and concrete
      failure causes in UI; verify legacy templates, authorization and history.
- [ ] Retain all attempts, actual timing/token/model/source and sanitized evidence.
- [ ] Close owned runtime; verify preservation and a repeatable recovery plan.

Local 10/10 results with Qwen are a baseline, not cloud Nova acceptance. Any
cloud model-quality or performance failure must be diagnosed and reported.
Gateway denial is distinct from model abstention and from actual packet isolation.
No broad network exception may be added to force a positive result.
