package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	roledata "github.com/telark/telark/internal/data/resources/role"
	userresource "github.com/telark/telark/internal/data/resources/user"
	notificationsclient "github.com/telark/telark/internal/rest/clients/notifications"
	autheps "github.com/telark/telark/internal/rest/endpoints/auth"
	"github.com/telark/telark/internal/rest/router"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	passkeyhandler "github.com/telark/telark/services/auth/internal/handlers/passkey"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
	"github.com/telark/telark/services/auth/internal/routes"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	// The chart sets the variable and the key layout is the security design, so both are pinned.
	inviteTTLEnv      = "ENROLL_INVITE_TTL_SEC"
	inviteKeyPrefix   = "auth:passkey:invite:"
	inviteOfKeyPrefix = "auth:passkey:invite-of:"
	chartTTLSeconds   = "900"
	chartTTL          = 15 * time.Minute
	idPlaceholder     = "{id}"
	usersOwnerID      = "u-users-owner"
	plainTargetID     = "u-plain"
	enrolledTargetID  = "u-enrolled"
	deletedTargetID   = "u-deleted"
	leavingTargetID   = "u-leaving"
	adminFullname     = "Test Admin"
	usersOwnerRoleID  = "r-users-owner"
	unknownToken      = "unknown-token"
	deviceName        = "laptop"
	deviceType        = "desktop"
)

func usersOwner() xauthz.Identity {
	return xauthz.Identity{UserID: usersOwnerID, Grants: xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeUsers: roledata.PermissionLevelOwner},
	}}
}

func adminOnAll() xauthz.Identity {
	return xauthz.Identity{UserID: adminCallID, Grants: xauthz.Grants{
		Levels: map[string]roledata.PermissionLevel{roledata.ScopeAll: roledata.PermissionLevelAdmin},
	}}
}

func inviteDirectory() *testutil.FakeExporter {
	adminRole, ownerRole := constants.BuiltInRoleAdmin, usersOwnerRoleID
	active := userresource.UserStatus{Phase: string(userresource.AccountPhaseActive)}
	return &testutil.FakeExporter{
		Users: map[string]*userresource.User{
			adminCallID:      {ID: adminCallID, Fullname: adminFullname, RoleRefs: []*string{&adminRole}, Status: active},
			usersOwnerID:     {ID: usersOwnerID, RoleRefs: []*string{&ownerRole}, Status: active},
			plainTargetID:    {ID: plainTargetID, Status: active},
			enrolledTargetID: {ID: enrolledTargetID, Status: active},
			deletedTargetID:  {ID: deletedTargetID, Status: active},
			leavingTargetID:  {ID: leavingTargetID, Status: active},
		},
		Roles: map[string]*roledata.AccessRole{
			adminRole: {ID: adminRole, Status: roledata.RoleStatusActive, ScopesAndPermissions: []roledata.ScopeAndPermissions{
				{Scope: roledata.ScopeAll, Level: roledata.PermissionLevelAdmin}}},
			ownerRole: {ID: ownerRole, Status: roledata.RoleStatusActive, ScopesAndPermissions: []roledata.ScopeAndPermissions{
				{Scope: roledata.ScopeUsers, Level: roledata.PermissionLevelOwner}}},
		},
		Passkeys: map[string]int{enrolledTargetID: constants.DefaultIncrementValue},
	}
}

func initWebAuthn(t *testing.T) {
	t.Helper()
	if err := webauthnhelper.InitWebAuthn(&config.WebAuthnConfig{RPName: "Test", ChallengeTimeout: 60}); err != nil {
		t.Fatalf("InitWebAuthn: %v", err)
	}
}

// The router has no authz middleware here, so the identity it would attach is set by hand.
func enrollLinkReq(method, targetID string, caller xauthz.Identity) *http.Request {
	path := strings.Replace(router.Pattern(autheps.UserEnrollLink), idPlaceholder, targetID, constants.DefaultIncrementValue)
	r := httptest.NewRequest(method, path, nil)
	r.Header.Set(constants.HeaderUserID, caller.UserID)
	return r.WithContext(xauthz.WithIdentity(r.Context(), caller))
}

