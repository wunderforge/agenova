# payment-client

Reference dataset for the Agenova E16 MCP fixture. The files describe one
incident in a small payment client.

- The client calls the upstream payment API with a total request deadline of
  5 seconds.
- Transient upstream errors are retried. The retry budget must fit inside the
  total deadline.
- `logs/timeout.log` holds the request log for the failing payment.
- `src/retry.txt` summarises the retry loop.
- `notes/incident-timeline.md` is the long incident timeline.
