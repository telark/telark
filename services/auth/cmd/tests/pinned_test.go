package tests

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	webauthnlib "github.com/go-webauthn/webauthn/webauthn"
	authdata "github.com/telark/telark/internal/data/auth"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	userresource "github.com/telark/telark/internal/data/resources/user"
	bootstrapeps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	redishelper "github.com/telark/telark/services/auth/internal/helpers/redis"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	pinnedUser     = "u-1"
	nextUser       = "u-2"
	pinnedMailbox  = "test@example.com"
	nextMailbox    = "next@example.com"
	requestID      = "0123456789abcdef"
	openCall       = `bootstrap/requests {"email":"%s"}`
	completeCall   = "bootstrap/requests/" + requestID + "/complete"
	completeOnce   = completeCall + ` {}`
	completeRevoke = completeCall + ` {"revoke":true}`
	completeKeep   = completeCall + ` {"keep":"%s"}`
	openAnswer     = "requests"
	completeKey    = "complete"
	frozenKey      = constants.RedisKeyPrefixFrozen + pinnedUser
	linkCredential = "link-passkey"
	takenOver      = "another-run"
	// The completion that revokes, after the hold.
	revokePass = 1
	// How long the test waits for break-glass to print its link (the hold alone takes 45 s), and how often it looks.
	linkWait = 2 * time.Minute
	linkPoll = 100 * time.Millisecond
	// A late registration lands this long after the run froze the account for its last look.
	lateLanding = time.Second
	// Labels the recovery runs' checks share.
	exitLabel  = "exit"
	callsLabel = "exporter calls"
	callSep    = ","
	linkLabel  = "link printed"
)

var (
	errNoLink       = errors.New("break-glass printed no link")
	errNoLastFreeze = errors.New("break-glass never froze the account for its last look")
)

func breakGlassRequest(kind, state, target string) telarkconfig.BootstrapRequest {
	return telarkconfig.BootstrapRequest{ID: requestID, Kind: kind, State: state, TargetEmail: target,
		OpenedAt: time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)}
}

func pinnedExporter() *testutil.FakeExporter {
	adminRole := constants.BuiltInRoleAdmin
	active := userresource.UserStatus{Phase: string(userresource.AccountPhaseActive)}
	return &testutil.FakeExporter{
		Users: map[string]*userresource.User{
			pinnedUser: {ID: pinnedUser, Email: pinnedMailbox, Bootstrap: true, RoleRefs: []*string{&adminRole}, Status: active},
			nextUser:   {ID: nextUser, Email: nextMailbox, Status: active},
		},
		Passkeys:  map[string]int{pinnedUser: constants.DefaultIncrementValue},
		Bootstrap: telarkconfig.BootstrapStatus{UserID: pinnedUser, Email: pinnedMailbox, PinnedAt: "2026-10-01T00:00:00Z"},
	}
}

func opened(mailbox string) string {
	return strings.Replace(openCall, "%s", mailbox, constants.DefaultIncrementValue)
}

