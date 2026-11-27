package authrequest

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	webauthnlib "github.com/go-webauthn/webauthn/webauthn"
	authdata "github.com/telark/telark/internal/data/auth"
	requestdata "github.com/telark/telark/internal/data/resources/authrequest"
	userresource "github.com/telark/telark/internal/data/resources/user"
	notificationsclient "github.com/telark/telark/internal/rest/clients/notifications"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	mixedCaseAdmin = " Test@Example.com "
	linkCredential = "link-passkey"
	oldCredential  = "b2xk"
	passkeyName    = "laptop"
	passkeyKind    = "platform"
	passkeySubject = "cred"
	googleSubject  = "sub"
)

func adminRole() []*string {
	role := constants.BuiltInRoleAdmin
	return []*string{&role}
}

func markedAdmin() *userresource.User {
	return &userresource.User{ID: pinnedUser, Email: adminMailbox, Bootstrap: true, RoleRefs: adminRole()}
}

// Made long before any request, so it is never the link's.
func oldPasskey() []authdata.Passkey {
	created := longAgo
	return []authdata.Passkey{{UserID: pinnedUser, CredentialID: oldCredential, CreationTimestamp: &created}}
}

// No account is pinned. The accounts sign with the passkeys the fake stores, so one a link registers shows.
func unpinnedExporter(user *userresource.User) *testutil.FakeExporter {
	fake := &testutil.FakeExporter{
		Users:       map[string]*userresource.User{},
		Credentials: map[string][]authdata.Passkey{pinnedUser: nil, testutil.CreatedUserID: nil},
	}
	if user != nil {
		fake.Users[user.ID] = user
	}
	return fake
}

// The link's passkey, saved whenever auth's write lands.
func saveLinkPasskey(t *testing.T, userID string) {
	t.Helper()
	passkey := authhelper.CreatePasskeyFromCredential(userID, &webauthnlib.Credential{ID: []byte(linkCredential)},
		passkeyName, passkeyKind, false, false)
	if _, err := authhelper.CreatePasskey(userID, passkey); err != nil {
		t.Fatal(err)
	}
}

// The record's window, from issue to expiry, is the token's lifetime.
func expectInviteWindow(t *testing.T, invite *userresource.Invite) {
	t.Helper()
	issued, issuedErr := time.Parse(time.RFC3339, invite.IssuedAt)
	expires, expiresErr := time.Parse(time.RFC3339, invite.ExpiresAt)
	if issuedErr != nil || expiresErr != nil || expires.Sub(issued) != linkTTL {
		t.Fatalf("invite window %s..%s, want %s", invite.IssuedAt, invite.ExpiresAt, linkTTL)
	}
}

// The account the request enrolls: marked, Admin once, and bound to nothing but passkeys.
func expectBootstrapAdmin(t *testing.T, user userresource.User) {
	t.Helper()
	admins := constants.DefaultInitValue
	for _, role := range user.RoleRefs {
		if role != nil && *role == constants.BuiltInRoleAdmin {
			admins++
		}
	}
	bound := slices.ContainsFunc(user.Identities, func(identity *userresource.UserIdentity) bool {
		return identity.Provider != constants.IdentityProviderPasskey
	})
	if user.Email != adminMailbox || !user.Bootstrap || admins != constants.DefaultIncrementValue || bound {
		t.Fatalf("account = %+v, want %s marked, Admin once and bound to passkeys only", user, adminMailbox)
	}
}

// Before the pin a request acts at once: it creates or promotes the account (dropping a Google identity bound
// earlier), seals a 10-minute link like the Members dialog's, and completes once that link registers a passkey.
func TestEnrollsBeforeThePin(t *testing.T) {
	passkey := &userresource.UserIdentity{Provider: constants.IdentityProviderPasskey, Subject: passkeySubject}
	google := &userresource.UserIdentity{Provider: constants.IdentityProviderGoogle, Subject: googleSubject}
	cases := []enrollCase{
		{"fresh install, dashboard exposed", dashboard, mixedCaseAdmin, nil, testutil.CreatedUserID},
		{"admin without the marker, port-forward install", constants.EmptyString, adminMailbox,
			&userresource.User{ID: pinnedUser, Email: adminMailbox, RoleRefs: adminRole()}, pinnedUser},
		{"member bound to Google", dashboard, adminMailbox,
			&userresource.User{ID: pinnedUser, Email: adminMailbox, Identities: []*userresource.UserIdentity{google, passkey}}, pinnedUser},
		{"nothing to change", dashboard, adminMailbox, markedAdmin(), pinnedUser},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) { enrollsBeforeThePin(t, c) })
		})
	}
}

