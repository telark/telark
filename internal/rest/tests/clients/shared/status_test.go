package shared

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/telark/rest/clients/shared"
)

const sessionRefsBody = `{"data":{"items":[{"userId":"u1","metadata":{"name":"session-a"}},{"userId":"u1","metadata":{"name":"session-b"}}]}}`

// The exporter answers 410 for a session that has lapsed; the sentinel lets a
// resolver report a verdict instead of an outage.
func TestSingleResponseStatusSentinels(t *testing.T) {
	tests := map[string]struct {
		status int
		want   error
	}{
		"not found": {http.StatusNotFound, shared.ErrNotFound},
		"gone":      {http.StatusGone, shared.ErrGone},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			body := &trackingBody{reader: strings.NewReader(dataBody)}
			_, err := sessionWith(trackingTransport(tc.status, body)).GetSessionByToken(secretToken)
			if !errors.Is(err, tc.want) {
				t.Fatalf("GetSessionByToken error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestListSessionRefsByUserReturnsNames(t *testing.T) {
	body := &trackingBody{reader: strings.NewReader(sessionRefsBody)}
	refs, err := sessionWith(trackingTransport(http.StatusOK, body)).ListSessionRefsByUser("u1")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(refs, []string{"session-a", "session-b"}) {
		t.Fatalf("refs = %v, want [session-a session-b]", refs)
	}
}