// Once an account is pinned, break-glass opens a request instead of acting. Without --enroll it prints the pin and
// writes nothing; a request still waiting gets its page's link and no token (a recovery's says how it completes on
// its own, with no word of an approval); another open request stops the run. Every run leaves the run lock free.
func TestBreakGlassOpensARequestOncePinned(t *testing.T) {
	t.Setenv(constants.EnvBootstrapAdmin, pinnedMailbox)
	t.Setenv(constants.EnvRPOrigin, "https://telark.example.com")
	recovery, replacement, pending := telarkconfig.BootstrapKindRecover, telarkconfig.BootstrapKindReplace, telarkconfig.BootstrapStatePending
	waiting := &bootstrapeps.RequestResult{Request: breakGlassRequest(recovery, pending, constants.EmptyString)}
	recoveryLink := []string{"https://telark.example.com/break-glass#request=" + requestID + "&kind=recover",
		"Run this command again after ", "can deny it"}
	cases := []struct {
		name     string
		args     []string
		pinnedAs string
		answer   *bootstrapeps.RequestResult
		code     int
		out      []string
		notOut   string
		stderr   string
		calls    []string
	}{
		{"without --enroll", nil, pinnedMailbox, nil, constants.DefaultInitValue,
			[]string{"The bootstrap account is test@example.com", "Newest request: " + requestID}, "#request=", constants.EmptyString, nil},
		{"recovery waiting for its 72 hours", []string{flagEnroll}, pinnedMailbox, waiting, constants.DefaultInitValue, recoveryLink,
			"approve", constants.EmptyString, []string{opened(pinnedMailbox)}},
		// The account changed its email to the one the chart now names: its recovery is still the caller's.
		{"pinned under an earlier email", []string{flagEnroll}, "earlier@example.com", waiting, constants.DefaultInitValue,
			recoveryLink, "approve", constants.EmptyString, []string{opened(pinnedMailbox)}},
		{"replacement waiting for approval", []string{flagEnroll}, "earlier@example.com",
			&bootstrapeps.RequestResult{Request: breakGlassRequest(replacement, pending, pinnedMailbox)}, constants.DefaultInitValue,
			[]string{"https://telark.example.com/break-glass#request=" + requestID + "&kind=replace", "once the bootstrap account approves it"},
			"/register#", constants.EmptyString, []string{opened(pinnedMailbox)}},
		{"another request is open", []string{flagEnroll}, pinnedMailbox,
			&bootstrapeps.RequestResult{Request: breakGlassRequest(replacement, pending, nextMailbox)},
			constants.ExitCodeError, nil, "#request=", "Another request is open: " + requestID, []string{opened(pinnedMailbox)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			redisServer.FlushAll()
			fake := pinnedExporter()
			fake.Bootstrap.Email = c.pinnedAs
			fake.Bootstrap.Requests = []telarkconfig.BootstrapRequest{
				breakGlassRequest(recovery, telarkconfig.BootstrapStateCanceled, constants.EmptyString),
			}
			if c.answer != nil {
				fake.RequestAnswers = map[string]bootstrapeps.RequestResult{openAnswer: *c.answer}
			}
			testutil.StubBackend(t, fake)

			code, out, logged := runBreakGlass(t, append([]string{flagEmail, pinnedMailbox}, c.args...)...)
			testutil.Equal(t, "exit", code, c.code)
			for _, want := range c.out {
				if !strings.Contains(out, want) {
					t.Fatalf("stdout %q lacks %q", out, want)
				}
			}
			testutil.Equal(t, "stdout says "+c.notOut, strings.Contains(out, c.notOut), false)
			if !strings.Contains(logged, c.stderr) {
				t.Fatalf("stderr %q lacks %q", logged, c.stderr)
			}
			testutil.Equal(t, "exporter calls", strings.Join(fake.RequestCalls(), ","), strings.Join(c.calls, ","))
			testutil.Equal(t, "tokens", len(enrollTokenOwners()), constants.DefaultInitValue)
			testutil.Equal(t, "run lock", redisServer.Exists(constants.RedisKeyBreakGlassRun), false)
		})
	}
}

// The pinned account's ways in before a recovery: break-glass's invite, a link its session minted, and a
// registration and a sign-in that session started, still open. Returns the links' tokens.
func openWaysIn(t *testing.T) []string {
	t.Helper()
	redishelper.NewRedisClientWithRetry(context.Background())
	invite, _, err := authhelper.IssueInvite(pinnedUser, constants.EmptyString, breakGlassTTL)
	if err != nil {
		t.Fatal(err)
	}
	ownLink, _, err := authhelper.CreateEnrollToken(pinnedUser)
	if err != nil {
		t.Fatal(err)
	}
	for _, challenge := range []string{"aW4tZmxpZ2h0LXJlZ2lzdHJhdGlvbg", "aW4tZmxpZ2h0LXNpZ24taW4"} {
		if err := webauthnhelper.StoreChallenge(pinnedUser, challenge, constants.EmptyString); err != nil {
			t.Fatal(err)
		}
	}
	return []string{invite, ownLink}
}

