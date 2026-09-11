# Stage 1: Build
FROM golang:1.26.5-alpine3.24 AS build

WORKDIR /app

RUN apk add --no-cache ca-certificates

# Add Go modules and download dependencies
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main . && \
    go clean -modcache && \
    adduser -D -u 10001 notifier && \
    rm -rf /root/.cache

# Stage 2: Final Minimal Image
FROM scratch

WORKDIR /app

COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /app/main .
USER 10001
ENTRYPOINT ["./main"]