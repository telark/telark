package nats

import (
	"testing"

	"github.com/plsyro/x-ware/nats/core"
)

func TestNewNatsManager(t *testing.T) {
	nm := core.NewNatsManager()
	if nm == nil {
		t.Fatal("NewNatsManager should not return nil")
	}

	// Test interface implementation
	var _ core.NatsManagerInterface = nm
}

func TestNatsManager_IsConnected_Initial(t *testing.T) {
	nm := core.NewNatsManager()

	// Initially should not be connected
	if nm.IsConnected() {
		t.Error("Nats manager should not be connected initially")
	}
}

func TestNatsManager_Close(t *testing.T) {
	nm := core.NewNatsManager()

	// Should not panic
	err := nm.Close()
	if err != nil {
		t.Errorf("Close should not return error: %v", err)
	}
}
