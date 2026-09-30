package nats

import (
	"context"
	"testing"

	natscore "github.com/telark/x-ware/nats/core"
	natsinit "github.com/telark/x-ware/nats/init"
)

func dialUnreachable() (*natscore.NATSClient, error) {
	return nil, context.DeadlineExceeded
}

func TestNatsNewClientWithRetry_NilContextOrDial(t *testing.T) {
	natsinit.SetClient(nil)
	t.Cleanup(func() { natsinit.SetClient(nil) })

	// A nil context is the guard under test; context.TODO() would dial and block.
	var nilCtx context.Context
	if c := natsinit.NewClientWithRetry(nilCtx, dialUnreachable, natsinit.RetryConfig{}, nil); c != nil {
		t.Errorf("expected nil for a nil context, got %v", c)
	}
	if c := natsinit.NewClientWithRetry(t.Context(), nil, natsinit.RetryConfig{}, nil); c != nil {
		t.Errorf("expected nil for a nil dial, got %v", c)
	}
	if natsinit.Client() != nil {
		t.Error("the cached client must stay nil when the guards trip")
	}
}

func TestNatsNewClientWithRetry_GivingUpKeepsCachedClient(t *testing.T) {
	stale := &natscore.NATSClient{}
	natsinit.SetClient(stale)
	t.Cleanup(func() { natsinit.SetClient(nil) })

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if c := natsinit.NewClientWithRetry(ctx, dialUnreachable, natsinit.RetryConfig{}, nil); c != nil {
		t.Errorf("expected nil when giving up, got %v", c)
	}
	if natsinit.Client() != stale {
		t.Error("giving up must leave the cached client untouched")
	}
}
