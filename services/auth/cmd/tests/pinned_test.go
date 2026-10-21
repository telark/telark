package tests

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

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
	pinnedUser    = "u-1"
	nextUser      = "u-2"
	pinnedMailbox = "test@example.com"
	nextMailbox   = "next@example.com"
	requestID     = "0123456789abcdef"
	openCall      = `bootstrap/requests {"email":"%s"}`
	completeCall  = "bootstrap/requests/" + requestID + "/complete"
	completeOnce  = completeCall + ` {"revoke":false}`
	completeLast  = completeCall + ` {"revoke":true}`
	openAnswer    = "requests"
	completeKey   = "complete"
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

// Once an account is pinned, break-glass opens a request instead of acting. Without --enroll it prints the pin and
// writes nothing; a request still waiting gets the approval link and no token; another open request stops the run.
func TestBreakGlassOpensARequestOncePinned(t *testing.T) {
	t.Setenv(constants.EnvBootstrapAdmin, pinnedMailbox)
	t.Setenv(constants.EnvRPOrigin, "https://telark.example.com")
	recovery := telarkconfig.BootstrapKindRecover
	waiting := &bootstrapeps.RequestResult{Request: breakGlassRequest(recovery, telarkconfig.BootstrapStatePending, constants.EmptyString)}
	approvalLink := []string{"https://telark.example.com/break-glass#request=" + requestID, "72 hours", "a sign-in cancels it"}
	opened := []string{strings.Replace(openCall, "%s", pinnedMailbox, constants.DefaultIncrementValue)}
	cases := []struct {
		name     string
		args     []string
		pinnedAs string
		answer   *bootstrapeps.RequestResult
		code     int
		out      []string
		stderr   string
		calls    []string
	}{
		{"without --enroll", nil, pinnedMailbox, nil, constants.DefaultInitValue,
			[]string{"The bootstrap account is test@example.com", "Newest request: " + requestID}, constants.EmptyString, nil},
		{"waiting for approval", []string{flagEnroll}, pinnedMailbox, waiting, constants.DefaultInitValue, approvalLink,
			constants.EmptyString, opened},
		// The account changed its email to the one the chart now names: its recovery is still the caller's.
		{"pinned under an earlier email", []string{flagEnroll}, "earlier@example.com", waiting, constants.DefaultInitValue,
			approvalLink, constants.EmptyString, opened},
		{"another request is open", []string{flagEnroll}, pinnedMailbox,
			&bootstrapeps.RequestResult{
				Request: breakGlassRequest(telarkconfig.BootstrapKindReplace, telarkconfig.BootstrapStatePending, nextMailbox),
			},
			constants.ExitCodeError, nil, "Another break-glass request is open", opened},
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
			if !strings.Contains(logged, c.stderr) {
				t.Fatalf("stderr %q lacks %q", logged, c.stderr)
			}
			testutil.Equal(t, "exporter calls", strings.Join(fake.RequestCalls(), ","), strings.Join(c.calls, ","))
			testutil.Equal(t, "tokens", len(enrollTokenOwners()), constants.DefaultInitValue)
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
		if err := webauthnhelper.StoreChallenge(pinnedUser, challenge); err != nil {
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
			seen.frozen = append(seen.frozen, redisServer.Exists(constants.RedisKeyPrefixFrozen+pinnedUser))
			seen.at = append(seen.at, time.Now())
			if onComplete != nil {
				onComplete()
			}
		}
	}
	return seen
}

// A recovery fakes break-glass's first steps against an eligible recovery request.
func recoveryExporter(complete *bootstrapeps.RequestResult) *testutil.FakeExporter {
	fake := pinnedExporter()
	fake.RequestAnswers = map[string]bootstrapeps.RequestResult{
		openAnswer: {Request: breakGlassRequest(telarkconfig.BootstrapKindRecover, telarkconfig.BootstrapStateApproved, constants.EmptyString),
			Eligible: true},
	}
	if complete != nil {
		fake.RequestAnswers[completeKey] = *complete
	}
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
	testutil.Equal(t, "exporter calls", strings.Join(fake.RequestCalls(), ","),
		strings.Replace(openCall, "%s", pinnedMailbox, constants.DefaultIncrementValue)+","+completeOnce)
	testutil.Equal(t, "thawed", redisServer.Exists(constants.RedisKeyPrefixFrozen+pinnedUser), false)
	testutil.Equal(t, "link printed", strings.Contains(out, "#enroll="), false)
}

