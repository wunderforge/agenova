# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0

# Build with the repository root as context. Pin both base images to approved
# digests in the private deployment repository before promotion.
ARG GO_IMAGE=golang:1.22-alpine
ARG RUNTIME_IMAGE=gcr.io/distroless/static-debian12:nonroot

FROM ${GO_IMAGE} AS build
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/agenova-mcp-kubernetes ./cmd/agenova-mcp-kubernetes
RUN mkdir -p /out && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/agenova-mcp-kubernetes ./cmd/agenova-mcp-kubernetes
RUN case "$TARGETARCH" in \
      amd64) expected=629d3f410e09bf49b64ae7079f7f0bda1191efed311f7d37fdbab0ad5b0ec2b7 ;; \
      arm64) expected=59f7ee8e477fae658447607dc3c8790ac17a1b016c01c622c12070e969e2d4e7 ;; \
      *) echo "unsupported kubectl architecture: $TARGETARCH" >&2; exit 1 ;; \
    esac && \
    wget -q "https://dl.k8s.io/release/v1.36.1/bin/linux/${TARGETARCH}/kubectl" -O /out/kubectl && \
    echo "${expected}  /out/kubectl" | sha256sum -c - && chmod +x /out/kubectl

FROM ${RUNTIME_IMAGE}
COPY --from=build --chown=65532:65532 /out/agenova-mcp-kubernetes /agenova-mcp-kubernetes
COPY --from=build --chown=65532:65532 /out/kubectl /usr/local/bin/kubectl
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/agenova-mcp-kubernetes"]
