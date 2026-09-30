package routes

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	authdata "github.com/telark/telark/internal/data/auth"
	dataconstants "github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/rest/clients/auth/passkey"
	"github.com/telark/telark/internal/rest/clients/auth/session"
	"github.com/telark/telark/internal/rest/clients/categories"
	"github.com/telark/telark/internal/rest/clients/groups"
	"github.com/telark/telark/internal/rest/clients/notifications"
)

const (
	none             = ""
	userID           = "user-1"
	credentialID     = "cred/+a=="
	escapedCred      = "cred%2F+a=="
	credentialHeader = "X-Credential-ID"
	scope            = "plan environments&x=1"
	groupID          = "g-1"
	finalizerName    = "telark.io/group-cleanup"
	notificationID   = "n-1"
	secretToken      = "raw-token"
	okBody           = `{"status":200,"data":{"items":[]}}`
)

type request struct {
	method, path, query, credential string
	body                            string
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func capture(got *request) roundTripFunc {
	return func(r *http.Request) (*http.Response, error) {
		*got = request{
			method: r.Method, path: r.URL.EscapedPath(), query: r.URL.RawQuery,
			credential: r.Header.Get(credentialHeader),
		}
		if r.Body != nil {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return nil, err
			}
			got.body = string(body)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(okBody)), Header: http.Header{}}, nil
	}
}

type routeCase struct {
	name       string
	invoke     func(rt http.RoundTripper)
	method     string
	path       string
	query      string
	bodyNeedle string
}

func routeCases() []routeCase {
	return []routeCase{
		{"passkey by credential path", func(rt http.RoundTripper) {
			c := passkey.NewClient()
			c.GetHTTPClient().Transport = rt
			_, _ = c.GetPasskeyByUserAndCredentialID(userID, credentialID)
		}, http.MethodGet, "/api/v1/internal/auth/passkeys/" + escapedCred, none, none},
		{"passkey delete by credential path", func(rt http.RoundTripper) {
			c := passkey.NewClient()
			c.GetHTTPClient().Transport = rt
			c.DeletePasskeyByUserAndCredentialID(userID, credentialID, true)
		}, http.MethodDelete, "/api/v1/internal/auth/passkeys/" + escapedCred, none, "forceLastDelete"},
		{"sessions listed by user query", func(rt http.RoundTripper) {
			c := session.NewClient()
			c.GetHTTPClient().Transport = rt
			_, _ = c.ListSessionRefsByUser(userID)
		}, http.MethodGet, "/api/v1/auth/sessions", "user=" + userID, none},
		{"self session delete", func(rt http.RoundTripper) {
			c := session.NewClient()
			c.GetHTTPClient().Transport = rt
			c.DeleteSessionByToken(secretToken)
		}, http.MethodDelete, "/api/v1/auth/sessions/self", none, none},
		{"categories by scope query", func(rt http.RoundTripper) {
			c := categories.NewClient()
			c.GetHTTPClient().Transport = rt
			_, _ = c.GetCategoriesByScope(scope)
		}, http.MethodGet, "/api/v1/categories", "scope=plan+environments%26x%3D1", none},
		{"finalizer add is a put", func(rt http.RoundTripper) {
			c := groups.NewClient()
			c.GetHTTPClient().Transport = rt
			c.AddFinalizer(groupID, finalizerName)
		}, http.MethodPut, "/api/v1/cleanup/groups/" + groupID + "/finalizer", none, finalizerName},
		{"finalizer remove is a delete", func(rt http.RoundTripper) {
			c := groups.NewClient()
			c.GetHTTPClient().Transport = rt
			c.RemoveFinalizer(groupID, finalizerName)
		}, http.MethodDelete, "/api/v1/cleanup/groups/" + groupID + "/finalizer", none, finalizerName},
		{"notification read is a post", func(rt http.RoundTripper) {
			c := notifications.NewClient()
			c.GetHTTPClient().Transport = rt
			c.MarkRead(context.Background(), userID, notificationID)
		}, http.MethodPost, "/api/v1/notifications/" + notificationID + "/read", "userId=" + userID, none},
	}
}

func TestClientsAddressResources(t *testing.T) {
	for _, tc := range routeCases() {
		t.Run(tc.name, func(t *testing.T) {
			var got request
			tc.invoke(capture(&got))
			if got.method != tc.method || got.path != tc.path || got.query != tc.query {
				t.Fatalf("got %s %s?%s, want %s %s?%s", got.method, got.path, got.query, tc.method, tc.path, tc.query)
			}
			if got.credential != none {
				t.Fatalf("%s header must not be sent, got %q", credentialHeader, got.credential)
			}
			if !strings.Contains(got.body, tc.bodyNeedle) {
				t.Fatalf("body %q does not contain %q", got.body, tc.bodyNeedle)
			}
		})
	}
}

func TestSelfSessionHeaderNeverCarriesTheRawToken(t *testing.T) {
	var sent string
	c := session.NewClient()
	c.GetHTTPClient().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		sent = r.Header.Get(dataconstants.HeaderSessionToken)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(okBody)), Header: http.Header{}}, nil
	})
	c.PatchSessionByToken(secretToken, map[string]any{})
	if sent != authdata.SessionName(secretToken) {
		t.Fatalf("header = %q, want the session name", sent)
	}
}
