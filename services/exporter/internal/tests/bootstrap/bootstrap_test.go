package bootstrap

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/internal/rest/base"
	bootstrapeps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/internal/rest/router"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/bootstrap"
	"github.com/telark/telark/services/exporter/internal/constants"
	envmanager "github.com/telark/telark/services/exporter/internal/managers/envs"
	"github.com/telark/telark/services/exporter/internal/routes"
	notiftypes "github.com/telark/telark/services/exporter/internal/types/notifications"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

const (
	requestID       = "0123456789abcdef"
	unknownRequest  = "ffffffffffffffff"
	requestIDLength = 16
	approve         = "approve"
	cancel          = "cancel"
	complete        = "complete"
	completeOnce    = `{"revoke":false}`
	completeRevoke  = `{"revoke":true}`
	signedInBody    = `{"reason":"signed-in"}`
	deniedBody      = `{"reason":"denied"}`
	expiredBody     = `{"reason":"expired"}`
	markBody        = `{"bootstrap":true}`
	unmarkBody      = `{"bootstrap":false}`
	bootstrapEnv    = "BOOTSTRAP_ADMIN"
	justUnder       = 71 * time.Hour
	justOver        = 73 * time.Hour
	longAgo         = 100 * time.Hour
	runs            = 2
	none            = 0
	one             = 1
	two             = 2
)

func adminSession(id string) xauthz.Identity {
	return xauthz.Identity{UserID: id, Grants: xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeAll: roledata.PermissionLevelAdmin},
	}}
}

func pending(opened time.Time) telarkconfig.BootstrapRequest {
	return recovery(telarkconfig.BootstrapStatePending, opened)
}

func approved(opened time.Time) telarkconfig.BootstrapRequest {
	r := recovery(telarkconfig.BootstrapStateApproved, opened)
	r.ApprovedAt = stamp(opened.Add(time.Hour))
	return r
}

func signedInAt(at time.Time) *string {
	stamped := stamp(at)
	return &stamped
}

// A directory whose bootstrap account is pinned, with these requests.
func pinnedDirectory(t *testing.T, requests ...telarkconfig.BootstrapRequest) {
	t.Helper()
	installFake(t, append(directory(t), configWith(t, pinned(requests...)), active(pinnedID, pinnedEmail, true).seed(t))...)
}

// One bootstrap account. Once one is pinned, no caller, the service included, marks another account or unmarks the
// pinned one, and a session still may not touch the field. Before the pin the marker moves to the account just
// marked, so a second break-glass run fixes a mistyped email.
func TestMarkerFollowsThePin(t *testing.T) {
	cases := []struct {
		name     string
		status   *telarkconfig.BootstrapStatus
		identity xauthz.Identity
		target   string
		body     string
		want     int
	}{
		{"service marks another account", pinned(), internal, otherID, markBody, http.StatusConflict},
		{"service unmarks the pinned account", pinned(), internal, pinnedID, unmarkBody, http.StatusConflict},
		{"service repeats the pinned marker", pinned(), internal, pinnedID, markBody, http.StatusOK},
		{"session marks another account", pinned(), adminSession(adminID), otherID, markBody, http.StatusForbidden},
		{"service marks before any pin", nil, internal, otherID, markBody, http.StatusOK},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t, append(directory(t), configWith(t, c.status),
				active(pinnedID, pinnedEmail, true, adminRoleID).seed(t), active(otherID, otherEmail, false).seed(t))...)
			if w := serveAs(t, c.identity, http.MethodPatch, usersPath+pathSep+c.target, c.body); w.Code != c.want {
				t.Fatalf("patch = %d %s, want %d", w.Code, w.Body.String(), c.want)
			}
			written := c.want == http.StatusOK
			equal(t, "pinned account marked", storedUser(t, client, pinnedID).Bootstrap, !written || c.target == pinnedID)
			equal(t, "other account marked", storedUser(t, client, otherID).Bootstrap, written && c.target == otherID)
		})
	}
	t.Run("service creates a marked account while pinned", func(t *testing.T) {
		pinnedDirectory(t)
		body := `{"username":"new","email":"new@example.com","creationDate":"2026-10-16T00:00:00Z","bootstrap":true,"status":{"phase":"active"}}`
		if w := serveAs(t, internal, http.MethodPost, usersPath, body); w.Code != http.StatusConflict {
			t.Fatalf("create = %d %s, want 409", w.Code, w.Body.String())
		}
	})
}

