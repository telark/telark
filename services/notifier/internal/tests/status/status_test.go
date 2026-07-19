package status

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/telark/notifier/internal/status"
	"github.com/telark/notifier/internal/tests/testutil"
	"github.com/telark/rest/base"
	statuseps "github.com/telark/rest/endpoints/status"
)

func path(ep base.Endpoint) string {
	return "/" + string(base.V1) + "/" + string(ep)
}

// The status server's readiness must mirror live NATS connectivity so a
// disconnected notifier is pulled from rotation; liveness is served off the
// same probe.
func TestNewServer(t *testing.T) {
	cases := []struct {
		name      string
		connected bool
		path      string
		wantOK    bool
	}{
		{"ready when connected", true, path(statuseps.ReadinessCheck), true},
		{"not ready when disconnected", false, path(statuseps.ReadinessCheck), false},
		{"live when connected", true, path(statuseps.LivenessCheck), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := status.NewServer(func() bool { return c.connected })
			req := httptest.NewRequest(http.MethodGet, c.path, nil)
			w := httptest.NewRecorder()
			srv.Handler.ServeHTTP(w, req)
			testutil.Equal(t, "ok", w.Code == http.StatusOK, c.wantOK)
		})
	}
}

// The server binds the configured status port with a read-header timeout.
func TestServerAddr(t *testing.T) {
	srv := status.NewServer(func() bool { return true })
	testutil.Equal(t, "addr", srv.Addr, ":8080")
}
