package snapshots

import (
	"net/http"
	"net/http/httptest"
	"testing"

	snapshotsclient "github.com/telark/telark/internal/rest/clients/snapshots"
)

const (
	snapshotID        = "s1"
	snapshotScope     = "namespace"
	snapshotNamespace = "apps"
	okBody            = `{"status":200,"operation":"Success"}`
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDeleteSnapshotChecksGeneration(t *testing.T) {
	cases := []struct {
		name, generation, wantQuery string
		wantErr                     bool
	}{
		{name: "numeric", generation: "3", wantQuery: "generation=3&namespace=apps&scope=namespace"},
		{name: "not a number", generation: "latest", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var query string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				query = r.URL.RawQuery
				if _, err := w.Write([]byte(okBody)); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			t.Cleanup(server.Close)
			client := snapshotsclient.NewClient()
			client.GetHTTPClient().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				r.URL.Scheme = "http"
				r.URL.Host = server.Listener.Addr().String()
				return http.DefaultTransport.RoundTrip(r)
			})

			_, err := client.DeleteSnapshot(snapshotID, snapshotScope, snapshotNamespace, tc.generation)
			if (err != nil) != tc.wantErr || query != tc.wantQuery {
				t.Fatalf("generation %q: query %q, error %v", tc.generation, query, err)
			}
		})
	}
}