func enrollsBeforeThePin(t *testing.T, c enrollCase) {
	t.Helper()
	t.Setenv(constants.EnvRPOrigin, c.origin)
	b := newBench(t, adminMailbox, unpinnedExporter(c.user))
	b.begin(t, b.request(firstRequest, c.email))
	ready := b.await(t, firstRequest, requestdata.PhaseLinkReady)
	link, token := b.openLink(t, ready)
	origin := c.origin
	if origin == constants.EmptyString {
		origin = constants.AuthRequestDashboardURL
	}
	testutil.Equal(t, "link", link, wantLink(origin, token, adminMailbox))
	testutil.Equal(t, "link reason", ready.Reason, fmt.Sprintf(constants.AuthRequestLinkReady, adminMailbox))
	invite := b.fake.User(c.owner).Status.Invite
	if invite == nil || invite.IssuedBy != constants.EmptyString || invite.ExpiresAt != ready.LinkExpiresAt {
		t.Fatalf("stored invite = %+v, want the reported expiry %s and no issuer", invite, ready.LinkExpiresAt)
	}
	expectInviteWindow(t, invite)
	testutil.Equal(t, "lifetime", redisServer.TTL(constants.RedisKeyPrefixInviteOf+c.owner), linkTTL)

	register(t, c.owner, token)
	last := b.end(t, firstRequest)
	b.expect(t, firstRequest, ending{
		[]string{requestdata.PhaseLinkReady, requestdata.PhaseCompleted}, fmt.Sprintf(constants.AuthRequestEnrolled, adminMailbox),
	})
	testutil.Equal(t, requestLabel, last.RequestID, constants.EmptyString)
	expectBootstrapAdmin(t, b.fake.User(c.owner))
	testutil.Equal(t, callsLabel, len(b.fake.RequestCalls()), constants.DefaultInitValue)
	testutil.Equal(t, lockLabel, redisServer.Exists(constants.RedisKeyAuthRequestRun), false)
}

// The link is an invite: a later request replaces the previous token, the record names no issuing user, and an
// account promoted with a passkey hears of the token when it is created and used.
func TestALaterRequestReplacesTheLink(t *testing.T) {
	created, used := notificationsclient.TypeEnrollLinkCreated, notificationsclient.TypeEnrollLinkUsed
	cases := []replacedLinkCase{
		{"account without a passkey", true, nil, []string{firstRequest, secondRequest}, nil},
		{"account with a passkey, promoted", false, oldPasskey(), []string{firstRequest}, []string{created, used}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) { replacesTheLink(t, c) })
		})
	}
}

func replacesTheLink(t *testing.T, c replacedLinkCase) {
	t.Helper()
	fake := unpinnedExporter(&userresource.User{ID: pinnedUser, Email: adminMailbox, Bootstrap: c.marked, RoleRefs: adminRole()})
	fake.Credentials[pinnedUser] = c.passkeys
	b := newBench(t, adminMailbox, fake)
	requests := make([]requestdata.AuthRequest, constants.DefaultInitValue, len(c.requests))
	for _, id := range c.requests {
		requests = append(requests, b.request(id, adminMailbox))
	}
	b.begin(t, requests...)

	final := c.requests[len(c.requests)-constants.DefaultIncrementValue]
	tokens := make([]string, constants.DefaultInitValue, len(c.requests))
	for _, id := range c.requests {
		_, token := b.openLink(t, b.await(t, id, requestdata.PhaseLinkReady))
		tokens = append(tokens, token)
		if id != final {
			b.last(t, id)
			b.expect(t, id, ending{[]string{requestdata.PhaseLinkReady, requestdata.PhaseExpired}, constants.AuthRequestLinkUnused})
		}
	}
	for _, replaced := range tokens[:len(tokens)-constants.DefaultIncrementValue] {
		if _, _, err := authhelper.ResolveEnrollToken(replaced); err == nil {
			t.Fatal("the replaced token still opens a registration")
		}
	}
	testutil.Equal(t, tokensLabel, enrollTokenOwners(), pinnedUser)

	register(t, pinnedUser, tokens[len(tokens)-constants.DefaultIncrementValue])
	user := fake.User(pinnedUser)
	authhelper.CompleteInvite(&user, true)
	b.end(t, final)
	b.expect(t, final, ending{
		[]string{requestdata.PhaseLinkReady, requestdata.PhaseCompleted}, fmt.Sprintf(constants.AuthRequestEnrolled, adminMailbox),
	})
	testutil.Equal(t, "notices", noticeTypes(t, fake.Notices()), strings.Join(c.notices, listSep))
}

