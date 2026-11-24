package authrequests

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/authrequest"
	"github.com/telark/telark/services/exporter/internal/authrequests"
	"github.com/telark/telark/services/exporter/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	nameSep        = ","
	oldestPending  = "req-g-pending"
	linkReady      = "req-e-link-ready"
	recovering     = "req-f-recovering"
	neverStarted   = "req-h-unset"
	unknownPhase   = "Paused"
	reason         = "the bootstrap account denied the request"
	bootstrapID    = "0123456789abcdef"
	approvalURL    = "https://telark.example.com/auth-request"
	second         = time.Minute
	third          = 2 * time.Minute
	fourth         = 3 * time.Minute
	wantPendingIDs = oldestPending + nameSep + linkReady + nameSep + recovering + nameSep + neverStarted
)

// One request per phase, stored out of order; the two a run left halfway share a creation second.
func everyPhase() []*unstructured.Unstructured {
	return []*unstructured.Unstructured{
		request(neverStarted, constants.EmptyString, opened.Add(fourth)),
		request(recovering, authrequest.PhaseRecovering, opened.Add(third)),
		request("req-d-waiting", authrequest.PhaseWaitingForApproval, opened),
		request(linkReady, authrequest.PhaseLinkReady, opened.Add(third)),
		request("req-c-completed", authrequest.PhaseCompleted, opened),
		request(oldestPending, authrequest.PhasePending, opened.Add(second)),
		request("req-b-denied", authrequest.PhaseDenied, opened),
		request("req-a-canceled", authrequest.PhaseCanceled, opened),
		request("req-i-expired", authrequest.PhaseExpired, opened),
		request("req-j-failed", authrequest.PhaseFailed, opened),
	}
}

func idsOf(pending []authrequest.AuthRequest) string {
	ids := make([]string, constants.DefaultInitValue, len(pending))
	for i := range pending {
		ids = append(ids, pending[i].ID)
	}
	return strings.Join(ids, nameSep)
}

// Pending is what no run has finished: not started, or left halfway. The oldest comes first and the name breaks a tie,
// whether the list comes from the synced mirror, with no apiserver list, or from the apiserver before the sync.
func TestPendingListsUnfinishedRequestsOldestFirst(t *testing.T) {
	for name, synced := range map[string]bool{"from the synced mirror": true, "from the apiserver before the sync": false} {
		t.Run(name, func(t *testing.T) {
			if synced {
				answer(installFake(t, everyPhase()...), verbList, nil, errDown)
				installMirror(t, everyPhase()...)
			} else {
				installFake(t, everyPhase()...)
			}

			pending, err := authrequests.Pending()
			if err != nil {
				t.Fatal(err)
			}
			equal(t, "pending", idsOf(pending), wantPendingIDs)
			equal(t, "oldest", pending[none], authrequest.AuthRequest{
				ID:        oldestPending,
				Spec:      authrequest.Spec{Email: email, RequestedBy: requester, RecipientKey: recipientKey},
				Status:    authrequest.Status{Phase: authrequest.PhasePending},
				CreatedAt: opened.Add(second).Format(time.RFC3339),
			})
		})
	}
}

// The mirror trails a status write: a request it still shows unfinished is pending only if the apiserver agrees, so
// the run that follows one that just ended neither repeats it nor fails it as left halfway.
func TestPendingReadsEachMirroredRequestBack(t *testing.T) {
	cases := []struct {
		name     string
		mirrored string
		stored   *unstructured.Unstructured
		getFails error
		want     string
		fails    bool
	}{
		{"still unfinished", authrequest.PhaseLinkReady, request(storedName, authrequest.PhaseLinkReady, opened), nil, storedName, false},
		{"finished since", authrequest.PhaseLinkReady, request(storedName, authrequest.PhaseCompleted, opened), nil, constants.EmptyString, false},
		{"resting since", constants.EmptyString, request(storedName, authrequest.PhaseWaitingForApproval, opened), nil,
			constants.EmptyString, false},
		{"deleted since", authrequest.PhasePending, request(missingName, authrequest.PhasePending, opened), nil, constants.EmptyString, false},
		{"the read fails", authrequest.PhasePending, request(storedName, authrequest.PhasePending, opened), errDown,
			constants.EmptyString, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t, c.stored)
			if c.getFails != nil {
				answer(client, verbGet, nil, c.getFails)
			}
			installMirror(t, request(storedName, c.mirrored, opened))

			pending, err := authrequests.Pending()
			equal(t, "fails", err != nil, c.fails)
			equal(t, "pending", idsOf(pending), c.want)
			equal(t, "lists", countOf(client, verbList), none)
		})
	}
}

