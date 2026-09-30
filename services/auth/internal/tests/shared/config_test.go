package shared

import (
	"testing"

	"github.com/telark/telark/services/auth/internal/helpers/shared"
)

// The cached config is built from the WebAuthn relying-party environment and
// memoised for later reads.
func TestGetCachedConfig(t *testing.T) {
	t.Setenv("RP_ID", "localhost")
	t.Setenv("RP_NAME", "Test")
	t.Setenv("RP_ORIGIN", "http://localhost:3000")

	cfg, err := shared.GetCachedConfig()
	if err != nil || cfg == nil {
		t.Fatalf("GetCachedConfig = (%v, %v)", cfg, err)
	}
}
