package shared

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/telark/rest/clients/auth/session"
	"github.com/telark/rest/clients/notifications"
	"github.com/telark/rest/constants"
	authendpoints "github.com/telark/rest/endpoints/auth"
	notifendpoints "github.com/telark/rest/endpoints/notifications"
)

const (
	secretToken     = "sess-tok-9f3c1d7b-do-not-log-me"
	secretUserID    = "user-7c21e0aa-do-not-log-me"
	transportFail   = "simulated transport failure"
	dedupDisabled   = "0"
	emptyBody       = ""
	logMissingIdent = "log line lost the endpoint identity, diagnostics are useless: %q"
	logLeakedValue  = "%s leaked %q: %s"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMain(m *testing.M) {
	if err := os.Setenv(constants.EnvExporterDurationLogDedupSec, dedupDisabled); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func notFound(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader(emptyBody)),
		Header:     http.Header{},
	}, nil
}

func transportError(*http.Request) (*http.Response, error) {
	return nil, errors.New(transportFail)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	original := os.Stdout
	os.Stdout = writer
	fn()
	os.Stdout = original
	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close pipe writer: %v", err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read captured output: %v", err)
	}
	return string(out)
}

func assertNoLeak(t *testing.T, subject, secret, actual string) {
	t.Helper()
	if strings.Contains(actual, secret) {
		t.Errorf(logLeakedValue, subject, secret, actual)
	}
}

func assertIdentity(t *testing.T, identity, actual string) {
	t.Helper()
	if !strings.Contains(actual, identity) {
		t.Errorf(logMissingIdent, actual)
	}
}

func TestSessionLookupFailureKeepsTokenOutOfLogs(t *testing.T) {
	tests := []struct {
		name      string
		transport roundTripFunc
	}{
		{name: "non-2xx response", transport: notFound},
		{name: "transport error", transport: transportError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := session.NewClient()
			client.GetHTTPClient().Transport = tt.transport

			var callErr error
			logged := captureStdout(t, func() {
				_, callErr = client.GetSessionByToken(secretToken)
			})

			if callErr == nil {
				t.Fatal("expected a failure from the session lookup")
			}
			assertNoLeak(t, "log", secretToken, logged)
			assertNoLeak(t, "returned error", secretToken, callErr.Error())
			assertIdentity(t, string(authendpoints.GetSessionByToken), logged)
		})
	}
}

func TestSessionDeleteFailureKeepsTokenOutOfLogs(t *testing.T) {
	client := session.NewClient()
	client.GetHTTPClient().Transport = roundTripFunc(transportError)

	var resp string
	logged := captureStdout(t, func() {
		resp = client.DeleteSessionByToken(secretToken).Message
	})

	assertNoLeak(t, "log", secretToken, logged)
	assertNoLeak(t, "response message", secretToken, resp)
	assertIdentity(t, string(authendpoints.DeleteSessionByToken), logged)
}

func TestRequestURLStillCarriesTheResolvedToken(t *testing.T) {
	var requestedPath string
	client := session.NewClient()
	client.GetHTTPClient().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requestedPath = r.URL.Path
		return notFound(r)
	})

	if _, err := client.GetSessionByToken(secretToken); err == nil {
		t.Fatal("expected a failure from the session lookup")
	}
	if !strings.Contains(requestedPath, secretToken) {
		t.Errorf("the token must still reach the server in the path, got %q", requestedPath)
	}
}

func TestQueryParameterValuesStayOutOfLogs(t *testing.T) {
	client := notifications.NewClient()
	client.GetHTTPClient().Transport = roundTripFunc(transportError)

	var callErr error
	logged := captureStdout(t, func() {
		_, callErr = client.List(context.Background(), secretUserID, notifications.ListOptions{})
	})

	if callErr == nil {
		t.Fatal("expected a failure from the notification list call")
	}
	assertNoLeak(t, "log", secretUserID, logged)
	assertNoLeak(t, "returned error", secretUserID, callErr.Error())
	assertIdentity(t, string(notifendpoints.List), logged)
}
