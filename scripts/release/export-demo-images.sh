#!/usr/bin/env bash
# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
set -euo pipefail
# Public build artifacts only. No cloud identity, runtime config or secrets.
mkdir -p .tmp/verified-images
docker buildx create --name agenova-verified-oci --driver docker-container --use
docker buildx build --platform linux/amd64 --label "org.opencontainers.image.revision=${GITHUB_SHA}" -f deploy/production/release.Dockerfile -t control-plane:verified --output type=oci,dest=.tmp/verified-images/control-plane.tar .
docker buildx build --platform linux/amd64 --label "org.opencontainers.image.revision=${GITHUB_SHA}" -f harness/demo/research-mcp/worker.Dockerfile -t researcher:verified --output type=oci,dest=.tmp/verified-images/researcher.tar .
docker buildx build --platform linux/amd64 --label "org.opencontainers.image.revision=${GITHUB_SHA}" -f harness/demo/research-mcp/recover.Dockerfile -t member-recovery:verified --output type=oci,dest=.tmp/verified-images/member-recovery.tar .
docker buildx build --platform linux/amd64 --label "org.opencontainers.image.revision=${GITHUB_SHA}" -t research-mcp:verified --output type=oci,dest=.tmp/verified-images/research-mcp.tar harness/integration/mcpfixture
printf '%s\n' "$GITHUB_SHA" > .tmp/verified-images/source-sha.txt
(cd .tmp/verified-images && sha256sum ./*.tar > SHA256SUMS)