func routed(w http.ResponseWriter, r *http.Request) {
	router.NewRouter(routes.Routes).ServeHTTP(w, r)
}

func issueLink(t *testing.T, targetID string, caller xauthz.Identity) passkeyhandler.EnrollLinkResponse {
	t.Helper()
	rec := httptest.NewRecorder()
	routed(rec, enrollLinkReq(http.MethodPost, targetID, caller))
	if rec.Code != http.StatusCreated {
		t.Fatalf("issue = %d, want %d (body %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var link passkeyhandler.EnrollLinkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &link); err != nil {
		t.Fatalf("decode link: %v", err)
	}
	return link
}

func revokeLink(t *testing.T, targetID string, caller xauthz.Identity) {
	t.Helper()
	rec := httptest.NewRecorder()
	routed(rec, enrollLinkReq(http.MethodDelete, targetID, caller))
	testutil.Equal(t, "revoke", rec.Code, http.StatusOK)
}

func startWith(token string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	passkeyhandler.RegisterStart(rec, jsonReq(`{"enrollToken":"`+token+`"}`))
	return rec
}

// Redis holds only the digest of a link, both keys live as long as the link (an hour
// unless the chart says otherwise), and the user record shows who issued it until when.
func TestInviteLinkStoresOnlyTheDigest(t *testing.T) {
	cases := []struct {
		name string
		env  string
		ttl  time.Duration
	}{
		{"default lifetime", constants.EmptyString, time.Hour},
		{"chart lifetime", chartTTLSeconds, chartTTL},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(inviteTTLEnv, c.env)
			fake := inviteDirectory()
			testutil.StubBackend(t, fake)
			link := issueLink(t, plainTargetID, usersOwner())
			digest := tokenDigest(link.Token)

			owner, err := redisServer.Get(inviteKeyPrefix + digest)
			if err != nil || owner != plainTargetID {
				t.Fatalf("link key = %q (%v), want %q", owner, err, plainTargetID)
			}
			pointer, err := redisServer.Get(inviteOfKeyPrefix + plainTargetID)
			if err != nil || pointer != digest {
				t.Fatalf("per-user key = %q (%v), want the digest", pointer, err)
			}
			testutil.Equal(t, "link TTL", redisServer.TTL(inviteKeyPrefix+digest), c.ttl)
			testutil.Equal(t, "per-user TTL", redisServer.TTL(inviteOfKeyPrefix+plainTargetID), c.ttl)
			expectNoRawToken(t, link.Token)

			invite := fake.User(plainTargetID).Status.Invite
			if invite == nil || invite.IssuedBy != usersOwnerID || invite.ExpiresAt != link.ExpiresAt {
				t.Fatalf("stored invite = %+v, want issuedBy %s, expiresAt %s", invite, usersOwnerID, link.ExpiresAt)
			}
			issued, issuedErr := time.Parse(time.RFC3339, invite.IssuedAt)
			expires, expiresErr := time.Parse(time.RFC3339, invite.ExpiresAt)
			if issuedErr != nil || expiresErr != nil || expires.Sub(issued) != c.ttl {
				t.Fatalf("invite window %s..%s, want %s", invite.IssuedAt, invite.ExpiresAt, c.ttl)
			}
			testutil.Equal(t, "notices", len(fake.Notices()), constants.DefaultInitValue)
		})
	}
}

func expectNoRawToken(t *testing.T, token string) {
	t.Helper()
	for _, key := range redisServer.Keys() {
		value, _ := redisServer.Get(key)
		if strings.Contains(key, token) || strings.Contains(value, token) {
			t.Fatalf("raw token stored in Redis under %q", key)
		}
	}
}

