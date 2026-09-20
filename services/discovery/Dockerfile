# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.27.0-alpine3.24 AS build

ARG TARGETARCH

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ENV GOEXPERIMENT=jsonv2 \
    GOMAXPROCS=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build,id=gobuild-$TARGETARCH \
    CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build \
        -ldflags="-w -s -extldflags '-static'" \
        -trimpath \
        -buildvcs=false \
        -o main .

RUN adduser -D -u 10001 discovery

FROM scratch
WORKDIR /app
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /app/main .
USER 10001
ENTRYPOINT ["./main"]