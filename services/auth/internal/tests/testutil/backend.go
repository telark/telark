package testutil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// The rest clients dial the exporter by its in-cluster name over the default
// transport; rerouting that transport lets a handler stand in for it.
func StubBackend(t *testing.T, handler http.Handler) {
	t.Helper()
	srv := httptest.NewServer(handler)
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		req := r.Clone(r.Context())
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return prev.RoundTrip(req)
	})
	t.Cleanup(func() {
		http.DefaultTransport = prev
		srv.Close()
	})
}