// At each completion the exporter receives: whether the pinned account was frozen, and when.
type completions struct {
	frozen []bool
	at     []time.Time
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

// A recovery fakes break-glass's first steps against an eligible recovery request, open until a test ends it.
func recoveryExporter(complete *bootstrapeps.RequestResult) *testutil.FakeExporter {
	fake := pinnedExporter()
	request := breakGlassRequest(telarkconfig.BootstrapKindRecover, telarkconfig.BootstrapStatePending, constants.EmptyString)
	fake.Bootstrap.Requests = []telarkconfig.BootstrapRequest{request}
	fake.RequestAnswers = map[string]bootstrapeps.RequestResult{openAnswer: {Request: request, Eligible: true}}
	if complete != nil {
		fake.RequestAnswers[completeKey] = *complete
	}
	return fake
}

// An eligible recovery of an account that signs with one passkey, made long before the request opened.
func signingRecovery() *testutil.FakeExporter {
	fake := recoveryExporter(&bootstrapeps.RequestResult{
		Request:  breakGlassRequest(telarkconfig.BootstrapKindRecover, telarkconfig.BootstrapStatePending, constants.EmptyString),
		Eligible: true,
	})
	created := "2026-01-01T00:00:00Z"
	fake.Credentials = map[string][]authdata.Passkey{pinnedUser: {{UserID: pinnedUser, CredentialID: "b2xk", CreationTimestamp: &created}}}
	return fake
}

// A refused first completion revoked nothing, so the account thaws at once.
func TestBreakGlassThawsWhenNothingWasRevoked(t *testing.T) {
	t.Setenv(constants.EnvBootstrapAdmin, pinnedMailbox)
	redisServer.FlushAll()
	fake := recoveryExporter(nil)
	testutil.StubBackend(t, fake)
	code, out, _ := runBreakGlass(t, flagEmail, pinnedMailbox, flagEnroll)
	testutil.Equal(t, "exit", code, constants.ExitCodeError)
	testutil.Equal(t, "exporter calls", strings.Join(fake.RequestCalls(), ","), opened(pinnedMailbox)+","+completeOnce)
	testutil.Equal(t, "thawed", redisServer.Exists(frozenKey), false)
	testutil.Equal(t, "link printed", strings.Contains(out, "#enroll="), false)
}

// A freeze that lapses mid-recovery stops it before the hold: a gap could have let a write through.
func TestBreakGlassStopsWhenTheFreezeLapses(t *testing.T) {
	t.Setenv(constants.EnvBootstrapAdmin, pinnedMailbox)
	redisServer.FlushAll()
	fake := recoveryExporter(&bootstrapeps.RequestResult{
		Request:  breakGlassRequest(telarkconfig.BootstrapKindRecover, telarkconfig.BootstrapStatePending, constants.EmptyString),
		Eligible: true,
	})
	watchFreeze(fake, func() { redisServer.Del(frozenKey) })
	testutil.StubBackend(t, fake)
	code, out, logged := runBreakGlass(t, flagEmail, pinnedMailbox, flagEnroll)
	testutil.Equal(t, "exit", code, constants.ExitCodeError)
	testutil.Equal(t, "says why", strings.Contains(logged, "stalled too long"), true)
	testutil.Equal(t, "exporter calls", strings.Join(fake.RequestCalls(), ","), opened(pinnedMailbox)+","+completeOnce)
	testutil.Equal(t, "link printed", strings.Contains(out, "#enroll="), false)
}

// The run revoked the pinned account's ways in: earlier links and open ceremonies gone, frozen at each completion,
// the revoking pass only once the bound on a write sent before the freeze has passed, thawed at the end, and the run
// lock released.
func expectRevoked(t *testing.T, links []string, seen *completions, began time.Time) {
	t.Helper()
	for _, token := range links {
		if _, _, err := authhelper.ResolveEnrollToken(token); err == nil {
			t.Error("an earlier link still resolves")
		}
	}
	testutil.Equal(t, "open ceremonies", ceremoniesOpen(), false)
	testutil.Equal(t, "frozen at each completion", slices.Contains(seen.frozen, false), false)
	testutil.Equal(t, "held", seen.at[revokePass].Sub(began) >= constants.RecoveryHold, true)
	testutil.Equal(t, "thawed", redisServer.Exists(frozenKey), false)
	testutil.Equal(t, "run lock", redisServer.Exists(constants.RedisKeyBreakGlassRun), false)
}

func ceremoniesOpen() bool {
	return slices.ContainsFunc(redisServer.Keys(), func(key string) bool {
		return strings.HasPrefix(key, constants.RedisKeyPrefixChallenge+pinnedUser+constants.ColonSeparator)
	})
}

// What the test saw once break-glass printed its link: the freeze held that link's digest for the link's window and
// refused every other ceremony, and what the test did then (register with the link, lapse its window, end the request).
type linkUse struct {
	window       time.Duration
	othersFrozen bool
	err          error
}

// Waits until break-glass's freeze holds the link it printed, then runs then with the link's digest.
func useTheLink(then func(digest string) error) <-chan linkUse {
	used := make(chan linkUse, constants.DefaultIncrementValue)
	go func() {
		digest, err := linkFreeze()
		use := linkUse{window: redisServer.TTL(frozenKey), err: err}
		use.othersFrozen = authhelper.RefuseFrozen(pinnedUser, constants.EmptyString) != nil
		if err == nil {
			use.err = then(digest)
		}
		used <- use
	}()
	return used
}

func linkFreeze() (string, error) {
	for deadline := time.Now().Add(linkWait); time.Now().Before(deadline); {
		frozen, _ := redisServer.Get(frozenKey)
		if digest, err := redisServer.Get(constants.RedisKeyPrefixInviteOf + pinnedUser); err == nil && frozen == digest {
			return digest, nil
		}
		time.Sleep(linkPoll)
	}
	return constants.EmptyString, errNoLink
}

// The way auth's registration finish does: the freeze checks with the link's digest, the token is spent and the passkey
// saved, without the WebAuthn ceremony.
func registerWith(digest string) error {
	if err := authhelper.RefuseFrozen(pinnedUser, digest); err != nil {
		return err
	}
	if err := authhelper.SpendEnrollToken(pinnedUser, digest); err != nil {
		return err
	}
	return saveLinkPasskey()
}

// The link's passkey, saved whenever auth's write lands.
func saveLinkPasskey() error {
	passkey := authhelper.CreatePasskeyFromCredential(pinnedUser, &webauthnlib.Credential{ID: []byte(linkCredential)},
		"laptop", "platform", false, false)
	_, err := authhelper.CreatePasskey(pinnedUser, passkey)
	return err
}

// Link first, wipe after. An eligible recovery freezes the account, drops its links and open ceremonies, ends its
// sessions, holds the freeze until a write sent before it has landed, drops again, revokes what a stopped run's link
// saved, and prints a 10-minute link. The freeze then holds that link for its window, letting through only the link's
// registration; once that passkey shows, the exporter keeps it alone and completes the request, and the account thaws.
func TestBreakGlassRecoversLinkFirst(t *testing.T) {
	t.Setenv(constants.EnvBootstrapAdmin, pinnedMailbox)
	redisServer.FlushAll()
	fake := signingRecovery()
	seen := watchFreeze(fake, nil)
	testutil.StubBackend(t, fake)
	links := openWaysIn(t)
	registration := useTheLink(registerWith)

	began := time.Now()
	code, out, logged := runBreakGlass(t, flagEmail, pinnedMailbox, flagEnroll)
	use := <-registration
	if use.err != nil {
		t.Fatalf("registration with the link: %v", use.err)
	}
	testutil.Equal(t, exitLabel, code, constants.DefaultInitValue)
	testutil.Equal(t, "stderr", logged, constants.EmptyString)
	testutil.Equal(t, "link window", use.window, constants.RecoveryLinkWindow)
	testutil.Equal(t, "other ceremonies refused meanwhile", use.othersFrozen, true)
	keep := fmt.Sprintf(completeKeep, base64.RawURLEncoding.EncodeToString([]byte(linkCredential)))
	testutil.Equal(t, callsLabel, strings.Join(fake.RequestCalls(), callSep),
		strings.Join([]string{opened(pinnedMailbox), completeOnce, completeRevoke, keep}, callSep))
	expectRevoked(t, links, seen, began)
	for _, want := range []string{"/register#enroll=", "Keep this command running", "now has only the new passkey"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout %q lacks %q", out, want)
		}
	}
}