// A link opens one registration for its user. One that is unknown, replaced by a newer
// link, used, revoked, expired or outlived by its user gets the same answer, so none can be told apart.
func TestInviteLinkOpensOneRegistration(t *testing.T) {
	initWebAuthn(t)
	fake := inviteDirectory()
	testutil.StubBackend(t, fake)
	refusal := startWith(unknownToken)
	testutil.Equal(t, "unknown link", refusal.Code, http.StatusUnauthorized)

	cases := []struct {
		name  string
		token func(t *testing.T) string
	}{
		{"replaced by a newer link", func(t *testing.T) string {
			replaced := issueLink(t, plainTargetID, usersOwner()).Token
			newest := issueLink(t, plainTargetID, usersOwner()).Token
			testutil.Equal(t, "newest link", startWith(newest).Code, http.StatusOK)
			return replaced
		}},
		{"used", func(t *testing.T) string {
			token := issueLink(t, plainTargetID, usersOwner()).Token
			testutil.Equal(t, "first use", startWith(token).Code, http.StatusOK)
			return token
		}},
		{"revoked", func(t *testing.T) string {
			token := issueLink(t, plainTargetID, usersOwner()).Token
			revokeLink(t, plainTargetID, usersOwner())
			testutil.Equal(t, "per-user key after revoke", redisServer.Exists(inviteOfKeyPrefix+plainTargetID), false)
			if status := fake.User(plainTargetID).Status; status.Invite != nil || status.InviteAcceptedAt != constants.EmptyString {
				t.Fatalf("invite still stored, or marked used, after revoke: %+v", status)
			}
			return token
		}},
		{"expired", func(t *testing.T) string {
			token := issueLink(t, plainTargetID, usersOwner()).Token
			redisServer.FastForward(time.Hour + time.Second)
			return token
		}},
		{"user deleted", func(t *testing.T) string {
			token := issueLink(t, deletedTargetID, usersOwner()).Token
			fake.RemoveUser(deletedTargetID)
			return token
		}},
		{"user being deleted", func(t *testing.T) string {
			token := issueLink(t, leavingTargetID, usersOwner()).Token
			fake.SetGone(leavingTargetID)
			return token
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := startWith(c.token(t))
			testutil.Equal(t, "status", rec.Code, refusal.Code)
			testutil.Equal(t, "answer", rec.Body.String(), refusal.Body.String())
		})
	}
}

// Revoking for a user that is being deleted, or is gone by the time the invite is cleared,
// is done: the link dies with its keys and no invite is left to clear.
func TestRevokeOutlivesTheUser(t *testing.T) {
	cases := []struct {
		name   string
		remove func(*testutil.FakeExporter, string)
		revoke func(t *testing.T)
	}{
		{"being deleted", (*testutil.FakeExporter).SetGone, func(t *testing.T) {
			revokeLink(t, plainTargetID, usersOwner())
		}},
		{"deleted before the invite is cleared", (*testutil.FakeExporter).RemoveUser, func(t *testing.T) {
			if err := authhelper.RevokeInvite(plainTargetID); err != nil {
				t.Fatalf("revoke = %v, want done", err)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := inviteDirectory()
			testutil.StubBackend(t, fake)
			token := issueLink(t, plainTargetID, usersOwner()).Token
			c.remove(fake, plainTargetID)
			c.revoke(t)
			testutil.Equal(t, "per-user key", redisServer.Exists(inviteOfKeyPrefix+plainTargetID), false)
			testutil.Equal(t, "revoked link", redisServer.Exists(inviteKeyPrefix+tokenDigest(token)), false)
		})
	}
}

// A link onto an account that already has a passkey is announced to that account,
// naming who created it, and a notice that fails never fails the link.
func TestRecoveryLinksNoticeTheAccount(t *testing.T) {
	cases := []struct {
		name        string
		target      string
		noticesDown bool
		notices     int
	}{
		{"account with a passkey", enrolledTargetID, false, constants.DefaultIncrementValue},
		{"account without one", plainTargetID, false, constants.DefaultInitValue},
		{"notice not delivered", enrolledTargetID, true, constants.DefaultInitValue},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := inviteDirectory()
			testutil.StubBackend(t, fake)
			fake.SetNoticesDown(c.noticesDown)
			issueLink(t, c.target, adminOnAll())

			notices := fake.Notices()
			testutil.Equal(t, "notices", len(notices), c.notices)
			for _, notice := range notices {
				testutil.Equal(t, "recipient", notice.UserID, c.target)
				testutil.Equal(t, "type", notice.Type, notificationsclient.TypeEnrollLinkCreated)
				testutil.Equal(t, "names the issuer", strings.Contains(notice.Message, adminFullname), true)
			}
		})
	}
}

