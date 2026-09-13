# Third-Party Notices

## Kubernetes SIGs Agent Sandbox

Agenova's `internal/runtime/agentsandbox` adapter contains minimal local wire
types derived from the public CRD/API shapes of Kubernetes SIGs Agent Sandbox
v0.4.6.

- Project: Kubernetes SIGs Agent Sandbox
- Source: https://github.com/kubernetes-sigs/agent-sandbox
- Version used for the verified adapter spike: v0.4.6
- License: Apache License 2.0 (`Apache-2.0`)
- Affected Agenova area: `internal/runtime/agentsandbox`

Agenova does not redistribute the Agent Sandbox controller or SDK. The adapter
interacts with an independently installed controller through Kubernetes
resources and `kubectl`.

## React fixture storyboard

The `ui/` runtime uses React and React DOM (MIT). Its development tools include
Vite and Vitest (MIT), TypeScript and Playwright (Apache-2.0), Testing Library
(MIT), and jsdom (MIT). Exact versions and transitive dependency licenses are
recorded in `ui/package-lock.json`; installed packages retain their upstream
license notices. The application build is local and no browser binaries are
committed or distributed by this repository.