// The wait ends with no registration, when the link's window ends or when a sign-in begun before the freeze cancels the
// request: the run freezes the account again, drops the link, holds the freeze, finds no new passkey, thaws while it
// still holds its lock and says why, so the account keeps its passkeys (no keep pass). The window ends when the link's
// freeze lapses, so the test lapses it at once instead of waiting out its 12 minutes.
func TestBreakGlassWaitEndsUnused(t *testing.T) {
	t.Setenv(constants.EnvBootstrapAdmin, pinnedMailbox)
	cases := []struct {
		name string
		end  func(fake *testutil.FakeExporter)
		says string
	}{
		{"the link expires", func(*testutil.FakeExporter) { redisServer.Del(frozenKey) }, "expired unused"},
		{"the request ends", endTheRequest, "was canceled by a sign-in or a deny"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			redisServer.FlushAll()
			fake := signingRecovery()
			seen := watchFreeze(fake, nil)
			testutil.StubBackend(t, fake)
			links := openWaysIn(t)
			ended := useTheLink(func(string) error { c.end(fake); return nil })

			began := time.Now()
			code, out, logged := runBreakGlass(t, flagEmail, pinnedMailbox, flagEnroll)
			if use := <-ended; use.err != nil {
				t.Fatal(use.err)
			}
			testutil.Equal(t, exitLabel, code, constants.ExitCodeError)
			testutil.Equal(t, "says so", strings.Contains(logged, c.says), true)
			testutil.Equal(t, callsLabel, strings.Join(fake.RequestCalls(), callSep),
				strings.Join([]string{opened(pinnedMailbox), completeOnce, completeRevoke}, callSep))
			expectRevoked(t, links, seen, began)
			testutil.Equal(t, "live links", len(enrollTokenOwners()), constants.DefaultInitValue)
			testutil.Equal(t, linkLabel, strings.Contains(out, "/register#enroll="), true)
		})
	}
}

