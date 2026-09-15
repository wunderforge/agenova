# Copyright 2026 Dapeng Zhang and Agenova contributors.
# SPDX-License-Identifier: Apache-2.0

# Build with the repository root as context, not this directory.
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY harness/integration/agentsandbox/testworker ./harness/integration/agentsandbox/testworker
COPY internal/workerprotocol ./internal/workerprotocol
COPY cmd/agenova-demo-worker ./cmd/agenova-demo-worker
RUN CGO_ENABLED=0 GOOS=linux go build -o /agenova-workerctl ./harness/integration/agentsandbox/testworker
RUN CGO_ENABLED=0 GOOS=linux go build -o /agenova-demo-worker ./cmd/agenova-demo-worker

FROM alpine:3.20
COPY --from=build /agenova-workerctl /agenova-workerctl
COPY --from=build /agenova-demo-worker /agenova-demo-worker
USER 65532:65532
ENTRYPOINT ["/agenova-workerctl", "serve"]
