# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/agenova-control-plane ./cmd/agenova-control-plane

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/agenova-control-plane /agenova-control-plane
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/agenova-control-plane"]