// As the cancel of a sign-in begun before the freeze does when it lands mid-run.
func endTheRequest(fake *testutil.FakeExporter) {
	status := pinnedExporter().Bootstrap
	status.Requests = []telarkconfig.BootstrapRequest{
		breakGlassRequest(telarkconfig.BootstrapKindRecover, telarkconfig.BootstrapStateCanceled, constants.EmptyString),
	}
	fake.SetBootstrap(status)
}

// A registration sent before a sign-in ended the request can land once the run froze the account for its last look: the
// run holds that freeze as the revoking pass does, so the look still hands the passkey to the exporter's keep pass,
// which refuses it on the canceled request (and deletes it), and the run thaws the account.
func TestBreakGlassLastLookWaitsOutTheHold(t *testing.T) {
	t.Setenv(constants.EnvBootstrapAdmin, pinnedMailbox)
	redisServer.FlushAll()
	fake := signingRecovery()
	// Runs under the fake's lock, so it may read the status SetBootstrap writes.
	fake.OnRequest = func(path string) {
		if strings.HasSuffix(path, completeKey) && !fake.Bootstrap.Newest().Open() {
			delete(fake.RequestAnswers, completeKey)
		}
	}
	testutil.StubBackend(t, fake)
	late := useTheLink(func(digest string) error {
		endTheRequest(fake)
		if err := lastFreeze(digest); err != nil {
			return err
		}
		time.Sleep(lateLanding)
		return saveLinkPasskey()
	})

	code, _, logged := runBreakGlass(t, flagEmail, pinnedMailbox, flagEnroll)
	if use := <-late; use.err != nil {
		t.Fatal(use.err)
	}
	keep := fmt.Sprintf(completeKeep, base64.RawURLEncoding.EncodeToString([]byte(linkCredential)))
	testutil.Equal(t, callsLabel, strings.Join(fake.RequestCalls(), callSep),
		strings.Join([]string{opened(pinnedMailbox), completeOnce, completeRevoke, keep}, callSep))
	testutil.Equal(t, exitLabel, code, constants.ExitCodeError)
	testutil.Equal(t, "says the keep pass failed", strings.Contains(logged, "Could not finish request "+requestID), true)
	testutil.Equal(t, "thawed after the refusal", redisServer.Exists(frozenKey), false)
	testutil.Equal(t, "run lock", redisServer.Exists(constants.RedisKeyBreakGlassRun), false)
}

