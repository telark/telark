package shared

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/telark/rest/base"
	"github.com/telark/rest/clients/auth/session"
	"github.com/telark/rest/clients/shared"
	authendpoints "github.com/telark/rest/endpoints/auth"
)

const (
	dataBody        = `{"data":{}}`
	itemsBody       = `{"data":{"items":[]}}`
	jsonArrayBody   = `[]`
	invalidJSONBody = `{`
	readFail        = "simulated read failure"
	singleClose     = 1
	noClose         = 0
	wantClosedOnce  = "the response body must be closed exactly once, got %d"
	wantNoClose     = "a transport failure leaves no body to close, got %d"
)

type trackingBody struct {
	reader  io.Reader
	readErr error
	closes  int
}

func (b *trackingBody) Read(p []byte) (int, error) {
	if b.readErr != nil {
		return noClose, b.readErr
	}
	return b.reader.Read(p)
}

func (b *trackingBody) Close() error {
	b.closes++
	return nil
}

func trackingTransport(status int, body *trackingBody) roundTripFunc {
	return func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: status, Body: body, Header: http.Header{}}, nil
	}
}

func sessionWith(rt http.RoundTripper) *session.Client {
	client := session.NewClient()
	client.GetHTTPClient().Transport = rt
	return client
}

func sharedWith(rt http.RoundTripper) *shared.Client {
	client := shared.New(base.Exporter)
	client.GetHTTPClient().Transport = rt
	return client
}

// Every close path of the single body owner: each case drives one parse route
// and asserts the connection is released exactly once.
func TestResponseBodyIsClosedExactlyOnce(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		readErr error
		invoke  func(rt http.RoundTripper)
	}{
		{
			name:   "generic response success",
			status: http.StatusOK,
			body:   okBody,
			invoke: func(rt http.RoundTripper) { sessionWith(rt).DeleteSessionByToken(secretToken) },
		},
		{
			name:   "generic response non-2xx",
			status: http.StatusInternalServerError,
			body:   peerBody,
			invoke: func(rt http.RoundTripper) { sessionWith(rt).DeleteSessionByToken(secretToken) },
		},
		{
			name:    "generic response read error",
			status:  http.StatusOK,
			readErr: errors.New(readFail),
			invoke:  func(rt http.RoundTripper) { sessionWith(rt).DeleteSessionByToken(secretToken) },
		},
		{
			name:   "generic response unmarshal error",
			status: http.StatusOK,
			body:   invalidJSONBody,
			invoke: func(rt http.RoundTripper) { sessionWith(rt).DeleteSessionByToken(secretToken) },
		},
		{
			name:   "generic response with error status",
			status: http.StatusInternalServerError,
			body:   peerBody,
			invoke: func(rt http.RoundTripper) {
				_, _ = sessionWith(rt).GetAllSessionsByUser(secretUserID)
			},
		},
		{
			name:   "typed get success",
			status: http.StatusOK,
			body:   dataBody,
			invoke: func(rt http.RoundTripper) { _, _ = sessionWith(rt).GetSessionByToken(secretToken) },
		},
		{
			name:   "typed get non-2xx",
			status: http.StatusNotFound,
			body:   emptyBody,
			invoke: func(rt http.RoundTripper) { _, _ = sessionWith(rt).GetSessionByToken(secretToken) },
		},
		{
			name:    "typed get read error",
			status:  http.StatusOK,
			readErr: errors.New(readFail),
			invoke:  func(rt http.RoundTripper) { _, _ = sessionWith(rt).GetSessionByToken(secretToken) },
		},
		{
			name:   "typed get unmarshal error",
			status: http.StatusOK,
			body:   invalidJSONBody,
			invoke: func(rt http.RoundTripper) { _, _ = sessionWith(rt).GetSessionByToken(secretToken) },
		},
		{
			name:   "typed list success",
			status: http.StatusOK,
			body:   itemsBody,
			invoke: func(rt http.RoundTripper) {
				_, _ = sessionWith(rt).GetAllSessionsByUser(secretUserID)
			},
		},
		{
			name:   "typed get with headers",
			status: http.StatusOK,
			body:   dataBody,
			invoke: func(rt http.RoundTripper) {
				_, _ = shared.GetWithHeaders[map[string]any](
					sharedWith(rt), authendpoints.GetSessionByToken, nil)
			},
		},
		{
			name:   "typed list with headers",
			status: http.StatusOK,
			body:   itemsBody,
			invoke: func(rt http.RoundTripper) {
				_, _ = shared.GetListWithHeaders[map[string]any](
					sharedWith(rt), authendpoints.GetSessionByToken, nil)
			},
		},
		{
			name:   "raw json with headers",
			status: http.StatusOK,
			body:   jsonArrayBody,
			invoke: func(rt http.RoundTripper) {
				_, _ = shared.GetRawJSONWithHeaders[json.RawMessage](
					sharedWith(rt), authendpoints.GetSessionByToken, nil)
			},
		},
		{
			name:   "generic response with headers",
			status: http.StatusOK,
			body:   okBody,
			invoke: func(rt http.RoundTripper) {
				shared.ExecuteRequestWithHeaders(
					sharedWith(rt), base.Post, authendpoints.GetSessionByToken, nil, nil)
			},
		},
		{
			name:   "generic response slice success",
			status: http.StatusOK,
			body:   jsonArrayBody,
			invoke: func(rt http.RoundTripper) {
				_, _ = sharedWith(rt).PostAndParseGenericResponses(authendpoints.GetSessionByToken)
			},
		},
		{
			name:   "generic response slice error status",
			status: http.StatusInternalServerError,
			body:   peerBody,
			invoke: func(rt http.RoundTripper) {
				_, _ = sharedWith(rt).PostAndParseGenericResponses(authendpoints.GetSessionByToken)
			},
		},
		{
			name:    "generic response slice read error",
			status:  http.StatusOK,
			readErr: errors.New(readFail),
			invoke: func(rt http.RoundTripper) {
				_, _ = sharedWith(rt).PostAndParseGenericResponses(authendpoints.GetSessionByToken)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := &trackingBody{reader: strings.NewReader(tt.body), readErr: tt.readErr}
			captureStdout(t, func() { tt.invoke(trackingTransport(tt.status, body)) })

			if body.closes != singleClose {
				t.Errorf(wantClosedOnce, body.closes)
			}
		})
	}
}

func TestTransportFailureLeavesNoBodyToClose(t *testing.T) {
	body := &trackingBody{reader: strings.NewReader(okBody)}

	logged := captureStdout(t, func() {
		if _, err := sessionWith(roundTripFunc(transportError)).GetSessionByToken(secretToken); err == nil {
			t.Error("expected a failure from the session lookup")
		}
	})

	assertNoLeak(t, "log", secretToken, logged)
	if body.closes != noClose {
		t.Errorf(wantNoClose, body.closes)
	}
}
