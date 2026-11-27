package authrequest

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	authdata "github.com/telark/telark/internal/data/auth"
	requestdata "github.com/telark/telark/internal/data/resources/authrequest"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	bootstrapeps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	frozenKey          = constants.RedisKeyPrefixFrozen + pinnedUser
	undatedCredential  = "dW5kYXRlZA"
	openRegistration   = "aW4tZmxpZ2h0LXJlZ2lzdHJhdGlvbg"
	openSignIn         = "aW4tZmxpZ2h0LXNpZ24taW4"
	unreadableOpenedAt = "never"
	// The completion that revokes, after the hold.
	revokePass = 1
	// A late registration lands this long after the run froze the account for its last look.
	lateLanding = time.Second
)

// An eligible recovery of the pinned account, open until a test ends it.
func recoveryExporter(complete *bootstrapeps.RequestResult) *testutil.FakeExporter {
	fake := pinnedExporter()
	request := bootstrapRequest(telarkconfig.BootstrapKindRecover, telarkconfig.BootstrapStatePending, constants.EmptyString)
	fake.Bootstrap.Requests = []telarkconfig.BootstrapRequest{request}
	fake.RequestAnswers = map[string]bootstrapeps.RequestResult{openAnswer: {Request: request, Eligible: true}}
	if complete != nil {
		fake.RequestAnswers[completeKey] = *complete
	}
	return fake
}

// An eligible recovery of an account that signs with one passkey made long before the request opened, and one stored
// without its creation time, which is never the link's.
func signingRecovery() *testutil.FakeExporter {
	fake := recoveryExporter(&bootstrapeps.RequestResult{
		Request:  bootstrapRequest(telarkconfig.BootstrapKindRecover, telarkconfig.BootstrapStatePending, constants.EmptyString),
		Eligible: true,
	})
	fake.Credentials = map[string][]authdata.Passkey{
		pinnedUser: append(oldPasskey(), authdata.Passkey{UserID: pinnedUser, CredentialID: undatedCredential}),
	}
	return fake
}

func watchFreeze(fake *testutil.FakeExporter, onComplete func()) *completions {
	seen := &completions{}
	fake.OnRequest = func(path string) {
		if strings.HasSuffix(path, completeKey) {
			seen.frozen = append(seen.frozen, redisServer.Exists(frozenKey))
			seen.at = append(seen.at, time.Now())
			if onComplete != nil {
				onComplete()
			}
		}
	}
	return seen
}

// The pinned account's ways in before a recovery: an earlier request's invite, a link its session minted, and a
// registration and a sign-in that session started, still open. Returns the links' tokens.
func openWaysIn(t *testing.T) []string {
	t.Helper()
	invite, _, err := authhelper.IssueInvite(pinnedUser, constants.EmptyString, linkTTL)
	if err != nil {
		t.Fatal(err)
	}
	ownLink, _, err := authhelper.CreateEnrollToken(pinnedUser)
	if err != nil {
		t.Fatal(err)
	}
	for _, challenge := range []string{openRegistration, openSignIn} {
		if err := webauthnhelper.StoreChallenge(pinnedUser, challenge, constants.EmptyString); err != nil {
			t.Fatal(err)
		}
	}
	return []string{invite, ownLink}
}

func ceremoniesOpen() bool {
	return slices.ContainsFunc(redisServer.Keys(), func(key string) bool {
		return strings.HasPrefix(key, constants.RedisKeyPrefixChallenge+pinnedUser+constants.ColonSeparator)
	})
}

// As the cancel of a sign-in begun before the freeze does when it lands mid-run.
func endTheRequest(fake *testutil.FakeExporter) {
	status := pinnedExporter().Bootstrap
	status.Requests = []telarkconfig.BootstrapRequest{
		bootstrapRequest(telarkconfig.BootstrapKindRecover, telarkconfig.BootstrapStateCanceled, constants.EmptyString),
	}
	fake.SetBootstrap(status)
}

