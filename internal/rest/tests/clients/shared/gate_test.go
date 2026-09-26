package shared

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/telark/rest/base"
	"github.com/telark/rest/clients/shared"
	"github.com/telark/rest/connectivity"
	authendpoints "github.com/telark/rest/endpoints/auth"
)

const noCalls = 0

// A manager with no Redis reports every target not ready.
func withTargetNotReady(t *testing.T) *int {
	t.Helper()
	connectivity.SetGlobal(connectivity.New(nil))
	t.Cleanup(func() { connectivity.SetGlobal(nil) })
	return new(int)
}

func gatedClient(t *testing.T, calls *int) *shared.Client {
	t.Helper()
	client := shared.New(base.Exporter)
	client.GetHTTPClient().Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		*calls++
		t.Error("a not-ready target must never be sent a request")
		return nil, http.ErrServerClosed
	})
	return client
}

func TestHeaderGetsFailFastLikeGatedGets(t *testing.T) {
	calls := withTargetNotReady(t)
	client := gatedClient(t, calls)
	endpoint := authendpoints.GetSessionByToken

	_, want := shared.GetTyped[map[string]any](client, endpoint)
	if want == nil {
		t.Fatal("the gated GetTyped must fail for a not-ready target")
	}

	_, withHeaders := shared.GetWithHeaders[map[string]any](client, endpoint, nil)
	_, list := shared.GetListWithHeaders[map[string]any](client, endpoint, nil)
	_, raw := shared.GetRawJSONWithHeaders[json.RawMessage](client, endpoint, nil)

	for name, err := range map[string]error{"GetWithHeaders": withHeaders, "GetListWithHeaders": list, "GetRawJSONWithHeaders": raw} {
		if err == nil || err.Error() != want.Error() {
			t.Errorf("%s error = %v, want %v", name, err, want)
		}
	}
	if *calls != noCalls {
		t.Errorf("transport called %d times", *calls)
	}
}

func TestExecuteRequestWithHeadersFailsFastLikeGatedRequests(t *testing.T) {
	calls := withTargetNotReady(t)
	client := gatedClient(t, calls)
	endpoint := authendpoints.GetSessionByToken

	captureStdout(t, func() {
		want := client.Delete(endpoint)
		got := shared.ExecuteRequestWithHeaders(client, base.Delete, endpoint, nil, nil)
		if got.Status != want.Status || got.Message != want.Message {
			t.Errorf("got %d %q, want %d %q", got.Status, got.Message, want.Status, want.Message)
		}
	})
	if *calls != noCalls {
		t.Errorf("transport called %d times", *calls)
	}
}
