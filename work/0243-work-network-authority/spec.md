# Feature Specification: Per-Work network authority and fail-closed egress

- Ticket: [#243](https://github.com/wunderforge/agenova/issues/243)
- PRD outcome: [Authority](../../docs/product/prd.md#4-claim-scoped-authority).

## Intent

Arbitrary compatible images remain supported; Internet access is task permission, not an image property. Add optional networkProfiles lists to requestedAccess, AgentTemplate capabilityCeiling, EffectiveAuthority and Policy AuthorityConstraints (StringSetConstraint). Initially only public-web is valid, at most once. Empty/omitted grants no outbound networking. Public-web means public TCP443 and necessary DNS, not FQDN, HTTP methods or TLS content enforcement.

## In Scope

- Resolve request intersect template and exact admitted Policy constraints. Public egress additionally requires an explicit matching Policy network cap; missing cap is deny. Snapshot all effective grants and bind them to the full request.
- Propagate resolved network input and absolute expiry to a backend-neutral runtime boundary. Default warm/new workers deny from process startup; activate only the concrete Running Work; revoke and confirm termination on terminal/cancel/expiry/recovery.
- Shared CLI/API/UI evidence records requested/effective profiles and bounded application/release facts. No packet payloads or credentials, no fictional ToolInvocation for native packet drops.

## Out of Scope

- Generic firewall, proxy, new CNI or chargeable resources without approval, changing worker protocol or restricting base images.

## Requirements

- Given the same principal/template/Policy, a Work without network request remains offline while a separately approved public-web Work can connect. Permission changes for new Works never widen old snapshots or peer workers.
- Unapproved/unknown profiles fail before allocation; startup isolation must be provided by the substrate, not a worker self-check or delay. In-memory evidence is explicitly simulated; unsupported real backend enforcement fails closed.
- Use independent backend expiry to bound service-outage exposure, test actual revoke latency and existing TCP connections, and never report release merely because a YAML object was deleted.

## Negative Cases

- Reject unsolicited Policy grants, template-wide allows, idle access, stale Work identity and ambiguous cleanup. Provider/secret handling and existing permissions remain unchanged.

## Compatibility

- Old YAML remains parseable with no network access; existing images, execution protocol, login and other authority dimensions stay compatible. Default Internet behavior intentionally tightens and old pools require controlled replacement. Historical evidence without a network guarantee stays unknown. No new database table is assumed.

## Open Decisions

- Product scope approved by Owner; any new infrastructure, shared RBAC expansion or branch-protection bypass requires separate approval.
