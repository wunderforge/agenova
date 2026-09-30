# Copyright 2026 Agenova contributors.
# SPDX-License-Identifier: Apache-2.0

# Build with this directory as the context. The module is independent of the
# Agenova root module; its dataset is mounted at runtime, not baked in.
FROM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux GOTOOLCHAIN=local go build -trimpath -ldflags="-s -w" -o /out/mcpfixture .

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/mcpfixture /mcpfixture
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/mcpfixture"]
