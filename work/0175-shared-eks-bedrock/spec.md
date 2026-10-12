# Feature Specification: E13 shared EKS service with governed Bedrock inference

- Ticket: [#175](https://github.com/wunderforge/agenova/issues/175).
- PRD outcome: [Reference installation](../../docs/product/prd.md#6-reference-installation-and-initial-policy-bootstrap).

## Intent

Extend the installed service to the selected cloud environment while preserving one governance and evidence contract.

## In Scope

Remote compatible images, Bedrock provider, protected shared API integration, two-user positive/negative cloud evidence.

## Out of Scope

Generic identity product, unrestricted images/providers, HA, persistence, LiteLLM and token-budget product.

## Requirements

- Given a trusted operator Platform configuration, installation uses its adapter-owned images and model binding; Work cannot override those choices.
- Given a logical profile allowed by active claim authority, Gateway invokes the configured Bedrock model using trusted server-side workload identity.
- Given denied authority, Gateway records denial and performs no external invocation.
- Given two validated user identities, the same service authorizes management, Work submission and evidence access per E14 rules, without trusting caller-supplied team fields.
- Given identical template/request inputs, organizational restrictions can only narrow effective authority; the admitted policy snapshot remains stable for that claim.
- Given no network-enforcement proof, the demo reports bypass prevention as unverified, not successful.

## Negative Cases

Authentication failure, unsupported provider/model/image configuration, unavailable AWS credentials, cross-user evidence access, cross-claim or terminal invocation fail visibly. No fallback provider and no credential material in errors or evidence.

## Compatibility

Preserve ClaimRequest/SandboxClaim/RuntimeBackend neutrality, canonical evidence, and the kind/Ollama path. Existing local reference modes remain explicitly labelled.

## Open Decisions

E14 identity contract is not delivered in current main. Before the shared API implementation, select an existing verified upstream integration or agree a narrow server-verified demo credential contract; do not silently turn local reference principals into real identities.
