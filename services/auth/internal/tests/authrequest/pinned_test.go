package authrequest

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	authdata "github.com/telark/telark/internal/data/auth"
	requestdata "github.com/telark/telark/internal/data/resources/authrequest"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	userresource "github.com/telark/telark/internal/data/resources/user"
	bootstrapeps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	earlierMailbox = "earlier@example.com"
	newMailbox     = "new@example.com"
	bootstrapID    = "0123456789abcdef"
	openCall       = `bootstrap/requests {"email":"%s","requestedBy":"` + requester + `"}`
	completeCall   = "bootstrap/requests/" + bootstrapID + "/complete"
	completeOnce   = completeCall + ` {}`
	completeRevoke = completeCall + ` {"revoke":true}`
	completeKeep   = completeCall + ` {"keep":"%s"}`
	openAnswer     = "requests"
	completeKey    = "complete"
	thirtyHours    = 30
	// The open and the one completion: a replacement revokes nothing.
	replacementCalls = 2
)

func bootstrapRequest(kind, state, target string) telarkconfig.BootstrapRequest {
	return telarkconfig.BootstrapRequest{ID: bootstrapID, Kind: kind, State: state, TargetEmail: target,
		OpenedAt: time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)}
}

func pinnedExporter() *testutil.FakeExporter {
	active := userresource.UserStatus{Phase: string(userresource.AccountPhaseActive)}
	return &testutil.FakeExporter{
		Users: map[string]*userresource.User{
			pinnedUser: {ID: pinnedUser, Email: adminMailbox, Bootstrap: true, RoleRefs: adminRole(), Status: active},
			nextUser:   {ID: nextUser, Email: nextMailbox, Status: active},
		},
		Passkeys:  map[string]int{pinnedUser: constants.DefaultIncrementValue},
		Bootstrap: telarkconfig.BootstrapStatus{UserID: pinnedUser, Email: adminMailbox, PinnedAt: longAgo},
	}
}

// A recovery of the pinned account, still waiting out the wait.
func waitingExporter() *testutil.FakeExporter {
	fake := pinnedExporter()
	fake.RequestAnswers = map[string]bootstrapeps.RequestResult{openAnswer: {
		Request: bootstrapRequest(telarkconfig.BootstrapKindRecover, telarkconfig.BootstrapStatePending, constants.EmptyString),
	}}
	return fake
}

func opened(mailbox string) string {
	return fmt.Sprintf(openCall, mailbox)
}

// Once an account is pinned, a request opens one in TelarkConfig in the requester's name instead of acting. One still
// waiting reports its page's link and when it completes on its own, and issues no token; another open request denies it.
func TestOpensARequestOncePinned(t *testing.T) {
	recovery, replacement := telarkconfig.BootstrapKindRecover, telarkconfig.BootstrapKindReplace
	day, waiting := constants.DefaultAuthRequestWaitHours, []string{requestdata.PhaseWaitingForApproval}
	replaces := func(mailbox string) string { return fmt.Sprintf(constants.AuthRequestKindReplace, mailbox) }
	recovers := func(mailbox string) string { return fmt.Sprintf(constants.AuthRequestKindRecover, mailbox) }
	cases := []waitingCase{
		{"recovery waiting out the wait", dashboard, day, adminMailbox, recovery, constants.EmptyString,
			ending{waiting, fmt.Sprintf(constants.AuthRequestWaitRecover, bootstrapID, recovers(adminMailbox))}},
		{"recovery waiting out a configured wait", dashboard, thirtyHours, adminMailbox, recovery, constants.EmptyString,
			ending{waiting, fmt.Sprintf(constants.AuthRequestWaitRecover, bootstrapID, recovers(adminMailbox))}},
		// The account changed its email to the one the chart now names: its recovery is still the requester's.
		{"pinned under an earlier email", dashboard, day, earlierMailbox, recovery, constants.EmptyString,
			ending{waiting, fmt.Sprintf(constants.AuthRequestWaitRecover, bootstrapID, recovers(earlierMailbox))}},
		{"replacement waiting for approval, port-forward install", constants.EmptyString, day, earlierMailbox, replacement, adminMailbox,
			ending{waiting, fmt.Sprintf(constants.AuthRequestWaitReplace, bootstrapID, replaces(adminMailbox))}},
		{"another request is open", dashboard, day, adminMailbox, replacement, nextMailbox, ending{
			[]string{requestdata.PhaseDenied}, fmt.Sprintf(constants.AuthRequestOtherRequest, bootstrapID, replaces(nextMailbox)),
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) { opensARequest(t, c) })
		})
	}
}