func equal[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

// Before the pin, marking an account that already has a passkey pins it at once: that passkey was its first.
func TestMarkingAnEnrolledAccountPinsIt(t *testing.T) {
	client := installFake(t, append(directory(t), configWith(t, nil), active(pinnedID, pinnedEmail, true).seed(t),
		active(otherID, otherEmail, false).seed(t), passkey(t, otherID, time.Now().Add(-time.Hour)))...)
	if w := serveAs(t, internal, http.MethodPatch, usersPath+pathSep+otherID, markBody); w.Code != http.StatusOK {
		t.Fatalf("patch = %d %s, want 200", w.Code, w.Body.String())
	}
	status := storedStatus(t, client)
	if !status.Pinned() || status.UserID != otherID {
		t.Fatalf("status = %+v, want pinned to %s", status, otherID)
	}
	equal(t, "previous marker", storedUser(t, client, pinnedID).Bootstrap, false)
}

// An older telark-crds drops status.bootstrap and still answers 200: a write that didn't keep it fails, so neither a
// request nor a pin is reported that isn't there.
func TestAPrunedStatusFails(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		client := installFake(t, append(directory(t), configWith(t, pinned()), active(pinnedID, pinnedEmail, true).seed(t))...)
		pruneBootstrap(t, client)
		open(t, pinnedEmail, http.StatusInternalServerError)
	})
	t.Run("pin", func(t *testing.T) {
		client := installFake(t, append(directory(t), configWith(t, nil), active(pinnedID, pinnedEmail, true).seed(t),
			passkey(t, pinnedID, time.Now().Add(-time.Hour)))...)
		pruneBootstrap(t, client)
		if err := bootstrap.PinEnrolled(newOptimizer(t)); err == nil {
			t.Fatal("PinEnrolled reported a pin the CR dropped")
		}
		equal(t, "pinned", storedStatus(t, client).Pinned(), false)
	})
}

// The first passkey of a bootstrap account pins it and takes the marker off every other account; a second passkey
// leaves the pin alone, and so does a passkey of an account without the marker.
func TestFirstPasskeyPins(t *testing.T) {
	client := installFake(t, append(directory(t), configWith(t, nil),
		active(pinnedID, pinnedEmail, true, adminRoleID).seed(t), active(otherID, otherEmail, true).seed(t))...)

	addPasskey(t, adminID)
	equal(t, "pinned by an unmarked account", storedStatus(t, client).Pinned(), false)
	addPasskey(t, pinnedID)
	first := storedStatus(t, client)
	if first.UserID != pinnedID || first.Email != pinnedEmail || !first.Pinned() {
		t.Fatalf("status after the first passkey = %+v, want pinned to %s", first, pinnedID)
	}
	equal(t, "other account marked", storedUser(t, client, otherID).Bootstrap, false)
	addPasskey(t, otherID)
	addPasskey(t, pinnedID)
	if again := storedStatus(t, client); again.UserID != pinnedID || again.PinnedAt != first.PinnedAt {
		t.Errorf("status after more passkeys = %+v, want the first pin %+v", again, first)
	}
}

// A reset removes the pin and keeps the requests: one it left open ends when the next first passkey pins again.
func TestNewPinEndsARequestLeftOpen(t *testing.T) {
	reset := &telarkconfig.BootstrapStatus{UserID: pinnedID, Email: pinnedEmail, Requests: []telarkconfig.BootstrapRequest{
		recovery(telarkconfig.BootstrapStatePending, time.Now().Add(-justOver)),
	}}
	client := installFake(t, append(directory(t), configWith(t, reset), active(pinnedID, pinnedEmail, true).seed(t))...)
	addPasskey(t, pinnedID)
	status := storedStatus(t, client)
	if newest := status.Newest(); !status.Pinned() || newest.State != telarkconfig.BootstrapStateCanceled ||
		newest.Reason != telarkconfig.BootstrapReasonSignedIn {
		t.Fatalf("status after the new pin = %+v, want pinned with the open request canceled", status)
	}
}

