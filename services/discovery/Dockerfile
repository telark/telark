# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.27.1-alpine3.24 AS build

ARG TARGETARCH

WORKDIR /app

RUN apk add --no-cache ca-certificates

# No `go mod download`: go.mod lists every service's dependencies; go build fetches only this one's.
COPY go.mod go.sum ./
COPY internal internal
COPY services/discovery services/discovery
COPY scripts/third-party-notices.sh scripts/

ENV GOEXPERIMENT=jsonv2 \
    GOMAXPROCS=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build,id=gobuild-$TARGETARCH \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build \
        -ldflags="-w -s -extldflags '-static'" \
        -trimpath \
        -buildvcs=false \
        -o main ./services/discovery && \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH sh scripts/third-party-notices.sh ./services/discovery > go-modules.txt

RUN adduser -D -u 10001 discovery

FROM scratch
LABEL org.opencontainers.image.licenses="Elastic-2.0"
WORKDIR /app
COPY --from=build /etc/passwd /etc/passwd
COPY LICENSE.md THIRD-PARTY-NOTICES.md /licenses/
COPY --from=build /app/go-modules.txt /licenses/
COPY --from=build /app/main .
USER 10001
ENTRYPOINT ["./main"]