// Waits until the run's own freeze replaced the link's.
func lastFreeze(digest string) error {
	for deadline := time.Now().Add(linkWait); time.Now().Before(deadline); time.Sleep(linkPoll) {
		if frozen, _ := redisServer.Get(frozenKey); frozen != digest {
			return nil
		}
	}
	return errNoLastFreeze
}

// One run at a time. A run that finds another run's lock stops before it writes anything and leaves that lock alone;
// a run whose lock lapsed and was taken by another run doesn't release that run's.
func TestBreakGlassRunsOneAtATime(t *testing.T) {
	t.Setenv(constants.EnvBootstrapAdmin, pinnedMailbox)
	waiting := map[string]bootstrapeps.RequestResult{openAnswer: {
		Request: breakGlassRequest(telarkconfig.BootstrapKindRecover, telarkconfig.BootstrapStatePending, constants.EmptyString),
	}}
	t.Run("another run holds the lock", func(t *testing.T) {
		redisServer.FlushAll()
		fake := pinnedExporter()
		fake.RequestAnswers = waiting
		testutil.StubBackend(t, fake)
		redishelper.NewRedisClientWithRetry(context.Background())
		other, err := authhelper.LockRun()
		if err != nil {
			t.Fatal(err)
		}
		code, _, logged := runBreakGlass(t, flagEmail, pinnedMailbox, flagEnroll)
		testutil.Equal(t, "exit", code, constants.ExitCodeError)
		testutil.Equal(t, "says why", strings.Contains(logged, "another run is in progress"), true)
		testutil.Equal(t, "exporter calls", len(fake.RequestCalls()), constants.DefaultInitValue)
		held, _ := redisServer.Get(constants.RedisKeyBreakGlassRun)
		testutil.Equal(t, "the other run's lock", held, other)
	})
	t.Run("a lock taken over meanwhile", func(t *testing.T) {
		redisServer.FlushAll()
		fake := pinnedExporter()
		fake.RequestAnswers = waiting
		var lifetime time.Duration
		fake.OnRequest = func(string) {
			lifetime = redisServer.TTL(constants.RedisKeyBreakGlassRun)
			_ = redisServer.Set(constants.RedisKeyBreakGlassRun, takenOver)
		}
		testutil.StubBackend(t, fake)
		code, _, _ := runBreakGlass(t, flagEmail, pinnedMailbox, flagEnroll)
		testutil.Equal(t, "exit", code, constants.DefaultInitValue)
		testutil.Equal(t, "exporter calls", len(fake.RequestCalls()), constants.DefaultIncrementValue)
		testutil.Equal(t, "lock lifetime", lifetime, time.Duration(constants.RedisTTLBreakGlassRun)*time.Second)
		held, _ := redisServer.Get(constants.RedisKeyBreakGlassRun)
		testutil.Equal(t, "the other run's lock", held, takenOver)
	})
	// The refused first completion would thaw, but the freeze may be the run's that took the lock over.
	t.Run("a lock lost before a thaw", func(t *testing.T) {
		redisServer.FlushAll()
		fake := recoveryExporter(nil)
		fake.OnRequest = func(path string) {
			if strings.HasSuffix(path, completeKey) {
				_ = redisServer.Set(constants.RedisKeyBreakGlassRun, takenOver)
				_ = redisServer.Set(frozenKey, takenOver)
			}
		}
		testutil.StubBackend(t, fake)
		code, _, _ := runBreakGlass(t, flagEmail, pinnedMailbox, flagEnroll)
		testutil.Equal(t, "exit", code, constants.ExitCodeError)
		frozen, _ := redisServer.Get(frozenKey)
		testutil.Equal(t, "the other run's freeze", frozen, takenOver)
		held, _ := redisServer.Get(constants.RedisKeyBreakGlassRun)
		testutil.Equal(t, "the other run's lock", held, takenOver)
	})
}

