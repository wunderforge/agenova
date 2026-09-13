# Feature Specification: Minimum agent filesystem boundary

- Ticket: [#89](https://github.com/wunderforge/agenova/issues/89)
- PRD outcome: [Backend-neutral execution](../../docs/product/prd.md#3-backend-neutral-execution).

Implemented for the reference contract. Real hostile-process isolation remains unverified until #51 supplies backend evidence.

## Intent

One worker assignment receives one writable task directory. Its agent process and subprocesses use normal filesystem APIs. Backend isolation makes that boundary effective; an in-memory contract model only expresses and tests the semantics.

## In Scope

- Directory lifetime and identity, ordinary local tools, one outside-boundary rule, supported output handoff, capability reporting and reference cases.

## Out of Scope

- Host-path mounts, persistent/cross-claim workspaces, file browsing/editing services, remote shells, general path policy, per-file governance or provider-specific public configuration.

## Requirements

### Directory and configuration

1. Given an authorized claim, allocation prepares one fresh, private writable directory and correlates it with that claim and backend allocation. The runtime selects its worker-visible absolute path; callers do not supply a host path. Examples may call it `W`; `W` is notation, not a new public field or mandatory literal `/workspace` path.
2. Before actual work starts, the worker receives that directory as its initial cwd. Agent code can create repository subdirectories and invoke git, compilers and tests directly. Scratch, a synthetic HOME and writable caches are children of W; supported artifact setup must redirect tools' mutable defaults there. No host HOME, global git config or credential helpers are inherited.
3. A prepared repository is copied from a controlled fixture/input or obtained through already-authorized external access. Task input and a repository URL do not grant access. Local git operations require no gateway filesystem proxy; remote fetch/push and model/tool access still obey existing gateway and credential boundaries. The reference fixture uses local git data with no external network requirement.
4. A second or replacement claim receives fresh data even if the backend reuses physical infrastructure. No claim receives another or previous claim's task directory or files. An allocation may not silently reuse another claim's files.

### One outside-boundary rule

**Only W and its descendants may contain writable task filesystem data. Runtime-supplied files outside W are read-only; every other filesystem data path is unavailable through the supported configuration.** No caller path allowlist, optional host mount or second writable task root exists in the MVP.

Containment means the resolved filesystem object, not a string prefix. Parent traversal, sibling-prefix paths, symlinks, hard links, junctions or other aliases cannot confer write access outside W or access to another claim/host data. Runtime executable/library files can remain readable/executable outside W. The working root itself cannot be replaced to point outside its allocation.

The backend profile owns substrate layout and enforcement. Runtime facilities such as devices and process interfaces are not extra task storage or an escape route; #48 must enumerate required facilities and gaps and #51 must verify the selected profile. This specification does not claim every operating-system object is a regular read-only file. A backend that requires another writable task directory, exposes host data, or cannot establish containment must report unsupported behavior rather than claim conformance. The reference model always declares that it does not isolate a hostile process.

### Termination, cleanup and outputs

- W is ephemeral and belongs to one allocation. It is usable for work only while the worker is running under the accepted lifecycle; preparation before start does not grant governed authority.
- The MVP output path is explicit export **before termination**. The worker prepares a diff (including an explicit choice about untracked files), commit bundle or regular artifact under W, then submits its bytes through an approved governed tool/output operation selected by the integration. #89 defines no production handoff API. A commit hash alone is not exported content unless the commit has already reached the caller-owned destination. The directory itself is never the retained output.
- The local reference evidence uses a trusted fixture collector as a clearly labeled stand-in for that external output destination. It does not introduce a new production gateway method, arbitrary host-path reader or artifact-storage service. The collector accepts only explicitly selected relative regular-file outputs under W, rejects aliases/escape/special files and applies fixture byte limits. Consumers must select their real existing handoff operation before claiming live integration.
- Export is not implicit success, and export failure is visible. A final diff after an abrupt failure/timeout is not guaranteed; previously acknowledged exports survive independently of workspace cleanup. No post-terminal gateway authority or interactive retrieval window is introduced.
- Termination must stop the worker and descendants before cleanup can be reported complete. Cleanup removes/releases W and prevents reassignment with old contents. There is no user-selectable retention period or later remount. This is logical resource cleanup, not a promise of physical secure erasure.
- Failed termination or cleanup produces separate evidence, preserves the original work outcome and quarantines the allocation from reuse until cleanup is confirmed. Repeated cleanup follows the merged #30 operation semantics and cannot delete another claim's data. Unknown handles fail explicitly.

### Capability and evidence contract

Describe the working-directory support, outside rule, cleanup and output behavior using neutral values associated with claim/backend identity. Report whether evidence is simulated, locally executed or real-backend verified, and name unsupported semantics. Exact fields and integration points are selected only against the merged #30 contract. Readiness supports Bound; it proves neither start nor success. No new claim phase or authority store is added.

## Negative Cases

The reference suite executes the model-level cases. Ticket #51 must repeat the
worker-observable cases on the real backend before reporting `BackendVerified`.

| ID | Case | Required observation |
| --- | --- | --- |
| FS-N0 | Allocation is Bound/ready but `Start` has not succeeded | Worker cannot read or write W; prepared sentinel remains untouched until explicit Start |
| FS-N1 | Write/create/delete/rename outside W, including a direct absolute path such as `/tmp/agenova-outside-sentinel` | Denied or unavailable; synthetic outside sentinel unchanged |
| FS-N2 | `..`, absolute outside path, sibling prefix, symlink/junction escape or hard-link alias | No outside mutation or host/cross-claim read; model labels simulated alias resolution |
| FS-N3 | Caller asks for host HOME, host credentials, host mounts or another claim's directory | Rejected before worker start; no successful configuration or inherited host credential helper |
| FS-N4 | Claim B attempts to use claim A's directory; replacement follows A | No access to A's task files; replacement is fresh |
| FS-N5 | Backend lacks the rule, root is unusable, or start fails | Explicit failure/gap; no fabricated conforming allocation or successful work |
| FS-N6 | Output selection escapes W, names a link/special file, exceeds fixture limit or export fails | No outside read and no successful export receipt; no widened authority |
| FS-N7 | Successful termination before cleanup, terminal outcome, or forced timeout occurs before export | Governed export denied; an unexported sentinel never reaches the destination; existing acknowledged export remains unchanged |
| FS-N8 | Termination remains incomplete when cleanup is requested; cleanup fails/repeats; unknown allocation | Cleanup cannot report release or permit reuse while worker/descendant termination is incomplete; errors remain separate from outcome; never clean another claim |

Positive cases: FS-P1 prepares a fixture repo and records cwd, command/tool versions, edits, local git diff, compilation and individual exits; FS-P2 exports known output bytes and verifies receipt/digest; FS-P3 terminates the worker and a background heartbeat descendant before cleanup, retaining evidence and the exported result but no reusable task directory; FS-P4 reads an allowed runtime fixture outside W while denying its mutation. The local compatibility fixture proves FS-P1/FS-P2 only, while model operations cover lifecycle ordering without claiming native isolation. FS-P3 is real-backend-only for v0 and remains mandatory in #51.

## Compatibility

- Preserve canonical AgentTemplate/ClaimRequest parsing and authority. No host-path, mount, generic provider-map or credentials field is added as a shortcut.
- Extend only the merged #30 allocation/observation values with the neutral boundary description. Do not migrate gateways from this ticket.
- #48 maps accepted requirements to native, translated, adapter-held or unsupported capabilities; #51 validates the same workload and negative boundary on a real backend. #52 owns gateway bypass/egress proof.
- #53 can continue fixture/mock work without depending on this unaccepted API. Its thin external tool client does not require wrapping ordinary local file I/O. Integration later supplies cwd and a supported export client without rebuilding authority semantics.

## Open Decisions

- None for the reference contract. #30 is merged, and the neutral description attaches to Allocation and Observation without changing its five operations or adding a file-access API.
- #48/#51 must still decide and prove the provider-specific layout. Unsupported semantics remain explicit gaps.
