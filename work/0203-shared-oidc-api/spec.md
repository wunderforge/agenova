# Feature specification: E14 member-bound Work

The existing Google/member service supplies a verified subject and active membership to Work admission. The same principal is used for Policy evaluation, claim attribution and owner-only evidence. Canonical `ClaimRequest` contains no identity field. Missing or revoked credentials, forged team input and cross-owner evidence references fail closed before external effects.

The current service uses revocable application sessions and native OS credential storage. The original [#203](https://github.com/wunderforge/agenova/issues/203) describes direct company JWT/JWKS bearer tokens and CLI token files. This difference needs a recorded Ticket/PR decision before M2 acceptance. No Policy, authority or evidence DTO is expanded to accommodate provider claims.

Installed acceptance requires two real company users using one service, identical Work intent, owner-only CLI/API/Portal evidence and sanitized negative cases. Synthetic integration tests prove only the application boundary.