// A freeze that lapses mid-recovery stops it before the last pass: a gap could have let a write through.
func TestBreakGlassStopsWhenTheFreezeLapses(t *testing.T) {
	t.Setenv(constants.EnvBootstrapAdmin, pinnedMailbox)
	redisServer.FlushAll()
	done := breakGlassRequest(telarkconfig.BootstrapKindRecover, telarkconfig.BootstrapStateCompleted, constants.EmptyString)
	fake := recoveryExporter(&bootstrapeps.RequestResult{Request: done, Eligible: true})
	watchFreeze(fake, func() { redisServer.Del(constants.RedisKeyPrefixFrozen + pinnedUser) })
	testutil.StubBackend(t, fake)
	code, out, logged := runBreakGlass(t, flagEmail, pinnedMailbox, flagEnroll)
	testutil.Equal(t, "exit", code, constants.ExitCodeError)
	testutil.Equal(t, "says why", strings.Contains(logged, "freeze lapsed"), true)
	testutil.Equal(t, "exporter calls", strings.Join(fake.RequestCalls(), ","),
		strings.Replace(openCall, "%s", pinnedMailbox, constants.DefaultIncrementValue)+","+completeOnce)
	testutil.Equal(t, "link printed", strings.Contains(out, "#enroll="), false)
}

// Whether a run revoked the pinned account's ways in: earlier links and open ceremonies gone, frozen at each
// completion, the last one only once the bound on a write sent before the freeze has passed, and thawed at the end.
func expectRevoked(t *testing.T, links []string, seen *completions, began time.Time, revoked bool) {
	t.Helper()
	for _, token := range links {
		if _, _, err := authhelper.ResolveEnrollToken(token); (err != nil) != revoked {
			t.Errorf("an earlier link resolves = %v, want dropped %v", err == nil, revoked)
		}
	}
	testutil.Equal(t, "open ceremonies dropped", !slices.ContainsFunc(redisServer.Keys(), func(key string) bool {
		return strings.HasPrefix(key, constants.RedisKeyPrefixChallenge+pinnedUser+constants.ColonSeparator)
	}), revoked)
	testutil.Equal(t, "frozen at each completion", slices.Equal(seen.frozen, []bool{true, true}), revoked)
	last := seen.at[len(seen.at)-constants.DefaultIncrementValue]
	testutil.Equal(t, "held", last.Sub(began) >= constants.RecoveryHold, revoked)
	testutil.Equal(t, "thawed", redisServer.Exists(constants.RedisKeyPrefixFrozen+pinnedUser), false)
}

// An eligible recovery freezes the account, drops every link and open ceremony of it, has the exporter complete it
// (which revokes its passkeys and sessions), holds the freeze until a write sent before it has landed, drops again,
// revokes once more whatever was enrolled since, thaws, and prints one 10-minute token. A replacement promotes the
// account the chart now names and revokes nothing; one that already has a passkey gets no token.
func TestBreakGlassCompletesAnEligibleRequest(t *testing.T) {
	approved := telarkconfig.BootstrapStateApproved
	completed := telarkconfig.BootstrapStateCompleted
	cases := []struct {
		name     string
		mailbox  string
		kind     string
		enrolled bool
		owner    string
		dropped  bool
		again    []string
	}{
		{"recovery", pinnedMailbox, telarkconfig.BootstrapKindRecover, false, pinnedUser, true, []string{completeLast}},
		{"replacement", nextMailbox, telarkconfig.BootstrapKindReplace, false, nextUser, false, nil},
		{"replacement onto an enrolled account", nextMailbox, telarkconfig.BootstrapKindReplace, true, constants.EmptyString, false, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(constants.EnvBootstrapAdmin, c.mailbox)
			redisServer.FlushAll()
			target := constants.EmptyString
			if c.kind == telarkconfig.BootstrapKindReplace {
				target = c.mailbox
			}
			fake := pinnedExporter()
			fake.RequestAnswers = map[string]bootstrapeps.RequestResult{
				openAnswer:  {Request: breakGlassRequest(c.kind, approved, target), Eligible: true},
				completeKey: {Request: breakGlassRequest(c.kind, completed, target), Eligible: true, Enrolled: c.enrolled},
			}
			seen := watchFreeze(fake, nil)
			testutil.StubBackend(t, fake)
			links := openWaysIn(t)

			began := time.Now()
			code, out, _ := runBreakGlass(t, flagEmail, c.mailbox, flagEnroll)
			testutil.Equal(t, "exit", code, constants.DefaultInitValue)
			calls := []string{strings.Replace(openCall, "%s", c.mailbox, constants.DefaultIncrementValue), completeOnce}
			testutil.Equal(t, "exporter calls", strings.Join(fake.RequestCalls(), ","),
				strings.Join(append(calls, c.again...), ","))
			expectRevoked(t, links, seen, began, c.dropped)
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
