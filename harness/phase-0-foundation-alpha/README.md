# Phase 0 Harness

The Phase 0 harness keeps evidence small and explicit. It validates local lifecycle behavior and static fixture boundaries.

## Scenarios

- `smoke-warmpool-claim`: a warm pool exists, a claim binds to it, and the claim represents one worker run.
- `smoke-tool-gateway-secret-boundary`: external system credentials are represented outside sandbox environment configuration.

## Run

```powershell
.\scripts\check.ps1 -All
```

On Linux or CI:

```bash
pwsh ./scripts/check.ps1 -All
```
