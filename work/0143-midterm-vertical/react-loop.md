# Demo agent: bounded ReAct loop

Owner requested a real multi-step agent on 15 September, not a one-shot answer or timed
UI simulation. This is an approved elaboration of #143's single-worker demo.

## Acceptance

- The kind worker repeatedly asks the real model to choose a structured action, executes
  a chosen governed tool, feeds the observation back, and asks for the next action.
- Model-selected file/order/round count varies; no hard-coded sequence or canned answer.
  At least two model turns; when a mock read is available, at least one observed read.
- Six model turns maximum; bounded messages and transcript; invalid action feedback is
  bounded, exhaustion fails rather than fabricating success.
- Only the existing effective git.read scope is advertised. Synthetic README/log/source
  artifacts are explicit mock tool data, not real repository edits or filesystem access.
- Host validates session binding, running authority, model profile, rooted task prompt,
  operation cap and final answer matching the last governed structured finish action.
- Host records agent turns and observations, and gateway decisions/attempts/outcomes.
  UI renders these real records with one small waiting lamp; no private reasoning trace.

## Gates

- `go test ./cmd/agenova-demo-worker ./internal/workerprotocol ./internal/runtime/agentsandbox ./internal/console`
- Worker tests prove two different model-selected paths and tool-observation feedback;
  transport tests reject forged final results, foreign claims, unrelated prompts and caps.
- Console tests prove mock tools enter the gateway and denied calls never reach adapter.
- Existing React unit/browser gates plus turn/provider correlation regression.
- `./scripts/check.ps1 -All`
- Opt-in kind/Ollama and browser gate from [demo.md](demo.md), using only the existing
  local model on synthetic inputs, no downloads or paid calls.

## Todo

- [x] Worker loop and strict action schema
- [x] Governed mock tool + host-owned turn facts and transport validation
- [x] UI shows turns and model/tool attempts without expensive lifecycle animation
- [x] Focused/full tests, real kind/Ollama evidence and refreshed local server

No multi-agent scheduling, new RuntimeBackend contract, real coding framework or broader
policy UI. Outcome model usage remains the final call's metadata, not aggregate loop usage.

## Gate-driven repair

The first real llama3.1 run exhausted six turns: it reread already observed files instead
of finishing. Synthetic path tests passed but did not predict this behavior. The repair
adds explicit turn/budget/seen-file state, stops redundant successful rereads, and asks
for a model-produced final action when evidence is sufficient. There is no fixed file
sequence or fallback answer. Exhaustion still fails closed.

Ordinary JSON mode admitted misspelled fields. In the installed Ollama 0.30.6,
an adversarial probe showed that the schema containing string `maxLength` constraints
was not enforced; the same probe with the simpler schema enforced required fields and
action enums. Keep the verified provider grammar subset and enforce byte limits in the
strict parser/host. This is local compatibility evidence, not a guarantee for other
providers. [Provider documentation](https://docs.ollama.com/capabilities/structured-outputs).

Observations now identify tool/file directly, avoiding ambiguous nested action examples.
Invalid action feedback explains mutually exclusive tool/finish fields. Tests also cover
failed tool observation recovery and invalid action recovery, not just happy paths.

## Real browser evidence (15 September)

- Final-image browser gate passed for `work-8e380c6e-b5f8-44b3-bc70-4a5aa90c5e29`:
  6 model turns, 4 observations (including one failed mock read and recovery),
  real model-selected reads, task-dependent answer, narrowed 45m to 30m authority,
  Tool Gateway mock activity, Model Gateway activity and confirmed cleanup.
- Prior compatible-image browser gate also passed for
  `work-14e5afc3-87a3-4a32-9748-df6ff22d36c6` (5 model turns, 3 observations).
- Evidence and screenshots: `.tmp/ui-kind-checkpoint/`. Failed-run evidence was saved
  before replacing only task-owned local servers. Current server retains both successes.
- One isolated live proof completed the task but failed runtime stop/cleanup; namespace
  `agenova-react-proof-0915-verified` is retained, not misreported as a clean success.
  Outcome and cleanup remain separate. Final independent proof and baseline below.

- Final independent `TestUIModelCheckpoint_Kind` passed (61.13s): 6 model turns,
  3 successful observations, same-claim/provider correlation and cleanup confirmed;
  Team B denial made zero backend/provider calls. Disposable proof namespace removed
  only after presence-checked cleanup. No private model reasoning was recorded.

- Final `./scripts/check.ps1 -All` passed: Go tests/vet, canonical frontend contracts,
  98 UI unit tests, production build and 41 browser tests, including ReAct polling,
  tool/model correlation, source isolation, one-lamp budget and stable scroll/DOM.
- Running local composition: Portal `http://127.0.0.1:5175/?mode=connected`, server
  `.tmp/agenova-console-react-v4.exe` at 8088, namespace `agenova-react-ui-0915-final`.
  Updated test worker image loaded; only idle test warm fixtures were replaced.
