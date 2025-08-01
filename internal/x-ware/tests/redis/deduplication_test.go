package redis

import (
	"testing"

	"github.com/plsyro/x-ware/redis/core"
)

type MockEvent struct {
	eventType string
}

func (m MockEvent) GetEventType() string {
	return m.eventType
}

func TestCreateDeduplicationKey(t *testing.T) {
	event := MockEvent{eventType: "test"}
	key := core.CreateDeduplicationKey("test-key", event)
	expected := "dedup:test-key:test"
	if key != expected {
		t.Errorf("Expected %s, got %s", expected, key)
	}
}

func TestGetEventType(t *testing.T) {
	event := MockEvent{eventType: "test"}
	eventType := core.GetEventType(event)
	if eventType != "test" {
		t.Errorf("Expected 'test', got %s", eventType)
	}
}

func TestGetEventType_Unknown(t *testing.T) {
	event := "not-an-event"
	eventType := core.GetEventType(event)
	if eventType != core.UnknownEventType {
		t.Errorf("Expected '%s', got %s", core.UnknownEventType, eventType)
	}
}