// A replacement promotes the account the chart now names, revokes nothing and freezes nothing, and prints one
// 10-minute token; one that already has a passkey gets no token.
func TestBreakGlassCompletesAReplacement(t *testing.T) {
	cases := []struct {
		name     string
		enrolled bool
		owner    string
	}{
		{"replacement", false, nextUser},
		{"replacement onto an enrolled account", true, constants.EmptyString},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(constants.EnvBootstrapAdmin, nextMailbox)
			redisServer.FlushAll()
			replacement := telarkconfig.BootstrapKindReplace
			fake := pinnedExporter()
			fake.RequestAnswers = map[string]bootstrapeps.RequestResult{
				openAnswer: {Request: breakGlassRequest(replacement, telarkconfig.BootstrapStateApproved, nextMailbox), Eligible: true},
				completeKey: {Request: breakGlassRequest(replacement, telarkconfig.BootstrapStateCompleted, nextMailbox), Eligible: true,
					Enrolled: c.enrolled},
			}
			seen := watchFreeze(fake, nil)
			testutil.StubBackend(t, fake)
			links := openWaysIn(t)

			code, out, _ := runBreakGlass(t, flagEmail, nextMailbox, flagEnroll)
			testutil.Equal(t, "exit", code, constants.DefaultInitValue)
			testutil.Equal(t, "exporter calls", strings.Join(fake.RequestCalls(), ","), opened(nextMailbox)+","+completeOnce)
			testutil.Equal(t, "frozen", slices.Contains(seen.frozen, true), false)
			testutil.Equal(t, "open ceremonies", ceremoniesOpen(), true)
			for _, token := range links {
				if owner, _, err := authhelper.ResolveEnrollToken(token); err != nil || owner != pinnedUser {
					t.Fatalf("the pinned account's earlier link resolves to %q (%v)", owner, err)
				}
			}
			testutil.Equal(t, "run lock", redisServer.Exists(constants.RedisKeyBreakGlassRun), false)
			if c.owner == constants.EmptyString {
				testutil.Equal(t, "already enrolled", strings.Contains(out, "already has a passkey"), true)
				testutil.Equal(t, "link printed", strings.Contains(out, "/register#"), false)
				return
			}
			_, fragment := enrollLink(t, out)
			if owner, _, err := authhelper.ResolveEnrollToken(linkToken(t, fragment)); err != nil || owner != c.owner {
				t.Fatalf("the printed token resolves to %q (%v), want %s", owner, err, c.owner)
			}
			testutil.Equal(t, "lifetime", redisServer.TTL(constants.RedisKeyPrefixInviteOf+c.owner), breakGlassTTL)
		})
	}
}
