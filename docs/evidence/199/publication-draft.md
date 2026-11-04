# Historical first-slice draft

Superseded by the [full-matrix publication drafts](full/publication-draft.md). Nothing below was published.

# Proposed PR

Title: test: capture real provider outage and recovery evidence for RI-3

Linked ticket: Refs #199 (partial pre-Demo delivery; do not close).
Task packet: `work/0199-provider-failure/task.md`.

A dedicated real Ollama container is stopped while an isolated installed Agenova service retains the same Platform and Work spec. Work fails without a result during the outage and succeeds after restart; provider completion counts, CLI JSON and cleanup receipts are retained in `docs/evidence/199/summary.md`.

Changed boundary: None. Product code is unchanged. Nearest deferred non-goal: worker/controller/control-plane fault matrix after the Demo. Post-terminal invocation replay and an independently instrumented fallback counter are not proven by this campaign.

Validation: two real installed campaigns; Docs; full check -All (exit 0, 52 browser smoke tests). Initial full-check environment prerequisite failure is retained. Owned namespace and container cleaned by recorded identities.

Known delivery constraint: the PR linked-ticket validator requires an automatic-closing keyword. This partial delivery intentionally does not say Closes #199; maintainer direction is needed for that repository rule. Do not modify the validator or misrepresent completion.

# Proposed #199 comment

Pre-Demo RI-3 evidence is prepared locally: on an isolated real installation, stopping a dedicated real Ollama container gives Failed Work, no answer, and zero provider completion delta; restarting it permits the identical spec to succeed. Provider state/probes, CLI JSON and cleanup receipts are retained. Both campaigns passed; Docs and full -All passed (52 browser smoke tests). No product code changed. This does not complete #199: post-Demo matrix, terminal-invocation replay and an independent fallback-attempt counter remain explicitly unproven/deferred. Evidence/PR links will be inserted only after approved publication.
