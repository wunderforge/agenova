# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0

# Build with the repository root as context. Pin both base images to approved
# digests in the private deployment repository before promotion.
ARG GO_IMAGE=golang:1.22-alpine
ARG RUNTIME_IMAGE=alpine:3.20

FROM ${GO_IMAGE} AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/agenova-workerctl ./cmd/agenova-workerctl
COPY cmd/agenova-demo-worker ./cmd/agenova-demo-worker
COPY internal/workerprotocol ./internal/workerprotocol
RUN mkdir -p /out
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/agenova-workerctl ./cmd/agenova-workerctl
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/agenova-demo-worker ./cmd/agenova-demo-worker

FROM ${RUNTIME_IMAGE}
RUN addgroup -S -g 65532 agenova && adduser -S -D -H -u 65532 -G agenova agenova
COPY --from=build --chown=65532:65532 /out/agenova-workerctl /agenova-workerctl
COPY --from=build --chown=65532:65532 /out/agenova-demo-worker /agenova-demo-worker
USER 65532:65532
ENTRYPOINT ["/agenova-workerctl", "serve"]
