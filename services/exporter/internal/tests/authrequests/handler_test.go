package authrequests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/authrequest"
	"github.com/telark/telark/services/exporter/internal/constants"
	authrequesthandler "github.com/telark/telark/services/exporter/internal/handlers/authrequests"
)

const (
	pendingPath   = "/api/v1/internal/auth/requests/pending"
	statusPath    = "/api/v1/internal/auth/requests/%s/status"
	completedBody = `{"phase":"Completed","reason":"the bootstrap account denied the request"}`
	pendingBody   = `{"phase":"Pending"}`
	numberBody    = `{"phase":5}`
	unknownBody   = `{"phase":"Completed","approvedBy":"ops-admin"}`
	brokenBody    = `{"phase":`
)

func TestPendingRoute(t *testing.T) {
	t.Run("lists the unfinished requests", func(t *testing.T) {
		installFake(t, everyPhase()...)

		w := serve(t, http.MethodGet, pendingPath, constants.EmptyString)
		if w.Code != http.StatusOK {
			t.Fatalf("GET pending = %d %s", w.Code, w.Body.String())
		}
		var envelope pendingEnvelope
		if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		equal(t, "items", idsOf(envelope.Data.Items), wantPendingIDs)
		equal(t, "message", envelope.Message, string(constants.MsgAuthRequestsListed))
		equal(t, "requester", envelope.Data.Items[none].RequestedBy, requester)
	})
	t.Run("a failed list is a 500", func(t *testing.T) {
		answer(installFake(t), verbList, nil, errDown)

		w := serve(t, http.MethodGet, pendingPath, constants.EmptyString)
		equal(t, "status", w.Code, http.StatusInternalServerError)
		equal(t, "names the cause", strings.Contains(w.Body.String(), errDown.Error()), true)
	})
}

// A refusal and a bad body are answers, 4xx, and only a 200 changes the stored status.
func TestStatusRoute(t *testing.T) {
	cases := []struct {
		name   string
		target string
		body   string
		fails  error
		want   int
		text   string
	}{
		{"a reported phase", storedName, completedBody, nil, http.StatusOK, string(constants.MsgAuthRequestStatusUpdated)},
		{"the phase a new request starts in", storedName, pendingBody, nil, http.StatusBadRequest, string(constants.ErrAuthRequestPhase)},
		{"a phase that isn't text", storedName, numberBody, nil, http.StatusBadRequest, v1alpha1.StatusPhase},
		{"a field the status lacks", storedName, unknownBody, nil, http.StatusBadRequest, constants.EmptyString},
		{"a body that isn't JSON", storedName, brokenBody, nil, http.StatusUnprocessableEntity, constants.EmptyString},
		{"an unknown request", missingName, completedBody, nil, http.StatusNotFound, string(constants.ErrAuthRequestNotFound)},
		{"a failed write", storedName, completedBody, errDown, http.StatusInternalServerError, errDown.Error()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t, request(storedName, authrequest.PhaseWaitingForApproval, opened))
			if c.fails != nil {
				answer(client, verbPatch, nil, c.fails)
			}

			w := serve(t, http.MethodPatch, fmt.Sprintf(statusPath, c.target), c.body)
			if w.Code != c.want {
				t.Fatalf("PATCH status = %d %s, want %d", w.Code, w.Body.String(), c.want)
			}
			equal(t, "names the cause", strings.Contains(w.Body.String(), c.text), true)
			phase := map[bool]string{true: authrequest.PhaseCompleted, false: authrequest.PhaseWaitingForApproval}[c.want == http.StatusOK]
			equal(t, "stored phase", storedStatus(t, client, storedName)[v1alpha1.StatusPhase], any(phase))
		})
	}
}

// Off its route the handler has no request name, and answers that instead of writing.
func TestStatusHandlerNeedsTheRequestName(t *testing.T) {
	client := installFake(t, request(storedName, authrequest.PhaseWaitingForApproval, opened))
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPatch, pendingPath, strings.NewReader(completedBody))

	authrequesthandler.PatchStatus()(w, r)

	equal(t, "status", w.Code, http.StatusBadRequest)
	equal(t, "patches", countOf(client, verbPatch), none)
}
