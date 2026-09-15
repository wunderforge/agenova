# Evidence — E9-T1 engineer Agent Artifact (#53)

Captured 2026-09-14 on macOS (darwin/arm64), branch `claude/0053-engineer-agent-artifact`.

## Reproducible build and binary identity

```
$ go build -o .tmp/engineer ./examples/engineer
$ ls -l .tmp/engineer
-rwxr-xr-x  1 tonyfeng  staff  3764594 Sep 14 19:03 .tmp/engineer
$ shasum -a 256 .tmp/engineer
5ab92380873e75913ef7d5104ffb13c70e455faa271f8d13781084a2f8269c69  .tmp/engineer
$ go version -m .tmp/engineer | head -6
.tmp/engineer: go1.27.0
	path	github.com/wunderforge/agenova/examples/engineer
	mod	github.com/wunderforge/agenova	v0.0.0-20260914084811-6d3541150a98+dirty
	dep	gopkg.in/yaml.v3	v3.0.1	h1:fxVm/GzAzEWqLHuvctI91KS9hhNmmWOoWu0XTYJS7CA=
	build	-buildmode=exe
	build	-compiler=gc
```

Dependency boundary check — the only non-stdlib dependency is the shared
YAML parser; no provider or gateway SDKs:

```
$ go list -deps ./examples/engineer | grep -v "^github.com/wunderforge/agenova" | grep "\."
gopkg.in/yaml.v3
```

## Fixture-driven run: canonical YAML input

```
$ ./.tmp/engineer --task harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-engineer.yaml
[FIXTURE MODE] tool calls are mocked — not live Agenova governance
engineer: claim request "fix-payment-timeout" template=engineer task=repository-change
engineer: invoked tool=git.read scope=repo:acme/payments result=ok
engineer: invoked tool=git.write scope=repo:acme/payments result=ok
engineer: invoked tool=github.pull-request scope=repo:acme/payments result=ok
engineer: completed 3 mocked tool invocations
exit=0
```

## Fixture-driven run: canonical JSON input (same binary, no rebuild)

```
$ ./.tmp/engineer --task harness/fixtures/contract/v0/inputs/claim-request/valid-team-a-engineer.json
[FIXTURE MODE] tool calls are mocked — not live Agenova governance
engineer: claim request "fix-payment-timeout" template=engineer task=repository-change
engineer: invoked tool=git.read scope=repo:acme/payments result=ok
engineer: invoked tool=git.write scope=repo:acme/payments result=ok
engineer: invoked tool=github.pull-request scope=repo:acme/payments result=ok
engineer: completed 3 mocked tool invocations
exit=0
```

## Invalid-input run: non-zero exit, actionable error, no tool sequence

```
$ ./.tmp/engineer --task harness/fixtures/contract/v0/inputs/claim-request/invalid-missing-task.json
engineer: invalid ClaimRequest in harness/fixtures/contract/v0/inputs/claim-request/invalid-missing-task.json: required-field: spec.task: value is required
exit=1
```

## Focused test gate

```
$ go test -count=1 -v ./examples/engineer/...
=== RUN   TestRunCanonicalYAMLFixture
--- PASS: TestRunCanonicalYAMLFixture (0.00s)
=== RUN   TestRunCanonicalJSONFixture
--- PASS: TestRunCanonicalJSONFixture (0.00s)
=== RUN   TestRunInvalidMissingTaskFixture
--- PASS: TestRunInvalidMissingTaskFixture (0.00s)
=== RUN   TestRunMissingTaskFlag
--- PASS: TestRunMissingTaskFlag (0.00s)
=== RUN   TestRunConfiguredToolFailureExitsNonZero
--- PASS: TestRunConfiguredToolFailureExitsNonZero (0.00s)
PASS
ok  	github.com/wunderforge/agenova/examples/engineer	0.435s
?   	github.com/wunderforge/agenova/examples/engineer/toolclient	[no test files]
```

## Repository baseline

**Deviation:** PowerShell is not installed on this macOS host, so
`.\scripts\check.ps1 -All` could not be executed literally. The Go gates it
runs (`scripts/checks/go.ps1` `Test-Go` plus the `Test-Fast` header check)
were reproduced command-for-command; the docs/frontend modules
(`Test-RequiredDocs`, `Test-MarkdownLinks`, `Test-Frontend`, …) were **not**
run and should be covered by a `check.ps1 -All` run in the standard
environment before merge.

```
[pass] gofmt (all repo Go files, excluding .git/.tmp/node_modules/dist)
[pass] SPDX headers (examples/**/*.go carry Apache-2.0 identifiers)
[pass] go mod tidy (no go.mod/go.sum diff)
[pass] go vet ./...
$ go test -count=1 ./...
ok  	github.com/wunderforge/agenova/api/v1alpha1	0.612s
ok  	github.com/wunderforge/agenova/cmd/agenova	5.515s
ok  	github.com/wunderforge/agenova/examples/engineer	1.524s
?   	github.com/wunderforge/agenova/examples/engineer/toolclient	[no test files]
ok  	github.com/wunderforge/agenova/harness/e2e	1.985s
ok  	github.com/wunderforge/agenova/harness/fixtures/contract/v0	2.512s
ok  	github.com/wunderforge/agenova/internal/app	2.790s
ok  	github.com/wunderforge/agenova/internal/authority	3.226s
ok  	github.com/wunderforge/agenova/internal/authorization	3.628s
ok  	github.com/wunderforge/agenova/internal/cli	3.921s
ok  	github.com/wunderforge/agenova/internal/facts	4.297s
ok  	github.com/wunderforge/agenova/internal/governance	4.160s
ok  	github.com/wunderforge/agenova/internal/issuance	4.197s
ok  	github.com/wunderforge/agenova/internal/modelgateway	4.101s
ok  	github.com/wunderforge/agenova/internal/operator	8.385s
ok  	github.com/wunderforge/agenova/internal/policy	4.086s
ok  	github.com/wunderforge/agenova/internal/runtime/agentsandbox	4.388s
ok  	github.com/wunderforge/agenova/internal/toolgateway	4.114s
ok  	github.com/wunderforge/agenova/ui/contractgen	4.011s
$ go test -run '^$' -tags integration ./harness/integration/agentsandbox/
ok  	github.com/wunderforge/agenova/harness/integration/agentsandbox	0.463s [no tests to run]
[pass] integration package compiles
```