// At every start: with no pin, the enrolled bootstrap account is pinned (BOOTSTRAP_ADMIN's first, else the oldest)
// and the other accounts lose the marker but keep their roles; a second marker found next to a pin goes too, as a
// rollback to 0.1.x can leave one. With none enrolled nothing is pinned. Each case runs twice.
func TestPinAtStart(t *testing.T) {
	t.Cleanup(envmanager.InitBootstrapAdmin)
	t.Setenv(bootstrapEnv, otherEmail)
	envmanager.InitBootstrapAdmin()
	older, newer := time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour)
	at := func(a account, created time.Time) account { a.created = created; return a }
	cases := []struct {
		name       string
		seeds      func(t *testing.T) []seed
		wantPinned string
	}{
		{"pinned, a second marker left by a rollback", func(t *testing.T) []seed {
			return []seed{configWith(t, pinned()), active(pinnedID, pinnedEmail, true).seed(t),
				active(otherID, otherEmail, true, adminRoleID).seed(t)}
		}, pinnedID},
		{"BOOTSTRAP_ADMIN's enrolled account first", func(t *testing.T) []seed {
			return []seed{configWith(t, nil), at(active(pinnedID, pinnedEmail, true), older).seed(t),
				at(active(otherID, otherEmail, true, adminRoleID), newer).seed(t),
				passkey(t, pinnedID, older), passkey(t, otherID, newer)}
		}, otherID},
		{"else the oldest enrolled", func(t *testing.T) []seed {
			return []seed{configWith(t, nil), at(active(pinnedID, pinnedEmail, true), older).seed(t),
				at(active(otherID, "else@example.com", true, adminRoleID), newer).seed(t),
				passkey(t, pinnedID, older), passkey(t, otherID, newer)}
		}, pinnedID},
		{"none enrolled", func(t *testing.T) []seed {
			return []seed{configWith(t, nil), active(pinnedID, pinnedEmail, true).seed(t),
				active(otherID, otherEmail, true, adminRoleID).seed(t)}
		}, constants.EmptyString},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t, append(directory(t), c.seeds(t)...)...)
			for range runs {
				if err := bootstrap.PinEnrolled(newOptimizer(t)); err != nil {
					t.Fatalf("PinEnrolled = %v", err)
				}
			}
			status := storedStatus(t, client)
			if status.UserID != c.wantPinned || status.Pinned() != (c.wantPinned != constants.EmptyString) {
				t.Fatalf("pin = %+v, want %q", status, c.wantPinned)
			}
			for _, id := range []string{pinnedID, otherID} {
				equal(t, id+" marked", storedUser(t, client, id).Bootstrap, c.wantPinned == constants.EmptyString || id == c.wantPinned)
			}
			equal(t, "roles kept", len(storedUser(t, client, otherID).RoleRefs), one)
		})
	}
}

// Each notice of a request names it, and only the pinned account and active Admins on ALL get one.
func expectNotices(t *testing.T, opened string, want map[string]int) {
	t.Helper()
	for id, count := range want {
		got := notices(t, id)
		equal(t, id+" notices", len(got), count)
		for _, notice := range got {
			equal(t, "type", notice.Type, notiftypes.TypeBootstrapRequest)
			equal(t, "request", notice.Metadata[notiftypes.MetaKeyRequestID], any(opened))
		}
	}
}

// A run while no account is pinned acts at once, so it opens nothing; once pinned, a run opens one request, a second
// run gets the same one back, and the pinned account and every active Admin on ALL hear of it.
func TestOpenOneRequestAtATime(t *testing.T) {
	withNotices(t)
	installFake(t, append(directory(t), configWith(t, nil), active(pinnedID, pinnedEmail, true).seed(t))...)
	open(t, pinnedEmail, http.StatusConflict)

	pinnedDirectory(t)
	first := open(t, pinnedEmail, http.StatusOK)
	if first.Request.Kind != telarkconfig.BootstrapKindRecover || first.Request.State != telarkconfig.BootstrapStatePending ||
		first.Eligible || len(first.Request.ID) != requestIDLength {
		t.Fatalf("opened %+v, want a pending recovery with a 16-character id", first)
	}
	equal(t, "second run", open(t, pinnedEmail, http.StatusOK).Request.ID, first.Request.ID)
	expectNotices(t, first.Request.ID, map[string]int{pinnedID: one, adminID: one, readerID: none, suspendedID: none})
}

