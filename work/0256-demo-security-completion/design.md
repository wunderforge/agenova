# Technical Design: Optional reference-agent execution reports

- Ticket: [#256](https://github.com/wunderforge/agenova/issues/256); [spec](spec.md).

## Decision

Keep RuntimeBackend and claim authority unchanged. Extend only the opt-in controlled execution edge and canonical optional evidence metadata. The worker advertises reference-agent features on model operations; the host supplies the corresponding bounded output schema. Ordinary agents request no features and retain the original schema. The host never learns reference-agent behavior by inspecting task text.

Completion metadata is bound to the governed final model action; native reports are matched to model-selected native actions by the authenticated original-Pod session. A native report is an untrusted worker observation with explicit provenance, never a network admission decision. Persist bounded facts before accepting subsequent actions. Independent CNI/control evidence remains a separate acceptance artifact.

## Alternatives

- Merely relabel Failed as Succeeded: rejected; hides incomplete goals and real faults.
- Parse task strings in the platform: rejected; agent owns task semantics.
- Gateway HTTP probe: rejected; cannot establish original Pod network isolation.
- Global shell tool/schema for all workers: rejected; unnecessary compatibility and exposure.

## Verification

Test original protocol and opt-in metadata, forbidden reports, recovery and evidence-write failures. Browser tests assert notices against facts, grouping under long titles/narrow screens, inherited Admin access and adapter display. Real network checks must exclude DNS-only, remote-site, proxy and application-denial causes, and record the original Pod UID. No new cloud permissions/resources are implied.

