# Integration design

Ticket [#218](https://github.com/wunderforge/agenova/issues/218); [acceptance](spec.md).

Base on member/durable #216, integrate pinned Tool history locally, and resolve composition semantically: retain PrepareFor/SetupFor; configure real tools; filter catalog per issued authority; retain journal durability and role checks. Adapt #217 diagnosis changes to durable behavior rather than reverting to its older in-memory base.

Control Plane contains Gateway/provider. Worker receives only filtered logical descriptors. Endpoint and credentials remain host-side. Matching CLI/Portal schema accompanies the CP/Worker handoff; MCP is independently deployed. Do not add speculative OTEL implementation.

Progress from focused integration regressions and native Windows to disposable TLS SQL, browser gates and Linux builds. Read no real OAuth/AWS secrets. Retain unresolved credential finding as a release restriction. Real EKS/Google/model/tool acceptance waits for Owner-controlled CI/CD deployment.
