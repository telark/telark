package testutil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// The rest clients dial the exporter by its in-cluster name over the default transport; rerouting that transport
// lets a handler stand in for it. It answers in-process, with no server, so a synctest bubble's clock can advance.
func StubBackend(t *testing.T, handler http.Handler) {
	t.Helper()
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		req := r.Clone(r.Context())
		if req.Body == nil {
			req.Body = http.NoBody // a server's request always has a body
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w.Result(), nil
	})
	t.Cleanup(func() { http.DefaultTransport = prev })
}
