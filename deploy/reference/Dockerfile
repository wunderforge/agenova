# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY api ./api
COPY internal ./internal
COPY cmd/agenova-control-plane ./cmd/agenova-control-plane
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/agenova-control-plane ./cmd/agenova-control-plane
RUN wget -q https://dl.k8s.io/release/v1.36.1/bin/linux/amd64/kubectl -O /out/kubectl && echo '629d3f410e09bf49b64ae7079f7f0bda1191efed311f7d37fdbab0ad5b0ec2b7  /out/kubectl' | sha256sum -c - && chmod +x /out/kubectl

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/agenova-control-plane /agenova-control-plane
COPY --from=build /out/kubectl /usr/local/bin/kubectl
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/agenova-control-plane"]
