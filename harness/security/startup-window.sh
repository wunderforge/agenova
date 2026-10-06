#!/usr/bin/env bash
# Checks whether a freshly started claim worker can reach a protected target
# before the generated NetworkPolicy takes effect. It creates a labelled
# SandboxClaim, starts direct requests the moment the worker accepts exec, and
# leaves the verdict to the Ollama log (count of the unique marker).
#
#   startup-window.sh --context kind-agenova-k8s-lab --namespace agenova-system \
#     --mode cold|pool --marker <unique-text> [--target-ip <ollama-ipv4>] [--attempts 30]
#
#   cold: claim without a warm pool, so the runtime starts a new Pod.
#   pool: first claim consumes the warm Pod, the probed claim uses the warm pool
#         like the Agenova adapter does while the pool is being replenished.
set -euo pipefail

context="" namespace="" mode="" marker="" target_ip="" attempts=30
template="agenova-tmpl-engineer" pool="agenova-pool-pool-engineer"
while [ $# -gt 0 ]; do
  case "$1" in
    --context) context="$2"; shift 2 ;;
    --namespace) namespace="$2"; shift 2 ;;
    --mode) mode="$2"; shift 2 ;;
    --marker) marker="$2"; shift 2 ;;
    --target-ip) target_ip="$2"; shift 2 ;;
    --attempts) attempts="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done
for v in context namespace mode marker; do
  [ -n "${!v}" ] || { echo "--$v is required" >&2; exit 2; }
done
case "$marker" in *[!a-zA-Z0-9-]*) echo "--marker must be [a-zA-Z0-9-]" >&2; exit 2 ;; esac

k() { kubectl --context "$context" "$@"; }
here="$(cd "$(dirname "$0")" && pwd)"

if [ -z "$target_ip" ]; then
  node="$(k get nodes -o jsonpath='{.items[0].metadata.name}')"
  target_ip="$(docker exec "$node" getent ahostsv4 host.docker.internal | awk 'NR==1{print $1}')"
fi

claim() {
  local name="$1" warm="$2"
  {
    printf 'apiVersion: extensions.agents.x-k8s.io/v1alpha1\nkind: SandboxClaim\n'
    printf 'metadata: {name: %s, labels: {e15.agenova.io/probe: startup-window}}\n' "$name"
    printf 'spec:\n  sandboxTemplateRef: {name: %s}\n' "$template"
    if [ -n "$warm" ]; then printf '  warmpool: %s\n' "$warm"; fi
  } | k -n "$namespace" apply -f - >/dev/null
}
bound_pod() {
  local name="$1" p=""
  for _ in $(seq 1 600); do
    p="$(k -n "$namespace" get sandboxclaim "$name" -o jsonpath='{.status.sandbox.name}' 2>/dev/null || true)"
    [ -n "$p" ] && k -n "$namespace" get pod "$p" >/dev/null 2>&1 && { echo "$p"; return; }
    sleep 0.1
  done
  echo "claim $name was not bound" >&2; exit 1
}
cleanup() { k -n "$namespace" delete sandboxclaim -l e15.agenova.io/probe=startup-window --ignore-not-found --wait=false >/dev/null; }
trap cleanup EXIT

before="$(bash "$here/ollama-count.sh" --match "$marker")"
case "$mode" in
  cold) claim "e15-startup-$marker" "" ;;
  pool) claim "e15-startup-$marker-consume" "$pool"; bound_pod "e15-startup-$marker-consume" >/dev/null
        claim "e15-startup-$marker" "$pool" ;;
  *) echo "--mode must be cold or pool" >&2; exit 2 ;;
esac
pod="$(bound_pod "e15-startup-$marker")"

# Retry exec until the container accepts it, then fire requests immediately.
for _ in $(seq 1 600); do
  if out="$(k -n "$namespace" exec -i "$pod" -c agent -- sh -s -- "$target_ip" "$marker" "$attempts" 2>/dev/null <<'EOF'
ip=$1 marker=$2 n=$3 i=1
while [ "$i" -le "$n" ]; do
  t=$(date +%s); r=$(wget -q -T 1 -O - "http://$ip:11434/$marker-$i" 2>&1); rc=$?
  printf '%s\t%s\t%s\t%s\n' "$i" "$t" "$rc" "$(echo "$r" | head -c 60)"
  i=$((i + 1)); [ "$rc" -eq 0 ] || [ "$r" != "${r#*timed out}" ] || sleep 1
done
EOF
)"; then break; fi
  sleep 0.1
done
[ -n "${out:-}" ] || { echo "worker never accepted exec" >&2; exit 1; }

started="$(k -n "$namespace" get pod "$pod" -o jsonpath='{.status.containerStatuses[0].state.running.startedAt}')"
created="$(k -n "$namespace" get pod "$pod" -o jsonpath='{.metadata.creationTimestamp}')"
after="$(bash "$here/ollama-count.sh" --match "$marker")"

printf '# mode=%s pod=%s created=%s containerStarted=%s target=%s:11434\n' "$mode" "$pod" "$created" "$started" "$target_ip"
printf 'attempt\tepoch\trc\tdetail\n%s\n' "$out"
printf '# ollama log lines with marker %s: before=%s after=%s\n' "$marker" "$before" "$after"