// Of two runs at once, one write wins and the other reads it back: one request, both get its id, one set of notices.
func TestConcurrentRunsOpenOneRequest(t *testing.T) {
	client := installFake(t, append(directory(t), configWith(t, pinned()), active(pinnedID, pinnedEmail, true).seed(t))...)
	// The first write waits until both runs have read, so the second one's compare-and-swap always loses. A reactor
	// never waits: the fake runs it under the lock every call takes, so the other run could not read.
	reads := make(chan struct{}, runs)
	client.PrependReactor("get", v1alpha1.TelarkConfigMetadata.Plural, func(k8stesting.Action) (bool, runtime.Object, error) {
		select {
		case reads <- struct{}{}:
		default: // only the first reads count
		}
		return false, nil, nil
	})
	var hold sync.Once
	var patches atomic.Int32
	k8sclient.SetDynamicClient(configPatchHook{Interface: client, before: func() {
		hold.Do(func() {
			for range runs {
				<-reads
			}
		})
		patches.Add(1)
	}})
	outcomes := make([]*bootstrap.Outcome, runs)
	var wg sync.WaitGroup
	for i := range outcomes {
		wg.Go(func() {
			outcome, err := bootstrap.Open(pinnedEmail)
			if err != nil {
				t.Errorf("open = %v", err)
			}
			outcomes[i] = outcome
		})
	}
	wg.Wait()
	first, second := outcomes[none], outcomes[one]
	if first.Result.Request.ID == constants.EmptyString || first.Result.Request.ID != second.Result.Request.ID {
		t.Fatalf("ids = %s and %s, want the same request twice", first.Result.Request.ID, second.Result.Request.ID)
	}
	equal(t, "changes to report", len(first.Changed)+len(second.Changed), one)
	equal(t, "requests", len(storedStatus(t, client).Requests), one)
	equal(t, "writes, the loser's refused", patches.Load(), int32(runs))
}

// Writers that keep winning the compare-and-swap is an answer, 409, after one patch per try.
func TestContendedStatusIsAConflict(t *testing.T) {
	client := installFake(t, append(directory(t), configWith(t, pinned()), active(pinnedID, pinnedEmail, true).seed(t))...)
	var patches atomic.Int32
	client.PrependReactor(verbPatch, v1alpha1.TelarkConfigMetadata.Plural, func(k8stesting.Action) (bool, runtime.Object, error) {
		patches.Add(1)
		return true, nil, apierrors.NewConflict(configGVR.GroupResource(), constants.TelarkConfigResourceName, errStale)
	})
	open(t, pinnedEmail, http.StatusConflict)
	equal(t, "patches", patches.Load(), int32(constants.BootstrapWriteAttempts))
}

// The recovery empties the cached passkey list at once, so nothing that reads it keeps an old passkey signing in.
func TestRecoveryEmptiesTheCachedPasskeyList(t *testing.T) {
	approved := recovery(telarkconfig.BootstrapStateApproved, time.Now().Add(-time.Hour))
	installFake(t, append(directory(t), configWith(t, pinned(approved)), active(pinnedID, pinnedEmail, true).seed(t),
		passkey(t, pinnedID, time.Now().Add(-time.Hour)))...)
	h := router.NewRouter(routes.InitRoutes(newOptimizer(t), nil))
	send := func(method, path, body string, headers map[string]string) string {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r.WithContext(xauthz.WithIdentity(r.Context(), internal)))
		return w.Body.String()
	}
	listed := func() bool {
		return strings.Contains(send(http.MethodGet, passkeysPath, constants.EmptyString,
			map[string]string{constants.HeaderUserID: pinnedID}), pinnedID+"-passkey-")
	}
	equal(t, "listed before", listed(), true)
	send(http.MethodPost, requestsPath+pathSep+requestID+pathSep+complete, completeOnce, nil)
	equal(t, "listed after the recovery", listed(), false)
}

