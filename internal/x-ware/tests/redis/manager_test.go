package redis

import (
	"testing"

	"github.com/telark/x-ware/redis/core"
)

func TestNewRedisManager(t *testing.T) {
	rm := core.NewRedisManager()
	if rm == nil {
		t.Fatal("NewRedisManager returned nil")
	}

	_ = rm
}

func TestRedisManager_IsConnected_Initial(t *testing.T) {
	rm := core.NewRedisManager()
	if rm.IsConnected() {
		t.Error("Expected IsConnected to return false initially")
	}
}

func TestRedisManager_Close(t *testing.T) {
	rm := core.NewRedisManager()
	err := rm.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}
