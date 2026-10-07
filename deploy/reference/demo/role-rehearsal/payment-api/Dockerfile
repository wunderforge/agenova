FROM golang:1.22-alpine AS build
WORKDIR /src
COPY server.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o /payment-api server.go

FROM alpine:3.20
COPY --from=build /payment-api /payment-api
EXPOSE 8080
ENTRYPOINT ["/payment-api"]
