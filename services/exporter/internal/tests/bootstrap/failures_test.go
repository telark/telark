package bootstrap

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/services/exporter/internal/bootstrap"
	"github.com/telark/telark/services/exporter/internal/constants"
	bootstraphandlers "github.com/telark/telark/services/exporter/internal/handlers/bootstrap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	verbGet       = "get"
	verbList      = "list"
	notJSON       = "not json"
	mistypedEmail = `{"email":5}`
	blankEmail    = `{"email":" "}`
	mistypedKeep  = `{"keep":5}`
	reissueAge    = 23 * time.Hour
	keyRequests   = "requests"
)

func replacement(state string, opened time.Time) telarkconfig.BootstrapRequest {
	r := recovery(state, opened)
	r.Kind, r.TargetEmail = telarkconfig.BootstrapKindReplace, otherEmail
	return r
}

func garbled(r telarkconfig.BootstrapRequest) telarkconfig.BootstrapRequest {
	r.OpenedAt = garbledStamp
	return r
}

func failing(client *dynamicfake.FakeDynamicClient, verb, resource string) {
	client.PrependReactor(verb, resource, func(k8stesting.Action) (bool, runtime.Object, error) { return true, nil, errDown })
}

func TestRequestRoutesRefuseMalformedBodies(t *testing.T) {
	cases := []struct {
		name, path, body string
		want             int
	}{
		{"open, not JSON", requestsPath, notJSON, http.StatusUnprocessableEntity},
		{"open, mistyped", requestsPath, mistypedEmail, http.StatusBadRequest},
		{"open, no email", requestsPath, blankEmail, http.StatusBadRequest},
		{"cancel, not JSON", requestsPath + pathSep + requestID + pathSep + cancel, notJSON, http.StatusUnprocessableEntity},
		{"complete, not JSON", requestsPath + pathSep + requestID + pathSep + complete, notJSON, http.StatusUnprocessableEntity},
		{"complete, mistyped", requestsPath + pathSep + requestID + pathSep + complete, mistypedKeep, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t, append(directory(t), configWith(t, pinned(pending(time.Now()))))...)
			equal(t, "status", serveAs(t, internal, http.MethodPost, c.path, c.body).Code, c.want)
			equal(t, "request", storedStatus(t, client).Newest().State, telarkconfig.BootstrapStatePending)
		})
	}
}

// Called outside the router, a handler without the request id refuses before it reads anything.
func TestRequestHandlersNeedTheRequestID(t *testing.T) {
	optimizer := newOptimizer(t)
	handlers := map[string]http.HandlerFunc{
		approve:  bootstraphandlers.ApproveRequest(),
		cancel:   bootstraphandlers.CancelRequest(optimizer),
		complete: bootstraphandlers.CompleteRequest(optimizer),
	}
	for name, handler := range handlers {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			handler(w, httptest.NewRequest(http.MethodPost, requestsPath, strings.NewReader(completeOnce)))
			equal(t, "status", w.Code, http.StatusBadRequest)
		})
	}
}

func TestStatusReadFailureIsAnError(t *testing.T) {
	client := installFake(t, append(directory(t), configWith(t, pinned()))...)
	failing(client, verbGet, v1alpha1.TelarkConfigMetadata.Plural)
	equal(t, "status", serveAs(t, adminSession(adminID), http.MethodGet, basePath, constants.EmptyString).Code,
		http.StatusInternalServerError)
}

// A status a cluster admin garbled is never trusted: each request it judges by a stamp that doesn't parse fails.
func TestGarbledStampsFailTheRequest(t *testing.T) {
	completedReplacement := replacement(telarkconfig.BootstrapStateCompleted, time.Now().Add(-justOver))
	completedReplacement.CompletedAt = garbledStamp
	cases := []struct {
		name    string
		request telarkconfig.BootstrapRequest
		call    func(t *testing.T)
	}{
		{"a run over an open request", garbled(pending(time.Now())),
			func(t *testing.T) { open(t, pinnedEmail, http.StatusInternalServerError) }},
		{"a run over a completed replacement", completedReplacement,
			func(t *testing.T) { open(t, otherEmail, http.StatusInternalServerError) }},
		{"an approval", garbled(replacement(telarkconfig.BootstrapStatePending, time.Now())),
			func(t *testing.T) { act(t, requestID, approve, constants.EmptyString, http.StatusInternalServerError) }},
		{"a sign-in's cancel", garbled(pending(time.Now())),
			func(t *testing.T) { act(t, requestID, cancel, signedInBody, http.StatusInternalServerError) }},
		{"a keep pass on an ended recovery", garbled(recovery(telarkconfig.BootstrapStateCanceled, time.Now())),
			func(t *testing.T) {
				act(t, requestID, complete, fmt.Sprintf(keepBody, requestID), http.StatusInternalServerError)
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t, append(directory(t), configWith(t, pinned(c.request)), active(pinnedID, pinnedEmail, true).seed(t))...)
			c.call(t)
			equal(t, "request", storedStatus(t, client).Newest().State, c.request.State)
		})
	}
}

