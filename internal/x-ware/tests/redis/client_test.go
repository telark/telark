package redis

import (
	"bufio"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/telark/x-ware/redis/core"
)

const (
	lifetimeMinutes    = 5
	reconnectRounds    = 20
	respArrayPrefix    = "*"
	respPingCommand    = "PING"
	respPong           = "+PONG\r\n"
	respUnknownError   = "-ERR unknown command\r\n"
	stubListenAddr     = "127.0.0.1:0"
	linesPerBulkString = 2
	commandNameLine    = 1
)

func TestInitClient_ConnMaxLifetimeInMinutes(t *testing.T) {
	t.Setenv(core.EnvRedisConnMaxLifetime, strconv.Itoa(lifetimeMinutes))

	client, err := core.InitClient()
	if err != nil {
		t.Fatalf("InitClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Client.Close() })

	want := lifetimeMinutes * time.Minute
	if got := client.Client.Options().ConnMaxLifetime; got != want {
		t.Errorf("ConnMaxLifetime = %v, want %v (same unit as ConnMaxIdleTime)", got, want)
	}
}

func TestRedisManager_ReconnectRacesHealthReads(t *testing.T) {
	host, port := startPingStub(t)
	t.Setenv(core.EnvRedisHost, host)
	t.Setenv(core.EnvRedisPort, port)

	rm := core.NewRedisManager()
	t.Cleanup(func() { _ = rm.Close() })

	var wg sync.WaitGroup
	wg.Go(func() {
		for range reconnectRounds {
			if err := rm.Reconnect(); err != nil {
				t.Errorf("Reconnect: %v", err)
				return
			}
		}
	})
	for range reconnectRounds {
		wg.Go(func() {
			_ = rm.IsConnected()
			_, _ = rm.GetConnectionStatus()
			_, _ = rm.GetPoolStats()
			_ = rm.HealthCheck(t.Context())
			_, _ = rm.GetConnectionInfo()
		})
	}
	wg.Wait()
}

// Answers PING with PONG and every other command (HELLO, CLIENT ...) with an error go-redis tolerates.
func startPingStub(t *testing.T) (host, port string) {
	t.Helper()
	ln, err := net.Listen("tcp", stubListenAddr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go serveStub(conn)
		}
	}()

	host, port, err = net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	return host, port
}

func serveStub(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	r := bufio.NewReader(conn)
	for {
		name, err := readCommand(r)
		if err != nil {
			return
		}
		reply := respUnknownError
		if strings.EqualFold(name, respPingCommand) {
			reply = respPong
		}
		if _, err := conn.Write([]byte(reply)); err != nil {
			return
		}
	}
}

// Returns the command name and consumes the rest of the RESP array.
func readCommand(r *bufio.Reader) (string, error) {
	header, err := r.ReadString('\n')
	if err != nil {
		return core.DefaultEmptyString, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, respArrayPrefix)))
	if err != nil {
		return core.DefaultEmptyString, err
	}
	var name string
	for i := range n * linesPerBulkString {
		line, err := r.ReadString('\n')
		if err != nil {
			return core.DefaultEmptyString, err
		}
		if i == commandNameLine {
			name = strings.TrimSpace(line)
		}
	}
	return name, nil
}
