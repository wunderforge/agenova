#!/usr/bin/env bash
# Probes protected and control destinations from inside one Pod on the kind
# reference and prints one tab-separated line per probe. Run it against a
# worker and against a control Pod; the comparison, not one run, classifies a
# target. Read-only apart from requests to the listed destinations.
#
#   probe-pod.sh --context kind-agenova-k8s-lab --namespace agenova-system \
#     --pod <pod> [--container agent] --marker <unique-text> [--ollama-ip <ipv4>]
set -euo pipefail

context="" namespace="" pod="" container="agent" marker="" ollama_ip=""
while [ $# -gt 0 ]; do
  case "$1" in
    --context) context="$2"; shift 2 ;;
    --namespace) namespace="$2"; shift 2 ;;
    --pod) pod="$2"; shift 2 ;;
    --container) container="$2"; shift 2 ;;
    --marker) marker="$2"; shift 2 ;;
    --ollama-ip) ollama_ip="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done
for v in context namespace pod marker; do
  [ -n "${!v}" ] || { echo "--$v is required" >&2; exit 2; }
done
case "$marker" in *[!a-zA-Z0-9-]*) echo "--marker must be [a-zA-Z0-9-]" >&2; exit 2 ;; esac

k() { kubectl --context "$context" "$@"; }

# Resolve destinations outside the Pod so DNS inside the Pod cannot hide them.
if [ -z "$ollama_ip" ]; then
  node="$(k get nodes -o jsonpath='{.items[0].metadata.name}')"
  ollama_ip="$(docker exec "$node" getent ahostsv4 host.docker.internal | awk 'NR==1{print $1}')"
fi
api_ip="$(k -n default get svc kubernetes -o jsonpath='{.spec.clusterIP}')"
node_ip="$(k get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}')"
dns_ip="$(k -n kube-system get svc kube-dns -o jsonpath='{.spec.clusterIP}')"
cp_svc_ip="$(k -n "$namespace" get svc agenova-control-plane -o jsonpath='{.spec.clusterIP}')"
cp_pod_ip="$(k -n "$namespace" get pods -l app.kubernetes.io/name=agenova-control-plane -o jsonpath='{.items[0].status.podIP}')"

printf '# pod=%s/%s container=%s marker=%s\n' "$namespace" "$pod" "$container" "$marker"
printf '# ollama=%s api=%s node=%s dns=%s cp-svc=%s cp-pod=%s\n' \
  "$ollama_ip" "$api_ip" "$node_ip" "$dns_ip" "$cp_svc_ip" "$cp_pod_ip"
printf 'target\taddress\trc\tseconds\tdetail\n'

# Busybox tools only: nc, wget, nslookup exist in the admitted worker image.
k -n "$namespace" exec -i "$pod" -c "$container" -- sh -s -- \
  "$ollama_ip" "$api_ip" "$node_ip" "$dns_ip" "$cp_svc_ip" "$cp_pod_ip" "$marker" <<'EOF'
ollama=$1 api=$2 node=$3 dns=$4 cpsvc=$5 cppod=$6 marker=$7
row() { printf '%s\t%s\t%s\t%s\t%s\n' "$1" "$2" "$3" "$4" "$(echo "$5" | tr '\t\n' '  ' | cut -c1-120)"; }
tcp() {
  s=$(date +%s); out=$(nc -z -w 3 "$2" "$3" 2>&1); rc=$?
  row "$1" "$2:$3" "$rc" "$(( $(date +%s) - s ))" "$out"
}
http() {
  s=$(date +%s); out=$(wget -q -T 3 -O - "$2" 2>&1); rc=$?
  row "$1" "$2" "$rc" "$(( $(date +%s) - s ))" "$out"
}
dns() {
  s=$(date +%s); out=$(nslookup "$2" 2>&1); rc=$?
  out=$(echo "$out" | grep -E 'Address|NXDOMAIN|timed out|can.t' | tail -2)
  row "$1" "$2" "$rc" "$(( $(date +%s) - s ))" "$out"
}
tcp  control-public-ip       1.1.1.1 443
dns  control-public-dns      example.com
tcp  control-local-refused   127.0.0.1 9
tcp  ollama-ip               "$ollama" 11434
http ollama-http-marker      "http://$ollama:11434/$marker"
dns  ollama-dns              host.docker.internal
http ollama-dns-http-marker  "http://host.docker.internal:11434/$marker-dns"
tcp  k8s-api-svc             "$api" 443
tcp  k8s-api-node            "$node" 6443
dns  k8s-api-dns             kubernetes.default.svc.cluster.local
tcp  kube-dns                "$dns" 53
tcp  control-plane-svc       "$cpsvc" 8080
tcp  control-plane-pod-8080  "$cppod" 8080
tcp  control-plane-pod-8081  "$cppod" 8081
tcp  metadata                169.254.169.254 80
EOF
