// Package testutil holds shared, generic helpers for the notifier test suites —
// message builders and lightweight fakes — so the tests stay DRY.
package testutil

import (
	"encoding/json"
	"testing"

	natssrvtest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	natscore "github.com/telark/x-ware/nats/core"
)

// randomPort asks the embedded NATS server to bind an ephemeral port.
const randomPort = -1

// NatsServer starts an embedded JetStream NATS server and returns a connected
// client (no streams — the caller creates them, so this suits both the direct
// Subscribe path and the manager's Start, which creates streams itself).
// Everything is torn down via t.Cleanup.
func NatsServer(t *testing.T) *natscore.NATSClient {
	t.Helper()
	opts := natssrvtest.DefaultTestOptions
	opts.Port = randomPort
	opts.JetStream = true
	opts.StoreDir = t.TempDir()
	srv := natssrvtest.RunServer(&opts)
	t.Cleanup(srv.Shutdown)

	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		t.Fatalf("nats connect: %v", err)
	}
	t.Cleanup(nc.Close)

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}
	return &natscore.NATSClient{Conn: nc, JetStream: js}
}

// Msg builds a bare NATS message carrying a parsed-message header — what the
// subscribers read via natscore.GetParsedMessageHeader.
func Msg(subject string, parsed *natscore.Message) *nats.Msg {
	m := &nats.Msg{Subject: subject, Header: nats.Header{}}
	if parsed != nil {
		b, err := json.Marshal(parsed)
		if err != nil {
			panic(err)
		}
		natscore.SetParsedMessageHeader(m, "", string(b))
	}
	return m
}

// RawMsg builds a bare NATS message with raw Data and no parsed header.
func RawMsg(subject string, data []byte) *nats.Msg {
	return &nats.Msg{Subject: subject, Data: data, Header: nats.Header{}}
}

// HeaderMsg builds a message whose parsed-message header is a raw string, used
// to exercise the malformed-JSON branches.
func HeaderMsg(subject, parsedRaw string) *nats.Msg {
	m := &nats.Msg{Subject: subject, Header: nats.Header{}}
	natscore.SetParsedMessageHeader(m, "", parsedRaw)
	return m
}

// Data marshals v to JSON for use as a raw NATS message payload, panicking on
// failure (test fixtures are always serialisable).
func Data(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// Resp is a generic response fake; it satisfies the subscribers' response
// interface structurally via GetStatus / GetMessage.
type Resp struct {
	Status  int
	Message string
}

func (r Resp) GetStatus() int     { return r.Status }
func (r Resp) GetMessage() string { return r.Message }

// Equal fails the test unless got == want.
func Equal[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