func (i *intercept) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet || r.URL.Path != i.path || i.left.Load() <= constants.DefaultInitValue {
		i.next.ServeHTTP(w, r)
		return
	}
	i.left.Add(-constants.DefaultIncrementValue)
	if i.then != nil {
		i.then()
	}
	if i.pass {
		i.next.ServeHTTP(w, r)
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
}

// Each run stops with the reason and no link, and releases the run lock unless another run took that over.
func expectStops(t *testing.T, cases []stopCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				fake := recoveryExporter(nil)
				b := newBench(t, c.email, fake)
				b.backend = c.backend(fake)
				last := b.run(t)
				b.expect(t, firstRequest, c.want)
				testutil.Equal(t, "sealed link", last.SealedLink, constants.EmptyString)
				testutil.Equal(t, callsLabel, strings.Join(fake.RequestCalls(), listSep), strings.Join(c.calls, listSep))
				frozen, _ := redisServer.Get(frozenKey)
				testutil.Equal(t, "freeze", frozen, c.frozen)
				held, _ := redisServer.Get(constants.RedisKeyAuthRequestRun)
				testutil.Equal(t, lockLabel, held, c.lock)
			})
		})
	}
}

// A recovery that can't go on thaws at once if it froze the account: nothing was revoked. A lapsed freeze stops it
// too (a gap could have let a write through), and a run that lost its lock leaves the freeze to the next run.
func TestRecoveryStopsBeforeTheLink(t *testing.T) {
	asIs := func(fake *testutil.FakeExporter) http.Handler { return fake }
	calls, none := []string{opened(adminMailbox)}, constants.EmptyString
	failed, begun := []string{requestdata.PhaseFailed}, []string{requestdata.PhaseRecovering, requestdata.PhaseFailed}
	runFailed := string(constants.ErrAuthRequestRunFailed)
	refused := ending{begun, fmt.Sprintf(string(constants.ErrAuthRequestCompleteFailed), bootstrapID, anyText)}
	expectStops(t, []stopCase{
		{"the open fails", adminMailbox, func(fake *testutil.FakeExporter) http.Handler {
			delete(fake.RequestAnswers, openAnswer)
			return fake
		}, ending{failed, fmt.Sprintf(string(constants.ErrAuthRequestOpenFailed), anyText)}, calls, none, none},
		{"the requester's lookup fails", adminMailbox, func(fake *testutil.FakeExporter) http.Handler {
			return refuse(fake, http.MethodGet, byEmailPath, http.StatusInternalServerError)
		}, ending{failed, fmt.Sprintf(string(constants.ErrAuthRequestLookupFailed), adminMailbox, anyText)}, calls, none, none},
		{"no account uses the requester's email", adminMailbox, func(fake *testutil.FakeExporter) http.Handler {
			delete(fake.Users, pinnedUser)
			return fake
		}, ending{[]string{requestdata.PhaseDenied}, fmt.Sprintf(constants.AuthRequestOtherRequest, bootstrapID,
			fmt.Sprintf(constants.AuthRequestKindRecover, adminMailbox))}, calls, none, none},
		{"the request's opening time is unreadable", adminMailbox, func(fake *testutil.FakeExporter) http.Handler {
			request := bootstrapRequest(telarkconfig.BootstrapKindRecover, telarkconfig.BootstrapStatePending, constants.EmptyString)
			request.OpenedAt = unreadableOpenedAt
			fake.RequestAnswers[openAnswer] = bootstrapeps.RequestResult{Request: request}
			return fake
		}, ending{failed, fmt.Sprintf(runFailed, anyText)}, calls, none, none},
		{"another run takes the lock over before the freeze", adminMailbox, func(fake *testutil.FakeExporter) http.Handler {
			fake.OnRequest = func(string) { _ = redisServer.Set(constants.RedisKeyAuthRequestRun, takenOver) }
			return fake
		}, ending{begun, fmt.Sprintf(runFailed, constants.ErrAuthRequestRunLost)}, calls, takenOver, none},
		// A key of the wrong type makes Redis refuse the drop.
		{"the links can't be dropped", adminMailbox, func(fake *testutil.FakeExporter) http.Handler {
			redisServer.HSet(constants.RedisKeyPrefixInviteOf+pinnedUser, linkCredential, linkCredential)
			return fake
		}, ending{begun, fmt.Sprintf(runFailed, anyText)}, calls, none, none},
		{"the first completion is refused", adminMailbox, asIs, refused, append(slices.Clone(calls), completeOnce), none, none},
		{"the freeze lapses before the hold", adminMailbox, func(fake *testutil.FakeExporter) http.Handler {
			fake.RequestAnswers[completeKey] = fake.RequestAnswers[openAnswer]
			watchFreeze(fake, func() { redisServer.Del(frozenKey) })
			return fake
		}, ending{begun, fmt.Sprintf(runFailed, constants.ErrRecoveryFreezeLost)}, append(slices.Clone(calls), completeOnce), none, none},
		{"the freeze lapses before the link", adminMailbox, func(fake *testutil.FakeExporter) http.Handler {
			fake.RequestAnswers[completeKey] = fake.RequestAnswers[openAnswer]
			seen := &completions{}
			seen = watchFreeze(fake, func() {
				if len(seen.at) > revokePass {
					redisServer.Del(frozenKey)
				}
			})
			return fake
		}, ending{begun, fmt.Sprintf(runFailed, constants.ErrRecoveryFreezeLost)},
			append(slices.Clone(calls), completeOnce, completeRevoke), none, none},
		{"the lock is lost before the thaw", adminMailbox, func(fake *testutil.FakeExporter) http.Handler {
			watchFreeze(fake, func() {
				_ = redisServer.Set(constants.RedisKeyAuthRequestRun, takenOver)
				_ = redisServer.Set(frozenKey, takenOver)
			})
			return fake
		}, refused, append(slices.Clone(calls), completeOnce), takenOver, takenOver},
	})
}

