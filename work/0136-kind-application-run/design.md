# Design: governed application run on kind

## Sequence

```text
canonical ClaimRequest YAML
  -> trusted reference principal and PolicyBundle
  -> assignment authorization
  -> Effective Authority resolution
  -> system-managed SandboxClaim issuance
  -> application RunService
  -> recording RuntimeBackend wrapper
  -> controlled Agent Sandbox adapter
  -> kind worker start/result/stop
  -> confirmed Kubernetes cleanup
```

The Team B path stops at assignment authorization. Its recording wrapper must observe zero calls.

## Boundary decisions

- The integration calls the same `app.SubmitClaimRequestFile` entry point used by `agenova run -f`; it does not reconstruct governance inside the test.
- Runtime registration is adapter setup. The reference runtime template key is mapped to the disposable controlled-worker image only at this composition edge.
- A recording wrapper observes the backend-neutral method calls and identities without reaching into adapter internals.
- The existing controlled worker provides actual start/result/stop acknowledgements. Ready remains infrastructure readiness only.
- Independent `kubectl get` checks confirm released resources; the test does not trust only adapter memory.

## Failure handling

- If allocation identity is unknown, retain resources and the unique namespace for inspection.
- If start or stop is unconfirmed, do not claim success or destructive cleanup.
- If cleanup is confirmed, remove the integration-owned template, pool, and namespace artifacts.
