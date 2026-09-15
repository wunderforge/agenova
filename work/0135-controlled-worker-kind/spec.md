# Feature Specification: Prove opt-in controlled worker start and stop on kind

- Ticket: [#135](https://github.com/wunderforge/agenova/issues/135)
- PRD outcome: [Backend-neutral execution](../../docs/product/prd.md#3-backend-neutral-execution)

## Intent

Allow one explicitly compatible Agent Sandbox image to acknowledge real work start and stop through an adapter-held protocol while leaving ordinary v0.4.6 capabilities unchanged.

## In Scope

- Fixed derivation of an argv-safe worker token from any system-issued claim ID.
- Start/result, status, stop and cleanup acknowledgements bound to that token and backend worker identity.
- In-process concurrency serialization and a separate opt-in integration tag.

## Out of Scope

- Filesystem/network isolation, production identity, restart recovery, arbitrary agents, and native upstream Start/Terminate.

## Requirements

- Given a Ready controlled worker, when Start succeeds, then an exact token-bound child-process result is confirmed before the adapter reports success.
- Given Start is in flight, when Terminate begins, then it waits and confirms stop before returning.
- Given Terminate is already in flight, when another Terminate begins, then it waits and returns the same idempotent result without a second stop.
- Given a system-issued claim ID with spaces, separators, Unicode or long text, when the worker protocol is invoked, then the adapter uses its fixed SHA-256 token rather than rejecting the claim.
- Given the default integration gate, when it runs without the disposable image, then the controlled test is excluded.

## Negative Cases

- Readiness never substitutes for the controlled start acknowledgement.
- Cleanup refuses running, starting, stopping or unknown work.
- Start after confirmed termination returns `ErrTerminated` without depending on mutable backend readiness.

## Compatibility

- `RuntimeBackend` and public APIs remain unchanged. `SpikeAdapter` continues to return `ErrUnsupported` for Start and Terminate.

## Open Decisions

- None. Promotion beyond this test protocol requires separate #51 evidence and review.

