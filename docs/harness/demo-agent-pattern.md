# The Demo-Agent Pattern — Using an Example Agent to Demonstrate Agenova

This is the reusable pattern behind Epic 9: how to prove and teach Agenova's
governance contract by building a demo agent. Follow it to build the next agent demo
(a different agent type, a different backend, a different denial case) without
re-deriving the approach.

## 1. The big idea

An AI agent should not be able to do whatever it wants. Every action it takes —
calling a tool, calling a model — must first be checked against a **claim**: a
system-issued permission slip that says exactly what this one worker run may do and
for how long. When the claim reaches a terminal state (succeeded, failed, expired),
the agent is locked out immediately. A request from an unauthorized caller is denied
**before a claim is ever created**, and that denial leaves inspectable evidence.

A demo agent exists to make that visible. It is fake-but-realistic: it runs entirely
in fixture/demo mode — no real agent binary, no real API keys, no production systems
— but every authorization decision, fact record, and denial in its output comes from
the **real** Agenova code paths. The agent is a prop; the governance is genuine.

### In plain words

**Agenova is a security guard for AI agents.** The product itself (Epics 1–8) is
already built and tested: it issues permission slips (claims), checks every tool and
model call against them, logs everything, and cuts off access the moment the slip
expires. The problem is that all this proof lives inside test files — there is no
way to sit someone down and *show* it working.

**A demo-agent epic builds the show, nothing more.** The "agent" in the demo is an
actor, not a real AI: a small program that plays the role Claude Code would play.
It doesn't think; its tool calls and model responses are scripted fakes. But the
guard checking it is the real Agenova code. Fake actor, real security — that is the
whole trick. (Later, a real agent can replace the actor, because the demo uses the
exact connection points a real agent supports.)

So a demo-agent epic is *neither* building a production agent *nor* testing
Agenova: the agent is a prop, and Epics 1–8 already have their own tests. It builds
the **demonstration** — the runnable story that proves and teaches what Agenova
does. That is why the tickets look the way they do: sample permission files, the
bad-actor scene, the two checkpoints the actor must pass through, the actor and its
script, a real ID badge so the checkpoints can't be fooled, and the packaging so
anyone can run the show.

## 2. The story every demo must tell

This is the demo narrative, in order. A new demo may focus on one chapter, but it
must fit this arc and must never fake an outcome the real path didn't produce.

1. **Request** — a declarative `ClaimRequest` (YAML/JSON) says what the agent wants
   to do and what access it requests. Requested access is intent, never authority.
2. **Bounded permission** — Agenova authorizes the caller, intersects the request
   with the template ceiling and policy, and issues a `SandboxClaim` with effective
   authority. Requests can narrow authority, never create it.
3. **Governed execution** — the running agent reaches tools and models only through
   governed interfaces (Tool/Model gateways). Each call is checked first; allowed
   calls are recorded as facts (`ToolInvocation`, `ModelInvocation`) attributed to
   exactly one claim.
4. **Lock-out** — the claim reaches a terminal state; the very next call is denied
   and no new fact appears. Expiry mid-run shows the same thing.
5. **The bad actor** — an unauthorized submission (wrong team, over-ceiling request)
   is denied *pre-claim* with a real `Decision` record naming the policy and reason;
   the claim store stays empty and zero facts are recorded.
6. **The evidence** — the demo ends with a deterministic summary: claim outcome,
   invocations with claim IDs, decisions. Evidence is the product; the agent's
   "work" is beside the point.

## 3. The layered build pattern

Epic 9 decomposes the demo into layers, each its own ticket with its own evidence.
Reuse this decomposition — and the existing layers — for any new demo.

| Layer | E9 instance | Depends on | What it proves |
|---|---|---|---|
| **1. Fixtures** — canonical inputs (agent template with capability ceiling, valid request, invalid/over-ceiling request) | #116 | nothing — do this first | The declarative contract is concrete and testable |
| **2. Pre-claim denial** — bad-actor demo through the real admission path | #115 | fixtures conventions only | The boundary holds before any claim exists |
| **3a. Governed tool interface** — proxy that checks `ToolGateway.Authorize()` before every tool call | #117 | Layer 1 | Tool access is claim-scoped and recorded |
| **3b. Governed model interface** — proxy that checks `ModelGateway.Authorize()` before every model call | #118 | Layer 1 (parallel with 3a) | Model access is claim-scoped and recorded |
| **4. The demo agent** — one binary wiring backend + both interfaces, runs the full narrative | #119 | 3a + 3b | The whole story in one command |
| **5. Workload identity** — short-lived claim-bound token, verified before any authority use | #121 | 3a, 3b, 4 + an owner design decision | Governance is enforced by proof, not assertion |
| **6. Packaging** — container image + compose so anyone can run it | #122 | 4 | Same story, no local toolchain |
| **7. Real backend** — the same governance against a real cluster lifecycle | #123 | everything above + healthy test cluster | Backend-neutrality is real |