// A request in flight when the recovery began can save a passkey after the first pass: break-glass's last pass, made
// while the account is frozen, purges it, where an unfrozen re-run still refuses (409) and purges nothing.
func TestFrozenLastPassPurgesAStraggler(t *testing.T) {
	approved := recovery(telarkconfig.BootstrapStateApproved, time.Now().Add(-time.Hour))
	client := installFake(t, append(directory(t), configWith(t, pinned(approved)), active(pinnedID, pinnedEmail, true).seed(t),
		passkey(t, pinnedID, time.Now().Add(-time.Hour)))...)
	act(t, requestID, complete, completeOnce, http.StatusOK)
	create(t, client, passkey(t, pinnedID, time.Now().Add(time.Minute)))
	act(t, requestID, complete, completeOnce, http.StatusConflict)
	equal(t, "straggler kept by a plain re-run", countOf(t, client, v1alpha1.PasskeyMetadata), one)
	done := act(t, requestID, complete, completeRevoke, http.StatusOK)
	equal(t, "straggler after the frozen pass", countOf(t, client, v1alpha1.PasskeyMetadata), none)
	equal(t, "enrolled", done.Enrolled, false)
}

// At start, the account that loses its marker reads unmarked at once: the Redis response cache outlives a restart.
func TestPinAtStartRefreshesTheCachedUsers(t *testing.T) {
	installFake(t, append(directory(t), configWith(t, nil), active(pinnedID, pinnedEmail, true).seed(t),
		passkey(t, pinnedID, time.Now().Add(-time.Hour)), active(otherID, otherEmail, true, adminRoleID).seed(t))...)
	optimizer := newOptimizer(t)
	h := router.NewRouter(routes.InitRoutes(optimizer, nil))
	marked := func() bool {
		r := httptest.NewRequest(http.MethodGet, usersPath+pathSep+otherID, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r.WithContext(xauthz.WithIdentity(r.Context(), adminSession(adminID))))
		return strings.Contains(w.Body.String(), `"bootstrap":true`)
	}
	equal(t, "marked before", marked(), true)
	if err := bootstrap.PinEnrolled(optimizer); err != nil {
		t.Fatal(err)
	}
	equal(t, "marked after the start", marked(), false)
}

// Eligible: approved before the 72-hour mark, or after it with no sign-in by the bootstrap account since the request
// opened; a sign-in after the mark expires it. A completed recovery leaves no passkey, session or passkey sign-in.
func TestSeventyTwoHours(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name      string
		request   telarkconfig.BootstrapRequest
		lastLogin *string
		want      int
	}{
		{"pending at 71 hours", pending(now.Add(-justUnder)), nil, http.StatusConflict},
		{"pending at 73 hours, no sign-in", pending(now.Add(-justOver)), nil, http.StatusOK},
		{"pending at 73 hours, signed in before", pending(now.Add(-justOver)), signedInAt(now.Add(-justOver - time.Hour)), http.StatusOK},
		{"pending at 73 hours, signed in since", pending(now.Add(-justOver)), signedInAt(now.Add(-time.Hour)), http.StatusConflict},
		{"approved at 71 hours", approved(now.Add(-justUnder)), signedInAt(now.Add(-time.Hour)), http.StatusOK},
		{"approved at 73 hours, signed in since", approved(now.Add(-justOver)), signedInAt(now.Add(-time.Hour)), http.StatusConflict},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			owner := active(pinnedID, pinnedEmail, true)
			owner.lastLogin = c.lastLogin
			client := installFake(t, append(directory(t), configWith(t, pinned(c.request)), owner.seed(t),
				passkey(t, pinnedID, now.Add(-longAgo)), session(t, pinnedID))...)
			act(t, requestID, complete, completeOnce, c.want)
			completed := c.want == http.StatusOK
			equal(t, "completed", storedStatus(t, client).Requests[0].State == telarkconfig.BootstrapStateCompleted, completed)
			if completed {
				equal(t, "passkeys", countOf(t, client, v1alpha1.PasskeyMetadata), none)
				equal(t, "sessions", countOf(t, client, v1alpha1.SessionMetadata), none)
				equal(t, "passkey sign-ins", len(storedUser(t, client, pinnedID).Identities), none)
			}
		})
	}
}

// After the mark, a request the bootstrap account signed in during ends as expired, and the next run opens another.
func TestSignInAfterTheMarkExpiresTheRequest(t *testing.T) {
	withNotices(t)
	owner := active(pinnedID, pinnedEmail, true)
	owner.lastLogin = signedInAt(time.Now().Add(-time.Hour))
	client := installFake(t, append(directory(t), configWith(t, pinned(pending(time.Now().Add(-justOver)))), owner.seed(t))...)
	opened := open(t, pinnedEmail, http.StatusOK)
	requests := storedStatus(t, client).Requests
	if opened.Request.ID == requestID || len(requests) != two || requests[one].State != telarkconfig.BootstrapStateCanceled ||
		requests[one].Reason != telarkconfig.BootstrapReasonExpired {
		t.Fatalf("requests = %+v, want a new one over the expired one", requests)
	}
	equal(t, "notices", len(notices(t, pinnedID)), two)
}

