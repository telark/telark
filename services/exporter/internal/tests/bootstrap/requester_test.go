package bootstrap

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/telark/telark/services/exporter/internal/bootstrap"
	"github.com/telark/telark/services/exporter/internal/constants"
)

const (
	emailOnlyBody   = `{"email":"admin@example.com"}`
	noRequesterBody = `{"email":"admin@example.com","requestedBy":""}`
)

// A request names the Kubernetes user who opened it: without one nothing opens, through the route or the package.
func TestOpenNeedsARequester(t *testing.T) {
	client := installFake(t, append(directory(t), configWith(t, pinned()), active(pinnedID, pinnedEmail, true).seed(t))...)
	for name, body := range map[string]string{"no requestedBy": emailOnlyBody, "an empty requestedBy": noRequesterBody} {
		w := serveAs(t, internal, http.MethodPost, requestsPath, body)
		equal(t, name+" status", w.Code, http.StatusBadRequest)
		equal(t, name+" names the field", strings.Contains(w.Body.String(), string(constants.ErrBootstrapRequesterNeeded)), true)
	}
	outcome, err := bootstrap.Open(newOptimizer(t), pinnedEmail, constants.EmptyString)
	if !errors.Is(err, bootstrap.ErrRequester) || outcome.Result.Request.ID != constants.EmptyString || len(outcome.Changed) != none {
		t.Fatalf("open = %+v, %v, want the requester refusal and no outcome", outcome, err)
	}
	equal(t, "requests", len(storedStatus(t, client).Requests), none)
}

// The requester is stored on the request, answered with it, and named first in every opened notice: the pinned
// account reads who a replacement names, another Admin on ALL reads that masked.
func TestOpenedRequestNamesItsRequester(t *testing.T) {
	hours := bootstrap.WaitHours()
	rule := fmt.Sprintf(constants.NotifBootstrapOpenedRule, hours)
	recoverText := fmt.Sprintf(constants.NotifBootstrapOpenedRecover, requester, hours)
	cases := []struct {
		name  string
		email string
		texts map[string]string
	}{
		{"recover", pinnedEmail, map[string]string{pinnedID: recoverText, adminID: recoverText}},
		{"replace", otherEmail, map[string]string{
			pinnedID: fmt.Sprintf(constants.NotifBootstrapOpenedReplace, requester, otherEmail) + rule,
			adminID:  fmt.Sprintf(constants.NotifBootstrapOpenedMasked, requester) + rule,
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withNotices(t)
			client := installFake(t, append(directory(t), configWith(t, pinned()), active(pinnedID, pinnedEmail, true, adminRoleID).seed(t))...)

			opened := open(t, c.email, http.StatusOK)

			equal(t, "answered requester", opened.Request.RequestedBy, requester)
			equal(t, "stored requester", storedStatus(t, client).Requests[none].RequestedBy, requester)
			for id, want := range c.texts {
				got := notices(t, id)
				if len(got) != one {
					t.Fatalf("%s got %d notices, want 1", id, len(got))
				}
				equal(t, id+" title", got[none].Title, constants.NotifBootstrapOpenedTitle)
				equal(t, id+" message", got[none].Message, want)
				equal(t, id+" reads the requester first", strings.HasPrefix(got[none].Message, requester+" opened an auth request"), true)
			}
		})
	}
}
