#!/usr/bin/env bash
# Cleanup-boundary regression for startup-window.sh, using a fake kubectl on
# PATH: no Docker, Kubernetes, Ollama or user kubeconfig.
#
#   bash harness/security/test-startup-window.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bin"
: >"$work/ollama.log"

# Claims live as files named after the claim, holding their UID. Every call is
# appended to calls.log so tests can assert which mutations were issued.
cat >"$work/bin/kubectl" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
echo "$*" >>"$FAKE_STATE/calls.log"
[ "$1 $2" = "--context ctx" ] || { echo "fake: missing --context ctx" >&2; exit 90; }
shift 2
[ "${1:-}" = -n ] && shift 2
claims="$FAKE_STATE/claims"
case "$*" in
  'create -f - -o jsonpath={.metadata.uid}')
    name="$(sed -n 's/^metadata: {name: \([^,]*\),.*/\1/p')"
    [ ! -e "$claims/$name" ] || { echo "AlreadyExists: $name" >&2; exit 1; }
    echo "uid-$name-new" >"$claims/$name"; printf 'uid-%s-new' "$name" ;;
  'get sandboxclaim '*' -o jsonpath={.status.sandbox.name}')
    [ -e "$claims/$3" ] && printf 'pod-%s' "$3" ;;
  'get pod '*' -o jsonpath='*) printf '2026-10-17T00:00:00Z' ;;
  'get pod '*) : ;;
  'exec '*) cat >/dev/null; printf '1\t0\t0\tok' ;;
  'delete --raw '*' -f -')
    name="${3##*/}" want="$(sed -n 's/.*"uid":"\([^"]*\)".*/\1/p')"
    [ -e "$claims/$name" ] || { echo "NotFound: $name" >&2; exit 1; }
    [ "$(cat "$claims/$name")" = "$want" ] || { echo "Conflict: uid precondition" >&2; exit 1; }
    rm "$claims/$name" ;;
  *) echo "fake kubectl: unhandled: $*" >&2; exit 1 ;;
esac
FAKE
chmod +x "$work/bin/kubectl"

fail() { echo "[fail] $1" >&2; exit 1; }
run_case() {
  export FAKE_STATE="$work/$1"; shift
  mkdir -p "$FAKE_STATE/claims"; : >"$FAKE_STATE/calls.log"
  PATH="$work/bin:$PATH" OLLAMA_SERVER_LOG="$work/ollama.log" \
    bash "$here/startup-window.sh" --context ctx --namespace ns --target-ip 192.0.2.1 "$@" \
    >"$FAKE_STATE/out" 2>"$FAKE_STATE/err" && status=0 || status=$?
}
seed() { mkdir -p "$work/$1/claims"; echo "uid-$2-other" >"$work/$1/claims/$2"; }
present() { [ -e "$work/$1/claims/$2" ]; }
no_mutation() { ! grep -E ' (create|delete) ' "$work/$1/calls.log" >/dev/null; }

run_case invalid-mode --mode invalid --marker a
[ "$status" = 2 ] || fail "invalid --mode exited $status, want 2"
no_mutation invalid-mode || fail "invalid --mode issued a create or delete"
echo '[pass] invalid input exits before any create or delete'

run_case invalid-attempts --mode cold --marker a --attempts x
[ "$status" = 2 ] && no_mutation invalid-attempts || fail "invalid --attempts was not rejected cleanly"
echo '[pass] invalid --attempts exits before any create or delete'

seed other-marker e15-startup-b
run_case other-marker --mode cold --marker a
[ "$status" = 0 ] || fail "cold run failed: $(cat "$work/other-marker/err")"
present other-marker e15-startup-a && fail "own claim e15-startup-a was not deleted"
present other-marker e15-startup-b || fail "another marker's claim was deleted"
grep -q 'sandboxclaims/e15-startup-b' "$work/other-marker/calls.log" && fail "cleanup touched another marker's claim"
echo "[pass] cleanup deletes its own claim and leaves another marker's claim"

seed existing-name e15-startup-a
run_case existing-name --mode pool --marker a
[ "$status" != 0 ] || fail "run adopted an existing claim name"
[ "$(cat "$work/existing-name/claims/e15-startup-a")" = uid-e15-startup-a-other ] || fail "existing claim was overwritten or deleted"
present existing-name e15-startup-a-consume && fail "claim created before the failure was not cleaned up"
echo '[pass] an existing claim is refused, kept intact, and earlier owned claims are cleaned up'