// A sign-in cancels a pending request only; the bootstrap account's denial is the one way to take back an approval.
func TestCancelRules(t *testing.T) {
	t.Run("an approval survives a sign-in and can still be denied", func(t *testing.T) {
		pinnedDirectory(t, pending(time.Now().Add(-time.Hour)))
		act(t, requestID, approve, constants.EmptyString, http.StatusOK)
		act(t, requestID, cancel, signedInBody, http.StatusConflict)
		act(t, requestID, approve, constants.EmptyString, http.StatusConflict)
		if r := act(t, requestID, cancel, deniedBody, http.StatusOK).Request; r.Reason != telarkconfig.BootstrapReasonDenied {
			t.Fatalf("request = %+v, want denied", r)
		}
	})
	t.Run("a sign-in cancels a pending request", func(t *testing.T) {
		pinnedDirectory(t, pending(time.Now().Add(-time.Hour)))
		act(t, unknownRequest, approve, constants.EmptyString, http.StatusNotFound)
		act(t, requestID, cancel, expiredBody, http.StatusBadRequest)
		r := act(t, requestID, cancel, signedInBody, http.StatusOK).Request
		if r.State != telarkconfig.BootstrapStateCanceled || r.Reason != telarkconfig.BootstrapReasonSignedIn {
			t.Fatalf("request = %+v, want canceled by the sign-in", r)
		}
	})
}

// A completed recovery run again re-issues while no passkey was enrolled since, purging again; once one was, the
// next run opens a new request.
func TestCompletedRecoveryReissues(t *testing.T) {
	client := installFake(t, append(directory(t), configWith(t, pinned()),
		active(pinnedID, pinnedEmail, true).seed(t), passkey(t, pinnedID, time.Now().Add(-time.Hour)))...)
	first := open(t, pinnedEmail, http.StatusOK)
	act(t, first.Request.ID, approve, constants.EmptyString, http.StatusOK)
	done := act(t, first.Request.ID, complete, completeOnce, http.StatusOK)
	if done.Request.State != telarkconfig.BootstrapStateCompleted || done.Enrolled {
		t.Fatalf("complete = %+v, want completed with nothing enrolled", done)
	}
	reissue := open(t, pinnedEmail, http.StatusOK)
	if reissue.Request.ID != first.Request.ID || !reissue.Eligible || reissue.Enrolled {
		t.Fatalf("run after completion = %+v, want the completed request to re-issue", reissue)
	}
	act(t, first.Request.ID, complete, completeOnce, http.StatusOK)

	create(t, client, passkey(t, pinnedID, time.Now().Add(time.Minute)))
	act(t, first.Request.ID, complete, completeOnce, http.StatusConflict)
	if next := open(t, pinnedEmail, http.StatusOK); next.Request.ID == first.Request.ID || next.Request.State != telarkconfig.BootstrapStatePending {
		t.Fatalf("run after a new passkey = %+v, want a new pending request", next)
	}
}

// A completed recovery re-issues for 24 hours after it completed, each time with a notice; after that a run opens a
// new request, and the old one completes no more.
func TestReissueEndsAfterADay(t *testing.T) {
	withNotices(t)
	completed := recovery(telarkconfig.BootstrapStateCompleted, time.Now().Add(-3*24*time.Hour))
	completed.CompletedAt = stamp(time.Now().Add(-23 * time.Hour))
	client := installFake(t, append(directory(t), configWith(t, pinned(completed)), active(pinnedID, pinnedEmail, true).seed(t))...)
	if again := open(t, pinnedEmail, http.StatusOK); again.Request.ID != requestID || !again.Eligible {
		t.Fatalf("run 23 hours after completion = %+v, want a re-issue", again)
	}
	reissued := notices(t, pinnedID)
	if len(reissued) != one || !slices.ContainsFunc(reissued, func(n notiftypes.Notification) bool {
		return n.Title == "Break-glass link issued again"
	}) {
		t.Fatalf("notices = %+v, want one re-issue notice", reissued)
	}

	late := recovery(telarkconfig.BootstrapStateCompleted, time.Now().Add(-3*24*time.Hour))
	late.CompletedAt = stamp(time.Now().Add(-25 * time.Hour))
	setStatus(t, client, pinned(late))
	act(t, requestID, complete, completeOnce, http.StatusConflict)
	if next := open(t, pinnedEmail, http.StatusOK); next.Request.ID == requestID || next.Request.State != telarkconfig.BootstrapStatePending {
		t.Fatalf("run 25 hours after completion = %+v, want a new pending request", next)
	}
}