func TestApprovalAfterTheWaitIsRefused(t *testing.T) {
	pinnedDirectory(t, replacement(telarkconfig.BootstrapStatePending, time.Now().Add(-justOver)))
	equal(t, "eligible", act(t, requestID, approve, constants.EmptyString, http.StatusConflict).Eligible, false)
}

func TestAPendingReplacementCompletesOnlyOnceApproved(t *testing.T) {
	client := installFake(t, append(directory(t), configWith(t, pinned(replacement(telarkconfig.BootstrapStatePending, time.Now()))),
		active(pinnedID, pinnedEmail, true).seed(t), active(otherID, otherEmail, false).seed(t))...)
	act(t, requestID, complete, completeOnce, http.StatusConflict)
	equal(t, "pin", storedStatus(t, client).UserID, pinnedID)
}

// A completed replacement whose account enrolled a passkey since neither completes again nor re-issues: a run opens a
// new request.
func TestAnEnrolledReplacementNeverReissues(t *testing.T) {
	replaced := replacement(telarkconfig.BootstrapStateCompleted, time.Now().Add(-justOver))
	replaced.CompletedAt = stamp(time.Now().Add(-reissueAge))
	installFake(t, append(directory(t),
		configWith(t, &telarkconfig.BootstrapStatus{UserID: otherID, Email: otherEmail, PinnedAt: replaced.CompletedAt,
			Requests: []telarkconfig.BootstrapRequest{replaced}}),
		active(otherID, otherEmail, true, adminRoleID).seed(t), passkey(t, otherID, time.Now().Add(-time.Hour)))...)
	act(t, requestID, complete, completeOnce, http.StatusConflict)
	if next := open(t, otherEmail, http.StatusOK); next.Request.ID == requestID || next.Request.State != telarkconfig.BootstrapStatePending {
		t.Fatalf("run after enrollment = %+v, want a new pending request", next)
	}
}

// A read or write the API server fails fails the request (500). Only a replacement has moved the pin by then: a
// completion within its day sets the marker again.
func TestAPIFailuresFailTheRequest(t *testing.T) {
	users, passkeys := v1alpha1.UserMetadata.Plural, v1alpha1.PasskeyMetadata.Plural
	approvedReplacement := func(t *testing.T) (*dynamicfake.FakeDynamicClient, string) {
		client := installFake(t, append(directory(t),
			configWith(t, pinned(replacement(telarkconfig.BootstrapStateApproved, time.Now().Add(-time.Hour)))),
			active(pinnedID, pinnedEmail, true).seed(t), active(otherID, otherEmail, false).seed(t))...)
		return client, completeOnce
	}
	openRecovery := func(t *testing.T) (*dynamicfake.FakeDynamicClient, string) {
		client, _, after := recoveryAccount(t)
		return client, fmt.Sprintf(keepBody, after)
	}
	endedRecovery := func(t *testing.T) (*dynamicfake.FakeDynamicClient, string) {
		client, _, after := recoveryAccount(t)
		setStatus(t, client, pinned(recovery(telarkconfig.BootstrapStateCanceled, time.Now().Add(-justOver))))
		return client, fmt.Sprintf(keepBody, after)
	}
	cases := []struct {
		name           string
		setup          func(t *testing.T) (*dynamicfake.FakeDynamicClient, string)
		verb, resource string
		action         string
		pin            string
	}{
		{"the users list of a run", func(t *testing.T) (*dynamicfake.FakeDynamicClient, string) {
			return installFake(t, append(directory(t), configWith(t, pinned()))...), constants.EmptyString
		}, verbList, users, constants.EmptyString, pinnedID},
		{"the marker a replacement moves", approvedReplacement, verbPatch, users, complete, otherID},
		{"the passkeys a keep pass checks", openRecovery, verbList, passkeys, complete, pinnedID},
		{"the passkey sign-ins a keep pass ends", openRecovery, verbPatch, users, complete, pinnedID},
		{"the passkey a keep pass on an ended recovery drops", endedRecovery, verbList, passkeys, complete, pinnedID},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, body := c.setup(t)
			before := storedStatus(t, client).Newest()
			failing(client, c.verb, c.resource)
			if c.action == constants.EmptyString {
				open(t, pinnedEmail, http.StatusInternalServerError)
				equal(t, "requests", len(storedStatus(t, client).Requests), none)
				return
			}
			act(t, requestID, c.action, body, http.StatusInternalServerError)
			equal(t, "pin", storedStatus(t, client).UserID, c.pin)
			if before.State == telarkconfig.BootstrapStatePending {
				equal(t, "request", storedStatus(t, client).Newest().State, before.State)
			}
		})
	}
}

