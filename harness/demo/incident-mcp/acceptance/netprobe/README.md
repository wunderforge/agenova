# Native network acceptance observer

This disposable probe is not an authorization mechanism or required Agent base
image. It independently attempts native sockets without using model/tool APIs.
Targets come only from operator-reviewed template command arguments; the process
does not read Work, identity tokens, credentials, task payloads or environment
secret files. Data is synthetic, and the only internal HTTP path is MCP `/healthz`.

## Local gates

```powershell
go test ./harness/demo/incident-mcp/acceptance/netprobe -count=1 -v
go vet ./harness/demo/incident-mcp/acceptance/netprobe
```

These tests use isolated loopback TLS/HTTP/UDP DNS servers and a child executable.
They make no public/cloud calls. A trusted local CA verifies the TLS positive
control normally; an untrusted certificate is classified as an invalid control,
never as evidence of network enforcement. Windows execution needs normal local
socket permission; do not weaken tests to bypass a sandbox restriction.

## Image and immutable template

Build the supplied Dockerfile from this directory, passing the existing reviewed
compatible Worker **digest**, not a mutable tag:

```powershell
docker build --build-arg 'WORKER_IMAGE=<reviewed repository@sha256:64hex>' `
  -t '<owned local test tag>' harness/demo/incident-mcp/acceptance/netprobe
```

It copies a static probe and CA bundle from an already pinned Go build image into
the selected Worker. The original `/agenova-workerctl` and demo worker remain
unchanged. No service, sidecar, listener, privileged mode, new protocol, extra AWS
resource or general Agent base-image requirement is introduced. Publish only via
the reviewed integrated release process, and register a new immutable template.

Set its static command to the following, substituting reviewed public IPv4/TLS
hostname/DNS name and the current private ClusterIP of `e16-mcp`. Do not take
these values from a submitted task:

```yaml
command:
  - /agenova-netprobe
  - -mode
  - wrap
  - -public-host
  - <approved public TLS hostname>
  - -public-ip
  - <approved public IPv4 matching that certificate>
  - -dns-name
  - <approved DNS hostname>
  - -mcp-ip
  - <reviewed private MCP ClusterIP>
  - -nonce
  - network-acceptance
  - -duration
  - 10m
  - -interval
  - 2s
  - -timeout
  - 500ms
  - -hold
```

`wrap` begins observing at process startup, then runs the unchanged
`/agenova-workerctl serve` child. Its control socket and independent
`/agenova-workerctl start|status|stop` commands are unchanged. Probe receipts go
only to container **stderr**, never into controlled-v1 command/stdin/stdout.
Child exit status and termination signals are forwarded; a child ignoring a
termination signal is killed after a three-second grace period. The probe does
not trust the child to report its own network state.

`-mode once` runs one bounded round and writes receipts to stdout; `-hold` adds a
single verified HEAD request for the positive control. Exit zero means a round
was observed, **not** that networking was permitted or blocked.

## Attempts and correlation

Each process has a random-suffixed nonce, monotonic sequence and UTC timestamps.
Each network result links its preceding attempt with `attemptSequence`. The
operator independently binds the log source to Pod UID, Work backend identity,
allocation incarnation and lifecycle time windows.

| Target | Native attempt | What a positive observation means |
| --- | --- | --- |
| `public-tls` | Direct IPv4 TCP443 and certificate-verified TLS with pinned SNI, independent of DNS/proxy | `verified-TLS-reachable`: a verified connection to that target succeeded |
| `dns` | Go resolver lookup, separate from pinned public TLS | `reachable`: a DNS response resolved the approved name |
| `direct-mcp-canary` | GET fixed MCP Service `/healthz`; optional `-mcp-ip` pins private IP:8080 and preserves original Host | `HTTP-reachable`: direct internal route exists; any HTTP response defeats a denial claim |
| `held-tls` | Repeated HEAD over an already established verified connection, retaining connection serial | `existing-connection-response`: that serial remains usable; require multiple samples to test persistence |

The public HEAD path carries only public generated correlation:
`/?agenova_network_probe=<nonce>&probe_attempt=<attemptSequence>&connection=<serial>`.
An existing public HTTPS endpoint such as the application's anonymous landing
may provide independent reverse-proxy access receipts. First verify the selected
IP, hostname, TLS, HEAD handling and access-log capture with a trusted positive
control. Do not follow redirects or send authentication. Receipts log hashes of
configured targets, outcomes and HTTP status, not request/response headers or
bodies. Proxy credentials and HTTP_PROXY are never used.

Without `-mcp-ip`, an offline DNS failure can prevent the internal canary before
TCP is attempted. That is **not** independent proof of a direct-IP block. The AWS
joint gate should therefore use the reviewed Service ClusterIP pin, with a
trusted control-plane positive canary to the identical service/path.

## Interpretation and limits

- Timeouts and connection errors say `enforcement-unverified`. Require a live,
  same-target independent positive control and actual Work/runtime evidence
  before attributing a failure to Policy. NXDOMAIN or TLS verification failure
  means an invalid positive control, not a successful denial test.
- Held-connection closure has unknown cause: the peer may close it. A successful
  before/after sample on the same serial is useful; closure alone cannot prove
  revocation. Confirm original Pod absence and separate replacement identity.
- Stream logs from startup rather than collecting only after deletion. Record
  samples from actual idle/startup, offline Running, approved Running and
  termination/recovery windows; missing windows are inconclusive. Runtime
  `Configured` facts are not packet observations, and stop acknowledgements or
  disappearance of logs are not independently verified termination.
- Duration is capped at 30 minutes; intervals are 500ms..30s and per-target
  timeouts 100ms..5s. Reports stop at 8192 records or 2MiB, explicitly marking
  incomplete observation. Probe completion does not stop the original worker or
  change Work expiry. A new process/replacement gets a new random nonce.
- HTTP status/receipt proves reachability, not arbitrary external credentials,
  hostile-kernel containment, DNS-content filtering or L7 policy enforcement.
  These are deterministic native attempts, **not an LLM jailbreak**.
- Running cross-scope/path attempts over controlled-v1 stdio remain a separate
  malicious-operation test. MCP SDK receipts prove actual tools/call routing;
  native health probes prove only direct networking. Never merge the two claims.

No cloud deployment or real-provider run is performed by these local tests.
