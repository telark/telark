# syntax=docker/dockerfile:1.7

# Stage 1: Build
FROM --platform=$BUILDPLATFORM golang:1.27.1-alpine3.24 AS build

ARG TARGETARCH

WORKDIR /app

RUN apk add --no-cache ca-certificates

# No `go mod download`: go.mod lists every service's dependencies; go build fetches only this one's.
COPY go.mod go.sum ./
COPY internal internal
COPY services/notifier services/notifier
COPY scripts/third-party-notices.sh scripts/

# Build the application
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build,id=gobuild-$TARGETARCH \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -ldflags="-w -s" -o main ./services/notifier && \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH sh scripts/third-party-notices.sh ./services/notifier > go-modules.txt && \
    adduser -D -u 10001 notifier

# Stage 2: Final Minimal Image
FROM scratch
LABEL org.opencontainers.image.licenses="Elastic-2.0"

WORKDIR /app

COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY LICENSE.md THIRD-PARTY-NOTICES.md /licenses/
COPY --from=build /app/go-modules.txt /licenses/
COPY --from=build /app/main .
USER 10001
ENTRYPOINT ["./main"]
