// Package testutil holds shared, generic helpers for the discovery test suites.
package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	tcfghelper "github.com/telark/telark/services/discovery/internal/helpers/telarkconfig"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Equal fails the test unless got == want.
func Equal[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

// CaptureStdout returns what fn printed: the rest response helpers log through a per-call logger on stdout.
func CaptureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	fn()
	os.Stdout = original
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// RedisEnv starts an embedded Redis and points REDIS_HOST/REDIS_PORT at it, so
// code that dials Redis through x-ware (env-resolved) hits the in-memory server.
func RedisEnv(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr := miniredis.RunT(t)
	t.Setenv("REDIS_HOST", mr.Host())
	t.Setenv("REDIS_PORT", mr.Port())
	return mr
}

// ExcludedNamespaces loads the list the way discovery does in a cluster: from the TelarkConfig the
// exporter serves. The first list loaded stays for the whole process, so a package shares one.
func ExcludedNamespaces(tb testing.TB, excluded ...string) {
	tb.Helper()
	body, err := json.Marshal(map[string]any{
		"status": http.StatusOK,
		"data":   telarkconfig.TelarkConfig{ExcludedNamespaces: excluded},
	})
	if err != nil {
		tb.Fatal(err)
	}
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}, nil
	})
	defer func() { http.DefaultTransport = prev }()
	if _, err := tcfghelper.ExcludedNamespaces(context.Background()); err != nil {
		tb.Fatal(err)
	}
}