// Enrolling through the link ends the invite: the pending marker gives way to the time it was used,
// no link stays live, and an account that already had a passkey is told one was added.
func TestEnrollmentThroughTheLinkClosesTheInvite(t *testing.T) {
	initWebAuthn(t)
	cases := []struct {
		name    string
		target  string
		notices []string
	}{
		{"first passkey", plainTargetID, nil},
		{"recovery", enrolledTargetID, []string{notificationsclient.TypeEnrollLinkCreated, notificationsclient.TypeEnrollLinkUsed}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := inviteDirectory()
			testutil.StubBackend(t, fake)
			start := startWith(issueLink(t, c.target, adminOnAll()).Token)
			testutil.Equal(t, "start", start.Code, http.StatusOK)
			var options passkeyhandler.RegisterStartResponse
			if err := json.Unmarshal(start.Body.Bytes(), &options); err != nil {
				t.Fatalf("start body: %v", err)
			}

			finish := jsonReq(registrationBody(t, options.Options.Response.Challenge.String(), webauthnHost))
			finish.Header.Set(constants.HeaderDeviceName, deviceName)
			finish.Header.Set(constants.HeaderDeviceType, deviceType)
			rec := httptest.NewRecorder()
			passkeyhandler.CreatePasskey(rec, finish)
			testutil.Equal(t, "finish", rec.Code, http.StatusCreated)

			status := fake.User(c.target).Status
			if status.Invite != nil {
				t.Fatalf("invite still pending after enrollment: %+v", status.Invite)
			}
			if _, err := time.Parse(time.RFC3339, status.InviteAcceptedAt); err != nil {
				t.Fatalf("inviteAcceptedAt = %q, want the enrollment time", status.InviteAcceptedAt)
			}
			testutil.Equal(t, "per-user key", redisServer.Exists(inviteOfKeyPrefix+c.target), false)
			notices := fake.Notices()
			types := make([]string, constants.DefaultInitValue, len(notices))
			for _, notice := range notices {
				types = append(types, notice.Type)
			}
			if !slices.Equal(types, c.notices) {
				t.Fatalf("notices = %v, want %v", types, c.notices)
			}
		})
	}
}

// Opening a link spends it, so its invite shows pending only while the ceremony it started can
// still finish, then expired. A link the account made for itself leaves a pending invite alone.
func TestOpenedLinkEndsTheInviteWithItsCeremony(t *testing.T) {
	initWebAuthn(t)
	cases := []struct {
		name     string
		open     func(t *testing.T) string
		shortens bool
	}{
		{"invite link", func(t *testing.T) string {
			return issueLink(t, enrolledTargetID, adminOnAll()).Token
		}, true},
		{"own link", func(t *testing.T) string {
			issueLink(t, enrolledTargetID, adminOnAll())
			token, _, err := authhelper.CreateEnrollToken(enrolledTargetID)
			if err != nil {
				t.Fatalf("own link: %v", err)
			}
			return token
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fake := inviteDirectory()
			testutil.StubBackend(t, fake)
			token := c.open(t)
			issued := *fake.User(enrolledTargetID).Status.Invite
			testutil.Equal(t, "open", startWith(token).Code, http.StatusOK)

			invite := fake.User(enrolledTargetID).Status.Invite
			if invite == nil || invite.IssuedBy != issued.IssuedBy || invite.IssuedAt != issued.IssuedAt {
				t.Fatalf("invite after opening = %+v, want the issued one %+v", invite, issued)
			}
			expires, err := time.Parse(time.RFC3339, invite.ExpiresAt)
			if err != nil {
				t.Fatalf("expiresAt %q: %v", invite.ExpiresAt, err)
			}
			testutil.Equal(t, "pending past the ceremony", time.Until(expires) > time.Minute, !c.shortens)
		})
	}
}