// Every notice goes to the account, and one about a new token names the auth request as its issuer.
func noticeTypes(t *testing.T, notices []notificationsclient.Notification) string {
	t.Helper()
	types := make([]string, constants.DefaultInitValue, len(notices))
	for _, notice := range notices {
		testutil.Equal(t, "recipient", notice.UserID, pinnedUser)
		issuer := string(constants.NoticeIssuerAuthRequest)
		if notice.Type == notificationsclient.TypeEnrollLinkCreated && !strings.Contains(notice.Message, issuer) {
			t.Fatalf("creation notice %q does not name %s", notice.Message, issuer)
		}
		types = append(types, notice.Type)
	}
	return strings.Join(types, listSep)
}

// Only a 404 means the account is missing: any other failed step is reported and stops the run with no link, and a
// marked account with a passkey and no pin is refused rather than acted on at once.
func TestFailsBeforeTheLink(t *testing.T) {
	gone := func(fake *testutil.FakeExporter) http.Handler {
		fake.SetGone(pinnedUser)
		return fake
	}
	refusing := func(method, part string, status int) func(fake *testutil.FakeExporter) http.Handler {
		return func(fake *testutil.FakeExporter) http.Handler { return refuse(fake, method, part, status) }
	}
	lookup := fmt.Sprintf(string(constants.ErrAuthRequestLookupFailed), adminMailbox, anyText)
	create := string(constants.ErrAuthRequestCreateFailed)
	member := &userresource.User{ID: pinnedUser, Email: adminMailbox}
	cases := []failedStepCase{
		{"the bootstrap read fails", nil, refusing(http.MethodGet, bootstrapPath, http.StatusInternalServerError),
			fmt.Sprintf(string(constants.ErrAuthRequestStatusFailed), anyText)},
		{"the lookup fails", nil, refusing(http.MethodGet, byEmailPath, http.StatusInternalServerError), lookup},
		{"the create is refused", nil, refusing(http.MethodPost, usersPath, http.StatusNotFound),
			fmt.Sprintf(create, adminMailbox, anyText)},
		{"the created account never shows", nil, refusing(http.MethodPost, usersPath, http.StatusCreated),
			fmt.Sprintf(create, adminMailbox, constants.AuthRequestNotCreated)},
		{"the passkey lookup fails", markedAdmin(), func(fake *testutil.FakeExporter) http.Handler {
			fake.FailNextPasskeyRead()
			return fake
		}, lookup},
		{"a marked account has a passkey and no pin", markedAdmin(), func(fake *testutil.FakeExporter) http.Handler {
			fake.Credentials[pinnedUser] = oldPasskey()
			return fake
		}, fmt.Sprintf(constants.AuthRequestPinMissing, adminMailbox)},
		{"the promotion is refused", member, gone,
			fmt.Sprintf(string(constants.ErrAuthRequestPatchFailed), adminMailbox, http.StatusGone, anyText)},
		{"the link's record is refused", markedAdmin(), gone, fmt.Sprintf(string(constants.ErrAuthRequestLinkFailed), anyText)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				fake := unpinnedExporter(c.user)
				b := newBench(t, adminMailbox, fake)
				b.backend = c.backend(fake)
				last := b.run(t)
				b.expect(t, firstRequest, ending{[]string{requestdata.PhaseFailed}, c.reason})
				testutil.Equal(t, "sealed link", last.SealedLink, constants.EmptyString)
				_, created := fake.Users[testutil.CreatedUserID]
				testutil.Equal(t, "account created", created, false)
				testutil.Equal(t, lockLabel, redisServer.Exists(constants.RedisKeyAuthRequestRun), false)
			})
		})
	}
}