// The keep pass completes a recovery whose account has nothing left to drop, its record included.
func TestKeepCompletesWhateverTheAccountHolds(t *testing.T) {
	cases := []struct {
		name  string
		owner func(t *testing.T, kept string) []seed
	}{
		{"only the kept passkey sign-in", func(t *testing.T, kept string) []seed {
			return []seed{withIdentities(t, active(pinnedID, pinnedEmail, true).seed(t), kept)}
		}},
		{"no account record", func(*testing.T, string) []seed { return nil }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opened := time.Now().Add(-justOver)
			old, fresh := passkey(t, pinnedID, opened.Add(-time.Hour)), passkey(t, pinnedID, time.Now().Add(-time.Minute))
			seeds := append(directory(t), configWith(t, pinned(pending(opened))), old, fresh)
			client := installFake(t, append(seeds, c.owner(t, fresh.obj.GetName())...)...)
			act(t, requestID, complete, fmt.Sprintf(keepBody, fresh.obj.GetName()), http.StatusOK)
			equal(t, "recovery", storedStatus(t, client).Newest().State, telarkconfig.BootstrapStateCompleted)
			equal(t, "kept passkey", passkeyStored(t, client, fresh.obj.GetName()), true)
		})
	}
}

// A recovery a cluster admin ends while its keep pass deletes is not completed over that end.
func TestARecoveryEndedDuringItsKeepPassStaysEnded(t *testing.T) {
	client, _, after := recoveryAccount(t)
	var once sync.Once
	k8sclient.SetDynamicClient(writeHook{Interface: client, resource: gvrOf(v1alpha1.PasskeyMetadata), before: func() {
		once.Do(func() {
			setStatus(t, client, pinned(recovery(telarkconfig.BootstrapStateCanceled, time.Now().Add(-justOver))))
		})
	}})
	act(t, requestID, complete, fmt.Sprintf(keepBody, after), http.StatusConflict)
	equal(t, "recovery", storedStatus(t, client).Newest().State, telarkconfig.BootstrapStateCanceled)
}

// Requests the API server stores as anything but a list.
func garbledConfig(t *testing.T) seed {
	t.Helper()
	config := configWith(t, pinned())
	path := []string{constants.FieldStatus, telarkconfig.FieldBootstrap, keyRequests}
	if err := unstructured.SetNestedField(config.obj.Object, garbledStamp, path...); err != nil {
		t.Fatal(err)
	}
	return config
}

// Past the reactors, which may be failing every read of the accounts.
func markedInTracker(t *testing.T, client *dynamicfake.FakeDynamicClient, id string) bool {
	t.Helper()
	obj, err := client.Tracker().Get(gvrOf(v1alpha1.UserMetadata), v1alpha1.UserMetadata.Namespace, id)
	if err != nil {
		t.Fatal(err)
	}
	marked, _, _ := unstructured.NestedBool(obj.(*unstructured.Unstructured).Object, constants.SpecField, constants.FieldBootstrap)
	return marked
}

// A status that can't be read, decoded or written fails the run and opens nothing.
func TestStatusFailuresFailARun(t *testing.T) {
	telarkconfigs := v1alpha1.TelarkConfigMetadata.Plural
	cases := []struct {
		name   string
		config func(t *testing.T) seed
		react  func(t *testing.T, client *dynamicfake.FakeDynamicClient)
	}{
		{"an unreadable config", func(t *testing.T) seed { return configWith(t, pinned()) },
			func(_ *testing.T, client *dynamicfake.FakeDynamicClient) { failing(client, verbGet, telarkconfigs) }},
		{"a garbled status", garbledConfig, func(*testing.T, *dynamicfake.FakeDynamicClient) {}},
		{"a refused write", func(t *testing.T) seed { return configWith(t, pinned()) },
			func(_ *testing.T, client *dynamicfake.FakeDynamicClient) { failing(client, verbPatch, telarkconfigs) }},
		{"a write answered with a garbled status", func(t *testing.T) seed { return configWith(t, pinned()) },
			func(t *testing.T, client *dynamicfake.FakeDynamicClient) {
				answer := garbledConfig(t).obj
				client.PrependReactor(verbPatch, telarkconfigs, func(k8stesting.Action) (bool, runtime.Object, error) {
					return true, answer, nil
				})
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t, append(directory(t), c.config(t), active(pinnedID, pinnedEmail, true).seed(t))...)
			c.react(t, client)
			open(t, pinnedEmail, http.StatusInternalServerError)
		})
	}
}