// A completed recovery waiting for its account's new passkey re-issues for that account only: a run for another
// email opens a replacement.
func TestCompletedRecoveryLeavesRoomForAReplacement(t *testing.T) {
	completed := recovery(telarkconfig.BootstrapStateCompleted, time.Now().Add(-justOver))
	completed.CompletedAt = stamp(time.Now().Add(-time.Hour))
	pinnedDirectory(t, completed)
	opened := open(t, otherEmail, http.StatusOK)
	if opened.Request.ID == requestID || opened.Request.Kind != telarkconfig.BootstrapKindReplace {
		t.Fatalf("run for another email = %+v, want a new replacement", opened.Request)
	}
}

// The pinned account is found by its current email, not the one it was pinned with: after it changed its email, a
// run with the new one re-issues its completed recovery.
func TestRecoveryFollowsTheAccountsEmail(t *testing.T) {
	const renamed = "renamed@example.com"
	completed := recovery(telarkconfig.BootstrapStateCompleted, time.Now().Add(-justOver))
	completed.CompletedAt = stamp(time.Now().Add(-time.Hour))
	installFake(t, append(directory(t), configWith(t, pinned(completed)), active(pinnedID, renamed, true).seed(t))...)
	if reissue := open(t, renamed, http.StatusOK); reissue.Request.ID != requestID || !reissue.Eligible {
		t.Fatalf("run with the account's new email = %+v, want its completed recovery to re-issue", reissue)
	}
}

// A replacement moves the marker and the pin to the account the email names; the previous account keeps its roles,
// and neither is revoked.
func TestReplacementMovesThePin(t *testing.T) {
	withNotices(t)
	client := installFake(t, append(directory(t), configWith(t, pinned()),
		active(pinnedID, pinnedEmail, true, adminRoleID).seed(t), passkey(t, pinnedID, time.Now().Add(-time.Hour)))...)
	opened := open(t, otherEmail, http.StatusOK)
	if opened.Request.Kind != telarkconfig.BootstrapKindReplace || opened.Request.TargetEmail != otherEmail {
		t.Fatalf("opened %+v, want a replacement naming %s", opened.Request, otherEmail)
	}
	act(t, opened.Request.ID, approve, constants.EmptyString, http.StatusOK)
	act(t, opened.Request.ID, complete, completeOnce, http.StatusConflict)

	create(t, client, active(otherID, otherEmail, false, adminRoleID).seed(t))
	done := act(t, opened.Request.ID, complete, completeOnce, http.StatusOK)
	status := storedStatus(t, client)
	if status.UserID != otherID || status.Email != otherEmail || done.Enrolled {
		t.Fatalf("pin = %+v, enrolled %v, want %s with nothing enrolled", status, done.Enrolled, otherID)
	}
	previous, next := storedUser(t, client, pinnedID), storedUser(t, client, otherID)
	equal(t, "previous account marked", previous.Bootstrap, false)
	equal(t, "new account marked", next.Bootstrap, true)
	equal(t, "previous account roles", len(previous.RoleRefs), one)
	equal(t, "previous account passkeys", countOf(t, client, v1alpha1.PasskeyMetadata), one)
	// The new account didn't exist when the request opened, and the opened notice names it.
	previousNotices := notices(t, pinnedID)
	equal(t, "previous account notices", len(previousNotices), two)
	if !slices.ContainsFunc(previousNotices, func(n notiftypes.Notification) bool {
		return n.Metadata[notiftypes.MetaKeyState] == telarkconfig.BootstrapStatePending && strings.Contains(n.Message, otherEmail)
	}) {
		t.Fatalf("notices %+v, want the opened one to name %s", previousNotices, otherEmail)
	}
	equal(t, "new account notices", len(notices(t, otherID)), one)
	// Every other Admin reads who may become the bootstrap account masked.
	for _, notice := range notices(t, adminID) {
		equal(t, "admin notice names the account", strings.Contains(notice.Message, otherEmail), false)
	}
}

