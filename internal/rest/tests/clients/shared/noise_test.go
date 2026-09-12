package shared

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/telark/rest/clients/auth/session"
	"github.com/telark/rest/clients/notifications"
	authendpoints "github.com/telark/rest/endpoints/auth"
)

const (
	peerBody       = `{"status":500,"message":"peer-body-must-not-be-logged"}`
	okBody         = `{"status":200,"operation":"Success"}`
	levelDebug     = "[DEBUG]"
	levelWarning   = "[WARNING]"
	levelError     = "[ERROR]"
	statusField    = "status=%d"
	methodField    = "method=%s"
	wantSilent     = "a successful call must log nothing, got: %q"
	wantOneLine    = "one failure must produce exactly one log line, got %d: %q"
	wantField      = "log line lost %q, a failure is no longer diagnosable: %s"
	wantNoBody     = "the peer response body reached the log: %s"
	wantInMessage  = "the response body must still reach the caller, got %q"
	wantLevel      = "expected level %s for this outcome, got: %s"
	unwantedLevels = "expected no %s for a routine %d, got: %s"
)

func respondWith(status int, body string) roundTripFunc {
	return func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{},
		}, nil
	}
}

func logLines(out string) []string {
	trimmed := strings.TrimSpace(out)
	if trimmed == emptyBody {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func TestSuccessfulCallLogsNothing(t *testing.T) {
	client := session.NewClient()
	client.GetHTTPClient().Transport = respondWith(http.StatusOK, okBody)

	logged := captureStdout(t, func() {
		client.DeleteSessionByToken(secretToken)
	})

	if strings.TrimSpace(logged) != emptyBody {
		t.Errorf(wantSilent, logged)
	}
}

func TestServerErrorLogsOnceAndKeepsTheBodyOutOfTheLog(t *testing.T) {
	client := session.NewClient()
	client.GetHTTPClient().Transport = respondWith(http.StatusInternalServerError, peerBody)

	var message string
	logged := captureStdout(t, func() {
		message = client.DeleteSessionByToken(secretToken).Message
	})

	lines := logLines(logged)
	if len(lines) != 1 {
		t.Fatalf(wantOneLine, len(lines), logged)
	}
	for _, want := range []string{
		string(authendpoints.DeleteSessionByToken),
		fmt.Sprintf(statusField, http.StatusInternalServerError),
		fmt.Sprintf(methodField, http.MethodDelete),
	} {
		if !strings.Contains(logged, want) {
			t.Errorf(wantField, want, logged)
		}
	}
	if strings.Contains(logged, peerBody) {
		t.Errorf(wantNoBody, logged)
	}
	if !strings.Contains(message, peerBody) {
		t.Errorf(wantInMessage, message)
	}
}

func TestRoutineNotFoundIsNotWarnOrError(t *testing.T) {
	client := session.NewClient()
	client.GetHTTPClient().Transport = roundTripFunc(notFound)

	logged := captureStdout(t, func() {
		if _, err := client.GetSessionByToken(secretToken); err == nil {
			t.Error("expected a failure from the session lookup")
		}
	})

	for _, level := range []string{levelWarning, levelError} {
		if strings.Contains(logged, level) {
			t.Errorf(unwantedLevels, level, http.StatusNotFound, logged)
		}
	}
	if !strings.Contains(logged, levelDebug) {
		t.Errorf(wantLevel, levelDebug, logged)
	}
}

func TestTransportFailureStaysAtWarn(t *testing.T) {
	client := notifications.NewClient()
	client.GetHTTPClient().Transport = roundTripFunc(transportError)

	logged := captureStdout(t, func() {
		if _, err := client.List(context.Background(), secretUserID, notifications.ListOptions{}); err == nil {
			t.Error("expected a failure from the notification list call")
		}
	})

	if len(logLines(logged)) != 1 {
		t.Fatalf(wantOneLine, len(logLines(logged)), logged)
	}
	if !strings.Contains(logged, levelWarning) {
		t.Errorf(wantLevel, levelWarning, logged)
	}
}
