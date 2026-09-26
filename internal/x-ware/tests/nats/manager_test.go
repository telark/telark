package nats

import (
	"testing"

	"github.com/telark/x-ware/nats/core"
)

func TestNewNatsManager(t *testing.T) {
	nm := core.NewNatsManager()
	if nm == nil {
		t.Fatal("NewNatsManager should not return nil")
	}
}

func TestNatsManager_IsConnected_Initial(t *testing.T) {
	nm := core.NewNatsManager()

	if nm.IsConnected() {
		t.Error("Nats manager should not be connected initially")
	}
}

func TestNatsManager_Close(t *testing.T) {
	nm := core.NewNatsManager()

	err := nm.Close()
	if err != nil {
		t.Errorf("Close should not return error: %v", err)
	}
}