// Earlier links and open ceremonies gone, frozen at each completion, the revoking pass only once a write sent before
// the freeze has landed, thawed at the end, and the run lock released.
func expectRevoked(t *testing.T, links []string, seen *completions) {
	t.Helper()
	for _, token := range links {
		if _, _, err := authhelper.ResolveEnrollToken(token); err == nil {
			t.Error("an earlier link still resolves")
		}
	}
	testutil.Equal(t, "open ceremonies", ceremoniesOpen(), false)
	testutil.Equal(t, "frozen at each completion", slices.Contains(seen.frozen, false), false)
	testutil.Equal(t, "held", seen.at[revokePass].Sub(seen.at[constants.DefaultInitValue]) >= constants.RecoveryHold, true)
	testutil.Equal(t, thawedLabel, redisServer.Exists(frozenKey), false)
	testutil.Equal(t, lockLabel, redisServer.Exists(constants.RedisKeyAuthRequestRun), false)
}

// Link first, wipe after: the run revokes the account's ways in, then its freeze lets only the sealed link's
// registration through; once that passkey shows, the exporter keeps it alone and the account thaws.
func TestRecoversLinkFirst(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := signingRecovery()
		seen := watchFreeze(fake, nil)
		b := newBench(t, adminMailbox, fake)
		links := openWaysIn(t)
		b.begin(t)

		recovering := b.await(t, firstRequest, requestdata.PhaseRecovering)
		testutil.Equal(t, "recovering reason", recovering.Reason, fmt.Sprintf(constants.AuthRequestRecovering, adminMailbox))
		ready := b.await(t, firstRequest, requestdata.PhaseLinkReady)
		link, token := b.openLink(t, ready)
		testutil.Equal(t, "link", link, wantLink(constants.AuthRequestDashboardURL, token, adminMailbox))
		_, digest, err := authhelper.ResolveEnrollToken(token)
		if err != nil {
			t.Fatal(err)
		}
		frozen, _ := redisServer.Get(frozenKey)
		testutil.Equal(t, "freeze holds the link", frozen, digest)
		testutil.Equal(t, "link window", redisServer.TTL(frozenKey), constants.RecoveryLinkWindow)
		testutil.Equal(t, "other ceremonies refused meanwhile", authhelper.RefuseFrozen(pinnedUser, constants.EmptyString) != nil, true)
		testutil.Equal(t, "link expiry", ready.LinkExpiresAt, fake.User(pinnedUser).Status.Invite.ExpiresAt)
		register(t, pinnedUser, token)

		b.end(t, firstRequest)
		b.expect(t, firstRequest, ending{
			[]string{requestdata.PhaseRecovering, requestdata.PhaseLinkReady, requestdata.PhaseCompleted},
			fmt.Sprintf(constants.AuthRequestRecovered, bootstrapID, adminMailbox),
		})
		for _, status := range b.writes(firstRequest) {
			testutil.Equal(t, requestLabel, status.RequestID, bootstrapID)
		}
		keep := fmt.Sprintf(completeKeep, base64.RawURLEncoding.EncodeToString([]byte(linkCredential)))
		testutil.Equal(t, callsLabel, strings.Join(fake.RequestCalls(), listSep),
			strings.Join([]string{opened(adminMailbox), completeOnce, completeRevoke, keep}, listSep))
		expectRevoked(t, links, seen)
	})
}

