#!/usr/bin/env bash
# Copyright 2026 Frank Y and Agenova contributors.
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

if [ "$#" -ne 1 ] || [ -z "$1" ]; then
  echo "Usage: reproduce.sh <explicit-kube-context>" >&2
  exit 2
fi

context=$1
repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd)
cd "$repo_root"

for command in go kubectl; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "missing required command: $command" >&2
    exit 1
  fi
done

echo "Experiment: Ready worker still rejects Start and Terminate"
echo "Repository commit: $(git rev-parse HEAD)"
echo "Repository dirty: $(if git diff --quiet && git diff --cached --quiet; then echo false; else echo true; fi)"
echo "kubectl client: $(kubectl version --client 2>&1 | head -n 1)"
echo "Kubernetes context: $context"
kubectl --context "$context" version

echo "Installed v0.4.6 CRD schemas:"
kubectl --context "$context" explain sandbox \
  --api-version=agents.x-k8s.io/v1alpha1 --recursive
for resource in sandboxtemplate sandboxwarmpool sandboxclaim; do
  kubectl --context "$context" explain "$resource" \
    --api-version=extensions.agents.x-k8s.io/v1alpha1 --recursive
done

echo "Command: go test -count=1 -v -tags integration -timeout 5m ./harness/integration/agentsandbox/ -run '^TestRuntimeBackend_AllocateObserveCleanup$' -args -kube-context '$context'"

go test -count=1 -v -tags integration -timeout 5m \
  ./harness/integration/agentsandbox/ \
  -run '^TestRuntimeBackend_AllocateObserveCleanup$' \
  -args -kube-context "$context"
