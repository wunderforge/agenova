# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0

# Test-only E15 image (#198). Same layout and user as the reference worker
# image (harness/integration/agentsandbox/testworker); only the binary at the
# worker entrypoint path is the scripted worker. Never push or publish it.
# Build with the repository root as context:
#   docker build -f harness/security/scriptedworker/Dockerfile -t agenova-e15-scriptedworker:kind .
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY harness/integration/agentsandbox/testworker ./harness/integration/agentsandbox/testworker
COPY harness/security/scriptedworker ./harness/security/scriptedworker
COPY internal/workerprotocol ./internal/workerprotocol
RUN CGO_ENABLED=0 GOOS=linux go build -o /agenova-workerctl ./harness/integration/agentsandbox/testworker
RUN CGO_ENABLED=0 GOOS=linux go build -o /agenova-demo-worker ./harness/security/scriptedworker

FROM alpine:3.20
COPY --from=build /agenova-workerctl /agenova-workerctl
COPY --from=build /agenova-demo-worker /agenova-demo-worker
USER 65532:65532
ENTRYPOINT ["/agenova-workerctl", "serve"]
