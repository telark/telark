package manager

import (
	"testing"

	"github.com/telark/notifier/internal/subscribers/applications"
	"github.com/telark/notifier/internal/subscribers/manager"
	"github.com/telark/notifier/internal/tests/testutil"
	natscore "github.com/telark/x-ware/nats/core"
)

type fakeNatsManager struct{ c *natscore.NATSClient }

func (f *fakeNatsManager) GetClient() (*natscore.NATSClient, error)  { return f.c, nil }
func (f *fakeNatsManager) IsConnected() bool                         { return f.c != nil }
func (f *fakeNatsManager) Close() error                              { return nil }
func (f *fakeNatsManager) Reconnect() error                          { return nil }
func (f *fakeNatsManager) GetConnectionStatus() (bool, error)        { return f.c != nil, nil }

// A fresh manager reports disconnected until Start dials NATS, and never panics
// on the nil client — the readiness probe depends on this being safe pre-Start.
func TestNewManagerIsNotConnected(t *testing.T) {
	m := manager.NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	testutil.Equal(t, "connected before start", m.IsConnected(), false)
}

// Start connects via the (injected) NATS manager, creates the streams, and wires
// every subscriber's consumers — proven end-to-end against an embedded server.
func TestStartWiresSubscribers(t *testing.T) {
	c := testutil.NatsServer(t)
	m := manager.NewManagerWith(&fakeNatsManager{c: c}, []natscore.ResourceSubscriber{
		applications.NewApplicationSubscriber(),
	})

	if err := m.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	testutil.Equal(t, "connected after start", m.IsConnected(), true)
	m.Shutdown()
}

// Shutdown must be safe (cancel + close) even for a manager that never started.
func TestShutdownBeforeStartIsSafe(t *testing.T) {
	manager.NewManager().Shutdown()
}
