package testutil

import (
	"encoding/json"
	"testing"

	natssrvtest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	natscore "github.com/telark/telark/internal/x-ware/nats/core"
)

// randomPort asks the embedded NATS server to bind an ephemeral port.
const randomPort = -1

// No streams: the manager's Start creates its own, direct Subscribe tests create them first.
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

func RawMsg(subject string, data []byte) *nats.Msg {
	return &nats.Msg{Subject: subject, Data: data, Header: nats.Header{}}
}

func HeaderMsg(subject, parsedRaw string) *nats.Msg {
	m := &nats.Msg{Subject: subject, Header: nats.Header{}}
	natscore.SetParsedMessageHeader(m, "", parsedRaw)
	return m
}

func Data(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

type Resp struct {
	Status  int
	Message string
}

func (r Resp) GetStatus() int     { return r.Status }
func (r Resp) GetMessage() string { return r.Message }

func Equal[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
