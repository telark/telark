package authz

import (
	"testing"

	"github.com/telark/telark/services/auth/internal/authz"
)

// NewResolver wires the client-backed record source into the shared basic
// resolver; it must always hand back a usable resolver.
func TestNewResolver(t *testing.T) {
	if authz.NewResolver() == nil {
		t.Fatal("NewResolver returned nil")
	}
}
