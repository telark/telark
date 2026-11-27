package check

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/telark/telark/cmd/telark/internal/tests/testutil"
)

const (
	skipFlag   = "--skip-update-check"
	skipEnv    = "TELARK_SKIP_UPDATE_CHECK"
	latestURL  = "https://api.github.com/repos/telark/telark/releases/latest"
	oneRequest = 1
)

func TestUpdateCheck(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
		err    error
		line   string
	}{
		"a newer release": {
			status: http.StatusOK, body: `{"tag_name": "v0.3.0", "html_url": "https://github.com/telark/telark/releases/tag/v0.3.0"}`,
			line: "‼ Telark 0.3.0 is out, and this CLI is 0.0.0-dev: https://github.com/telark/telark/releases/tag/v0.3.0\n",
		},
		"the same release": {
			status: http.StatusOK, body: testutil.LatestFound, line: "✔ This CLI (0.0.0-dev) is the latest release\n",
		},
		"no network": {
			err:  errors.New("dial tcp: no route to host"),
			line: `‼ The update check was skipped: Get "` + latestURL + `": dial tcp: no route to host` + "\n",
		},
		"a GitHub error": {
			status: http.StatusServiceUnavailable, line: "‼ The update check was skipped: GitHub answered Service Unavailable\n",
		},
		"an answer that isn't JSON": {
			status: http.StatusOK, body: "<html>", line: "‼ The update check was skipped: invalid character '<'",
		},
		"a tag that isn't a version": {
			status: http.StatusOK, body: `{"tag_name": "nightly"}`, line: "‼ The update check was skipped: could not parse \"nightly\" as version\n",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			github.Answer(t, c.status, c.body, c.err)
			result := run(t, testutil.Cluster{Objects: testutil.HealthyTelark()})
			testutil.Contains(t, stdoutLabel, result.Stdout, c.line)
			testutil.Equal(t, codeLabel, result.Code, passed)
			testutil.Equal(t, "requests", len(github.Requests), oneRequest)
		})
	}
}

func TestUpdateCheckRequest(t *testing.T) {
	github.Answer(t, http.StatusOK, testutil.LatestFound, nil)
	run(t, testutil.Cluster{Objects: testutil.FreshCluster()}, preFlag)
	if len(github.Requests) != oneRequest {
		t.Fatalf("requests: got %d, want 1", len(github.Requests))
	}
	request := github.Requests[firstIndex]
	testutil.Equal(t, "url", request.URL.String(), latestURL)
	testutil.Equal(t, "accept", request.Header.Get("Accept"), "application/vnd.github+json")
	testutil.Equal(t, "user agent", request.Header.Get("User-Agent"), "telark/0.0.0-dev")
}

func TestUpdateCheckOptOut(t *testing.T) {
	cases := map[string]struct {
		env      string
		args     []string
		requests int
	}{
		"the flag":                  {args: []string{skipFlag}},
		"the environment":           {env: "1"},
		"the environment saying no": {env: "false", requests: oneRequest},
		"a value that isn't a bool": {env: "please", requests: oneRequest},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			github.Answer(t, http.StatusOK, testutil.LatestFound, nil)
			t.Setenv(skipEnv, c.env)
			result := run(t, testutil.Cluster{Objects: testutil.HealthyTelark()}, c.args...)
			testutil.Equal(t, "requests", len(github.Requests), c.requests)
			testutil.Equal(t, "update line", strings.Contains(result.Stdout, "is the latest release"), c.requests == oneRequest)
		})
	}
}