// Only the pinned account reads its own email and a replacement's target; another Admin reads them blank.
func TestStatusMasksTheBootstrapIdentity(t *testing.T) {
	replace := pending(time.Now())
	replace.Kind, replace.TargetEmail = telarkconfig.BootstrapKindReplace, otherEmail
	installFake(t, append(directory(t), configWith(t, pinned(replace)), active(pinnedID, pinnedEmail, true, adminRoleID).seed(t))...)
	for id, visible := range map[string]bool{pinnedID: true, adminID: false} {
		body := serveAs(t, adminSession(id), http.MethodGet, basePath, constants.EmptyString).Body.String()
		equal(t, id+" reads the pinned email", strings.Contains(body, pinnedEmail), visible)
		equal(t, id+" reads the target email", strings.Contains(body, otherEmail), visible)
	}
}

// The bootstrap account's switch: off, a request's notices and record are its own and the services', and a config
// stored before the switch existed reads it on.
func TestBreakGlassAlertsSwitch(t *testing.T) {
	for _, off := range []bool{false, true} {
		t.Run(map[bool]string{false: "unset", true: "off"}[off], func(t *testing.T) {
			withNotices(t)
			config := configWith(t, pinned())
			if off {
				config = alertsOff(t, config)
			}
			installFake(t, append(directory(t), config, active(pinnedID, pinnedEmail, true, adminRoleID).seed(t))...)
			opened := open(t, pinnedEmail, http.StatusOK)
			adminNotices := map[bool]int{false: one, true: none}[off]
			expectNotices(t, opened.Request.ID, map[string]int{pinnedID: one, adminID: adminNotices})
			for name, identity := range map[string]xauthz.Identity{pinnedID: adminSession(pinnedID), adminID: adminSession(adminID),
				"service": internal} {
				view := statusAs(t, identity)
				equal(t, name+" notifyAdmins", view.NotifyAdmins, !off)
				equal(t, name+" requests", len(view.Requests), map[bool]int{true: none, false: one}[off && name == adminID])
			}
		})
	}
}

// The record lives in TelarkConfig, not in the notices its owner can clear, and the config route writes no part of it.
func TestRecordOutlivesNotices(t *testing.T) {
	withNotices(t)
	installFake(t, append(directory(t), configWith(t, pinned()), active(pinnedID, pinnedEmail, true, adminRoleID).seed(t))...)
	opened := open(t, pinnedEmail, http.StatusOK)
	notices(t, pinnedID)
	cleared := serveAs(t, adminSession(pinnedID), http.MethodDelete, noticesPath+"?userId="+pinnedID, constants.EmptyString)
	equal(t, "clear notices", cleared.Code, http.StatusOK)
	equal(t, "notices after clearing", len(notices(t, pinnedID)), none)
	w := serveAs(t, adminSession(adminID), http.MethodGet, basePath, constants.EmptyString)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), opened.Request.ID) {
		t.Fatalf("GET bootstrap = %d %s, want the request listed", w.Code, w.Body.String())
	}
	forged := `{"bootstrap":{"pinnedAt":null}}`
	for name, identity := range map[string]xauthz.Identity{"service": internal, "session": adminSession(adminID)} {
		equal(t, name+" config patch", serveAs(t, identity, http.MethodPatch, configPath, forged).Code, http.StatusBadRequest)
	}
}

// Admins on ALL read the record; only peers move a request, so a session, the bootstrap account's included, approves
// only through auth's passkey ceremony.
func TestRequestRoutesAreInternal(t *testing.T) {
	requirements := authz.Requirements()
	equal(t, "read", requirements[router.Key(base.Get, bootstrapeps.GetBootstrap)], xauthz.Administer(roledata.ScopeAll))
	for _, endpoint := range []base.Endpoint{bootstrapeps.OpenRequest, bootstrapeps.ApproveRequest,
		bootstrapeps.CancelRequest, bootstrapeps.CompleteRequest} {
		equal(t, string(endpoint), requirements[router.Key(base.Post, endpoint)].Access, xauthz.AccessInternal)
	}
}