Two structural facts to plan around:

- **The critical path is sequential**: fixtures → interfaces → agent → identity →
  real backend. Only 3a/3b (and the pre-claim denial) parallelize.
- **The demo layers consume the product read-only.** Claim lifecycle, gateways,
  facts, policy/authorization/authority/issuance already exist and are tested —
  plug into them, never modify or rebuild them, and never re-test them (that is
  `scripts/check.ps1` and the harness contract tests' job).

Out of scope until re-prioritized: parent/child lineage and multi-agent
orchestration (#110). Multi-claim demos use independent claims; the mismatch case is
"one running worker nominates the other claim's ID → denied, no fact."

## 4. The planning pattern

What the E9 one-week plan generalizes to:

- **Do fixtures on day one.** They gate everything and are the cheapest ticket.
- **Request owner decisions immediately, not when you reach the ticket.** The
  identity layer needed a design sign-off; asking on day one is what keeps the
  Friday slot safe. Any ticket with a "pending owner decision" note is a day-one
  action item.
- **Parallelize only where the graph allows** (the two interfaces; the denial demo
  alongside anything).
- **Verify external prerequisites early**: container runtime for the packaging
  layer, the test cluster + controller for the backend layer — smoke-test the
  cluster with the *existing* integration test before writing the new one.
- **Budget evidence per ticket, not at the end.** Every ticket requires the focused
  test gate, the repository baseline (`./scripts/check.ps1 -All`), and saved command
  output — prose is not evidence. This is real hours per ticket.
- **Keep a buffer before the demo date** for review feedback and a full rehearsal
  of the narrative in section 2.

Known risk shape: the biggest ticket usually also carries the external dependency
(for E9: identity + owner approval). Name a fallback in the plan (E9's: land the
backend test at fixture-depth claim IDs, clearly labelled, and note the gap).

## 5. Implementation rules for the demo artifacts

Condensed from the merged reference (`examples/engineer/`) and the E9 packets; new
demo code follows these unless the task packet records a deviation.

**Structure**
- Live under `examples/<name>/`; one ticket, one approved task packet
  (`work/<issue>-<slug>/task.md`) anchored to a PRD outcome, before any code.
- Input via a single `--task` flag loaded with the shared strict parsers
  (`v1alpha1.ParseClaimRequestYAML`/`JSON`); never a bespoke parser.
- `main()` only delegates to an injectable `run(args, stdout, stderr, deps) int`;
  exit codes: 0 ok, 1 failure, 2 usage.

**Access**
- Exactly one access seam: a `ToolClient`-style recording mock (fixture depth) or a
  gateway-checked proxy (governed depth). Recording is the proof — tests assert the
  recorded sequence/facts, so a bypassed call fails the test.
- No provider SDK imports, no credentials anywhere, no real OS/API calls in fixture
  mode. Internal packages are read-only.

**Honest labeling**
- Print the mode label before any invocation output: `[FIXTURE MODE] … — not live
  Agenova governance` or `[GOVERNED MODE] claim <id> …`; denials print `[DENIED]`
  with real evidence (principal/policy/reason, or both claim IDs on mismatch).
- A caller-supplied claim ID is a correlation value, not identity; only the verified
  claim token makes a path "live enforced". Never print token values. Never fall
  back silently from live to fixture.

**Tests and evidence**
- Minimum matrix: valid YAML run, valid JSON run, invalid input (non-zero exit,
  actionable error naming file + field, **zero invocations/facts, empty stdout**),
  plus the demo's denial case asserted on the fact store
  (`len(store.ToolInvocations(id))` unchanged), not on printed output.
- PR evidence: exact build command + binary identity, full run outputs (labels
  visible), invalid-input output, focused test output, passing baseline.

**Fixtures**
- New inputs under `harness/fixtures/contract/v0/inputs/<subject>/` with a
  `manifest.json` case (`<subject>.<valid|invalid>.<slug>`); invalid cases need an
  allowed category. The inventory test scans fixture bytes for provider vocabulary
  and secret-like values — both fail the build. Existing fixtures are read-only.
- Ceiling violations are detected by `internal/authority.Resolve()` (schema
  validation cannot see a template ceiling) — demo the rejection through that path.

## 6. Demo day — showing Agenova to an audience, step by step

The run order for presenting the finished E9 demo. Each act maps to a chapter of
the section-2 story. Commands assume all E9 tickets are merged; acts 5–7 need their
external prerequisites (Docker; the kind cluster with the Agent Sandbox controller).

**Act 0 — setup (before the audience arrives)**

```
go build ./...                      # everything compiles, one external dependency
./scripts/check.ps1 -All            # baseline green — keep the output handy
```

**Act 1 — show the permission request.** Open the canonical ClaimRequest fixture
(`harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-claude-code-task.yaml`).
Say: *"This is everything the agent asks for — which tools, which model, how long.
Asking grants nothing; it's intent that the system will narrow against a ceiling
and policy."*

**Act 2 — the bad actor is stopped at the door.**

```
go run ./examples/adversarial --task harness/fixtures/contract/v0/inputs/claim-request/invalid-team-b-unauthorized.yaml
```

The audience sees `[DENIED]` with the principal, the policy reference, and the
reason — and a non-zero exit. Say: *"No permission slip was ever created — the
claim store is empty. The denial itself is recorded evidence."*

**Act 3 — the governed run (the main event).**

```
go run ./examples/coding-agent --task harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-claude-code-task.yaml
```

Walk the output top to bottom as it prints:
1. `[GOVERNED MODE] claim <id> is Running` — the permission slip was issued and
   the agent is now working under it.
2. A tool call — **allowed and logged** with claim ID, tool, timestamp.
3. A model call — **allowed and logged** the same way.
4. The claim completes (`Succeeded`) — and the very next call is **denied**. Say:
   *"Permission ended; access ended. Immediately. No cleanup job, no grace period."*
5. The evidence summary — every action, attributed to exactly one claim. Say:
   *"This audit trail is the product. You always know who did what under which
   authority."*

**Act 4 — asking for too much fails before it starts.**

```
go run ./examples/coding-agent --task harness/fixtures/contract/v0/inputs/claim-request/invalid-claude-code-over-ceiling.yaml
```

Rejected before any claim is created; the error names the ceiling field that was
exceeded; zero invocations recorded.

**Act 5 — same show, containers.**

```
cd examples/coding-agent && docker compose up
```

Three services (tool checkpoint, model checkpoint, agent) run the identical story
and exit 0. Say: *"No Go toolchain, no laptop trick — one command."*

**Act 6 — same show, real Kubernetes.**

```
go test -v -tags integration -timeout 10m ./harness/integration/agentsandbox/ \
  -kube-context kind-agenova-k8s-lab -run TestGovernedCodingAgent
```

The claim binds to a real sandbox pod; the same allow → log → terminal-state →
deny sequence passes against a live cluster. Say: *"The governance layer didn't
change — only the backend did. That's backend-neutrality."*

**Act 7 (optional, once workload identity #121 is merged) — a real agent plugs in.**
Run the coding-agent, then copy-paste the launch command it prints
(`AGENOVA_CLAIM_TOKEN=… ANTHROPIC_BASE_URL=http://localhost:8181 claude
--mcp-config examples/toolproxy/mcp.json …`) to connect a real Claude Code process.
Its calls are authenticated by the claim-bound token, intercepted, checked, and
logged live. Be explicit with the audience: model responses are still mocked at
this stage — this act proves the *interception and identity seams* with real
traffic, not a full live coding session.

Rehearse the acts in order at least once before the real audience; capture each
act's output as the fallback if a live step misbehaves.

## 7. Checklist for the next demo

- [ ] Which chapters of the section-2 story does this demo tell? (At least one
      allow path *and* one denial path.)
- [ ] Which existing layers does it reuse, and what is the one new layer it adds?
- [ ] Ticket + approved packet; owner decisions identified and requested day one
- [ ] Dependency graph drawn; critical path and parallel work identified
- [ ] External prerequisites verified early (runtime, cluster, approvals)
- [ ] Artifacts follow section 5; evidence budgeted per ticket
- [ ] Buffer + rehearsal before the demo date