// With no registration the run looks a last time under a fresh freeze, then thaws: the account keeps its passkeys.
// The link's window ends when its freeze lapses, so the test lapses it, after a round whose failed status read counts as open.
func TestRecoveryWaitEndsUnused(t *testing.T) {
	begun := []string{requestdata.PhaseRecovering, requestdata.PhaseLinkReady}
	cases := []unusedCase{
		{"the link expires", func(t *testing.T, _ *testutil.FakeExporter, outage *intercept) {
			t.Helper()
			outage.left.Store(constants.DefaultIncrementValue)
			for deadline := time.Now().Add(runLimit); outage.left.Load() > constants.DefaultInitValue; time.Sleep(lookEvery) {
				if time.Now().After(deadline) {
					t.Fatal("the run never read the request's status while it waited")
				}
			}
			redisServer.Del(frozenKey)
		}, ending{append(slices.Clone(begun), requestdata.PhaseExpired), fmt.Sprintf(constants.AuthRequestLinkExpired, bootstrapID)}},
		{"the request ends", func(_ *testing.T, fake *testutil.FakeExporter, _ *intercept) {
			endTheRequest(fake)
		}, ending{
			append(slices.Clone(begun), requestdata.PhaseCanceled), fmt.Sprintf(constants.AuthRequestEnded, bootstrapID, adminMailbox),
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				fake := signingRecovery()
				seen := watchFreeze(fake, nil)
				b := newBench(t, adminMailbox, fake)
				outage := &intercept{next: fake, path: bootstrapPath}
				b.backend = outage
				links := openWaysIn(t)
				b.begin(t)
				b.await(t, firstRequest, requestdata.PhaseLinkReady)
				c.end(t, fake, outage)

				b.end(t, firstRequest)
				b.expect(t, firstRequest, c.want)
				testutil.Equal(t, callsLabel, strings.Join(fake.RequestCalls(), listSep),
					strings.Join([]string{opened(adminMailbox), completeOnce, completeRevoke}, listSep))
				expectRevoked(t, links, seen)
				testutil.Equal(t, tokensLabel, enrollTokenOwners(), constants.EmptyString)
			})
		})
	}
}

