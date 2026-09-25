package shared

import (
	"os"
	"strconv"
	"testing"

	"github.com/telark/x-ware/shared"
)

const (
	envKeyString   = "TEST_KEY"
	envKeyInt      = "TEST_INT"
	msgNoError     = "Expected no error, got %v"
	wantEnvInt     = 42
	wantDefaultInt = 10
)

func TestGetEnvString_WithValue(t *testing.T) {
	if err := os.Setenv(envKeyString, "test_value"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv(envKeyString); err != nil {
			t.Errorf("Failed to unset environment variable: %v", err)
		}
	}()

	value, err := shared.GetEnvString(shared.EnvConfig{
		Key:      envKeyString,
		Required: true,
	})
	if err != nil {
		t.Errorf(msgNoError, err)
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
		t.Errorf(msgNoError, err)
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
	if err := os.Setenv(envKeyInt, strconv.Itoa(wantEnvInt)); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv(envKeyInt); err != nil {
			t.Errorf("Failed to unset environment variable: %v", err)
		}
	}()

	value, err := shared.GetEnvInt(shared.EnvConfig{
		Key:      envKeyInt,
		Required: true,
	})
	if err != nil {
		t.Errorf(msgNoError, err)
	}
	if value != wantEnvInt {
		t.Errorf("Expected %d, got %d", wantEnvInt, value)
	}
}

func TestGetEnvInt_WithDefault(t *testing.T) {
	value, err := shared.GetEnvInt(shared.EnvConfig{
		Key:          "NON_EXISTENT_INT",
		DefaultValue: strconv.Itoa(wantDefaultInt),
		Required:     false,
	})
	if err != nil {
		t.Errorf(msgNoError, err)
	}
	if value != wantDefaultInt {
		t.Errorf("Expected %d, got %d", wantDefaultInt, value)
	}
}
