package performance

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/utils/performance"
)

func TestGetTimeoutForResource(t *testing.T) {
	// A resource+operation configured explicitly returns its specific timeout.
	if got := performance.GetTimeoutForResource(constants.ResourceApplication, constants.OpGet); got <= 0 {
		t.Errorf("configured timeout = %v, want > 0", got)
	}
	// An unconfigured resource falls back to the per-operation default.
	if got := performance.GetTimeoutForResource("unconfigured", constants.OpCreate); got <= 0 {
		t.Errorf("fallback timeout = %v, want > 0", got)
	}
	// An unknown operation lands on the global default.
	if got := performance.GetTimeoutForResource("unconfigured", "unknown-op"); got <= 0 {
		t.Errorf("default timeout = %v, want > 0", got)
	}
}

func TestKeyFunc(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/resources?limit=10", nil)
	if got := performance.KeyFunc(r); got != "cache:/resources?limit=10" {
		t.Errorf("KeyFunc = %q", got)
	}
}
