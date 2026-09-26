module github.com/telark/x-ware

go 1.27.1

replace github.com/telark/rest => /Users/houssem/Desktop/Github/internal/rest

replace github.com/telark/data => /Users/houssem/Desktop/Github/internal/data

require (
	github.com/cenkalti/backoff/v5 v5.0.3
	github.com/nats-io/nats.go v1.54.0
	github.com/redis/go-redis/v9 v9.22.0
	github.com/rs/cors v1.11.1
	github.com/telark/data v1.14.8
	github.com/telark/rest v0.14.2
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/klauspost/compress v1.20.0 // indirect
	github.com/nats-io/nkeys v0.4.16 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
)
