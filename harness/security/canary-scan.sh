#!/usr/bin/env bash
# Scans one Pod for credential exposure without printing secret values: it
# reports variable names, file paths and canary matches only. A planted canary
# that the scan misses invalidates a "nothing found" result.
#
#   canary-scan.sh --context kind-agenova-k8s-lab --namespace agenova-system \
#     --pod <pod> [--container agent] --canary <text-to-find>
set -euo pipefail

context="" namespace="" pod="" container="agent" canary=""
while [ $# -gt 0 ]; do
  case "$1" in
    --context) context="$2"; shift 2 ;;
    --namespace) namespace="$2"; shift 2 ;;
    --pod) pod="$2"; shift 2 ;;
    --container) container="$2"; shift 2 ;;
    --canary) canary="$2"; shift 2 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done
for v in context namespace pod canary; do
  [ -n "${!v}" ] || { echo "--$v is required" >&2; exit 2; }
done

k() { kubectl --context "$context" "$@"; }

printf '# pod=%s/%s container=%s\n' "$namespace" "$pod" "$container"
echo "## pod spec (names only)"
k -n "$namespace" get pod "$pod" -o jsonpath='serviceAccount={.spec.serviceAccountName}{"\n"}automountToken={.spec.automountServiceAccountToken}{"\n"}envNames={.spec.containers[*].env[*].name}{"\n"}envFrom={.spec.containers[*].envFrom[*]}{"\n"}volumes={.spec.volumes[*].name}{"\n"}mountPaths={.spec.containers[*].volumeMounts[*].mountPath}{"\n"}'

k -n "$namespace" exec -i "$pod" -c "$container" -- sh -s -- "$canary" <<'EOF'
canary=$1
echo "## environment variable names (process 1 and this shell)"
{ tr '\0' '\n' < /proc/1/environ 2>/dev/null; env; } | cut -d= -f1 | sort -u | tr '\n' ' '; echo
echo "## credential-like variable names"
{ tr '\0' '\n' < /proc/1/environ 2>/dev/null; env; } | cut -d= -f1 | sort -u \
  | grep -iE 'token|secret|key|pass|cred|auth|aws_|azure|google|ollama|openai|anthropic' || echo "(none)"
echo "## known credential paths"
for p in /var/run/secrets /run/secrets /root/.kube /root/.aws /root/.docker/config.json \
         /root/.config/gcloud /etc/agenova "$HOME/.kube" "$HOME/.aws"; do
  [ -e "$p" ] && echo "present: $p" || echo "absent:  $p"
done
echo "## mounts (non-pseudo filesystems)"
grep -vE ' (proc|sysfs|cgroup2?|devpts|mqueue|tmpfs) ' /proc/mounts | awk '{print $2, $3}'
echo "## files containing the canary (paths only)"
# Busybox grep has no --exclude-dir, so skip pseudo filesystems by hand.
hits=$(for d in /*; do
  case "$d" in /proc|/sys|/dev) ;; *) grep -rlF -- "$canary" "$d" 2>/dev/null ;; esac
done)
[ -n "$hits" ] && echo "$hits" || echo "(none)"
echo "## environment containing the canary (names only)"
{ tr '\0' '\n' < /proc/1/environ 2>/dev/null; env; } | grep -F -- "$canary" | cut -d= -f1 | sort -u || true
EOF