// A failed config read keeps the default, so a request still reaches the Admins on ALL.
func TestNotifyAdminsDefaultsOnAReadFailure(t *testing.T) {
	client := installFake(t, append(directory(t), alertsOff(t, configWith(t, pinned())))...)
	failing(client, verbGet, v1alpha1.TelarkConfigMetadata.Plural)
	equal(t, "notify admins", bootstrap.NotifyAdmins(), true)
}

// At start, a read the pin depends on that fails is returned.
func TestPinAtStartReturnsReadFailures(t *testing.T) {
	enrolled := func(t *testing.T) []seed {
		return []seed{configWith(t, nil), active(pinnedID, pinnedEmail, true).seed(t), passkey(t, pinnedID, time.Now().Add(-time.Hour))}
	}
	cases := []struct {
		name           string
		seeds          func(t *testing.T) []seed
		verb, resource string
		wantPinned     bool
	}{
		{"the config", enrolled, verbGet, v1alpha1.TelarkConfigMetadata.Plural, false},
		{"the accounts", enrolled, verbList, v1alpha1.UserMetadata.Plural, false},
		{"the passkeys", enrolled, verbList, v1alpha1.PasskeyMetadata.Plural, false},
		{"the accounts once pinned", func(t *testing.T) []seed {
			return []seed{configWith(t, pinned()), active(pinnedID, pinnedEmail, true).seed(t)}
		}, verbList, v1alpha1.UserMetadata.Plural, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t, append(directory(t), c.seeds(t)...)...)
			failing(client, c.verb, c.resource)
			if err := bootstrap.PinEnrolled(newOptimizer(t)); err == nil {
				t.Fatal("PinEnrolled hid a failed read")
			}
			equal(t, "pinned", storedStatus(t, client).Pinned(), c.wantPinned)
		})
	}
}

// A first passkey's pin that fails can't fail the passkey, saved by then: it is logged, and break-glass waits for the
// next start. The markers after a pin that landed are only logged too.
func TestFirstPasskeyPinFailuresAreLogged(t *testing.T) {
	cases := []struct {
		name, verb, resource string
		wantPinned           bool
	}{
		{"the account read", verbGet, v1alpha1.UserMetadata.Plural, false},
		{"the pin write", verbPatch, v1alpha1.TelarkConfigMetadata.Plural, false},
		{"the markers after the pin", verbList, v1alpha1.UserMetadata.Plural, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t, append(directory(t), configWith(t, nil), active(pinnedID, pinnedEmail, true).seed(t),
				active(otherID, otherEmail, true).seed(t))...)
			failing(client, c.verb, c.resource)
			bootstrap.PinAtFirstPasskey(newOptimizer(t), pinnedID)
			equal(t, "pinned", storedStatus(t, client).Pinned(), c.wantPinned)
			equal(t, "other account marked", markedInTracker(t, client, otherID), true)
		})
	}
}

// After a write that marked an account, a failed passkey lookup still moves the marker; a failed config read moves nothing.
func TestMarkerSetFailuresAreLogged(t *testing.T) {
	cases := []struct {
		name, verb, resource string
		otherKeepsMarker     bool
	}{
		{"the passkey lookup", verbList, v1alpha1.PasskeyMetadata.Plural, false},
		{"the config read", verbGet, v1alpha1.TelarkConfigMetadata.Plural, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t, append(directory(t), configWith(t, nil), active(pinnedID, pinnedEmail, true).seed(t),
				active(otherID, otherEmail, true).seed(t))...)
			failing(client, c.verb, c.resource)
			bootstrap.MarkerSet(newOptimizer(t), pinnedID)
			equal(t, "pinned", storedStatus(t, client).Pinned(), false)
			equal(t, "other account marked", markedInTracker(t, client, otherID), c.otherKeepsMarker)
		})
	}
}

// A directory that can't be listed when the notices go out costs the Admins theirs, never the request.
func TestANoticeWithoutTheAdminsStillAnswers(t *testing.T) {
	client := installFake(t, append(directory(t), configWith(t, pinned(pending(time.Now()))), active(pinnedID, pinnedEmail, true).seed(t))...)
	failing(client, verbList, v1alpha1.UserMetadata.Plural)
	act(t, requestID, cancel, signedInBody, http.StatusOK)
	equal(t, "request", storedStatus(t, client).Newest().State, telarkconfig.BootstrapStateCanceled)
}
