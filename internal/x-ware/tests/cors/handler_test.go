package cors

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/telark/x-ware/cors"
)

const (
	uiOrigin      = "https://ui.example.com"
	otherOrigin   = "https://evil.example.com"
	allowOrigin   = "Access-Control-Allow-Origin"
	allowCreds    = "Access-Control-Allow-Credentials"
	allowHeaders  = "Access-Control-Allow-Headers"
	requestMethod = "Access-Control-Request-Method"
	requestHeader = "Access-Control-Request-Headers"
	credsTrue     = "true"
	noHeader      = ""
)

func serve(origin, method string, headers ...string) *httptest.ResponseRecorder {
	handler := cors.NewCORS()(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(method, "/api/v1/status/live", http.NoBody)
	req.Header.Set("Origin", origin)
	if method == http.MethodOptions {
		req.Header.Set(requestMethod, http.MethodGet)
		for _, h := range headers {
			req.Header.Add(requestHeader, h)
		}
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestAllowedOriginsFromEnv(t *testing.T) {
	cases := []struct {
		name string
		env  string
		want []string
	}{
		{"unset", noHeader, nil},
		{"list with blanks", " https://a.example.com , ,https://b.example.com", []string{"https://a.example.com", "https://b.example.com"}},
		{"wildcard dropped", "*," + uiOrigin, []string{uiOrigin}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(cors.EnvAllowedOrigins, tc.env)
			if got := cors.AllowedOrigins(); !slices.Equal(got, tc.want) {
				t.Fatalf("AllowedOrigins() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNoOriginsMeansNoCORSHeaders(t *testing.T) {
	t.Setenv(cors.EnvAllowedOrigins, noHeader)
	rec := serve(uiOrigin, http.MethodGet)
	if got := rec.Header().Get(allowOrigin); got != noHeader {
		t.Fatalf("%s = %q, want none", allowOrigin, got)
	}
	if got := rec.Header().Get(allowCreds); got != noHeader {
		t.Fatalf("%s = %q, want none", allowCreds, got)
	}
}

func TestConfiguredOriginGetsCredentialsOthersDoNot(t *testing.T) {
	t.Setenv(cors.EnvAllowedOrigins, uiOrigin)

	rec := serve(uiOrigin, http.MethodGet)
	if rec.Header().Get(allowOrigin) != uiOrigin || rec.Header().Get(allowCreds) != credsTrue {
		t.Fatalf("configured origin: headers = %v", rec.Header())
	}
	if got := serve(otherOrigin, http.MethodGet).Header().Get(allowOrigin); got != noHeader {
		t.Fatalf("unlisted origin allowed: %q", got)
	}
}

func TestPreflightRejectsDroppedHeaders(t *testing.T) {
	t.Setenv(cors.EnvAllowedOrigins, uiOrigin)
	for _, header := range []string{"authorization", "x-user-id"} {
		rec := serve(uiOrigin, http.MethodOptions, header)
		if rec.Header().Get(allowOrigin) != noHeader || rec.Header().Get(allowHeaders) != noHeader {
			t.Fatalf("preflight for %s was allowed: %v", header, rec.Header())
		}
	}
	rec := serve(uiOrigin, http.MethodOptions, "x-session-token")
	if rec.Header().Get(allowOrigin) != uiOrigin {
		t.Fatalf("preflight for X-Session-Token refused: %v", rec.Header())
	}
}