// A registration sent before a sign-in ended the request can land during the last look's hold: the look retries a
// failed read and hands the passkey to the keep pass, which refuses it on the canceled request, and the run thaws.
func TestLastLookWaitsOutTheHold(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := signingRecovery()
		// Runs under the fake's lock, so it may read the status SetBootstrap writes.
		fake.OnRequest = func(path string) {
			if strings.HasSuffix(path, completeKey) && !fake.Bootstrap.Newest().Open() {
				delete(fake.RequestAnswers, completeKey)
			}
		}
		b := newBench(t, adminMailbox, fake)
		b.begin(t)
		_, token := b.openLink(t, b.await(t, firstRequest, requestdata.PhaseLinkReady))
		_, digest, err := authhelper.ResolveEnrollToken(token)
		if err != nil {
			t.Fatal(err)
		}
		endTheRequest(fake)
		awaitLastFreeze(t, digest)
		time.Sleep(lateLanding)
		saveLinkPasskey(t, pinnedUser)
		fake.FailNextPasskeyRead()

		b.end(t, firstRequest)
		b.expect(t, firstRequest, ending{
			[]string{requestdata.PhaseRecovering, requestdata.PhaseLinkReady, requestdata.PhaseFailed},
			fmt.Sprintf(string(constants.ErrAuthRequestCompleteFailed), bootstrapID, anyText),
		})
		keep := fmt.Sprintf(completeKeep, base64.RawURLEncoding.EncodeToString([]byte(linkCredential)))
		testutil.Equal(t, callsLabel, strings.Join(fake.RequestCalls(), listSep),
			strings.Join([]string{opened(adminMailbox), completeOnce, completeRevoke, keep}, listSep))
		testutil.Equal(t, thawedLabel, redisServer.Exists(frozenKey), false)
		testutil.Equal(t, lockLabel, redisServer.Exists(constants.RedisKeyAuthRequestRun), false)
	})
}

// Once the link is sealed, a run that loses its lock stops where it is and leaves the freeze to the run that took it.
// A last look that can't read the passkeys thaws the account and says so.
func TestRecoveryStopsAfterTheLink(t *testing.T) {
	registers := func(t *testing.T, token string) {
		t.Helper()
		register(t, pinnedUser, token)
	}
	lapses := func(*testing.T, string) { redisServer.Del(frozenKey) }
	lapsesThenHolds := func(t *testing.T, _ string) {
		t.Helper()
		redisServer.Del(frozenKey)
		for deadline := time.Now().Add(runLimit); !redisServer.Exists(frozenKey); time.Sleep(lookEvery) {
			if time.Now().After(deadline) {
				t.Fatal("the run never froze the account for its last look")
			}
		}
	}
	lost := fmt.Sprintf(string(constants.ErrAuthRequestRunFailed), constants.ErrAuthRequestRunLost)
	cases := []afterLinkCase{
		{"the lock is lost before the keep pass", bootstrapPath, constants.DefaultIncrementValue, true, true, registers, lost, true, takenOver},
		{"the lock is lost before the last look's freeze", bootstrapPath, constants.DefaultIncrementValue, true, true, lapses, lost,
			false, takenOver},
		{"the lock is lost during the last look", passkeysPath, constants.DefaultIncrementValue, false, true, lapsesThenHolds, lost,
			true, takenOver},
		{"the last look can't read the passkeys", passkeysPath, constants.RecoveryLastLookAttempts, false, false, lapsesThenHolds,
			fmt.Sprintf(string(constants.ErrAuthRequestRunFailed), anyText), false, constants.EmptyString},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				fake := signingRecovery()
				b := newBench(t, adminMailbox, fake)
				met := &intercept{next: fake, path: c.path, pass: c.pass}
				if c.takeover {
					met.then = func() { _ = redisServer.Set(constants.RedisKeyAuthRequestRun, takenOver) }
				}
				b.backend = met
				b.begin(t)
				_, token := b.openLink(t, b.await(t, firstRequest, requestdata.PhaseLinkReady))
				c.act(t, token)
				met.left.Store(c.reads)

				b.end(t, firstRequest)
				b.expect(t, firstRequest, ending{
					[]string{requestdata.PhaseRecovering, requestdata.PhaseLinkReady, requestdata.PhaseFailed}, c.reason,
				})
				testutil.Equal(t, "frozen", redisServer.Exists(frozenKey), c.frozen)
				held, _ := redisServer.Get(constants.RedisKeyAuthRequestRun)
				testutil.Equal(t, lockLabel, held, c.lock)
			})
		})
	}
}

// Waits until the run's own freeze replaced the link's.
func awaitLastFreeze(t *testing.T, digest string) {
	t.Helper()
	for deadline := time.Now().Add(runLimit); time.Now().Before(deadline); time.Sleep(lookEvery) {
		if frozen, _ := redisServer.Get(frozenKey); frozen != digest {
			return
		}
	}
	t.Fatal("the run never froze the account for its last look")
}
