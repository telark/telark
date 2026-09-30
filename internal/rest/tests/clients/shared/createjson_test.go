package shared

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	dataconstants "github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/rest/base"
	"github.com/telark/telark/internal/rest/clients/shared"
	"github.com/telark/telark/internal/rest/endpoints/reports"
)

const (
	testServiceToken = "test-service-token"
	verbatimBody     = `{"planId":"p","n":1}`
)

// The token is read once per process, so it must be set before any test in
// this binary sends its first request.
func init() {
	if err := os.Setenv(dataconstants.EnvServiceToken, testServiceToken); err != nil {
		panic(err)
	}
}

func forwardTo(server *httptest.Server) roundTripFunc {
	return func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = server.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	}
}

func TestCreateJSONPostsBodyVerbatim(t *testing.T) {
	var gotBody, gotToken, gotContentType, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		gotBody = string(body)
		gotToken = r.Header.Get(dataconstants.HeaderServiceToken)
		gotContentType = r.Header.Get("Content-Type")
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(okBody)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	client := shared.New(base.Exporter)
	client.GetHTTPClient().Transport = forwardTo(server)

	resp := client.CreateJSON(reports.CreatePlanReport, json.RawMessage(verbatimBody))
	if resp == nil {
		t.Fatal("expected a response")
	}
	if gotMethod != string(base.Post) {
		t.Fatalf("expected method %q, got %q", base.Post, gotMethod)
	}
	if gotBody != verbatimBody {
		t.Fatalf("expected body %q, got %q", verbatimBody, gotBody)
	}
	if gotToken != testServiceToken {
		t.Fatalf("expected %s %q, got %q", dataconstants.HeaderServiceToken, testServiceToken, gotToken)
	}
	if gotContentType != string(base.JSON) {
		t.Fatalf("expected Content-Type %q, got %q", base.JSON, gotContentType)
	}
}
