package shared

import (
	"os"
	"testing"

	"github.com/telark/x-ware/shared"
)

func TestGetEnvString_WithValue(t *testing.T) {
	if err := os.Setenv("TEST_KEY", "test_value"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("TEST_KEY"); err != nil {
			t.Errorf("Failed to unset environment variable: %v", err)
		}
	}()

	value, err := shared.GetEnvString(shared.EnvConfig{
		Key:      "TEST_KEY",
		Required: true,
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if value != "test_value" {
		t.Errorf("Expected 'test_value', got %s", value)
	}
}

func TestGetEnvString_WithDefault(t *testing.T) {
	value, err := shared.GetEnvString(shared.EnvConfig{
		Key:          "NON_EXISTENT_KEY",
		DefaultValue: "default_value",
		Required:     false,
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if value != "default_value" {
		t.Errorf("Expected 'default_value', got %s", value)
	}
}

func TestGetEnvString_Required(t *testing.T) {
	_, err := shared.GetEnvString(shared.EnvConfig{
		Key:      "NON_EXISTENT_KEY",
		Required: true,
		ErrorMsg: "Key is required",
	})
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestGetEnvInt_WithValue(t *testing.T) {
	if err := os.Setenv("TEST_INT", "42"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("TEST_INT"); err != nil {
			t.Errorf("Failed to unset environment variable: %v", err)
		}
	}()

	value, err := shared.GetEnvInt(shared.EnvConfig{
		Key:      "TEST_INT",
		Required: true,
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if value != 42 {
		t.Errorf("Expected 42, got %d", value)
	}
}

func TestGetEnvInt_WithDefault(t *testing.T) {
	value, err := shared.GetEnvInt(shared.EnvConfig{
		Key:          "NON_EXISTENT_INT",
		DefaultValue: "10",
		Required:     false,
	})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if value != 10 {
		t.Errorf("Expected 10, got %d", value)
	}
}