func opensARequest(t *testing.T, c waitingCase) {
	t.Helper()
	t.Setenv(constants.EnvRPOrigin, c.origin)
	t.Setenv(constants.EnvAuthRequestWaitHours, strconv.Itoa(c.hours))
	fake := pinnedExporter()
	fake.Bootstrap.Email = c.pinnedAs
	fake.Bootstrap.Requests = []telarkconfig.BootstrapRequest{
		bootstrapRequest(telarkconfig.BootstrapKindRecover, telarkconfig.BootstrapStateCanceled, constants.EmptyString),
	}
	request := bootstrapRequest(c.kind, telarkconfig.BootstrapStatePending, c.target)
	fake.RequestAnswers = map[string]bootstrapeps.RequestResult{openAnswer: {Request: request}}
	b := newBench(t, adminMailbox, fake)
	last := b.run(t)
	b.expect(t, firstRequest, c.want)

	want := requestdata.Status{Phase: last.Phase, Reason: last.Reason}
	if last.Phase == requestdata.PhaseWaitingForApproval {
		origin := c.origin
		if origin == constants.EmptyString {
			origin = constants.AuthRequestDashboardURL
		}
		openedAt, err := time.Parse(time.RFC3339, request.OpenedAt)
		if err != nil {
			t.Fatal(err)
		}
		want.RequestID = bootstrapID
		want.ApprovalURL = fmt.Sprintf(constants.AuthRequestApprovalURL, origin, bootstrapID, c.kind)
		want.CompletesAt = openedAt.Add(time.Duration(c.hours) * time.Hour).Format(constants.TimeFormatRFC3339)
	}
	testutil.Equal(t, "status", last, want)
	testutil.Equal(t, callsLabel, strings.Join(fake.RequestCalls(), listSep), opened(adminMailbox))
	testutil.Equal(t, tokensLabel, enrollTokenOwners(), constants.EmptyString)
	testutil.Equal(t, lockLabel, redisServer.Exists(constants.RedisKeyAuthRequestRun), false)
}

// A run whose lock lapsed and was taken by another run doesn't release that run's.
func TestLeavesALockAnotherRunTook(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fake := waitingExporter()
		var lifetime time.Duration
		fake.OnRequest = func(string) {
			lifetime = redisServer.TTL(constants.RedisKeyAuthRequestRun)
			_ = redisServer.Set(constants.RedisKeyAuthRequestRun, takenOver)
		}
		b := newBench(t, adminMailbox, fake)
		b.run(t)
		testutil.Equal(t, phasesLabel, b.phases(firstRequest), requestdata.PhaseWaitingForApproval)
		testutil.Equal(t, callsLabel, strings.Join(fake.RequestCalls(), listSep), opened(adminMailbox))
		testutil.Equal(t, "lock lifetime", lifetime, time.Duration(constants.RedisTTLAuthRequestRun)*time.Second)
		held, _ := redisServer.Get(constants.RedisKeyAuthRequestRun)
		testutil.Equal(t, lockLabel, held, takenOver)
	})
}

// A replacement promotes the named account without the marker and revokes nothing. An enrolled one completes at once;
// else the run seals one 10-minute link and waits for its passkey until the link expires.
func TestCompletesAReplacement(t *testing.T) {
	ready := requestdata.PhaseLinkReady
	cases := []replacementCase{
		{"the link registers a passkey", false, true,
			ending{[]string{ready, requestdata.PhaseCompleted}, fmt.Sprintf(constants.AuthRequestEnrolled, nextMailbox)}},
		{"the link expires unused", false, false, ending{[]string{ready, requestdata.PhaseExpired}, constants.AuthRequestLinkUnused}},
		{"onto an enrolled account", true, false,
			ending{[]string{requestdata.PhaseCompleted}, fmt.Sprintf(constants.AuthRequestReplaced, bootstrapID, nextMailbox)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) { completesAReplacement(t, c) })
		})
	}
}

