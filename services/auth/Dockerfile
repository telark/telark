# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.27.1-alpine3.24 AS build

ARG TARGETARCH

WORKDIR /app

RUN apk add --no-cache ca-certificates

# No `go mod download`: go.mod lists every service's dependencies; go build fetches only this one's.
COPY go.mod go.sum ./
COPY internal internal
COPY services/auth services/auth

ENV GOEXPERIMENT=jsonv2 \
    GOMAXPROCS=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build,id=gobuild-$TARGETARCH \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build \
        -ldflags="-w -s -extldflags '-static'" \
        -trimpath \
        -buildvcs=false \
        -o main ./services/auth

RUN adduser -D -u 10001 auth-service

FROM alpine:3.24.2
WORKDIR /app
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /app/main .
USER 10001
ENTRYPOINT ["./main"]