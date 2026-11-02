package nats

import (
	"testing"

	natssrvtest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	natscore "github.com/telark/telark/internal/x-ware/nats/core"
	"github.com/telark/telark/internal/x-ware/nats/streams"
)

const (
	randomPort    = -1
	foreignStream = "foreign"
	otherSubject  = "other.subject"
)

func natsClient(t *testing.T) *natscore.NATSClient {
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

func TestCreateStreams(t *testing.T) {
	cases := []struct {
		name     string
		existing *nats.StreamConfig
		wantErr  bool
	}{
		{name: "fresh server"},
		{
			name:     "same name with another config is kept",
			existing: &nats.StreamConfig{Name: natscore.GetStreamName(natscore.Grouper), Subjects: []string{otherSubject}},
		},
		{
			name:     "subject owned by another stream fails",
			existing: &nats.StreamConfig{Name: foreignStream, Subjects: []string{natscore.GenerateSubjectName(natscore.Grouper)}},
			wantErr:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := natsClient(t)
			if tc.existing != nil {
				if _, err := c.JetStream.AddStream(tc.existing); err != nil {
					t.Fatal(err)
				}
			}
			if err := streams.CreateStreams(c); (err != nil) != tc.wantErr {
				t.Fatalf("CreateStreams err = %v, wantErr %v", err, tc.wantErr)
			}
			if err := streams.CreateStreams(c); (err != nil) != tc.wantErr {
				t.Errorf("second CreateStreams err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
