# syntax=docker/dockerfile:1.7

FROM golang:1.26.5-alpine3.24 AS build

WORKDIR /app

ARG ORG
ARG URL
ARG USER
ARG PAT
RUN apk add --no-cache --virtual .build-deps ca-certificates git && \
    go env -w GOPRIVATE=${URL}/${ORG}/* && \
    git config --global url."https://${USER}:${PAT}@${URL}".insteadOf "https://${URL}"

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

ENV GOEXPERIMENT=jsonv2 \
    GOMAXPROCS=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
        -ldflags="-w -s -extldflags '-static'" \
        -trimpath \
        -buildvcs=false \
        -o main .

RUN adduser -D -u 10001 exporter && \
    rm -rf /root/.gitconfig

FROM scratch
WORKDIR /app
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /app/main .
USER 10001
ENTRYPOINT ["./main"]