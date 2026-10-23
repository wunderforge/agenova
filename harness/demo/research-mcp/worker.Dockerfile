# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0
# Separate application image; the platform and existing Agent images are unchanged.
# Build from a reviewed public source context with the repository root layout.
FROM golang:1.27.2-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY harness/integration/agentsandbox/testworker ./harness/integration/agentsandbox/testworker
COPY internal/workerprotocol ./internal/workerprotocol
COPY cmd/agenova-demo-worker ./cmd/agenova-demo-worker
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /agenova-workerctl ./harness/integration/agentsandbox/testworker
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w -X main.agentRole=researcher" -o /agenova-demo-worker ./cmd/agenova-demo-worker

FROM alpine:3.20
COPY --from=build /agenova-workerctl /agenova-workerctl
COPY --from=build /agenova-demo-worker /agenova-demo-worker
USER 65532:65532
ENTRYPOINT ["/agenova-workerctl", "serve"]
