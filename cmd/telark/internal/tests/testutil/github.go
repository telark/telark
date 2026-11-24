package testutil

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

const (
	gitHubHost  = "api.github.com"
	LatestFound = `{"tag_name": "v0.0.0", "html_url": "https://github.com/telark/telark/releases/tag/v0.0.0"}`
)

// It swaps a process-wide transport, so only a TestMain may call it.
func InstallGitHub() *GitHub {
	fake := &GitHub{Next: http.DefaultTransport, Status: http.StatusOK, Body: LatestFound}
	http.DefaultTransport = fake
	return fake
}

func (g *GitHub) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Host != gitHubHost {
		return g.Next.RoundTrip(request)
	}
	g.Requests = append(g.Requests, request)
	if g.Err != nil {
		return nil, g.Err
	}
	return respond(request, g.Status, jsonType, g.Body), nil
}

func respond(request *http.Request, status int, mediaType, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{contentType: []string{mediaType}},
		Request:    request,
	}
}

func (g *GitHub) Answer(t testing.TB, status int, body string, err error) {
	g.Status, g.Body, g.Err, g.Requests = status, body, err, nil
	t.Cleanup(func() { g.Status, g.Body, g.Err, g.Requests = http.StatusOK, LatestFound, nil, nil })
}
