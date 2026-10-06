#!/usr/bin/env bash
# Creates or deletes the probe control Pod: the admitted worker image without
# the Agent Sandbox template label, so the generated NetworkPolicy does not
# select it. With --deny it also creates an experimental default-deny egress
# NetworkPolicy for that Pod only. Both are E15 test objects, never a Platform
# capability, and `down` removes them.
#
#   control-pod.sh up|down --context kind-agenova-k8s-lab --namespace agenova-system \
#     [--name e15-probe-control] [--image agenova-testworker:kind] [--deny]
set -euo pipefail

action="${1:-}"; shift || true
context="" namespace="" name="e15-probe-control" image="agenova-testworker:kind" deny=false
while [ $# -gt 0 ]; do
  case "$1" in
    --context) context="$2"; shift 2 ;;
    --namespace) namespace="$2"; shift 2 ;;
    --name) name="$2"; shift 2 ;;
    --image) image="$2"; shift 2 ;;
    --deny) deny=true; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done
[ -n "$context" ] && [ -n "$namespace" ] || { echo "--context and --namespace are required" >&2; exit 2; }

k() { kubectl --context "$context" "$@"; }

case "$action" in
  up)
    k -n "$namespace" apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: $name
  labels:
    e15.agenova.io/probe: $name
spec:
  automountServiceAccountToken: false
  dnsPolicy: None
  dnsConfig:
    nameservers: ["8.8.8.8", "1.1.1.1"]
  containers:
  - name: agent
    image: $image
    imagePullPolicy: Never
    command: ["sleep", "3600"]
EOF
    if [ "$deny" = true ]; then
      k -n "$namespace" apply -f - <<EOF
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: $name-experimental-deny
  labels:
    e15.agenova.io/probe: $name
spec:
  podSelector:
    matchLabels:
      e15.agenova.io/probe: $name
  policyTypes: ["Egress"]
EOF
    fi
    k -n "$namespace" wait --for=condition=Ready "pod/$name" --timeout=60s
    ;;
  down)
    k -n "$namespace" delete networkpolicy,pod -l "e15.agenova.io/probe=$name" --ignore-not-found --wait=true
    ;;
  *) echo "usage: $0 up|down --context <ctx> --namespace <ns> [--name n] [--deny]" >&2; exit 2 ;;
esac
