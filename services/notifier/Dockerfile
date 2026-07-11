# Stage 1: Build
FROM golang:1.26.5-alpine3.24 AS build

WORKDIR /app

# Configure Go Private Modules
ARG ORG
ARG URL
ARG USER
ARG PAT
RUN apk add --no-cache ca-certificates && \
    apk add --no-cache --virtual .build-deps git && \
    go env -w GOPRIVATE=${URL}/${ORG}/* && \
    git config --global url."https://${USER}:${PAT}@${URL}".insteadOf "https://${URL}"

# Add Go modules and download dependencies
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main . && \
    go clean -modcache && \
    adduser -D -u 10001 notifier && \
    apk del .build-deps && \
    rm -rf /root/.gitconfig && \
    rm -rf /root/.cache 

# Stage 2: Final Minimal Image
FROM scratch

WORKDIR /app

COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /app/main .
USER 10001
ENTRYPOINT ["./main"]