func TestPendingFailsWhenTheListFails(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"the apiserver refuses", errDown, errDown.Error()},
		{"the apiserver answers no list", nil, string(constants.ErrInvalidResourceTypeReturned)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			answer(installFake(t, everyPhase()...), verbList, nil, c.err)

			pending, err := authrequests.Pending()
			if err == nil || pending != nil {
				t.Fatalf("pending = %v, %v, want an error and no list", pending, err)
			}
			equal(t, "names the cause", strings.Contains(err.Error(), c.want), true)
		})
	}
}

// A phase outside the ones auth reports is refused before any write; a write the stored CRD drops is an error even
// though the apiserver answered it.
func TestSetStatusRefusals(t *testing.T) {
	cases := []struct {
		name    string
		target  string
		phase   string
		reply   runtime.Object
		fails   error
		want    error
		text    string
		patches int
	}{
		{"the phase a new request starts in", storedName, authrequest.PhasePending, nil, nil, authrequests.ErrPhase,
			string(constants.ErrAuthRequestPhase), none},
		{"no phase", storedName, constants.EmptyString, nil, nil, authrequests.ErrPhase, string(constants.ErrAuthRequestPhase), none},
		{"an unknown phase", storedName, unknownPhase, nil, nil, authrequests.ErrPhase, string(constants.ErrAuthRequestPhase), none},
		{"an unknown request", missingName, authrequest.PhaseCompleted, nil, nil, authrequests.ErrNotFound,
			string(constants.ErrAuthRequestNotFound), one},
		{"a failed write", storedName, authrequest.PhaseCompleted, nil, errDown, nil, errDown.Error(), one},
		{"a status the stored CRD prunes", storedName, authrequest.PhaseCompleted, request(storedName, constants.EmptyString, opened), nil,
			nil, fmt.Sprintf(string(constants.ErrAuthRequestStatusPruned), storedName), one},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t, request(storedName, authrequest.PhasePending, opened))
			if c.reply != nil || c.fails != nil {
				answer(client, verbPatch, c.reply, c.fails)
			}

			err := authrequests.SetStatus(c.target, authrequest.Status{Phase: c.phase})
			if err == nil {
				t.Fatal("the status was accepted")
			}
			for _, refusal := range []error{authrequests.ErrPhase, authrequests.ErrNotFound} {
				equal(t, refusal.Error(), errors.Is(err, refusal), errors.Is(refusal, c.want))
			}
			equal(t, "names the cause", strings.Contains(err.Error(), c.text), true)
			equal(t, "patches", countOf(client, verbPatch), c.patches)
			equal(t, "stored phase", storedStatus(t, client, storedName)[v1alpha1.StatusPhase], any(authrequest.PhasePending))
		})
	}
}

// Each phase auth reports is stored with the fields sent beside it, and a field an earlier step set stays.
func TestSetStatusMergesEachReportedPhase(t *testing.T) {
	reported := []string{
		authrequest.PhaseWaitingForApproval, authrequest.PhaseRecovering, authrequest.PhaseLinkReady, authrequest.PhaseCompleted,
		authrequest.PhaseDenied, authrequest.PhaseCanceled, authrequest.PhaseExpired, authrequest.PhaseFailed,
	}
	for _, phase := range reported {
		t.Run(phase, func(t *testing.T) {
			earlier := request(storedName, authrequest.PhaseWaitingForApproval, opened)
			if err := unstructured.SetNestedField(earlier.Object, approvalURL, constants.FieldStatus, v1alpha1.StatusApprovalURL); err != nil {
				t.Fatal(err)
			}
			client := installFake(t, earlier)

			if err := authrequests.SetStatus(storedName, authrequest.Status{Phase: phase, Reason: reason, RequestID: bootstrapID}); err != nil {
				t.Fatal(err)
			}
			status := storedStatus(t, client, storedName)
			want := map[string]string{
				v1alpha1.StatusPhase: phase, v1alpha1.StatusReason: reason, v1alpha1.StatusRequestID: bootstrapID,
				v1alpha1.StatusApprovalURL: approvalURL,
			}
			for field, value := range want {
				equal(t, field, status[field], any(value))
			}
			equal(t, "status fields", len(status), len(want))
		})
	}
}
