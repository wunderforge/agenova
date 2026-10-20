# Feature Specification: E13 AWS demo infrastructure prerequisite

- Ticket: [#175](https://github.com/wunderforge/agenova/issues/175)
- PRD outcome: [Reference installation](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap).

## Intent

Provide external demo infrastructure to an operator without making cluster creation part of Agenova.

## In Scope

- Repeatable Terraform plan/apply/destroy for a named Sydney demo cluster.
- Explicit operator access and AWS service roles; bounded worker capacity and network access.
- Budget alert setup and cost/lifetime documentation.

## Out of Scope

- Product IAM/authentication implementation, Bedrock inference integration, or full Epic completion.
- Root credentials, IAM access keys/passwords in code/state, general SSO rollout and production reliability.

## Requirements

- Given an authenticated non-root operator, when planning, then the account must match an explicit allowlist and region must be Sydney.
- Given permitted operator CIDRs, when applying, then the public Kubernetes API is restricted to those CIDRs and its private endpoint is enabled.
- Worker capacity defaults to one modest compatible node; size/type are finalized after pricing and workload checks.
- IAM node roles have no Bedrock inference permission. Future trusted gateway identity is separate.
- Budget notifications use a user-selected recipient and confirmed USD equivalent; thresholds are alerts only.
- State, saved plans, credentials and local variable values are excluded from git. Preserve state securely until cleanup completes.
- Given successful creation, when verifying, then ACTIVE cluster and Ready node are observed; Agent Sandbox and Agenova readiness are separately assessed.

## Negative Cases

- Root principal, wrong account, unrestricted API access and invalid configuration fail before paid deployment.
- Unavailable service/permissions yield an explicit blocker; no fallback or automatic privilege expansion.

## Compatibility

Existing product schemas, adapters, reference commands and kind/Ollama behavior remain unchanged.

## Resolved Deployment Inputs

- Budget AUD100/month; initial session ten hours after readiness, then destroy.
- Sydney account 931228356546, existing non-root operator via temporary CLI login.
- EKS1.35 standard support, one On-Demand t3.medium; Agent Sandbox compatibility remains a later installation check.
- Exact readiness timestamp and cleanup deadline are captured during live verification; see [handoff](HANDOFF.md).