func completesAReplacement(t *testing.T, c replacementCase) {
	t.Helper()
	replacement := telarkconfig.BootstrapKindReplace
	fake := pinnedExporter()
	fake.Credentials = map[string][]authdata.Passkey{nextUser: nil}
	fake.RequestAnswers = map[string]bootstrapeps.RequestResult{
		openAnswer: {Request: bootstrapRequest(replacement, telarkconfig.BootstrapStateApproved, nextMailbox), Eligible: true},
		completeKey: {Request: bootstrapRequest(replacement, telarkconfig.BootstrapStateCompleted, nextMailbox), Eligible: true,
			Enrolled: c.enrolled},
	}
	seen := watchFreeze(fake, nil)
	b := newBench(t, nextMailbox, fake)
	links := openWaysIn(t)
	b.begin(t)
	token := constants.EmptyString
	if !c.enrolled {
		var link string
		link, token = b.openLink(t, b.await(t, firstRequest, requestdata.PhaseLinkReady))
		testutil.Equal(t, "link", link, wantLink(constants.AuthRequestDashboardURL, token, nextMailbox))
		testutil.Equal(t, "lifetime", redisServer.TTL(constants.RedisKeyPrefixInviteOf+nextUser), linkTTL)
		if owner, _, err := authhelper.ResolveEnrollToken(token); err != nil || owner != nextUser {
			t.Fatalf("the sealed token resolves to %q (%v), want %s", owner, err, nextUser)
		}
	} else {
		b.last(t, firstRequest)
	}
	// Checked before the link's wait ends, which outlasts the pinned account's own links.
	testutil.Equal(t, callsLabel, strings.Join(fake.RequestCalls(), listSep), opened(nextMailbox)+listSep+completeOnce)
	testutil.Equal(t, "open ceremonies", ceremoniesOpen(), true)
	for _, earlier := range links {
		if owner, _, err := authhelper.ResolveEnrollToken(earlier); err != nil || owner != pinnedUser {
			t.Fatalf("the pinned account's earlier link resolves to %q (%v)", owner, err)
		}
	}
	if c.registers {
		register(t, nextUser, token)
	}
	last := b.end(t, firstRequest)
	b.expect(t, firstRequest, c.want)
	testutil.Equal(t, requestLabel, last.RequestID, bootstrapID)
	testutil.Equal(t, "calls once the run ended", len(fake.RequestCalls()), replacementCalls)
	testutil.Equal(t, "frozen", slices.Contains(seen.frozen, true), false)
	promoted := fake.User(nextUser)
	testutil.Equal(t, "promoted without the marker", authhelper.HasAdminRole(promoted.RoleRefs) && !promoted.Bootstrap, true)
	testutil.Equal(t, lockLabel, redisServer.Exists(constants.RedisKeyAuthRequestRun), false)
}

// A replacement the exporter refuses at any step stops the run.
func TestReplacementStopsOnARefusal(t *testing.T) {
	replacementTo := func(fake *testutil.FakeExporter, target string) {
		fake.RequestAnswers[openAnswer] = bootstrapeps.RequestResult{Eligible: true,
			Request: bootstrapRequest(telarkconfig.BootstrapKindReplace, telarkconfig.BootstrapStateApproved, target)}
	}
	failed := []string{requestdata.PhaseFailed}
	expectStops(t, []stopCase{
		{"the account can't be created", newMailbox, func(fake *testutil.FakeExporter) http.Handler {
			replacementTo(fake, newMailbox)
			return refuse(fake, http.MethodPost, usersPath, http.StatusConflict)
		}, ending{failed, fmt.Sprintf(string(constants.ErrAuthRequestCreateFailed), newMailbox, anyText)},
			[]string{opened(newMailbox)}, constants.EmptyString, constants.EmptyString},
		{"the promotion is refused", nextMailbox, func(fake *testutil.FakeExporter) http.Handler {
			replacementTo(fake, nextMailbox)
			fake.SetGone(nextUser)
			return fake
		}, ending{failed, fmt.Sprintf(string(constants.ErrAuthRequestPatchFailed), nextMailbox, http.StatusGone, anyText)},
			[]string{opened(nextMailbox)}, constants.EmptyString, constants.EmptyString},
		{"the completion is refused", nextMailbox, func(fake *testutil.FakeExporter) http.Handler {
			replacementTo(fake, nextMailbox)
			return fake
		}, ending{failed, fmt.Sprintf(string(constants.ErrAuthRequestCompleteFailed), bootstrapID, anyText)},
			[]string{opened(nextMailbox), completeOnce}, constants.EmptyString, constants.EmptyString},
	})
}
