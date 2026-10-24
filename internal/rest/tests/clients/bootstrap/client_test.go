package bootstrap

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	dataconstants "github.com/telark/telark/internal/data/constants"
	bootstrapclient "github.com/telark/telark/internal/rest/clients/bootstrap"
	"github.com/telark/telark/internal/rest/constants"
	eps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/internal/rest/response"
)

const (
	testServiceToken = "test-service-token"
	requestID        = "br-1"
	email            = "test@example.com"
	reason           = "not me"
	keepCredential   = "cred-1"
	refusal          = "request is not open"
	statusPath       = "/api/v1/bootstrap"
	openPath         = "/api/v1/bootstrap/requests"
	approvePath      = "/api/v1/bootstrap/requests/br-1/approve"
	cancelPath       = "/api/v1/bootstrap/requests/br-1/cancel"
	completePath     = "/api/v1/bootstrap/requests/br-1/complete"
	okBody           = `{"status":200,"operation":"Success"}`
	statusBody       = `{"status":200,"data":{"email":"test@example.com","requests":[{"id":"br-1","kind":"recovery","state":"open"}]}}`
	openBody         = `{"email":"test@example.com"}`
	cancelBody       = `{"reason":"not me"}`
	completeBody     = `{"revoke":true,"keep":"cred-1"}`
)

// The token is read once per process, so it must be set before any test in
// this binary sends its first request.
func init() {
	if err := os.Setenv(dataconstants.EnvServiceToken, testServiceToken); err != nil {
		panic(err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type captured struct {
	method, path, token, cacheControl, body string
}

func serve(t *testing.T, reply string) (*bootstrapclient.Client, *captured) {
	t.Helper()
	got := &captured{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		*got = captured{
			method: r.Method, path: r.URL.Path, token: r.Header.Get(dataconstants.HeaderServiceToken),
			cacheControl: r.Header.Get(constants.HeaderCacheControl), body: string(body),
		}
		if _, err := w.Write([]byte(reply)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := bootstrapclient.NewClient()
	client.GetHTTPClient().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = server.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(r)
	})
	return client, got
}

func TestStatusReadsThePinUncached(t *testing.T) {
	client, got := serve(t, statusBody)

	status, err := client.Status()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if got.method != http.MethodGet || got.path != statusPath || got.cacheControl != constants.CacheControlNoCache {
		t.Fatalf("expected GET %s with no-cache, got %s %s cache=%q", statusPath, got.method, got.path, got.cacheControl)
	}
	if status.Email != email || len(status.Requests) != dataconstants.SingleItem || status.Newest().ID != requestID {
		t.Fatalf("status did not decode: %+v", status)
	}
}

func TestRequestCallsPostTheirBodyToTheirRoute(t *testing.T) {
	cases := []struct {
		name     string
		call     func(*bootstrapclient.Client) *response.GenericResponse
		wantPath string
		wantBody string
	}{
		{"open", func(c *bootstrapclient.Client) *response.GenericResponse { return c.Open(email) }, openPath, openBody},
		{"approve", func(c *bootstrapclient.Client) *response.GenericResponse {
			return c.Approve(requestID)
		}, approvePath, dataconstants.EmptyString},
		{"cancel", func(c *bootstrapclient.Client) *response.GenericResponse { return c.Cancel(requestID, reason) }, cancelPath, cancelBody},
		{"complete", func(c *bootstrapclient.Client) *response.GenericResponse {
			return c.Complete(requestID, eps.CompleteBody{Revoke: true, Keep: keepCredential})
		}, completePath, completeBody},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, got := serve(t, okBody)

			resp := tc.call(client)
			if resp == nil || resp.Status != http.StatusOK {
				t.Fatalf("expected status %d, got %+v", http.StatusOK, resp)
			}
			if got.method != http.MethodPost || got.path != tc.wantPath {
				t.Fatalf("expected POST %s, got %s %s", tc.wantPath, got.method, got.path)
			}
			if got.token != testServiceToken {
				t.Fatalf("expected %s %q, got %q", dataconstants.HeaderServiceToken, testServiceToken, got.token)
			}
			if got.body != tc.wantBody {
				t.Fatalf("body = %s, want %s", got.body, tc.wantBody)
			}
		})
	}
}

func TestResult(t *testing.T) {
	cases := []struct {
		name    string
		resp    *response.GenericResponse
		wantErr bool
	}{
		{"a 200 carries the request", &response.GenericResponse{Status: http.StatusOK, Data: map[string]any{
			"request": map[string]any{"id": requestID}, "eligible": true, "enrolled": true,
		}}, false},
		{"any other status is the exporter's refusal", &response.GenericResponse{Status: http.StatusConflict, Message: refusal}, true},
		{"data that cannot be encoded", &response.GenericResponse{Status: http.StatusOK, Data: make(chan int)}, true},
		{"data of the wrong shape", &response.GenericResponse{Status: http.StatusOK, Data: requestID}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := bootstrapclient.Result(tc.resp)
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), tc.resp.Message) {
					t.Fatalf("expected an error carrying %q, got result=%+v err=%v", tc.resp.Message, result, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("result: %v", err)
			}
			if result.Request.ID != requestID || !result.Eligible || !result.Enrolled {
				t.Fatalf("result did not decode: %+v", result)
			}
		})
	}
}
